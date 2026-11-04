// Package webui serves the web app from the binary. Vite builds web/ into
// this package's dist/ directory, and the build tag webui compiles it in:
//
//	(cd web && npm ci && npm run build)
//	go build -tags webui ./cmd/tdeemon
//
// Without the tag the server works the same but serves a short notice at /
// instead of the app, so Go builds and tests never need Node.
package webui

import (
	"bytes"
	"fmt"
	"io"
	"io/fs"
	"mime"
	"net/http"
	"path"
	"strconv"
	"strings"
	"time"
)

// assetsDir holds Vite's output files, whose names carry a content hash
// (build.assetsDir in web/vite.config.ts).
const assetsDir = "assets/"

// contentTypes are the types of the web app's files whose extension the
// standard library does not know and a minimal image has no mime.types
// for: the installed app's manifest.
var contentTypes = map[string]string{".webmanifest": "application/manifest+json"}

// encodings are the precompressed copies of an asset the build writes next
// to it (web/build/precompress.ts), the smallest first.
var encodings = []struct{ coding, ext string }{{"br", ".br"}, {"gzip", ".gz"}}

// New returns the handler for the paths the API does not own, and whether
// the web app is compiled in.
func New() (http.Handler, bool, error) {
	fsys, err := embeddedFS()
	if err != nil {
		return nil, false, err
	}
	if fsys == nil {
		return http.HandlerFunc(noApp), false, nil
	}
	h, err := Handler(fsys)
	if err != nil {
		return nil, false, err
	}
	return h, true, nil
}

// Handler serves the built web app in fsys:
//   - files under assets/ with a year-long immutable cache, because a new
//     build gives every changed file a new name, and of those with a
//     precompressed copy, the one the client accepts (Brotli, else gzip),
//     with its Content-Encoding and the original's Content-Type;
//   - other files (the icons, the manifest and the service worker of the
//     installed app) with revalidation on every use, so a new service
//     worker is found at once;
//   - index.html for every other path, so the app's client-side router can
//     own it, never cached, so a deploy takes effect on the next page load
//     of a browser without the app's service worker; with it, the worker
//     answers a navigation from its precache, and its new version waits
//     for the user's Reload;
//   - 404 instead for a missing file under assets/, so a tab still open
//     from the previous build gets an error for its old script, not HTML,
//     and for a missing path with an extension that a page asks for (a
//     favicon, robots.txt). A navigation to such a path still gets the app,
//     so a client route may end in a dot-separated token (/invite/a.b.c).
func Handler(fsys fs.FS) (http.Handler, error) {
	index, err := fs.ReadFile(fsys, "index.html")
	if err != nil {
		return nil, fmt.Errorf("web app: %w", err)
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// The mux has already cleaned the path; this only drops the slash.
		name := strings.TrimPrefix(path.Clean("/"+r.URL.Path), "/")
		if name != "" && name != "index.html" {
			if info, err := fs.Stat(fsys, name); err == nil && !info.IsDir() {
				w.Header().Set("Cache-Control", cacheControl(name))
				if strings.HasPrefix(name, assetsDir) && serveEncoded(w, r, fsys, name) {
					return
				}
				if ct, ok := contentTypes[path.Ext(name)]; ok {
					w.Header().Set("Content-Type", ct)
				}
				http.ServeFileFS(w, r, fsys, name) //nolint:gosec // G703: path.Clean above, and an fs.FS rejects names with ..
				return
			}
			if path.Ext(name) != "" {
				// The answer below depends on these headers; caches must
				// not hand a 404 for a fetch to a later navigation.
				w.Header().Add("Vary", "Sec-Fetch-Mode, Accept")
			}
			if strings.HasPrefix(name, assetsDir) || (path.Ext(name) != "" && !isNavigation(r)) {
				w.Header().Set("Cache-Control", "no-store")
				http.NotFound(w, r)
				return
			}
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Header().Set("Cache-Control", "no-store")
		w.Header().Set("Content-Length", strconv.Itoa(len(index)))
		_, _ = w.Write(index)
	}), nil
}

// serveEncoded answers with the precompressed copy of name that r accepts,
// if there is one, and reports whether it did. An asset with a copy varies
// by Accept-Encoding, whichever answer it gets.
func serveEncoded(w http.ResponseWriter, r *http.Request, fsys fs.FS, name string) bool {
	// One list, however many fields it came in (RFC 9110, 5.3).
	accept := strings.Join(r.Header.Values("Accept-Encoding"), ",")
	varies := false
	for _, enc := range encodings {
		if _, err := fs.Stat(fsys, name+enc.ext); err != nil {
			continue
		}
		if !varies {
			w.Header().Add("Vary", "Accept-Encoding")
			varies = true
		}
		if !accepts(accept, enc.coding) {
			continue
		}
		data, err := fs.ReadFile(fsys, name+enc.ext)
		if err != nil {
			continue
		}
		ctype, ok := contentTypes[path.Ext(name)]
		if !ok {
			ctype = mime.TypeByExtension(path.Ext(name))
		}
		if ctype == "" {
			ctype = "application/octet-stream"
		}
		w.Header().Set("Content-Type", ctype)
		w.Header().Set("Content-Encoding", enc.coding)
		http.ServeContent(w, r, name, time.Time{}, bytes.NewReader(data))
		return true
	}
	return false
}

// accepts reports whether an Accept-Encoding header takes coding: named
// with a q above 0, or not named and * is (RFC 9110, 12.5.3).
func accepts(header, coding string) bool {
	star := false
	for part := range strings.SplitSeq(header, ",") {
		token, params, _ := strings.Cut(part, ";")
		token = strings.TrimSpace(token)
		ok := qualityAboveZero(params)
		switch {
		case strings.EqualFold(token, coding):
			return ok
		case token == "*":
			star = ok
		}
	}
	return star
}

// qualityAboveZero reads the q of an Accept-Encoding entry's parameters:
// 1 when there is none, and 0 for one that does not parse.
func qualityAboveZero(params string) bool {
	for param := range strings.SplitSeq(params, ";") {
		key, value, _ := strings.Cut(strings.TrimSpace(param), "=")
		if !strings.EqualFold(strings.TrimSpace(key), "q") {
			continue
		}
		q, err := strconv.ParseFloat(strings.TrimSpace(value), 64)
		return err == nil && q > 0
	}
	return true
}

// isNavigation reports whether r loads a page, as opposed to a script,
// image or fetch: browsers say so in Sec-Fetch-Mode; for clients that do
// not send it, a request that accepts HTML counts.
func isNavigation(r *http.Request) bool {
	if mode := r.Header.Get("Sec-Fetch-Mode"); mode != "" {
		return mode == "navigate"
	}
	return strings.Contains(r.Header.Get("Accept"), "text/html")
}

func cacheControl(name string) string {
	if strings.HasPrefix(name, assetsDir) {
		return "public, max-age=31536000, immutable"
	}
	return "no-cache"
}

// noApp answers in a build without the web app.
func noApp(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	_, _ = io.WriteString(w, "tdeemon is running, but this build has no web app.\n"+
		"Build it with `npm ci && npm run build` in web/, then `go build -tags webui ./cmd/tdeemon`.\n")
}
