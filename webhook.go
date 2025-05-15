package main

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"log/slog"
	"maps"
	"net"
	"net/http"
	"strconv"
	"sync"
	"time"

	"github.com/prometheus/client_golang/prometheus"
)

var (
	pcRequestRecv = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "webhook_requests_received_total",
			Help: "The total number of requests received",
		},
		[]string{"webhook", "status"},
	)
	pcRequestForwarded = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "webhook_requests_forwarded_total",
			Help: "The total number of requests forwarded",
		},
		[]string{"webhook", "target", "status"},
	)
	pcRequestProcessed = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "webhook_requests_processed_total",
			Help: "The total number of requests processed",
		},
		[]string{"webhook", "target", "success"},
	)
)

type WebHookHandler struct {
	*WebHook
	http *http.Client
	wg   sync.WaitGroup
	log  *slog.Logger
}

func webhookHandlers(ctx context.Context, configPath string, globalWG *sync.WaitGroup) ([]*WebHookHandler, error) {
	webhooks, err := loadConfig(configPath)
	if err != nil {
		return nil, fmt.Errorf("unable to load webhook config err:%w", err)
	}

	var handlers []*WebHookHandler
	for _, wh := range webhooks {
		globalWG.Add(1)
		h, err := webHookHandler(ctx, log, wh, globalWG)
		if err != nil {
			return nil, fmt.Errorf("unable to create webhook id:%s err:%w", wh.Path, err)
		}
		handlers = append(handlers, h)
	}
	return handlers, nil
}

func webHookHandler(ctx context.Context, log *slog.Logger, wh *WebHook, globalWG *sync.WaitGroup) (*WebHookHandler, error) {
	h := &WebHookHandler{
		WebHook: wh,
		http: &http.Client{
			Timeout: 15 * time.Second,
		},
		log: log.With("webhook", wh.Path),
	}

	// defaults
	if h.Response.Code == 0 {
		h.Response.Code = 204
	}

	// start shutdown watcher
	go func() {
		// wait for shutdown signal
		<-ctx.Done()

		// check if all proxy are forwarded
		h.wg.Wait()

		globalWG.Done()
	}()

	return h, nil
}

func (wh *WebHookHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// only process expected method
	if r.Method != wh.Method {
		wh.log.Error("invalid request received", "received", r.Method, "expected", wh.Method)
		w.WriteHeader(http.StatusBadRequest)
		pcRequestRecv.WithLabelValues(wh.Path, "400").Inc()
		return
	}

	header := maps.Clone(r.Header)
	clientIP, _, _ := net.SplitHostPort(r.RemoteAddr)

	body, err := io.ReadAll(r.Body)
	if err != nil {
		wh.log.Error("unable to process event", "err", err)
		w.WriteHeader(http.StatusBadRequest)
		pcRequestRecv.WithLabelValues(wh.Path, "400").Inc()
		return
	}

	// If handler takes longer to respond, then Sender might terminates
	// the connection and consider the delivery a failure.
	for _, t := range wh.Targets {
		wh.wg.Add(1)
		go func() {
			defer wh.wg.Done()
			ok := wh.proxy(t, header, clientIP, bytes.NewReader(body))
			pcRequestProcessed.WithLabelValues(wh.Path, t, strconv.FormatBool(ok)).Inc()
		}()
	}

	for _, h := range wh.Response.Headers {
		w.Header().Set(h.Name, h.GetValue())
	}
	w.WriteHeader(wh.Response.Code)
	w.Write([]byte(wh.Response.Body))
	pcRequestRecv.WithLabelValues(wh.Path, strconv.Itoa(wh.Response.Code)).Inc()
}

func (wh *WebHookHandler) proxy(target string, header http.Header, clientIP string, body io.Reader) bool {
	req, err := http.NewRequest(wh.Method, target, body)
	if err != nil {
		wh.log.Error("unable to create new http req", "target", target, "err", err)
		return false
	}

	maps.Copy(req.Header, header)

	if clientIP != "" {
		req.Header.Add("X-Forwarded-For", clientIP)
	}

	resp, err := wh.http.Do(req)
	if err != nil {
		wh.log.Error("unable to send new http req", "target", target, "err", err)
		return false
	}

	pcRequestForwarded.WithLabelValues(wh.Path, target, strconv.Itoa(resp.StatusCode)).Inc()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		wh.log.Error("unexpected status received from target", "target", target, "code", resp.StatusCode)
		return false
	}

	return true
}
