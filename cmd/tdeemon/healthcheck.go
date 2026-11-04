package main

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"net/netip"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/zarnautovic/tdeemon/internal/config"
)

// healthcheckTimeout bounds a health check: a server that does not answer
// /healthz in this time counts as down.
const healthcheckTimeout = 5 * time.Second

// healthcheck asks the server on this machine for /healthz and succeeds
// when it answers 200. It is the container's health check, run next to the
// server: it reads TDEEMON_LISTEN alone, so a check never fails on the rest
// of the configuration, and it never opens the database.
func healthcheck(ctx context.Context, args []string, lookup config.LookupFunc) error {
	if len(args) > 0 {
		return fmt.Errorf("%w: healthcheck takes no arguments", errUsage)
	}
	ctx, cancel := context.WithTimeout(ctx, healthcheckTimeout)
	defer cancel()
	target, err := healthzURL(ctx, lookup)
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, target, http.NoBody)
	if err != nil {
		return fmt.Errorf("health check: %w", err)
	}
	// No proxy: the server is on this machine, whatever HTTP_PROXY says
	// (Go skips it for a loopback address only, not for one TDEEMON_LISTEN
	// names on another interface).
	client := &http.Client{Transport: &http.Transport{Proxy: nil}}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("health check: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("health check: %s answered %s", target, resp.Status)
	}
	return nil
}

// healthzURL is the address of /healthz on the server TDEEMON_LISTEN
// binds: on the loopback address when it binds every address.
func healthzURL(ctx context.Context, lookup config.LookupFunc) (string, error) {
	listen := ":8080"
	if v, ok := lookup(config.Prefix + "LISTEN"); ok && strings.TrimSpace(v) != "" {
		listen = strings.TrimSpace(v)
	}
	host, port, err := net.SplitHostPort(listen)
	if err != nil {
		return "", fmt.Errorf("%sLISTEN %q: want host:port, e.g. :8080: %w", config.Prefix, listen, err)
	}
	if addr, err := netip.ParseAddr(host); host == "" || (err == nil && addr.IsUnspecified()) {
		host = "127.0.0.1"
	}
	// The server takes a service's name for a port (:http); a URL takes a
	// number only.
	n, err := net.DefaultResolver.LookupPort(ctx, "tcp", port)
	if err != nil {
		return "", fmt.Errorf("%sLISTEN %q: %w", config.Prefix, listen, err)
	}
	// Through url.URL, which escapes an IPv6 zone's % ([fe80::1%25eth0]).
	u := url.URL{Scheme: "http", Host: net.JoinHostPort(host, strconv.Itoa(n)), Path: "/healthz"}
	return u.String(), nil
}
