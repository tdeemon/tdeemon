package main

import (
	"bytes"
	"errors"
	"io"
	"strings"
	"testing"
	"time"

	"github.com/zarnautovic/tdeemon/internal/store"
)

// user unlink removes a user's identities, at one issuer or all, so the
// next sign-in links anew.
func TestUserUnlink(t *testing.T) {
	t.Parallel()

	env, dbPath := instance(t, nil)
	if err := run(t.Context(), []string{"user", "create", "--email", "ana@example.com"}, env, noInput(), io.Discard, io.Discard); err != nil {
		t.Fatal(err)
	}
	db, err := store.Open(t.Context(), dbPath)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	now := time.Now()
	for _, iss := range []string{"https://a.example", "https://b.example"} {
		id := store.Identity{Issuer: iss, Subject: "s", Email: "ana@example.com", EmailVerified: true}
		if _, err := db.SignInWithIdentity(t.Context(), id, now); err != nil {
			t.Fatal(err)
		}
	}
	var out bytes.Buffer
	if err := run(t.Context(), []string{"user", "unlink", "--email", "ANA@example.com", "--issuer", "https://a.example"}, env, noInput(), &out, io.Discard); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "Unlinked 1 ") {
		t.Errorf("said %q", out.String())
	}
	out.Reset()
	if err := run(t.Context(), []string{"user", "unlink", "--email", "ana@example.com"}, env, noInput(), &out, io.Discard); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "Unlinked 1 ") {
		t.Errorf("said %q", out.String())
	}
	if err := run(t.Context(), []string{"user", "unlink", "--email", "eve@example.com"}, env, noInput(), io.Discard, io.Discard); err == nil || errors.Is(err, errUsage) {
		t.Errorf("no such user: %v", err)
	}
	for name, args := range map[string][]string{"no email": {}, "an argument": {"--email", "ana@example.com", "x"}} {
		if err := run(t.Context(), append([]string{"user", "unlink"}, args...), env, noInput(), io.Discard, io.Discard); !errors.Is(err, errUsage) {
			t.Errorf("%s: %v", name, err)
		}
	}
}
