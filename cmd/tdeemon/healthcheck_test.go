package main

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// The health check passes against the server run starts, at the address
// TDEEMON_LISTEN names, and fails once it has stopped.
func TestHealthcheckAgainstTheServer(t *testing.T) {
	t.Parallel()

	env := map[string]string{
		"TDEEMON_LISTEN":     "127.0.0.1:0",
		"TDEEMON_BASE_URL":   "http://localhost",
		"TDEEMON_SECRET_KEY": strings.Repeat("s", 32),
		"TDEEMON_DATA_DIR":   filepath.Join(t.TempDir(), "data"),
	}
	ctx, cancel := context.WithCancel(t.Context())
	var logs syncBuffer
	done := make(chan error, 1)
	go func() { done <- run(ctx, nil, lookup(env), noInput(), io.Discard, &logs) }()
	waitFor(t, &logs, `"msg":"listening"`)
	addr := regexp.MustCompile(`"msg":"listening","addr":"([^"]+)"`).FindStringSubmatch(logs.String())
	if addr == nil {
		cancel()
		if err := <-done; err != nil {
			t.Errorf("run returned %v, want nil", err)
		}
		t.Fatalf("no address in the logs:\n%s", logs.String())
	}

	// Only the address: the check reads nothing else of the configuration.
	check := lookup(map[string]string{"TDEEMON_LISTEN": addr[1]})
	if err := run(t.Context(), []string{"healthcheck"}, check, noInput(), io.Discard, io.Discard); err != nil {
		t.Errorf("healthcheck of a running server = %v, want nil", err)
	}

	cancel()
	if err := <-done; err != nil {
		t.Fatalf("run returned %v, want nil", err)
	}
	err := run(t.Context(), []string{"healthcheck"}, check, noInput(), io.Discard, io.Discard)
	if err == nil || errors.Is(err, errUsage) {
		t.Errorf("healthcheck of a stopped server = %v, want a failure", err)
	}
	var out strings.Builder
	if code := exitCode(err, &out); code != 1 {
		t.Errorf("exit code %d, want 1 (printed %q)", code, out.String())
	}
}

// A server that answers /healthz with anything but 200 is not healthy.
func TestHealthcheckWantsOK(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		status int
		ok     bool
	}{
		{http.StatusOK, true},
		{http.StatusServiceUnavailable, false},
		{http.StatusNotFound, false},
	} {
		var path string
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			path = r.URL.Path
			w.WriteHeader(tc.status)
		}))
		err := healthcheck(t.Context(), nil, lookup(map[string]string{"TDEEMON_LISTEN": srv.Listener.Addr().String()}))
		srv.Close()
		if (err == nil) != tc.ok {
			t.Errorf("status %d: healthcheck = %v, want ok %v", tc.status, err, tc.ok)
		}
		if path != "/healthz" {
			t.Errorf("status %d: asked for %q, want /healthz", tc.status, path)
		}
	}
}

// A server that binds every address is asked on the loopback one, and the
// default is the server's, :8080.
func TestHealthzURL(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		listen string
		set    bool
		want   string
	}{
		{"", false, "http://127.0.0.1:8080/healthz"},
		{"  ", true, "http://127.0.0.1:8080/healthz"},
		{":9000", true, "http://127.0.0.1:9000/healthz"},
		{" :9000 ", true, "http://127.0.0.1:9000/healthz"},
		{"0.0.0.0:9000", true, "http://127.0.0.1:9000/healthz"},
		{"[::]:9000", true, "http://127.0.0.1:9000/healthz"},
		{"127.0.0.2:9000", true, "http://127.0.0.2:9000/healthz"},
		{"[::1]:9000", true, "http://[::1]:9000/healthz"},
		{"localhost:9000", true, "http://localhost:9000/healthz"},
		{":http", true, "http://127.0.0.1:80/healthz"},
		{"[fe80::1%eth0]:9000", true, "http://[fe80::1%25eth0]:9000/healthz"},
	} {
		env := map[string]string{}
		if tc.set {
			env["TDEEMON_LISTEN"] = tc.listen
		}
		got, err := healthzURL(t.Context(), lookup(env))
		if err != nil || got != tc.want {
			t.Errorf("TDEEMON_LISTEN %q: %q, %v; want %q", tc.listen, got, err, tc.want)
		}
	}
	for _, bad := range []string{"8080", ":no-such-service"} {
		if _, err := healthzURL(t.Context(), lookup(map[string]string{"TDEEMON_LISTEN": bad})); err == nil {
			t.Errorf("TDEEMON_LISTEN %q was accepted", bad)
		}
	}
}

// The check never creates the data directory nor opens the database, even
// with the whole configuration in its environment.
func TestHealthcheckLeavesTheDataAlone(t *testing.T) {
	t.Parallel()

	dataDir := filepath.Join(t.TempDir(), "data")
	srv := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))
	t.Cleanup(srv.Close)
	env := lookup(map[string]string{
		"TDEEMON_LISTEN":     srv.Listener.Addr().String(),
		"TDEEMON_BASE_URL":   "http://localhost",
		"TDEEMON_SECRET_KEY": strings.Repeat("s", 32),
		"TDEEMON_DATA_DIR":   dataDir,
	})
	if err := run(t.Context(), []string{"healthcheck"}, env, noInput(), io.Discard, io.Discard); err != nil {
		t.Fatalf("healthcheck = %v", err)
	}
	if _, err := os.Stat(dataDir); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("the data directory exists after a health check (stat: %v)", err)
	}
}
