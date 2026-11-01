package api_test

import (
	"net/http"
	"slices"
	"strings"
	"testing"
	"uuid"

	"github.com/zarnautovic/tdeemon/internal/domain/user"
)

// addUser adds Zoe, without a password, to in's instance.
func (in *instance) addUser(t *testing.T, role user.Role) user.User {
	t.Helper()
	now := in.clock.Now()
	u := user.User{
		ID: uuid.NewV7(), Email: "zoe@example.com", Name: "Zoe", Role: role,
		Locale: "en", TimeZone: "UTC", DayStartHour: 4, CreatedAt: now, UpdatedAt: now,
	}
	if err := in.db.CreateUser(t.Context(), u, ""); err != nil {
		t.Fatalf("CreateUser: %v", err)
	}
	return u
}

// Deleting an account takes every row of the user's, in every table of
// user data, and leaves the other user's as they were; the session it
// came in ends, and so do the user's tokens.
func TestDeleteAccountTakesAllTheirData(t *testing.T) {
	t.Parallel()

	e := newIsolationEnv(t, false)
	ivo, ana := e.other, e.owner
	anasRows, _ := e.rowsOf(t, ana.user.ID)
	rec := e.in.do(t, http.MethodDelete, "/api/v1/account", `{"email": "IVO@example.com"}`, []*http.Cookie{ivo.session}, nil)
	if rec.Code != http.StatusNoContent {
		t.Fatalf("delete ivo's account: %d %s", rec.Code, rec.Body.String())
	}
	if c := rec.Result().Cookies(); len(c) != 1 || c[0].MaxAge >= 0 {
		t.Errorf("cookies = %v, want the session's cleared", c)
	}
	if rows, _ := e.rowsOf(t, ivo.user.ID); len(rows) != 0 {
		t.Errorf("ivo's rows after the delete:\n%v", rows)
	}
	if rows, _ := e.rowsOf(t, ana.user.ID); !slices.Equal(rows, anasRows) {
		t.Errorf("ana's rows changed:\n%v\nwant\n%v", rows, anasRows)
	}
	if rec := e.in.do(t, http.MethodGet, "/api/v1/auth/session", "", []*http.Cookie{ivo.session}, nil); rec.Code != http.StatusUnauthorized {
		t.Errorf("ivo's session after: %d, want 401", rec.Code)
	}
	if rec := e.in.do(t, http.MethodGet, "/api/v1/auth/whoami", "", nil, bearer(ivo.token.Token)); rec.Code != http.StatusUnauthorized {
		t.Errorf("ivo's token after: %d, want 401", rec.Code)
	}
	logs := e.in.logs.String()
	if !strings.Contains(logs, "deleted an account") || strings.Contains(logs, ivo.user.Email) {
		t.Errorf("the log says the delete by ids alone, want:\n%s", logs)
	}
}

// An account goes only with its own address typed, and never the
// instance's only admin: someone must be able to manage it.
func TestDeleteAccountRefuses(t *testing.T) {
	t.Parallel()

	in, session := signedIn(t)
	cookies := []*http.Cookie{session}
	for _, c := range []struct {
		body string
		want int
	}{
		{`{"email": "ivo@example.com"}`, http.StatusUnprocessableEntity},
		{`{}`, http.StatusUnprocessableEntity},
		{`{"email": "ana@example.com"}`, http.StatusConflict},
	} {
		if rec := in.do(t, http.MethodDelete, "/api/v1/account", c.body, cookies, nil); rec.Code != c.want {
			t.Errorf("%s: %d %s, want %d", c.body, rec.Code, rec.Body.String(), c.want)
		}
	}
	if rec := in.do(t, http.MethodGet, "/api/v1/auth/session", "", cookies, nil); rec.Code != http.StatusOK {
		t.Errorf("the session after the refusals: %d, want 200", rec.Code)
	}
	token := mcpToken(t, in, session, `"read", "tools"`)
	if rec := in.do(t, http.MethodDelete, "/api/v1/account", `{"email": "ana@example.com"}`, nil, bearer(token)); rec.Code != http.StatusForbidden {
		t.Errorf("with an API token: %d %s, want 403", rec.Code, rec.Body.String())
	}
}

// The admin deletes another user, or another admin, but not the last
// one; a user deletes nobody.
func TestDeleteUser(t *testing.T) {
	t.Parallel()

	in, session := signedIn(t)
	cookies := []*http.Cookie{session}
	in.addUser(t, user.RoleAdmin)
	if rec := in.do(t, http.MethodDelete, "/api/v1/users/"+in.user.ID.String(), "", cookies, nil); rec.Code != http.StatusNoContent {
		t.Fatalf("ana deletes herself with another admin there: %d %s", rec.Code, rec.Body.String())
	}
	in2, session2 := signedIn(t)
	zoe2 := in2.addUser(t, user.RoleUser)
	for _, c := range []struct {
		id   string
		want int
	}{
		{zoe2.ID.String(), http.StatusNoContent},
		{zoe2.ID.String(), http.StatusNotFound},
		{uuid.NewV7().String(), http.StatusNotFound},
		{"not-a-uuid", http.StatusUnprocessableEntity},
		{in2.user.ID.String(), http.StatusConflict},
	} {
		if rec := in2.do(t, http.MethodDelete, "/api/v1/users/"+c.id, "", []*http.Cookie{session2}, nil); rec.Code != c.want {
			t.Errorf("delete %s: %d %s, want %d", c.id, rec.Code, rec.Body.String(), c.want)
		}
	}
}
