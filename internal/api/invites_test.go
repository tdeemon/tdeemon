package api_test

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"
	"time"
	"uuid"

	"github.com/zarnautovic/tdeemon/internal/domain/user"
)

// invite has the admin of session invite address, and returns the token
// of the link.
func (in *instance) invite(t *testing.T, session *http.Cookie, address string) string {
	t.Helper()
	rec := in.do(t, http.MethodPost, "/api/v1/invites", `{"email": "`+address+`"}`, []*http.Cookie{session}, nil)
	var got struct {
		URL string `json:"url"`
	}
	if rec.Code != http.StatusCreated || json.Unmarshal(rec.Body.Bytes(), &got) != nil {
		t.Fatalf("invite %s: %d %s", address, rec.Code, rec.Body.String())
	}
	_, token, ok := strings.Cut(got.URL, "/invite#")
	if !ok || !strings.HasPrefix(got.URL, "http://localhost/invite#") {
		t.Fatalf("the link %q", got.URL)
	}
	return token
}

// An invite's link makes one account for its address, with its role and
// the name, password, time zone and language given, signed in at once;
// then it works no more.
func TestAcceptInvite(t *testing.T) {
	t.Parallel()

	in, session := signedIn(t)
	rec := in.do(t, http.MethodPost, "/api/v1/invites", `{"email": " Zoe@Example.com ", "role": "admin"}`, []*http.Cookie{session}, nil)
	var made struct {
		URL string `json:"url"`
	}
	if rec.Code != http.StatusCreated || json.Unmarshal(rec.Body.Bytes(), &made) != nil || !strings.Contains(rec.Body.String(), `"email":"zoe@example.com"`) {
		t.Fatalf("invite: %d %s", rec.Code, rec.Body.String())
	}
	_, token, _ := strings.Cut(made.URL, "#")
	if rec := in.do(t, http.MethodPost, "/api/v1/invites/check", `{"token": "`+token+`"}`, nil, nil); rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), "zoe@example.com") {
		t.Errorf("check: %d %s", rec.Code, rec.Body.String())
	}
	body := `{"token": "` + token + `", "name": "Zoe", "password": "` + password + `", "timezone": "Europe/Zagreb", "locale": "hr"}`
	rec = in.do(t, http.MethodPost, "/api/v1/invites/accept", body, nil, nil)
	if rec.Code != http.StatusCreated {
		t.Fatalf("accept: %d %s", rec.Code, rec.Body.String())
	}
	zoeSession := sessionCookie(t, rec)
	who := in.do(t, http.MethodGet, "/api/v1/auth/session", "", []*http.Cookie{zoeSession}, nil)
	for _, want := range []string{`"email":"zoe@example.com"`, `"role":"admin"`, `"timezone":"Europe/Zagreb"`, `"locale":"hr"`, `"name":"Zoe"`} {
		if !strings.Contains(who.Body.String(), want) {
			t.Errorf("zoe's session = %s, want %s", who.Body.String(), want)
		}
	}
	if rec := in.login(t, "zoe@example.com", password); rec.Code != http.StatusOK {
		t.Errorf("zoe's password: %d", rec.Code)
	}
	if rec := in.do(t, http.MethodPost, "/api/v1/invites/check", `{"token": "`+token+`"}`, nil, nil); rec.Code != http.StatusNotFound {
		t.Errorf("check after: %d %s, want 404", rec.Code, rec.Body.String())
	}
	if rec := in.do(t, http.MethodPost, "/api/v1/invites/accept", body, nil, nil); rec.Code != http.StatusNotFound {
		t.Errorf("accept after: %d %s, want 404", rec.Code, rec.Body.String())
	}
	if rec := in.do(t, http.MethodGet, "/api/v1/invites", "", []*http.Cookie{session}, nil); strings.Contains(rec.Body.String(), "zoe@") {
		t.Errorf("the invites after: %s, want none", rec.Body.String())
	}
}

// An invite does not work once it expired, was replaced or revoked, nor
// for an address a user has; a bad password or time zone makes nothing.
func TestInviteRefuses(t *testing.T) {
	t.Parallel()

	in, session := signedIn(t)
	cookies := []*http.Cookie{session}
	accept := func(token, pw, tz string) int {
		body := `{"token": "` + token + `", "name": "Zoe", "password": "` + pw + `", "timezone": "` + tz + `", "locale": "en"}`
		return in.do(t, http.MethodPost, "/api/v1/invites/accept", body, nil, nil).Code
	}
	if rec := in.do(t, http.MethodPost, "/api/v1/invites", `{"email": "ana@example.com"}`, cookies, nil); rec.Code != http.StatusConflict {
		t.Errorf("an address ana has: %d, want 409", rec.Code)
	}
	old := in.invite(t, session, "zoe@example.com")
	fresh := in.invite(t, session, "zoe@example.com")
	if got := accept(old, password, "UTC"); got != http.StatusNotFound {
		t.Errorf("the replaced invite: %d, want 404", got)
	}
	if got := accept(fresh, "short", "UTC"); got != http.StatusUnprocessableEntity {
		t.Errorf("a short password: %d, want 422", got)
	}
	if got := accept(fresh, password, "Mars/Olympus"); got != http.StatusUnprocessableEntity {
		t.Errorf("a time zone that is none: %d, want 422", got)
	}
	in.clock.Set(in.clock.Now().Add(7*24*time.Hour + time.Second))
	if got := accept(fresh, password, "UTC"); got != http.StatusNotFound {
		t.Errorf("an expired invite: %d, want 404", got)
	}
	revoked := in.invite(t, session, "mia@example.com")
	list := in.do(t, http.MethodGet, "/api/v1/invites", "", cookies, nil)
	var got struct {
		Invites []struct {
			ID, Email string
		} `json:"invites"`
	}
	_ = json.Unmarshal(list.Body.Bytes(), &got)
	if len(got.Invites) != 2 || got.Invites[0].Email != "mia@example.com" {
		t.Fatalf("invites = %+v, want mia's and zoe's expired one, the newest first", got.Invites)
	}
	if rec := in.do(t, http.MethodDelete, "/api/v1/invites/"+got.Invites[0].ID, "", cookies, nil); rec.Code != http.StatusNoContent {
		t.Errorf("revoke: %d", rec.Code)
	}
	if got := accept(revoked, password, "UTC"); got != http.StatusNotFound {
		t.Errorf("a revoked invite: %d, want 404", got)
	}
	// A user made since the invite.
	taken := in.invite(t, session, "eve@example.com")
	now := in.clock.Now()
	eve := user.User{
		ID: uuid.NewV7(), Email: "eve@example.com", Name: "Eve", Role: user.RoleUser,
		Locale: "en", TimeZone: "UTC", DayStartHour: 4, CreatedAt: now, UpdatedAt: now,
	}
	if err := in.db.CreateUser(t.Context(), eve, ""); err != nil {
		t.Fatal(err)
	}
	if got := accept(taken, password, "UTC"); got != http.StatusConflict {
		t.Errorf("an address a user has since: %d, want 409", got)
	}
}

// An admin's invites go when they are an admin no more, deactivated or
// made a user: a link of theirs makes no account, an admin's least.
func TestInvitesGoWithTheirAdmin(t *testing.T) {
	t.Parallel()

	for _, change := range []string{`{"active": false}`, `{"role": "user"}`} {
		in, session := signedIn(t)
		eve := in.invite(t, session, "eve@example.com")
		in.addUser(t, user.RoleAdmin) // so that ana is not the last admin
		rec := in.do(t, http.MethodPatch, "/api/v1/users/"+in.user.ID.String(), change, []*http.Cookie{session}, nil)
		if rec.Code != http.StatusOK {
			t.Fatalf("%s: %d %s", change, rec.Code, rec.Body.String())
		}
		if rec := in.do(t, http.MethodPost, "/api/v1/invites/check", `{"token": "`+eve+`"}`, nil, nil); rec.Code != http.StatusNotFound {
			t.Errorf("%s: ana's invite after: %d, want 404", change, rec.Code)
		}
	}
}
