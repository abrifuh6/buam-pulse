package checks

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

// Test servers bind to 127.0.0.1, which the SSRF guard blocks by design.
// Disabling it here is the point of the flag — the guard itself is covered by
// validate_test.go.
func opts(expected int) Options {
	return Options{
		ExpectedStatus:      expected,
		Timeout:             2 * time.Second,
		AllowPrivateTargets: true,
	}
}

func TestHTTPOK(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(200)
	}))
	defer srv.Close()

	r := HTTP(context.Background(), srv.URL, opts(200))
	if !r.OK || r.StatusCode != 200 {
		t.Fatalf("expected OK 200, got %+v", r)
	}
	if r.Kind != FailNone {
		t.Errorf("expected no failure kind, got %q", r.Kind)
	}
}

func TestHTTPWrongStatus(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(503)
	}))
	defer srv.Close()

	r := HTTP(context.Background(), srv.URL, opts(200))
	if r.OK {
		t.Fatalf("expected failure on 503, got %+v", r)
	}
	if r.Kind != FailStatus {
		t.Errorf("expected kind %q, got %q", FailStatus, r.Kind)
	}
}

// The case status codes miss entirely: HTTP 200 with an error rendered in the
// body. This is the whole reason keyword checks exist.
func TestKeywordMustBePresent(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte("<html><body>Welcome to the shop</body></html>"))
	}))
	defer srv.Close()

	o := opts(200)
	o.Keyword = "Welcome"
	o.KeywordPresent = true
	if r := HTTP(context.Background(), srv.URL, o); !r.OK {
		t.Fatalf("expected pass when keyword present, got %+v", r)
	}

	o.Keyword = "Checkout"
	r := HTTP(context.Background(), srv.URL, o)
	if r.OK {
		t.Fatal("expected failure when required keyword is missing")
	}
	if r.Kind != FailKeyword {
		t.Errorf("expected kind %q, got %q", FailKeyword, r.Kind)
	}
}

func TestKeywordMustBeAbsent(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte("Fatal error: database connection refused"))
	}))
	defer srv.Close()

	o := opts(200)
	o.Keyword = "Fatal error"
	o.KeywordPresent = false

	r := HTTP(context.Background(), srv.URL, o)
	if r.OK {
		t.Fatal("expected failure when forbidden keyword is present")
	}
	if r.Kind != FailKeyword {
		t.Errorf("expected kind %q, got %q", FailKeyword, r.Kind)
	}
}

func TestKeywordIsCaseInsensitive(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte("WELCOME BACK"))
	}))
	defer srv.Close()

	o := opts(200)
	o.Keyword = "welcome"
	o.KeywordPresent = true
	if r := HTTP(context.Background(), srv.URL, o); !r.OK {
		t.Fatalf("expected case-insensitive match, got %+v", r)
	}
}

func TestTimeoutIsDistinctFromConnectFailure(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		time.Sleep(300 * time.Millisecond)
		w.WriteHeader(200)
	}))
	defer srv.Close()

	o := opts(200)
	o.Timeout = 50 * time.Millisecond

	r := HTTP(context.Background(), srv.URL, o)
	if r.OK {
		t.Fatal("expected timeout failure")
	}
	if r.Kind != FailTimeout {
		t.Errorf("expected kind %q, got %q", FailTimeout, r.Kind)
	}
}

// TLS details come back on an https target; the test server's self-signed cert
// is rejected by default, which is itself the correct behaviour for monitoring.
func TestHTTPSWithUntrustedCertFails(t *testing.T) {
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(200)
	}))
	defer srv.Close()

	o := opts(200)
	o.CheckSSL = true

	r := HTTP(context.Background(), srv.URL, o)
	if r.OK {
		t.Fatal("expected failure on untrusted certificate")
	}
	if r.Kind != FailConnect {
		t.Errorf("expected kind %q, got %q", FailConnect, r.Kind)
	}
}
