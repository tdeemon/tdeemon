package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"
	"uuid"

	"github.com/zarnautovic/tdeemon/internal/domain/user"
)

// Invite is an admin's invitation of a new user: a link that makes one
// account for Email, with Role, until ExpiresAt.
type Invite struct {
	ID uuid.UUID
	// By is the admin who made it.
	By        uuid.UUID
	Email     string
	Role      user.Role
	CreatedAt time.Time
	ExpiresAt time.Time
}

const inviteColumns = "id, user_id, email, role, created_at, expires_at"

func scanInvite(scan func(dest ...any) error) (Invite, error) {
	var (
		inv                         Invite
		id, by, role, created, exps string
	)
	if err := scan(&id, &by, &inv.Email, &role, &created, &exps); err != nil {
		return Invite{}, err
	}
	inv.Role = user.Role(role)
	var err error
	if inv.ID, err = uuid.Parse(id); err != nil {
		return Invite{}, fmt.Errorf("invite id %q: %w", id, err)
	}
	if inv.By, err = uuid.Parse(by); err != nil {
		return Invite{}, fmt.Errorf("invite user_id %q: %w", by, err)
	}
	if inv.CreatedAt, err = parseTime(created); err != nil {
		return Invite{}, err
	}
	if inv.ExpiresAt, err = parseTime(exps); err != nil {
		return Invite{}, err
	}
	return inv, nil
}

// CreateInvite stores inv under tokenHash (auth.SessionID of the link's
// token), replacing an invite of the same address, or returns
// ErrEmailTaken when a user has the address already.
func (db *DB) CreateInvite(ctx context.Context, inv Invite, tokenHash string) error {
	if inv.Role != user.RoleAdmin && inv.Role != user.RoleUser {
		return fmt.Errorf("create invite: role %q: want %s or %s", inv.Role, user.RoleAdmin, user.RoleUser)
	}
	return db.WriteTx(ctx, func(tx *sql.Tx) error {
		var taken bool
		if err := tx.QueryRowContext(ctx, "SELECT EXISTS (SELECT 1 FROM users WHERE email = ?)", inv.Email).Scan(&taken); err != nil {
			return fmt.Errorf("create invite: %w", err)
		}
		if taken {
			return ErrEmailTaken
		}
		if _, err := tx.ExecContext(ctx, "DELETE FROM invites WHERE email = ?", inv.Email); err != nil {
			return fmt.Errorf("create invite: %w", err)
		}
		_, err := tx.ExecContext(ctx, "INSERT INTO invites (id, token_hash, user_id, email, role, created_at, expires_at) "+
			"VALUES (?, ?, ?, ?, ?, ?, ?)", inv.ID.String(), tokenHash, inv.By.String(), inv.Email, string(inv.Role),
			formatTime(inv.CreatedAt), formatTime(inv.ExpiresAt))
		if err != nil {
			return fmt.Errorf("create invite: %w", err)
		}
		return nil
	})
}

// Invites returns the invites the admin with id made, expired ones too,
// the newest first.
func (db *DB) Invites(ctx context.Context, by uuid.UUID) ([]Invite, error) {
	rows, err := db.read.QueryContext(ctx, "SELECT "+inviteColumns+" FROM invites WHERE user_id = ? ORDER BY created_at DESC, id DESC",
		by.String())
	if err != nil {
		return nil, fmt.Errorf("invites: %w", err)
	}
	defer func() { _ = rows.Close() }()
	out := []Invite{}
	for rows.Next() {
		inv, err := scanInvite(rows.Scan)
		if err != nil {
			return nil, fmt.Errorf("invites: %w", err)
		}
		out = append(out, inv)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("invites: %w", err)
	}
	return out, nil
}

// DeleteInvite revokes the invite with id the admin by made, and reports
// whether there was one.
func (db *DB) DeleteInvite(ctx context.Context, by, id uuid.UUID) (bool, error) {
	var deleted bool
	err := db.WriteTx(ctx, func(tx *sql.Tx) error {
		res, err := tx.ExecContext(ctx, "DELETE FROM invites WHERE id = ? AND user_id = ?", id.String(), by.String())
		if err != nil {
			return err
		}
		n, err := res.RowsAffected()
		deleted = n > 0
		return err
	})
	if err != nil {
		return false, fmt.Errorf("delete invite: %w", err)
	}
	return deleted, nil
}

// InviteByToken returns the invite stored under tokenHash, or
// ErrNotFound when there is none or it has expired by now.
func (db *DB) InviteByToken(ctx context.Context, tokenHash string, now time.Time) (Invite, error) {
	row := db.read.QueryRowContext(ctx, "SELECT "+inviteColumns+" FROM invites WHERE token_hash = ? AND expires_at > ?",
		tokenHash, formatTime(now))
	inv, err := scanInvite(row.Scan)
	if errors.Is(err, sql.ErrNoRows) {
		return Invite{}, ErrNotFound
	}
	if err != nil {
		return Invite{}, fmt.Errorf("invite: %w", err)
	}
	return inv, nil
}

// Acceptance is what an invite's link makes: the user, with the invite's
// address and role, their password's hash ("" for none) or the identity
// they signed in with (nil for none), and their first session.
type Acceptance struct {
	User         user.User
	PasswordHash string
	Identity     *Identity
	Session      Session
}

// AcceptInvite makes a's user by the invite stored under tokenHash, with
// the invite's address and role, links a's identity, starts a's session
// and deletes the invite, all at once; ErrNotFound when the invite is
// gone or has expired by now, ErrEmailTaken when a user has its address,
// and ErrAlreadyLinked when the identity is another user's. It returns
// the user made.
func (db *DB) AcceptInvite(ctx context.Context, tokenHash string, a Acceptance, now time.Time) (user.User, error) {
	u := a.User
	err := db.WriteTx(ctx, func(tx *sql.Tx) error {
		inv, err := scanInvite(tx.QueryRowContext(ctx, "SELECT "+inviteColumns+" FROM invites WHERE token_hash = ? AND expires_at > ?",
			tokenHash, formatTime(now)).Scan)
		if errors.Is(err, sql.ErrNoRows) {
			return ErrNotFound
		} else if err != nil {
			return err
		}
		u.Email, u.Role = inv.Email, inv.Role
		if err := u.Validate(); err != nil {
			return fmt.Errorf("the user: %w", err)
		}
		var taken bool
		if err := tx.QueryRowContext(ctx, "SELECT EXISTS (SELECT 1 FROM users WHERE email = ?)", u.Email).Scan(&taken); err != nil {
			return err
		}
		if taken {
			return ErrEmailTaken
		}
		hash := sql.NullString{String: a.PasswordHash, Valid: a.PasswordHash != ""}
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO users (id, email, name, role, password_hash, locale, timezone, day_start_hour, created_at, updated_at)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			u.ID.String(), u.Email, u.Name, string(u.Role), hash, u.Locale, u.TimeZone, u.DayStartHour,
			formatTime(u.CreatedAt), formatTime(u.UpdatedAt)); err != nil {
			return err
		}
		if a.Identity != nil {
			var linked bool
			if err := tx.QueryRowContext(ctx, "SELECT EXISTS (SELECT 1 FROM user_identities WHERE issuer = ? AND subject = ?)",
				a.Identity.Issuer, a.Identity.Subject).Scan(&linked); err != nil {
				return err
			}
			if linked {
				return ErrAlreadyLinked
			}
			if _, err := tx.ExecContext(ctx, "INSERT INTO user_identities (issuer, subject, user_id, created_at) VALUES (?, ?, ?, ?)",
				a.Identity.Issuer, a.Identity.Subject, u.ID.String(), formatTime(now)); err != nil {
				return err
			}
		}
		if err := createSession(ctx, tx, a.Session); err != nil {
			return err
		}
		_, err = tx.ExecContext(ctx, "DELETE FROM invites WHERE id = ?", inv.ID.String())
		return err
	})
	switch {
	case errors.Is(err, ErrNotFound), errors.Is(err, ErrEmailTaken), errors.Is(err, ErrAlreadyLinked):
		return user.User{}, err
	case err != nil:
		return user.User{}, fmt.Errorf("accept invite: %w", err)
	}
	return u, nil
}
