//go:build webui

package webui

import (
	"bytes"
	"compress/gzip"
	"io"
	"io/fs"
	"net/http"
	"path"
	"regexp"
	"strings"
	"testing"
)

var (
	// tags finds the script and link elements of a built index.html.
	tags = regexp.MustCompile(`<(?:script|link)\b[^>]*>`)
	// attr reads one attribute of such an element.
	attr = func(tag, name string) string {
		m := regexp.MustCompile(`\b` + name + `="([^"]*)"`).FindStringSubmatch(tag)
		if m == nil {
			return ""
		}
		return m[1]
	}
)

// TestEmbeddedApp runs only with the webui tag, after the web app has been
// built: the real build must be in the binary and served, every file its
// index.html references must be there, and the scripts and stylesheets,
// whose names carry a hash, must get the long cache. That catches a Vite
// assetsDir that no longer matches this package's. Other references
// (an icon from web/public) are revalidated by design.
func TestEmbeddedApp(t *testing.T) {
	t.Parallel()

	h, embedded, err := New()
	if err != nil || !embedded {
		t.Fatalf("New() embedded = %v, err = %v; want the built app", embedded, err)
	}
	rec := get(t, h, http.MethodGet, "/")
	if rec.Code != 200 || !strings.Contains(rec.Body.String(), `<div id="root">`) {
		t.Fatalf("GET / = %d, body %q; want the app's index.html", rec.Code, rec.Body.String())
	}

	hashed := 0
	for _, tag := range tags.FindAllString(rec.Body.String(), -1) {
		ref := attr(tag, "src")
		if ref == "" {
			ref = attr(tag, "href")
		}
		if !strings.HasPrefix(ref, "/") {
			continue
		}
		rel := attr(tag, "rel")
		isHashed := strings.HasPrefix(tag, "<script") || rel == "stylesheet" || rel == "modulepreload"

		asset := get(t, h, http.MethodGet, ref)
		if asset.Code != 200 {
			t.Errorf("GET %s = %d, want 200", ref, asset.Code)
		}
		if isHashed {
			hashed++
			if got := asset.Header().Get("Cache-Control"); got != "public, max-age=31536000, immutable" {
				t.Errorf("GET %s: Cache-Control %q, want the immutable cache", ref, got)
			}
		}
	}
	if hashed == 0 {
		t.Fatal("index.html references no script or stylesheet")
	}
}

// TestEmbeddedCopies runs on the real build too: each script and stylesheet
// under assets/ of a KiB or more has its Brotli and gzip copies
// (web/build/precompress.ts; a tiny one may compress to no less),
// the gzip one the original's bytes, and a client that takes them gets
// them with the original's type.
func TestEmbeddedCopies(t *testing.T) {
	t.Parallel()

	fsys, err := embeddedFS()
	if err != nil {
		t.Fatal(err)
	}
	h, err := Handler(fsys)
	if err != nil {
		t.Fatal(err)
	}
	names, err := fs.Glob(fsys, assetsDir+"*")
	if err != nil {
		t.Fatal(err)
	}
	checked := 0
	for _, name := range names {
		if ext := path.Ext(name); ext != ".js" && ext != ".css" {
			continue
		}
		original, _ := fs.ReadFile(fsys, name)
		for _, enc := range encodings {
			copied, err := fs.ReadFile(fsys, name+enc.ext)
			if err != nil {
				if len(original) >= 1024 {
					t.Errorf("%s: no %s copy: %v", name, enc.coding, err)
				}
				continue
			}
			checked++
			checkCopy(t, h, name, enc.coding, original, copied)
		}
	}
	if checked == 0 {
		t.Fatal("no copy of a script or stylesheet under assets/")
	}
}

// checkCopy checks that a client taking coding gets copied, with name's
// type, and for gzip that copied holds original.
func checkCopy(t *testing.T, h http.Handler, name, coding string, original, copied []byte) {
	t.Helper()

	rec := get(t, h, http.MethodGet, "/"+name, "Accept-Encoding", coding)
	if rec.Header().Get("Content-Encoding") != coding || !bytes.Equal(rec.Body.Bytes(), copied) {
		t.Errorf("GET %s with %s: Content-Encoding %q, %d bytes; want the copy's %d", name, coding, rec.Header().Get("Content-Encoding"), rec.Body.Len(), len(copied))
	}
	want := map[string]string{".js": "text/javascript", ".css": "text/css"}[path.Ext(name)]
	if !strings.HasPrefix(rec.Header().Get("Content-Type"), want) {
		t.Errorf("GET %s with %s: Content-Type %q, want %s", name, coding, rec.Header().Get("Content-Type"), want)
	}
	if coding != "gzip" {
		return
	}
	zr, err := gzip.NewReader(bytes.NewReader(copied))
	if err != nil {
		t.Errorf("%s.gz: %v", name, err)
		return
	}
	if plain, err := io.ReadAll(zr); err != nil || !bytes.Equal(plain, original) {
		t.Errorf("%s.gz is not the original's bytes (err %v)", name, err)
	}
}
