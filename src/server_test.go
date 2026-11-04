package main

import (
	"bytes"
	"context"
	"io"
	"log"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func post(body string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodPost, "/calculator", strings.NewReader(body))
	res := httptest.NewRecorder()
	routes(newLogger("ERROR")).ServeHTTP(res, req)
	return res
}

func TestRoutesMethodRestrictions(t *testing.T) {
	routes := routes(newLogger("ERROR"))
	for _, tc := range []struct {
		path, method, allow string
	}{
		{"/supported", http.MethodPost, http.MethodGet},
		{"/healthz", http.MethodPost, http.MethodGet},
		{"/calculator", http.MethodGet, http.MethodPost},
	} {
		req := httptest.NewRequest(tc.method, tc.path, nil)
		res := httptest.NewRecorder()
		routes.ServeHTTP(res, req)
		if res.Code != http.StatusMethodNotAllowed || res.Header().Get("Allow") != tc.allow {
			t.Fatalf("%s %s: status=%d allow=%q", tc.method, tc.path, res.Code, res.Header().Get("Allow"))
		}
	}
}

func TestHealthz(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	res := httptest.NewRecorder()
	routes(newLogger("ERROR")).ServeHTTP(res, req)
	if res.Code != http.StatusOK || !strings.Contains(res.Body.String(), `"status":"ok"`) {
		t.Fatalf("status=%d body=%s", res.Code, res.Body.String())
	}
}

func TestCalculatorOverloadReturns422(t *testing.T) {
	res := post(`{"output":"json","dbtype":"replica_set","dimension":{"id":2},"loadtype":{"id":4},"connections":10000,"mongodbversion":{"major":7,"minor":0,"patch":0}}`)
	if res.Code != http.StatusUnprocessableEntity {
		t.Fatalf("status=%d body=%s", res.Code, res.Body.String())
	}
	if !strings.Contains(res.Body.String(), `"type": 3001`) || !strings.Contains(res.Body.String(), `"answer": {}`) {
		t.Fatalf("unexpected overload response: %s", res.Body.String())
	}
}

func TestCalculatorRejectsMalformedRequests(t *testing.T) {
	valid := `"dbtype":"replica_set","dimension":{"id":4},"loadtype":{"id":2},"connections":10,"mongodbversion":{"major":8,"minor":0,"patch":0}`
	for _, tc := range []struct {
		name, body string
		status     int
		want       string
	}{
		{"empty", "  ", http.StatusBadRequest, "empty request body"},
		{"unknown field", "{" + valid + `,"shards":3}`, http.StatusBadRequest, `unknown field \"shards\"`},
		{"trailing data", "{" + valid + "} {}", http.StatusBadRequest, "unexpected data"},
		{"float connections", `{"connections":10.5}`, http.StatusBadRequest, "malformed JSON"},
		{"unknown output", `{"output":"xml",` + valid + "}", http.StatusBadRequest, "unsupported output"},
		{"NaN memory human", `{"output":"human","dbtype":"replica_set","dimension":{"id":999,"cpu":4000,"memory":"NaNGi"},"loadtype":{"id":2},"connections":10,"mongodbversion":{"major":8,"minor":0,"patch":0}}`, http.StatusBadRequest, "invalid memory quantity"},
		{"too large", `{"output":"` + strings.Repeat("x", maxRequestBodyBytes) + `"}`, http.StatusRequestEntityTooLarge, "exceeds"},
	} {
		res := post(tc.body)
		if res.Code != tc.status || !strings.Contains(res.Body.String(), tc.want) {
			t.Errorf("%s: status=%d body=%s", tc.name, res.Code, res.Body.String())
		}
	}
}

func TestHumanOutputOmitsEmptySections(t *testing.T) {
	res := post(`{"output":"human","dbtype":"replica_set","dimension":{"id":4},"loadtype":{"id":2},"connections":10,"mongodbversion":{"major":8,"minor":0,"patch":0}}`)
	body := res.Body.String()
	if res.Code != http.StatusOK || strings.Contains(body, "[pmm-client.configuration]") || !strings.Contains(body, "\n\n[mongod.configuration]") {
		t.Fatalf("status=%d body=%s", res.Code, body)
	}
}

func TestServeDrainsInFlightRequests(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	entered, release := make(chan struct{}), make(chan struct{})
	server := &http.Server{Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		close(entered)
		<-release
		_, _ = w.Write([]byte("done"))
	})}
	ctx, cancel := context.WithCancel(context.Background())
	served := make(chan error, 1)
	go func() { served <- serve(ctx, server, listener, newLogger("ERROR"), 5*time.Second) }()

	response := make(chan string, 1)
	go func() {
		res, err := http.Get("http://" + listener.Addr().String())
		if err != nil {
			response <- "error: " + err.Error()
			return
		}
		defer res.Body.Close()
		body, _ := io.ReadAll(res.Body)
		response <- string(body)
	}()
	<-entered
	cancel()
	select {
	case err := <-served:
		t.Fatalf("serve returned before the in-flight request finished: %v", err)
	case <-time.After(200 * time.Millisecond):
	}
	close(release)
	if got := <-response; got != "done" {
		t.Fatalf("in-flight request was cut off: %q", got)
	}
	if err := <-served; err != nil {
		t.Fatalf("serve returned %v", err)
	}
}

func TestLogLevelFilters(t *testing.T) {
	var out bytes.Buffer
	l := newLogger("ERROR")
	l.out = log.New(&out, "", 0)
	l.Debugf("debug")
	l.Infof("info")
	l.Errorf("error")
	if out.String() != "ERROR: error\n" {
		t.Fatalf("ERROR level logged %q", out.String())
	}
	out.Reset()
	l = newLogger("bogus")
	l.out = log.New(&out, "", 0)
	l.Debugf("debug")
	l.Infof("info")
	if out.String() != "INFO: info\n" {
		t.Fatalf("unknown level should fall back to INFO, logged %q", out.String())
	}
}
