package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

var (
	slogLevel = new(slog.LevelVar)
	log       *slog.Logger
)

func init() {
	slogLevel.Set(slog.LevelInfo)
	log = slog.New(slog.NewTextHandler(
		os.Stderr,
		&slog.HandlerOptions{
			Level: slogLevel,
		},
	))
}

func usage() {
	fmt.Fprintf(os.Stderr, "NAME:\n")
	fmt.Fprintf(os.Stderr, "\twebhook-fanout-proxy - webhook-fanout-proxy is a service to forward webhook events to given targets.\n")
	fmt.Fprintf(os.Stderr, "\nUsage:\n")
	fmt.Fprintf(os.Stderr, "\twebhook-fanout-proxy [global options]\n")
	fmt.Fprintf(os.Stderr, "\nGLOBAL OPTIONS:\n")
	fmt.Fprintf(os.Stderr, "\t-log-level value         (default: 'info') Log level\n")
	fmt.Fprintf(os.Stderr, "\t-config value            (default: '/etc/webhook-fanout-proxy/config.yaml') Absolute path to the config file.\n")
	fmt.Fprintf(os.Stderr, "\t-http-bind-address value (default: ':9001') The address the web server binds to.\n")

	os.Exit(2)
}

func main() {
	var globalWG sync.WaitGroup
	ctx, cancel := context.WithCancel(context.Background())

	flagLogLevel := flag.String("log-level", "info", "Log level")
	flagConfig := flag.String("config", "/etc/webhook-fanout-proxy/config.yaml", "Absolute path to the config file")
	flagHttpBind := flag.String("http-bind-address", ":9001", "The address the web server binds to")

	flag.Usage = usage
	flag.Parse()

	slogLevel.UnmarshalText([]byte(*flagLogLevel))

	reg := prometheus.NewRegistry()
	reg.MustRegister(pcRequestRecv, pcRequestForwarded)

	mux := http.NewServeMux()
	server := &http.Server{
		Addr:              *flagHttpBind,
		ReadTimeout:       5 * time.Second,
		WriteTimeout:      10 * time.Second,
		IdleTimeout:       5 * time.Second,
		ReadHeaderTimeout: 1 * time.Second,
		Handler:           mux,
	}

	webhooks, err := webhookHandlers(ctx, *flagConfig, &globalWG)
	if err != nil {
		log.Error("unable to load webhooks", "err", err)
		os.Exit(1)
	}

	// register webhook handlers
	for _, wh := range webhooks {
		log.Info("registering webhook", "path", wh.Path)
		mux.Handle(wh.Path, wh)
	}

	mux.Handle("/metrics", promhttp.Handler())

	go func() {
		log.Info("starting web server", "add", *flagHttpBind)
		if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Error("unable to start server", "err", err)
			os.Exit(1)
		}
	}()

	//listenForShutdown
	go func() {
		stop := make(chan os.Signal, 2)
		signal.Notify(stop, os.Interrupt, syscall.SIGTERM)

		<-stop

		log.Info("shutting down...")
		if err := server.Shutdown(ctx); err != nil {
			log.Error("failed to shutdown http server", "err", err)
		}

		cancel()

		<-stop
		log.Error("second signal received, terminating")
		os.Exit(1)
	}()

	// wait for all handlers to process events
	globalWG.Wait()
}
