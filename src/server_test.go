package main

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestRoutesMethodRestrictions(t *testing.T) {
	routes := routes(newLogger("ERROR"))
	for _, tc := range []struct {
		path, method, allow string
	}{
		{"/supported", http.MethodPost, http.MethodGet},
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

func TestCalculatorOverloadReturns422(t *testing.T) {
	body := `{"output":"json","dbtype":"replica_set","dimension":{"id":2},"loadtype":{"id":4},"connections":10000,"mongodbversion":{"major":7,"minor":0,"patch":0}}`
	req := httptest.NewRequest(http.MethodPost, "/calculator", strings.NewReader(body))
	res := httptest.NewRecorder()
	routes(newLogger("ERROR")).ServeHTTP(res, req)
	if res.Code != http.StatusUnprocessableEntity {
		t.Fatalf("status=%d body=%s", res.Code, res.Body.String())
	}
	if !strings.Contains(res.Body.String(), `"type": 3001`) || !strings.Contains(res.Body.String(), `"answer": {}`) {
		t.Fatalf("unexpected overload response: %s", res.Body.String())
	}
}
