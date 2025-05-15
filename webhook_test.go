package main

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
)

func TestWebHookHandler_ServeHTTP(t *testing.T) {
	wg := sync.WaitGroup{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, `{"error": "not found"}`, http.StatusNotFound)
	}))
	defer server.Close()

	webhooks, err := webhookHandlers(t.Context(), "test_config.yaml", &wg)
	if err != nil {
		t.Fatal(err)
	}

	type req struct {
		method string
		path   string
		body   string
	}

	t.Run("webhook-test1", func(t *testing.T) {
		tests := []struct {
			name      string
			req       req
			respSatus int
			exptBody  string
		}{
			{
				"valid",
				req{"POST", "/webhook/test1", `{"something":"some"}`},
				200, "ok",
			},
			{
				"wrong-method",
				req{"GET", "/webhook/test1", "{}"},
				400, "",
			},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {

				event, err := http.NewRequest(tt.req.method, tt.req.path, strings.NewReader(tt.req.body))
				if err != nil {
					t.Fatal(err)
				}

				rr := httptest.NewRecorder()
				webhooks[0].ServeHTTP(rr, event)

				// Check the status code and body is what we expect.
				if status := rr.Code; status != tt.respSatus {
					t.Errorf("ServeHTTP() handler returned wrong status code: got %v want %v",
						status, tt.respSatus)
				}
				if rr.Body.String() != tt.exptBody {
					t.Errorf("ServeHTTP() handler returned unexpected body: got %v want %v",
						rr.Body.String(), tt.exptBody)
				}
			})
		}
	})

	t.Run("webhook-test2", func(t *testing.T) {
		tests := []struct {
			name      string
			req       req
			respSatus int
			exptBody  string
		}{
			{
				"wrong-method",
				req{"GET", "/test2", "{}"},
				400, "",
			},
			{
				"valid-tes2",
				req{"POST", "/test2", `{"something":"some"}`},
				204, "",
			},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {

				event, err := http.NewRequest(tt.req.method, tt.req.path, strings.NewReader(tt.req.body))
				if err != nil {
					t.Fatal(err)
				}

				rr := httptest.NewRecorder()
				webhooks[1].ServeHTTP(rr, event)

				// Check the status code and body is what we expect.
				if status := rr.Code; status != tt.respSatus {
					t.Errorf("ServeHTTP() handler returned wrong status code: got %v want %v",
						status, tt.respSatus)
				}
				if rr.Body.String() != tt.exptBody {
					t.Errorf("ServeHTTP() handler returned unexpected body: got %v want %v",
						rr.Body.String(), tt.exptBody)
				}
			})
		}
	})

}

func TestWebHookHandler_ServeHTTP_proxy(t *testing.T) {
	wg := sync.WaitGroup{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, `{"error": "not found"}`, http.StatusNotFound)
	}))
	defer server.Close()

	webhooks, err := webhookHandlers(t.Context(), "test_config.yaml", &wg)
	if err != nil {
		t.Fatal(err)
	}

	type req struct {
		method string
		path   string
		body   string
	}

	matchRequest := func(r *http.Request, want req) {
		if r.Method != want.method {
			t.Errorf("proxy() method mismatch : got %v want %v", r.URL.Path, want.path)
		}
		body, _ := io.ReadAll(r.Body)

		if string(body) != want.body {
			t.Errorf("proxy() body mismatch : got %v want %v", r.URL.Path, want.path)
		}
	}

	t.Run("webhook-test1", func(t *testing.T) {
		var tReqCount1, tReqCount2 int
		tests := []struct {
			name string
			req  req
		}{
			{
				"valid",
				req{"POST", "/webhook/test1", `{"something":"some"}`},
			},
			{
				"valid-non-json",
				req{"POST", "/webhook/test1", `not-json`},
			},
		}
		for i, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {

				target1 := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					matchRequest(r, tt.req)
					tReqCount1 += 1
				}))
				defer target1.Close()

				target2 := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					matchRequest(r, tt.req)
					tReqCount2 += 1
				}))
				defer target2.Close()

				webhooks[0].Targets = append(webhooks[0].Targets, target1.URL, target2.URL)

				event, err := http.NewRequest(tt.req.method, tt.req.path, strings.NewReader(tt.req.body))
				if err != nil {
					t.Fatal(err)
				}

				rr := httptest.NewRecorder()
				webhooks[0].ServeHTTP(rr, event)

				webhooks[0].wg.Wait()

				if tReqCount1 != i+1 {
					t.Errorf("target1 request counter mismatch : got %v want %v", tReqCount1, i+1)
				}
				if tReqCount2 != i+1 {
					t.Errorf("target1 request counter mismatch : got %v want %v", tReqCount1, i+1)
				}

			})
		}
	})
}
