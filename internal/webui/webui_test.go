package webui

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"testing/fstest"
)

var app = fstest.MapFS{
	"index.html":                {Data: []byte("<!doctype html><div id=root></div>")},
	"assets/index-B7Vj6WSe.js":  {Data: []byte("console.log(1)")},
	"assets/index-CMUHlmXG.css": {Data: []byte("body{}")},
	"favicon.svg":               {Data: []byte("<svg/>")},
	"manifest.webmanifest":      {Data: []byte(`{"name":"tdeemon"}`)},
	"sw.js":                     {Data: []byte("self.skipWaiting()")},
}

func get(t *testing.T, h http.Handler, method, path string, header ...string) *httptest.ResponseRecorder {
	t.Helper()

	req := httptest.NewRequestWithContext(t.Context(), method, path, nil)
	for i := 0; i+1 < len(header); i += 2 {
		req.Header.Set(header[i], header[i+1])
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

func TestHandler(t *testing.T) {
	t.Parallel()

	h, err := Handler(app)
	if err != nil {
		t.Fatalf("Handler: %v", err)
	}

	const (
		index     = "<!doctype html><div id=root></div>"
		immutable = "public, max-age=31536000, immutable"
	)
	navigate := []string{"Sec-Fetch-Mode", "navigate"}
	tests := []struct {
		name         string
		method       string
		path         string
		header       []string
		wantStatus   int
		wantType     string
		wantCache    string
		wantBody     string
		wantNoBodyOf string
	}{
		{name: "root", method: http.MethodGet, path: "/", wantStatus: 200, wantType: "text/html; charset=utf-8", wantCache: "no-store", wantBody: index},
		{name: "client route", method: http.MethodGet, path: "/day/2026-10-20", wantStatus: 200, wantType: "text/html; charset=utf-8", wantCache: "no-store", wantBody: index},
		{name: "index.html itself", method: http.MethodGet, path: "/index.html", wantStatus: 200, wantCache: "no-store", wantBody: index},
		{name: "directory", method: http.MethodGet, path: "/assets", wantStatus: 200, wantCache: "no-store", wantBody: index},
		{name: "hashed script", method: http.MethodGet, path: "/assets/index-B7Vj6WSe.js", wantStatus: 200, wantType: "text/javascript; charset=utf-8", wantCache: immutable, wantBody: "console.log(1)"},
		{name: "hashed stylesheet", method: http.MethodGet, path: "/assets/index-CMUHlmXG.css", wantStatus: 200, wantType: "text/css; charset=utf-8", wantCache: immutable},
		{name: "unhashed file", method: http.MethodGet, path: "/favicon.svg", wantStatus: 200, wantType: "image/svg+xml", wantCache: "no-cache"},
		// The installed app's manifest, as its type, which no mime.types
		// of a minimal image gives; and its service worker, revalidated,
		// so a new version is found at once.
		{name: "manifest", method: http.MethodGet, path: "/manifest.webmanifest", wantStatus: 200, wantType: "application/manifest+json", wantCache: "no-cache"},
		{name: "service worker", method: http.MethodGet, path: "/sw.js", wantStatus: 200, wantType: "text/javascript; charset=utf-8", wantCache: "no-cache"},
		{name: "script of an old build", method: http.MethodGet, path: "/assets/index-OLD.js", wantStatus: 404, wantNoBodyOf: index},
		{name: "missing file", method: http.MethodGet, path: "/robots.txt", wantStatus: 404, wantNoBodyOf: index},
		{name: "HEAD", method: http.MethodHead, path: "/", wantStatus: 200, wantCache: "no-store"},
		// A client route may end in a dot-separated token.
		{name: "navigation to a dotted route", method: http.MethodGet, path: "/invite/aaa.bbb.ccc", header: navigate, wantStatus: 200, wantCache: "no-store", wantBody: index},
		{name: "dotted route, no Sec-Fetch-Mode", method: http.MethodGet, path: "/day/1.5", header: []string{"Accept", "text/html,application/xhtml+xml"}, wantStatus: 200, wantBody: index},
		{name: "fetch of a dotted path", method: http.MethodGet, path: "/favicon.ico", header: []string{"Sec-Fetch-Mode", "no-cors"}, wantStatus: 404, wantNoBodyOf: index},
		{name: "navigation to an old asset", method: http.MethodGet, path: "/assets/index-OLD.js", header: navigate, wantStatus: 404, wantNoBodyOf: index},
		// Sec-Fetch-Mode decides when present; Accept is only the fallback.
		{name: "fetch that accepts HTML", method: http.MethodGet, path: "/x.y", header: []string{"Sec-Fetch-Mode", "cors", "Accept", "text/html"}, wantStatus: 404, wantNoBodyOf: index},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			rec := get(t, h, tt.method, tt.path, tt.header...)
			if rec.Code != tt.wantStatus {
				t.Errorf("status = %d, want %d", rec.Code, tt.wantStatus)
			}
			if tt.wantType != "" && rec.Header().Get("Content-Type") != tt.wantType {
				t.Errorf("Content-Type = %q, want %q", rec.Header().Get("Content-Type"), tt.wantType)
			}
			if tt.wantCache != "" && rec.Header().Get("Cache-Control") != tt.wantCache {
				t.Errorf("Cache-Control = %q, want %q", rec.Header().Get("Cache-Control"), tt.wantCache)
			}
			if tt.wantBody != "" && rec.Body.String() != tt.wantBody {
				t.Errorf("body = %q, want %q", rec.Body.String(), tt.wantBody)
			}
			if tt.wantNoBodyOf != "" && strings.Contains(rec.Body.String(), tt.wantNoBodyOf) {
				t.Errorf("body is the app's index.html, want a plain 404")
			}
		})
	}
}

// TestHandlerVariesOnNavigation checks the headers that keep a cache from
// serving one kind of request's answer to the other.
func TestHandlerVariesOnNavigation(t *testing.T) {
	t.Parallel()

	h, err := Handler(app)
	if err != nil {
		t.Fatalf("Handler: %v", err)
	}
	for _, rec := range []*httptest.ResponseRecorder{
		get(t, h, http.MethodGet, "/invite/a.b.c", "Sec-Fetch-Mode", "navigate"),
		get(t, h, http.MethodGet, "/invite/a.b.c", "Sec-Fetch-Mode", "no-cors"),
	} {
		if got := rec.Header().Get("Vary"); got != "Sec-Fetch-Mode, Accept" {
			t.Errorf("status %d: Vary = %q, want Sec-Fetch-Mode, Accept", rec.Code, got)
		}
		if got := rec.Header().Get("Cache-Control"); got != "no-store" {
			t.Errorf("status %d: Cache-Control = %q, want no-store", rec.Code, got)
		}
	}
	if got := get(t, h, http.MethodGet, "/assets/index-OLD.js").Header().Get("Cache-Control"); got != "no-store" {
		t.Errorf("404 for an old asset: Cache-Control = %q, want no-store", got)
	}
}

func TestHandlerNeedsIndex(t *testing.T) {
	t.Parallel()

	if _, err := Handler(fstest.MapFS{"assets/x.js": {Data: []byte("1")}}); err == nil {
		t.Fatal("Handler accepted a build without index.html")
	}
}

func TestNoApp(t *testing.T) {
	t.Parallel()

	h := http.HandlerFunc(noApp)
	if rec := get(t, h, http.MethodGet, "/"); rec.Code != 200 || !strings.Contains(rec.Body.String(), "no web app") {
		t.Errorf("GET / = %d %q, want the notice", rec.Code, rec.Body.String())
	}
	if rec := get(t, h, http.MethodGet, "/day/today"); rec.Code != 404 {
		t.Errorf("GET /day/today = %d, want 404", rec.Code)
	}
}

// An asset with precompressed copies goes as the one the client accepts,
// Brotli first, with the original's type; without one it goes as it is.
// Either way it varies by Accept-Encoding.
func TestEncodedAssets(t *testing.T) {
	t.Parallel()

	const js = "assets/index-B7Vj6WSe.js"
	fsys := fstest.MapFS{
		"index.html":                   {Data: []byte("<!doctype html>")},
		js:                             {Data: []byte("console.log(1)")},
		js + ".br":                     {Data: []byte("brotli bytes")},
		js + ".gz":                     {Data: []byte("gzip bytes")},
		"assets/index-CMUHlmXG.css":    {Data: []byte("body{}")},
		"assets/index-CMUHlmXG.css.gz": {Data: []byte("gzip css")},
		"assets/plain-D1.svg":          {Data: []byte("<svg/>")},
		"sw.js":                        {Data: []byte("self.skipWaiting()")},
		"sw.js.br":                     {Data: []byte("not served")},
		"assets/app.webmanifest":       {Data: []byte("{}")},
		"assets/app.webmanifest.gz":    {Data: []byte("gzip manifest")},
	}
	h, err := Handler(fsys)
	if err != nil {
		t.Fatalf("Handler: %v", err)
	}

	for _, tc := range []struct {
		name, path, accept         string
		wantBody, wantEnc, wantTyp string
		wantVary                   bool
	}{
		{"brotli first", "/" + js, "gzip, deflate, br, zstd", "brotli bytes", "br", "text/javascript; charset=utf-8", true},
		{"gzip", "/" + js, "gzip", "gzip bytes", "gzip", "text/javascript; charset=utf-8", true},
		{"brotli refused", "/" + js, "br;q=0, gzip;q=0.5", "gzip bytes", "gzip", "text/javascript; charset=utf-8", true},
		{"any", "/" + js, "*", "brotli bytes", "br", "text/javascript; charset=utf-8", true},
		{"any but brotli", "/" + js, "br;q=0, *", "gzip bytes", "gzip", "text/javascript; charset=utf-8", true},
		{"case and spaces", "/" + js, " GZip ; Q=1 ", "gzip bytes", "gzip", "text/javascript; charset=utf-8", true},
		{"none accepted", "/" + js, "identity", "console.log(1)", "", "text/javascript; charset=utf-8", true},
		{"no header", "/" + js, "", "console.log(1)", "", "text/javascript; charset=utf-8", true},
		{"a bad q", "/" + js, "br;q=x, gzip;q=", "console.log(1)", "", "text/javascript; charset=utf-8", true},
		{"gzip only on disk", "/assets/index-CMUHlmXG.css", "br, gzip", "gzip css", "gzip", "text/css; charset=utf-8", true},
		{"a type of our own", "/assets/app.webmanifest", "gzip", "gzip manifest", "gzip", "application/manifest+json", true},
		{"no copy", "/assets/plain-D1.svg", "br, gzip", "<svg/>", "", "image/svg+xml", false},
		{"outside assets", "/sw.js", "br", "self.skipWaiting()", "", "text/javascript; charset=utf-8", false},
	} {
		rec := get(t, h, http.MethodGet, tc.path, "Accept-Encoding", tc.accept)
		if rec.Code != http.StatusOK || rec.Body.String() != tc.wantBody {
			t.Errorf("%s: %d %q, want 200 %q", tc.name, rec.Code, rec.Body.String(), tc.wantBody)
		}
		if got := rec.Header().Get("Content-Encoding"); got != tc.wantEnc {
			t.Errorf("%s: Content-Encoding %q, want %q", tc.name, got, tc.wantEnc)
		}
		if got := rec.Header().Get("Content-Type"); got != tc.wantTyp {
			t.Errorf("%s: Content-Type %q, want %q", tc.name, got, tc.wantTyp)
		}
		if got := rec.Header().Get("Vary") == "Accept-Encoding"; got != tc.wantVary {
			t.Errorf("%s: Vary %q, want Accept-Encoding %v", tc.name, rec.Header().Get("Vary"), tc.wantVary)
		}
		if got := rec.Header().Get("Cache-Control"); strings.HasPrefix(tc.path, "/assets/") && got != "public, max-age=31536000, immutable" {
			t.Errorf("%s: Cache-Control %q, want the immutable cache", tc.name, got)
		}
	}

	// HEAD and a range work on the copy as on any file.
	if rec := get(t, h, http.MethodHead, "/"+js, "Accept-Encoding", "br"); rec.Code != http.StatusOK || rec.Header().Get("Content-Encoding") != "br" || rec.Body.Len() != 0 {
		t.Errorf("HEAD: %d %q, encoding %q", rec.Code, rec.Body.String(), rec.Header().Get("Content-Encoding"))
	}
	if rec := get(t, h, http.MethodGet, "/"+js, "Accept-Encoding", "br", "Range", "bytes=0-5"); rec.Code != http.StatusPartialContent || rec.Body.String() != "brotli" {
		t.Errorf("range: %d %q, want 206 %q", rec.Code, rec.Body.String(), "brotli")
	}
}
