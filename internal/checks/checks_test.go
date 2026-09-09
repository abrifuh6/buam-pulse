package checks

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestHTTPOK(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(200) }))
	defer srv.Close()
	r := HTTP(context.Background(), srv.URL, 200, 2*time.Second)
	if !r.OK || r.StatusCode != 200 {
		t.Fatalf("expected OK 200, got %+v", r)
	}
}

func TestHTTPWrongStatus(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(503) }))
	defer srv.Close()
	r := HTTP(context.Background(), srv.URL, 200, 2*time.Second)
	if r.OK {
		t.Fatalf("expected failure on 503, got %+v", r)
	}
}
