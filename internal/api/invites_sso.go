package api

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"time"
	"uuid"

	"github.com/danielgtaylor/huma/v2"

	"github.com/tdeemon/tdeemon/internal/auth"
	"github.com/tdeemon/tdeemon/internal/domain/user"
	"github.com/tdeemon/tdeemon/internal/store"
)

type inviteSSOInput struct {
	Body struct {
		Token    string `json:"token" minLength:"1" maxLength:"100" doc:"The token of the invite's link, after its #."`
		TimeZone string `json:"timezone" maxLength:"64" example:"Europe/Zagreb" doc:"An IANA time zone, the browser's."`
		Locale   string `json:"locale" enum:"en,hr" doc:"The language of the app."`
	}
}

type inviteSSOOutput struct {
	SetCookie    http.Cookie `header:"Set-Cookie" doc:"The sign-in's flow cookie."`
	CacheControl string      `header:"Cache-Control" doc:"no-store"`
	Body         struct {
		URL string `json:"url" doc:"The provider's page to send the browser to; back from it, the account is made and signed in."`
	}
}

// registerInviteSSO registers the start of an invite's account by single
// sign-on; without a provider (o nil) it answers 404.
func registerInviteSSO(a huma.API, o *oidcLogin) {
	huma.Register(a, huma.Operation{
		OperationID: "start-invite-sso",
		Method:      http.MethodPost,
		Path:        Prefix + "/invites/sso",
		Summary:     "Make your account by an invite, through the sign-in provider",
		Description: "Needs nothing but the link's token: starts a sign-in at the OpenID Connect provider, and answers " +
			"the page to send the browser to. Back from it, the account is made with the invite's address and role, " +
			"the provider's name, and the time zone and language given, linked to the identity the provider vouches " +
			"for, and signed in; the browser lands on /. 404 for a link that does not work, or an instance without a " +
			"provider.",
		Tags:   []string{"auth"},
		Errors: []int{http.StatusNotFound, http.StatusUnprocessableEntity, http.StatusBadGateway},
	}, func(ctx context.Context, in *inviteSSOInput) (*inviteSSOOutput, error) {
		if o == nil {
			return nil, huma.Error404NotFound("this instance has no sign-in provider: choose a password")
		}
		return o.startInvite(ctx, in)
	})
}

func (o *oidcLogin) startInvite(ctx context.Context, in *inviteSSOInput) (*inviteSSOOutput, error) {
	s := o.sessions
	if _, err := s.db.InviteByToken(ctx, auth.SessionID(in.Body.Token), s.now()); errors.Is(err, store.ErrNotFound) {
		return nil, errNoInvite()
	} else if err != nil {
		return nil, internalError("checking the invite failed", err)
	}
	if err := user.ValidateTimeZone(in.Body.TimeZone); err != nil {
		return nil, huma.Error422UnprocessableEntity(err.Error())
	}
	cookie, to, err := o.begin(ctx, "/", &flowInvite{Token: in.Body.Token, TimeZone: in.Body.TimeZone, Locale: in.Body.Locale})
	if err != nil {
		return nil, huma.Error502BadGateway("the sign-in provider cannot be reached; try again, or choose a password")
	}
	out := &inviteSSOOutput{SetCookie: *cookie, CacheControl: "no-store"}
	out.Body.URL = to
	return out, nil
}

// acceptInvite makes the account of f's invite for the identity id the
// provider vouched for, signs it in, and sends the browser on.
func (o *oidcLogin) acceptInvite(w http.ResponseWriter, r *http.Request, id store.Identity, f flow, now time.Time) {
	ctx := r.Context()
	s := o.sessions
	tokenHash := auth.SessionID(f.Invite.Token)
	inv, err := s.db.InviteByToken(ctx, tokenHash, now)
	if errors.Is(err, store.ErrNotFound) {
		writeProblem(w, http.StatusNotFound, "this invite does not work: it expired or was used; ask for another")
		return
	} else if err != nil {
		s.logger.ErrorContext(ctx, "checking an invite failed", "error", err)
		writeProblem(w, http.StatusInternalServerError, "making the account failed")
		return
	}
	now = now.UTC().Truncate(time.Second)
	u := user.User{
		ID: uuid.NewV7(), Email: inv.Email, Name: inviteeName(id.Name, inv.Email), Role: inv.Role, Locale: f.Invite.Locale,
		TimeZone: f.Invite.TimeZone, DayStartHour: user.DefaultDayStartHour, CreatedAt: now, UpdatedAt: now,
	}
	cookie, _, err := s.start(ctx, u, now, "invite-oidc", func(sess store.Session) error {
		_, err := s.db.AcceptInvite(ctx, tokenHash, store.Acceptance{User: u, Identity: &id, Session: sess}, now)
		return err
	})
	switch {
	case errors.Is(err, store.ErrNotFound):
		writeProblem(w, http.StatusNotFound, "this invite does not work: it expired or was used; ask for another")
		return
	case errors.Is(err, store.ErrEmailTaken):
		writeProblem(w, http.StatusConflict, "a user has this address already: sign in instead")
		return
	case errors.Is(err, store.ErrAlreadyLinked):
		writeProblem(w, http.StatusConflict, "your account at the sign-in provider is another user's here: sign in instead")
		return
	case err != nil:
		s.logger.ErrorContext(ctx, "accepting an invite by single sign-on failed", "error", err)
		writeProblem(w, http.StatusInternalServerError, "making the account failed")
		return
	}
	http.SetCookie(w, &cookie)
	http.Redirect(w, r, safeReturnTo(f.ReturnTo), http.StatusSeeOther)
}

// inviteeName is the name of an account an invite makes by single
// sign-on: the provider's, or the part of the address before the @, cut
// to what a name may be.
func inviteeName(given, email string) string {
	name := given
	if user.ValidateName(name) != nil {
		name, _, _ = strings.Cut(email, "@")
	}
	if user.ValidateName(name) != nil {
		name = "New user"
	}
	return name
}
