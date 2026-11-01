-- An admin's invitation of a new user: a link that makes one account for
-- its address, once, before it expires. The link's token is shown once;
-- the row keeps only its SHA-256. The account made, the row goes.
CREATE TABLE invites (
    -- A UUIDv7: the handle the admin revokes it by.
    id TEXT PRIMARY KEY CHECK (length(id) = 36),
    -- The hex SHA-256 of the link's token.
    token_hash TEXT NOT NULL CHECK (length(token_hash) = 64),
    -- The admin who made it, whose invites go with them.
    user_id TEXT NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    -- The address the account will sign in with, normalized.
    email TEXT NOT NULL CHECK (email <> ''),
    role TEXT NOT NULL CHECK (role IN ('admin', 'user')),
    created_at TEXT NOT NULL,
    expires_at TEXT NOT NULL
) STRICT;

CREATE UNIQUE INDEX invites_token_hash ON invites (token_hash);

-- One invite an address: a new one replaces the one before.
CREATE UNIQUE INDEX invites_email ON invites (email);

CREATE INDEX invites_user_id ON invites (user_id);
