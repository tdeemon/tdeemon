package api_test

import (
	"archive/zip"
	"bytes"
	"encoding/csv"
	"encoding/json"
	"io"
	"net/http"
	"slices"
	"strings"
	"testing"
)

// unzipExport returns the files of an export, by name.
func unzipExport(t *testing.T, body []byte) map[string]string {
	t.Helper()
	z, err := zip.NewReader(bytes.NewReader(body), int64(len(body)))
	if err != nil {
		t.Fatalf("the export is no ZIP: %v", err)
	}
	files := map[string]string{}
	for _, f := range z.File {
		r, err := f.Open()
		if err != nil {
			t.Fatal(err)
		}
		b, err := io.ReadAll(r)
		_ = r.Close()
		if err != nil {
			t.Fatal(err)
		}
		files[f.Name] = string(b)
	}
	return files
}

// isolateExport is export-account's case: the export holds the other's
// data, and none of the owner's.
func isolateExport(t *testing.T, e isolationEnv) {
	t.Helper()
	rec := e.in.do(t, http.MethodGet, "/api/v1/account/export", "", []*http.Cookie{e.other.session}, nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("the other's export: %d %s", rec.Code, rec.Body.String())
	}
	for name, f := range unzipExport(t, rec.Body.Bytes()) {
		for _, v := range e.ownerValues {
			if strings.Contains(f, v) {
				t.Errorf("%s holds the owner's %q", name, v)
			}
		}
	}
}

// An export holds every row of the user's, in every table of user data,
// in tdeemon.json and a CSV file each; it leaves out the secrets, and it
// takes a web session.
func TestExportAccount(t *testing.T) {
	t.Parallel()

	e := newIsolationEnv(t, false)
	ivo := e.other
	rec := e.in.do(t, http.MethodGet, "/api/v1/account/export", "", []*http.Cookie{ivo.session}, nil)
	if rec.Code != http.StatusOK || rec.Header().Get("Content-Type") != "application/zip" ||
		!strings.HasPrefix(rec.Header().Get("Content-Disposition"), `attachment; filename="tdeemon-export-`) {
		t.Fatalf("export: %d %v", rec.Code, rec.Header())
	}
	files := unzipExport(t, rec.Body.Bytes())
	var doc struct {
		Format string                      `json:"format"`
		UserID string                      `json:"user_id"`
		Tables map[string][]map[string]any `json:"tables"`
	}
	if err := json.Unmarshal([]byte(files["tdeemon.json"]), &doc); err != nil {
		t.Fatalf("tdeemon.json: %v", err)
	}
	if doc.Format != "tdeemon-export/1" || doc.UserID != ivo.user.ID.String() {
		t.Errorf("format %q, user %q", doc.Format, doc.UserID)
	}
	exportHasEveryRow(t, e, files, doc.Tables)
	exportLeavesSecretsOut(t, files)
	if !strings.Contains(files["tdeemon.json"], ivo.bread.Name) || !strings.Contains(files["csv/foods.csv"], ivo.bread.Name) {
		t.Error("the export lacks ivo's bread")
	}
	if rec := e.in.do(t, http.MethodGet, "/api/v1/account/export", "", nil, bearer(ivo.token.Token)); rec.Code != http.StatusForbidden {
		t.Errorf("with an API token: %d, want 403", rec.Code)
	}
}

// exportHasEveryRow fails unless the other's export has every table of
// their rows, as many rows as the database has, in the JSON and in its
// CSV file; but the sessions and the search index.
func exportHasEveryRow(t *testing.T, e isolationEnv, files map[string]string, tables map[string][]map[string]any) {
	t.Helper()
	ivo := e.other
	rows, _ := e.rowsOf(t, ivo.user.ID)
	perTable := map[string]int{}
	for _, tb := range e.userTables(t) {
		r, _ := e.tableRows(t, tb, ivo.user.ID)
		perTable[tb.name] = len(r)
	}
	if len(rows) == 0 {
		t.Fatal("ivo has no rows")
	}
	for name, n := range perTable {
		if name == "sessions" || name == "foods_fts" {
			if _, ok := tables[name]; ok {
				t.Errorf("the export has %s", name)
			}
			continue
		}
		if got := len(tables[name]); got != n {
			t.Errorf("%s: %d rows in tdeemon.json, want %d", name, got, n)
		}
		records, err := csv.NewReader(strings.NewReader(files["csv/"+name+".csv"])).ReadAll()
		if err != nil || len(records) != n+1 {
			t.Errorf("csv/%s.csv: %d records, %v; want a header and %d rows", name, len(records), err, n)
		}
	}
}

// exportLeavesSecretsOut fails if an export has a secret's column.
func exportLeavesSecretsOut(t *testing.T, files map[string]string) {
	t.Helper()
	for table, cols := range map[string][]string{
		"users": {"password_hash"}, "api_tokens": {"token_hash"}, "push_subscriptions": {"endpoint", "p256dh", "auth"},
	} {
		header, _ := csv.NewReader(strings.NewReader(files["csv/"+table+".csv"])).Read()
		for _, c := range cols {
			if slices.Contains(header, c) || strings.Contains(files["tdeemon.json"], `"`+c+`"`) {
				t.Errorf("%s.%s is in the export", table, c)
			}
		}
	}
}
