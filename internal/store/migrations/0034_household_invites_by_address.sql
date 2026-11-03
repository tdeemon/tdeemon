-- An invite into a household names the address it was made to, not a
-- user: one waits for whoever has the address, now or once they have an
-- account, and an address of no user is invited as any other. The rows
-- carry the address of the user each invited.

CREATE TABLE household_invites_new (
    -- A UUIDv7 the server makes.
    id TEXT PRIMARY KEY CHECK (length(id) = 36),
    user_id TEXT NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    -- The address invited, normalized as a user's is (lower case, NFC):
    -- the user of it answers.
    email TEXT NOT NULL CHECK (email <> ''),
    created_at TEXT NOT NULL,
    expires_at TEXT NOT NULL,
    UNIQUE (user_id, email)
) STRICT;

INSERT INTO household_invites_new (id, user_id, email, created_at, expires_at)
SELECT
    i.id,
    i.user_id,
    u.email,
    i.created_at,
    i.expires_at
FROM household_invites AS i
INNER JOIN users AS u ON i.invited_id = u.id;

DROP TABLE household_invites;

ALTER TABLE household_invites_new RENAME TO household_invites;

CREATE INDEX household_invites_email ON household_invites (email);
