package store

import (
	"context"
	"database/sql"
	"fmt"
	"slices"
	"strings"
	"uuid"
)

// ExportTable is one table of a user's export: its columns, and the
// user's rows of it, each value an int64, a float64, a string or nil.
type ExportTable struct {
	Name    string
	Columns []string
	Rows    [][]any
}

// exportLeavesOut are the tables an export leaves out, and the columns of
// the others: secrets, which the user never sees and which would let
// whoever reads the export in (a password's hash, a token's, a session's;
// a device's push subscription, which lets one notify it), and a table
// that only mirrors another (the foods' search index).
var exportLeavesOut = map[string][]string{
	"sessions":           nil, // the whole table
	"foods_fts":          nil,
	"users":              {"password_hash"},
	"api_tokens":         {"token_hash"},
	"invites":            {"token_hash"},
	"push_subscriptions": {"endpoint", "p256dh", "auth"},
}

// ExportUser returns all the data of the user with id, a table at a time,
// in one read so the tables agree: the users row and the rows of every
// table with a user_id column, which is every table of a user's data
// (migrations/README.md), but for what exportLeavesOut leaves out. A
// table the user has no rows in is there, empty.
func (db *DB) ExportUser(ctx context.Context, id uuid.UUID) ([]ExportTable, error) {
	var out []ExportTable
	err := db.readTx(ctx, func(tx *sql.Tx) error {
		names, err := userTableNames(ctx, tx)
		if err != nil {
			return err
		}
		for _, name := range names {
			t, err := exportTable(ctx, tx, name, id)
			if err != nil {
				return err
			}
			out = append(out, t)
		}
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("export user: %w", err)
	}
	return out, nil
}

// userTableNames lists users and the tables with a user_id column, by
// name, but the tables exportLeavesOut leaves out whole.
func userTableNames(ctx context.Context, tx *sql.Tx) ([]string, error) {
	rows, err := tx.QueryContext(ctx, `
		SELECT m.name FROM sqlite_master AS m
		WHERE m.type = 'table' AND (m.name = 'users'
			OR EXISTS (SELECT 1 FROM pragma_table_info(m.name) WHERE name = 'user_id'))
		ORDER BY m.name`)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	var names []string
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			return nil, err
		}
		if cols, out := exportLeavesOut[name]; out && cols == nil {
			continue
		}
		names = append(names, name)
	}
	return names, rows.Err()
}

// exportTable reads the user's rows of the table name, in the order they
// were written.
func exportTable(ctx context.Context, tx *sql.Tx, name string, id uuid.UUID) (ExportTable, error) {
	t := ExportTable{Name: name, Rows: [][]any{}}
	cols, err := tableColumns(ctx, tx, name)
	if err != nil {
		return t, fmt.Errorf("%s: %w", name, err)
	}
	quoted := make([]string, 0, len(cols))
	for _, c := range cols {
		if !slices.Contains(exportLeavesOut[name], c) {
			t.Columns = append(t.Columns, c)
			quoted = append(quoted, quoteIdent(c))
		}
	}
	key := "user_id"
	if name == "users" {
		key = "id"
	}
	q := "SELECT " + strings.Join(quoted, ", ") + " FROM " + quoteIdent(name) + " WHERE " + key + " = ? ORDER BY rowid" //nolint:gosec // G202: names from sqlite_master, quoted as identifiers
	rows, err := tx.QueryContext(ctx, q, id.String())
	if err != nil {
		return t, fmt.Errorf("%s: %w", name, err)
	}
	defer func() { _ = rows.Close() }()
	for rows.Next() {
		vals := make([]any, len(t.Columns))
		ptrs := make([]any, len(vals))
		for i := range vals {
			ptrs[i] = &vals[i]
		}
		if err := rows.Scan(ptrs...); err != nil {
			return t, fmt.Errorf("%s: %w", name, err)
		}
		for i, v := range vals {
			if b, ok := v.([]byte); ok {
				vals[i] = string(b)
			}
		}
		t.Rows = append(t.Rows, vals)
	}
	return t, rows.Err()
}

// tableColumns returns the columns of the table name, in their order.
func tableColumns(ctx context.Context, tx *sql.Tx, name string) ([]string, error) {
	rows, err := tx.QueryContext(ctx, "SELECT name FROM pragma_table_info(?) ORDER BY cid", name)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	var cols []string
	for rows.Next() {
		var c string
		if err := rows.Scan(&c); err != nil {
			return nil, err
		}
		cols = append(cols, c)
	}
	return cols, rows.Err()
}

// quoteIdent quotes name as an SQL identifier.
func quoteIdent(name string) string {
	return `"` + strings.ReplaceAll(name, `"`, `""`) + `"`
}
