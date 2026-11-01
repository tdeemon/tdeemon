package api

import (
	"context"
	"errors"
	"net/http"
	"time"
	"uuid"

	"github.com/danielgtaylor/huma/v2"

	"github.com/zarnautovic/tdeemon/internal/auth"
	"github.com/zarnautovic/tdeemon/internal/domain/user"
	"github.com/zarnautovic/tdeemon/internal/store"
)

// inviteLifetime is how long an invite's link works.
const inviteLifetime = 7 * 24 * time.Hour

// invitePath is the web app's page an invite's link opens, the token
// after its # so that it reaches no server's log.
const invitePath = "/invite#"

type inviteBody struct {
	ID        string    `json:"id" format:"uuid" doc:"The invite's ID, to revoke it by."`
	Email     string    `json:"email" doc:"The address the account will sign in with."`
	Role      string    `json:"role" enum:"admin,user" doc:"The account's role."`
	CreatedAt time.Time `json:"created_at" doc:"When it was made."`
	ExpiresAt time.Time `json:"expires_at" doc:"When its link stops working."`
}

func newInviteBody(inv store.Invite) inviteBody {
	return inviteBody{
		ID: inv.ID.String(), Email: inv.Email, Role: string(inv.Role),
		CreatedAt: inv.CreatedAt.UTC(), ExpiresAt: inv.ExpiresAt.UTC(),
	}
}

type createInviteInput struct {
	Body struct {
		Email string `json:"email" maxLength:"254" doc:"The address the new user will sign in with."`
		Role  string `json:"role,omitempty" enum:"admin,user" default:"user" doc:"Their role; user unless set."`
	}
}

type createInviteOutput struct {
	CacheControl string `header:"Cache-Control" doc:"no-store"`
	Body         struct {
		Invite inviteBody `json:"invite"`
		URL    string     `json:"url" doc:"The link to send them, shown once: it makes one account for the address, until the invite expires."`
	}
}

// createInvite makes an invite of a new user, for the admin.
func (s *sessions) createInvite(ctx context.Context, in *createInviteInput) (*createInviteOutput, error) {
	p, err := requireAdmin(ctx)
	if err != nil {
		return nil, err
	}
	email := user.NormalizeEmail(in.Body.Email)
	if err := user.ValidateEmail(email); err != nil {
		return nil, huma.Error422UnprocessableEntity(err.Error())
	}
	now := s.now().UTC().Truncate(time.Second)
	inv := store.Invite{
		ID: uuid.NewV7(), By: p.User.ID, Email: email, Role: user.Role(in.Body.Role),
		CreatedAt: now, ExpiresAt: now.Add(inviteLifetime),
	}
	token, hash := auth.NewSessionToken()
	switch err := s.db.CreateInvite(ctx, inv, hash); {
	case errors.Is(err, store.ErrEmailTaken):
		return nil, huma.Error409Conflict("a user has this address already")
	case errors.Is(err, store.ErrNotAdmin):
		return nil, huma.Error403Forbidden("this needs the instance's admin")
	case errors.Is(err, store.ErrInvitedByAnother):
		return nil, huma.Error409Conflict("another admin invited this address; they can revoke their invite")
	case err != nil:
		return nil, internalError("making the invite failed", err)
	}
	s.logger.InfoContext(ctx, "made an invite", "invite_id", inv.ID.String(), "by", p.User.ID.String())
	out := &createInviteOutput{CacheControl: "no-store"}
	out.Body.Invite = newInviteBody(inv)
	out.Body.URL = s.origin + invitePath + token
	return out, nil
}

type listInvitesOutput struct {
	CacheControl string `header:"Cache-Control" doc:"no-store"`
	Body         struct {
		Invites []inviteBody `json:"invites" nullable:"false" doc:"The invites you made that no account took yet, expired ones too, the newest first."`
	}
}

func (s *sessions) listInvites(ctx context.Context, _ *struct{}) (*listInvitesOutput, error) {
	p, err := requireAdmin(ctx)
	if err != nil {
		return nil, err
	}
	invites, err := s.db.Invites(ctx, p.User.ID)
	if err != nil {
		return nil, internalError("listing the invites failed", err)
	}
	out := &listInvitesOutput{CacheControl: "no-store"}
	out.Body.Invites = make([]inviteBody, len(invites))
	for i, inv := range invites {
		out.Body.Invites[i] = newInviteBody(inv)
	}
	return out, nil
}

type inviteIDInput struct {
	ID string `path:"id" format:"uuid"`
}

func (s *sessions) deleteInvite(ctx context.Context, in *inviteIDInput) (*struct{}, error) {
	p, err := requireAdmin(ctx)
	if err != nil {
		return nil, err
	}
	id, err := uuid.Parse(in.ID)
	if err != nil {
		return nil, huma.Error422UnprocessableEntity("the id is not a UUID")
	}
	ok, err := s.db.DeleteInvite(ctx, p.User.ID, id)
	if err != nil {
		return nil, internalError("revoking the invite failed", err)
	}
	if !ok {
		return nil, huma.Error404NotFound("you made no invite with that id")
	}
	return &struct{}{}, nil
}

type checkInviteInput struct {
	Body struct {
		Token string `json:"token" minLength:"1" maxLength:"100" doc:"The token of the invite's link, after its #."`
	}
}

type checkInviteOutput struct {
	CacheControl string `header:"Cache-Control" doc:"no-store"`
	Body         struct {
		Email     string    `json:"email" doc:"The address the account will sign in with."`
		ExpiresAt time.Time `json:"expires_at" doc:"When the link stops working."`
	}
}

// errNoInvite answers a token of no invite, or of one that expired or
// made its account already.
func errNoInvite() error {
	return huma.Error404NotFound("this invite does not work: it expired or was used; ask for another")
}

// checkInvite says what an invite's link makes, for its page.
func (s *sessions) checkInvite(ctx context.Context, in *checkInviteInput) (*checkInviteOutput, error) {
	inv, err := s.db.InviteByToken(ctx, auth.SessionID(in.Body.Token), s.now())
	if errors.Is(err, store.ErrNotFound) {
		return nil, errNoInvite()
	} else if err != nil {
		return nil, internalError("checking the invite failed", err)
	}
	out := &checkInviteOutput{CacheControl: "no-store"}
	out.Body.Email, out.Body.ExpiresAt = inv.Email, inv.ExpiresAt.UTC()
	return out, nil
}

type acceptInviteInput struct {
	Body struct {
		Token    string `json:"token" minLength:"1" maxLength:"100" doc:"The token of the invite's link, after its #."`
		Name     string `json:"name" minLength:"1" maxLength:"100" doc:"The name the app shows."`
		Password string `json:"password" maxLength:"1024" doc:"The account's password: 15 to 256 characters."`
		TimeZone string `json:"timezone" maxLength:"64" example:"Europe/Zagreb" doc:"An IANA time zone, the browser's."`
		Locale   string `json:"locale" enum:"en,hr" doc:"The language of the app."`
	}
}

// acceptInvite makes the account an invite's link is for, with a
// password, and signs it in.
func (s *sessions) acceptInvite(ctx context.Context, in *acceptInviteInput) (*sessionOutput, error) {
	tokenHash := auth.SessionID(in.Body.Token)
	now := s.now()
	inv, err := s.db.InviteByToken(ctx, tokenHash, now)
	if errors.Is(err, store.ErrNotFound) {
		return nil, errNoInvite()
	} else if err != nil {
		return nil, internalError("accepting the invite failed", err)
	}
	if err := auth.CheckPassword(in.Body.Password); err != nil {
		return nil, huma.Error422UnprocessableEntity(err.Error())
	}
	now = now.UTC().Truncate(time.Second)
	u := user.User{
		ID: uuid.NewV7(), Email: inv.Email, Name: in.Body.Name, Role: inv.Role, Locale: in.Body.Locale,
		TimeZone: in.Body.TimeZone, DayStartHour: user.DefaultDayStartHour, CreatedAt: now, UpdatedAt: now,
	}
	if err := u.Validate(); err != nil {
		return nil, huma.Error422UnprocessableEntity(err.Error())
	}
	hash, err := auth.HashPassword(ctx, in.Body.Password)
	if err != nil {
		if errors.Is(err, auth.ErrBusy) || errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
			return nil, errBusy()
		}
		return nil, internalError("accepting the invite failed", err)
	}
	cookie, sess, err := s.start(ctx, u, now, "invite", func(sess store.Session) error {
		_, err := s.db.AcceptInvite(ctx, tokenHash, store.Acceptance{User: u, PasswordHash: hash, Session: sess}, now)
		return err
	})
	switch {
	case errors.Is(err, store.ErrNotFound):
		return nil, errNoInvite()
	case errors.Is(err, store.ErrEmailTaken):
		return nil, huma.Error409Conflict("a user has this address already: sign in instead")
	case err != nil:
		return nil, internalError("accepting the invite failed", err)
	}
	return &sessionOutput{
		SetCookie: cookie, CacheControl: "no-store",
		Body: sessionBody{User: newUserBody(u), ExpiresAt: sess.ExpiresAt.UTC()},
	}, nil
}

func (s *sessions) registerInvites(a huma.API) {
	admin := []int{http.StatusUnauthorized, http.StatusForbidden}
	huma.Register(a, huma.Operation{
		OperationID: "create-invite",
		Method:      http.MethodPost,
		Path:        Prefix + "/invites",
		Summary:     "Invite a new user",
		Description: "Needs the admin's web session. Answers the link to send them, once: it makes one account for the " +
			"address, with the role, in the next 7 days. A new invite of an address replaces yours before; an address " +
			"a user has already, or another admin invited, is refused (409).",
		Tags:          []string{"users"},
		DefaultStatus: http.StatusCreated,
		Errors:        append(admin, http.StatusConflict, http.StatusUnprocessableEntity),
	}, s.createInvite)
	huma.Register(a, huma.Operation{
		OperationID: "list-invites",
		Method:      http.MethodGet,
		Path:        Prefix + "/invites",
		Summary:     "List your invites",
		Description: "Needs the admin's web session.",
		Tags:        []string{"users"},
		Errors:      admin,
	}, s.listInvites)
	huma.Register(a, huma.Operation{
		OperationID:   "delete-invite",
		Method:        http.MethodDelete,
		Path:          Prefix + "/invites/{id}",
		Summary:       "Revoke an invite",
		Description:   "Needs the admin's web session; its link then makes nothing.",
		Tags:          []string{"users"},
		DefaultStatus: http.StatusNoContent,
		Errors:        append(admin, http.StatusNotFound, http.StatusUnprocessableEntity),
	}, s.deleteInvite)
	huma.Register(a, huma.Operation{
		OperationID: "check-invite",
		Method:      http.MethodPost,
		Path:        Prefix + "/invites/check",
		Summary:     "Check an invite's link",
		Description: "Needs nothing but the link's token: the address the account will have, and until when the link " +
			"works; 404 for a link that does not work.",
		Tags:   []string{"auth"},
		Errors: []int{http.StatusNotFound, http.StatusUnprocessableEntity},
	}, s.checkInvite)
	huma.Register(a, huma.Operation{
		OperationID: "accept-invite",
		Method:      http.MethodPost,
		Path:        Prefix + "/invites/accept",
		Summary:     "Make your account by an invite",
		Description: "Needs nothing but the link's token: makes the account with the invite's address and role, the " +
			"name, password, time zone and language given, and signs it in, as a sign-in does. The link then works no " +
			"more. 404 for a link that does not work, 409 for an address a user has since.",
		Tags:          []string{"auth"},
		DefaultStatus: http.StatusCreated,
		Errors:        []int{http.StatusNotFound, http.StatusConflict, http.StatusUnprocessableEntity, http.StatusServiceUnavailable},
	}, s.acceptInvite)
}
