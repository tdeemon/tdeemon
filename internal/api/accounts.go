package api

import (
	"context"
	"errors"
	"net/http"
	"uuid"

	"github.com/danielgtaylor/huma/v2"

	"github.com/zarnautovic/tdeemon/internal/auth"
	"github.com/zarnautovic/tdeemon/internal/domain/user"
	"github.com/zarnautovic/tdeemon/internal/store"
)

// requireAdmin returns the principal of a request made in an admin's web
// session.
func requireAdmin(ctx context.Context) (auth.Principal, error) {
	p, err := requireSession(ctx)
	if err != nil {
		return auth.Principal{}, err
	}
	if p.User.Role != user.RoleAdmin {
		return auth.Principal{}, huma.Error403Forbidden("this needs the instance's admin")
	}
	return p, nil
}

type deleteAccountInput struct {
	Body struct {
		Email string `json:"email" maxLength:"254" doc:"The account's email address, typed again to say it is this account that goes."`
	}
}

type deleteAccountOutput struct {
	SetCookie    http.Cookie `header:"Set-Cookie" doc:"Clears the session cookie."`
	CacheControl string      `header:"Cache-Control" doc:"no-store"`
}

type userIDInput struct {
	ID string `path:"id" format:"uuid"`
}

// deleteAccount deletes the signed-in user's account with all their data.
func (s *sessions) deleteAccount(ctx context.Context, in *deleteAccountInput) (*deleteAccountOutput, error) {
	p, err := requireSession(ctx)
	if err != nil {
		return nil, err
	}
	if user.NormalizeEmail(in.Body.Email) != p.User.Email {
		return nil, huma.Error422UnprocessableEntity("the email address is not this account's")
	}
	// An account another delete took since the session was checked is
	// gone as asked: no 404.
	if err := s.deleteUser(ctx, p, p.User.ID); err != nil && !errors.Is(err, store.ErrNotFound) {
		return nil, deleteError(err)
	}
	return &deleteAccountOutput{SetCookie: s.clearedCookie(), CacheControl: "no-store"}, nil
}

// adminDeleteUser deletes a user's account with all their data, for the
// admin.
func (s *sessions) adminDeleteUser(ctx context.Context, in *userIDInput) (*struct{}, error) {
	p, err := requireAdmin(ctx)
	if err != nil {
		return nil, err
	}
	id, err := uuid.Parse(in.ID)
	if err != nil {
		return nil, huma.Error422UnprocessableEntity("the id is not a UUID")
	}
	if err := s.deleteUser(ctx, p, id); err != nil {
		return nil, deleteError(err)
	}
	return &struct{}{}, nil
}

// deleteUser deletes the user with id for p, and says so in the log by
// their ids alone.
func (s *sessions) deleteUser(ctx context.Context, p auth.Principal, id uuid.UUID) error {
	if _, err := s.db.DeleteUser(ctx, id); err != nil {
		return err
	}
	s.logger.InfoContext(ctx, "deleted an account", "user_id", id.String(), "by", p.User.ID.String())
	return nil
}

// deleteError is the answer to a delete of a user that failed with err.
func deleteError(err error) error {
	switch {
	case errors.Is(err, store.ErrNotFound):
		return huma.Error404NotFound("no such user")
	case errors.Is(err, store.ErrLastAdmin):
		return huma.Error409Conflict("this is the instance's only admin: make another user an admin first")
	}
	return internalError("deleting the account failed", err)
}

func (s *sessions) registerAccounts(a huma.API) {
	huma.Register(a, huma.Operation{
		OperationID: "delete-account",
		Method:      http.MethodDelete,
		Path:        Prefix + "/account",
		Summary:     "Delete your account",
		Description: "Needs a web session. Deletes your account and all your data: meals, foods, recipes, measurements, " +
			"the plan and its history, devices and keys; it cannot be undone. The body repeats the account's email " +
			"address. The instance's only admin cannot be deleted (409). Snapshots of the database taken before keep " +
			"the data until they go.",
		Tags:          []string{"settings"},
		DefaultStatus: http.StatusNoContent,
		Errors:        []int{http.StatusUnauthorized, http.StatusForbidden, http.StatusConflict, http.StatusUnprocessableEntity},
	}, s.deleteAccount)
	huma.Register(a, huma.Operation{
		OperationID: "delete-user",
		Method:      http.MethodDelete,
		Path:        Prefix + "/users/{id}",
		Summary:     "Delete a user",
		Description: "Needs the admin's web session. Deletes the user's account and all their data; it cannot be " +
			"undone. The instance's only admin cannot be deleted (409).",
		Tags:          []string{"users"},
		DefaultStatus: http.StatusNoContent,
		Errors:        []int{http.StatusUnauthorized, http.StatusForbidden, http.StatusNotFound, http.StatusConflict, http.StatusUnprocessableEntity},
	}, s.adminDeleteUser)
}
