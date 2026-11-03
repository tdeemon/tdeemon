package api

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/danielgtaylor/huma/v2"

	"github.com/zarnautovic/tdeemon/internal/store"
)

// exportFormat names the shape of tdeemon.json in an export: its tables
// and their columns as the database has them, so a later version's
// export may differ.
const exportFormat = "tdeemon-export/1"

type exportOutput struct {
	ContentType        string `header:"Content-Type" doc:"application/zip"`
	ContentDisposition string `header:"Content-Disposition" doc:"An attachment, tdeemon-export-<date>.zip."`
	CacheControl       string `header:"Cache-Control" doc:"no-store"`
	Body               []byte
}

// exportJSON is tdeemon.json in an export.
type exportJSON struct {
	Format     string                      `json:"format"`
	ExportedAt time.Time                   `json:"exported_at"`
	UserID     string                      `json:"user_id"`
	Tables     map[string][]map[string]any `json:"tables"`
}

// exportAccount answers all the signed-in user's data, as a ZIP of
// tdeemon.json and a CSV file of each table.
func (s *sessions) exportAccount(ctx context.Context, _ *struct{}) (*exportOutput, error) {
	p, err := requireSession(ctx)
	if err != nil {
		return nil, err
	}
	tables, err := s.db.ExportUser(ctx, p.User.ID)
	if err != nil {
		return nil, internalError("exporting the data failed", err)
	}
	now := s.now().UTC()
	body, err := exportZip(tables, p.User.ID.String(), now)
	if err != nil {
		return nil, internalError("exporting the data failed", err)
	}
	s.logger.InfoContext(ctx, "exported an account", "user_id", p.User.ID.String())
	return &exportOutput{
		ContentType:        "application/zip",
		ContentDisposition: `attachment; filename="tdeemon-export-` + now.Format(time.DateOnly) + `.zip"`,
		CacheControl:       "no-store",
		Body:               body,
	}, nil
}

// exportZip writes tables to a ZIP: tdeemon.json, every table in one
// document, and csv/<table>.csv, a table each, its columns as the header.
func exportZip(tables []store.ExportTable, userID string, now time.Time) ([]byte, error) {
	doc := exportJSON{Format: exportFormat, ExportedAt: now, UserID: userID, Tables: map[string][]map[string]any{}}
	for _, t := range tables {
		rows := make([]map[string]any, len(t.Rows))
		for i, r := range t.Rows {
			rows[i] = make(map[string]any, len(t.Columns))
			for j, c := range t.Columns {
				rows[i][c] = r[j]
			}
		}
		doc.Tables[t.Name] = rows
	}
	var buf bytes.Buffer
	z := zip.NewWriter(&buf)
	write := func(name string, fill func(w *bytes.Buffer) error) error {
		var b bytes.Buffer
		if err := fill(&b); err != nil {
			return fmt.Errorf("%s: %w", name, err)
		}
		w, err := z.CreateHeader(&zip.FileHeader{Name: name, Method: zip.Deflate, Modified: now})
		if err != nil {
			return fmt.Errorf("%s: %w", name, err)
		}
		_, err = w.Write(b.Bytes())
		return err
	}
	err := write("tdeemon.json", func(w *bytes.Buffer) error {
		enc := json.NewEncoder(w)
		enc.SetIndent("", "  ")
		return enc.Encode(doc)
	})
	if err != nil {
		return nil, err
	}
	for _, t := range tables {
		err := write("csv/"+t.Name+".csv", func(w *bytes.Buffer) error {
			c := csv.NewWriter(w)
			if err := c.Write(t.Columns); err != nil {
				return err
			}
			for _, r := range t.Rows {
				rec := make([]string, len(r))
				for i, v := range r {
					rec[i] = csvValue(v)
				}
				if err := c.Write(rec); err != nil {
					return err
				}
			}
			c.Flush()
			return c.Error()
		})
		if err != nil {
			return nil, err
		}
	}
	if err := z.Close(); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// csvValue is a value of a table's in a CSV file: NULL empty, a number
// as Go writes it shortest, and text that a spreadsheet would take for a
// formula (a food's name from Open Food Facts, say "=HYPERLINK(…)")
// after a quote, as OWASP advises; tdeemon.json keeps it as it is.
func csvValue(v any) string {
	switch v := v.(type) {
	case nil:
		return ""
	case int64:
		return strconv.FormatInt(v, 10)
	case float64:
		return strconv.FormatFloat(v, 'g', -1, 64)
	case string:
		if v != "" && strings.ContainsRune("=+-@\t\r", rune(v[0])) {
			return "'" + v
		}
		return v
	}
	return fmt.Sprint(v)
}

func (s *sessions) registerExport(a huma.API) {
	huma.Register(a, huma.Operation{
		OperationID: "export-account",
		Method:      http.MethodGet,
		Path:        Prefix + "/account/export",
		Summary:     "Export your data",
		Description: "Needs a web session. All your data, as a ZIP: `tdeemon.json`, every table of your data with your " +
			"rows (format `" + exportFormat + "`: the tables and columns as the database has them), your household's row and " +
			"the invites into a household made to you among them, and `csv/<table>.csv` " +
			"of each. Left out are secrets: hashes of the password, the API tokens and the sessions, and the devices' " +
			"push subscriptions.",
		Tags: []string{"settings"},
		Responses: map[string]*huma.Response{
			"200": {
				Description: "The export.",
				Content:     map[string]*huma.MediaType{"application/zip": {Schema: &huma.Schema{Type: "string", Format: "binary"}}},
			},
		},
		Errors: []int{http.StatusUnauthorized, http.StatusForbidden},
	}, s.exportAccount)
}
