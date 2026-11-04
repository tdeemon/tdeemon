package main

import (
	"bytes"
	"errors"
	"io"
	"strings"
	"testing"

	"github.com/tdeemon/tdeemon/internal/store"
)

// user delete deletes a user with --yes, and never the only admin.
func TestUserDelete(t *testing.T) {
	t.Parallel()

	env, dbPath := instance(t, nil)
	for _, args := range [][]string{
		{"--email", "ana@example.com", "--admin"},
		{"--email", "ivo@example.com"},
	} {
		if err := run(t.Context(), append([]string{"user", "create"}, args...), env, noInput(), io.Discard, io.Discard); err != nil {
			t.Fatal(err)
		}
	}
	for name, args := range map[string][]string{
		"no --yes":    {"--email", "ivo@example.com"},
		"no email":    {"--yes"},
		"an argument": {"--email", "ivo@example.com", "--yes", "more"},
		"a bad flag":  {"--mail", "ivo@example.com", "--yes"},
	} {
		if err := run(t.Context(), append([]string{"user", "delete"}, args...), env, noInput(), io.Discard, io.Discard); !errors.Is(err, errUsage) {
			t.Errorf("%s: %v, want a usage error", name, err)
		}
	}
	for name, args := range map[string][]string{
		"no such user":   {"--email", "eve@example.com", "--yes"},
		"the only admin": {"--email", "ana@example.com", "--yes"},
	} {
		if err := run(t.Context(), append([]string{"user", "delete"}, args...), env, noInput(), io.Discard, io.Discard); err == nil || errors.Is(err, errUsage) {
			t.Errorf("%s: %v, want a refusal", name, err)
		}
	}
	var out bytes.Buffer
	if err := run(t.Context(), []string{"user", "delete", "--email", "IVO@example.com", "--yes"}, env, noInput(), &out, io.Discard); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "Deleted ivo@example.com") {
		t.Errorf("said %q", out.String())
	}
	db, err := store.Open(t.Context(), dbPath)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	if _, err := db.UserByEmail(t.Context(), "ivo@example.com"); !errors.Is(err, store.ErrNotFound) {
		t.Errorf("ivo after the delete: %v, want ErrNotFound", err)
	}
	if _, err := db.UserByEmail(t.Context(), "ana@example.com"); err != nil {
		t.Errorf("ana after the delete: %v", err)
	}
}
