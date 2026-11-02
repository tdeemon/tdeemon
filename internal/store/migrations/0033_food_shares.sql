-- A household (story 408): users who keep one kitchen share every food
-- and recipe of theirs. Each member keeps a food of their own, as a
-- shared recipe's members do (story 403), and a share group ties those
-- foods together; recipe_shares becomes food_shares, a group of any
-- food, its seats each the household's or an explicit share's. No one is
-- in a household yet: the shares there are stay explicit ones.

-- The members of a share group, one row each, as recipe_shares had them:
-- their seat in it, their food (food_id, a recipe's or a plain food's),
-- the group's version and when they left (NULL while a member). A seat
-- is the household's (household = 1), which a leave of the household
-- takes away, or an explicit share's (0), of a recipe shared with a
-- user (403), which a stop of that share ends.
CREATE TABLE food_shares (
    share_id TEXT NOT NULL CHECK (length(share_id) = 36),
    seat INTEGER NOT NULL CHECK (seat >= 1),
    user_id TEXT NOT NULL,
    food_id TEXT NOT NULL,
    version INTEGER NOT NULL DEFAULT 1 CHECK (version >= 1),
    left_at TEXT,
    household INTEGER NOT NULL DEFAULT 0 CHECK (household IN (0, 1)),
    PRIMARY KEY (share_id, user_id),
    UNIQUE (share_id, seat),
    UNIQUE (user_id, food_id),
    FOREIGN KEY (user_id, food_id) REFERENCES foods (user_id, id) ON DELETE CASCADE
) STRICT;

INSERT INTO food_shares (rowid, share_id, seat, user_id, food_id, version, left_at, household)
SELECT
    rowid,
    share_id,
    seat,
    user_id,
    food_id,
    version,
    left_at,
    0 AS household
FROM recipe_shares;

DROP TABLE recipe_shares;

-- A household: two or more users who share their foods and recipes. It
-- holds nothing of a member's; its members are in household_members.
CREATE TABLE households (
    -- A UUIDv7 the server makes.
    id TEXT PRIMARY KEY CHECK (length(id) = 36),
    created_at TEXT NOT NULL
) STRICT;

-- The members of a household, one row a user: a user is in one
-- household at most. A user's deletion takes their row; the store
-- leaves the household first, and dissolves one left with one member.
CREATE TABLE household_members (
    user_id TEXT PRIMARY KEY REFERENCES users (id) ON DELETE CASCADE,
    household_id TEXT NOT NULL REFERENCES households (id) ON DELETE CASCADE,
    joined_at TEXT NOT NULL
) STRICT;

CREATE INDEX household_members_household ON household_members (household_id);

-- The invites into a household waiting for an answer: who invited
-- (user_id) whom (invited_id). The inviting side is the inviter's
-- household when the invite is accepted, or the inviter alone, with whom
-- the accept makes one. A revoke, a decline or an accept deletes the
-- row; one waits per inviter and invited user, and either's deletion
-- takes it.
CREATE TABLE household_invites (
    -- A UUIDv7 the server makes.
    id TEXT PRIMARY KEY CHECK (length(id) = 36),
    user_id TEXT NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    invited_id TEXT NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    created_at TEXT NOT NULL,
    expires_at TEXT NOT NULL,
    UNIQUE (user_id, invited_id),
    CHECK (invited_id <> user_id)
) STRICT;

CREATE INDEX household_invites_invited ON household_invites (invited_id);

-- The events, rebuilt for joining a household and leaving it.
CREATE TABLE events_new (
    -- A UUIDv7 the store makes: the order of the ids is the order of
    -- the writes.
    id TEXT NOT NULL CHECK (length(id) = 36),
    user_id TEXT NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    at TEXT NOT NULL,
    -- The user, an import, or the autopilot, whose writes undo leaves.
    actor TEXT NOT NULL CHECK (actor IN ('user', 'import', 'autopilot')),
    client TEXT NOT NULL CHECK (client <> ''),
    -- The AI model that acted for the user, or ''.
    model TEXT NOT NULL DEFAULT '',
    operation TEXT NOT NULL CHECK (operation IN (
        'food.save', 'food.archive', 'recipe.save', 'meal.create', 'meal.replace', 'meal.delete',
        'day.exclude', 'day.include', 'metrics.merge', 'metric.set', 'metric.delete', 'undo',
        'profile.set', 'phase.set', 'phase.delete', 'target.set', 'target.delete',
        'day.training', 'plan.weekdays', 'limits.set', 'notes.set', 'workouts.set',
        'day.checkin', 'proposal.create', 'proposal.status', 'autopilot.set', 'autopilot.review',
        'recipe.share', 'recipe.unshare', 'household.join', 'household.leave'
    )),
    -- What changed: a food's (a recipe's too, which a share or its stop
    -- names by the user's own recipe), a meal's or a proposal's
    -- id, a day, a day and a metric, a phase's start, a target's first
    -- day, "profile", "plan", "limits", "notes", "autopilot", "household",
    -- a review's day ("review/YYYY-MM-DD") or a source of workouts, or
    -- for an undo the event it took back.
    entity TEXT NOT NULL CHECK (entity <> ''),
    -- The entity's state as JSON; NULL where there was none.
    before_state TEXT CHECK (before_state IS NULL OR json_valid(before_state)),
    after_state TEXT CHECK (after_state IS NULL OR json_valid(after_state)),
    -- The undo that took the event back.
    undone_by TEXT CHECK (undone_by IS NULL OR length(undone_by) = 36),
    -- 1 when undo passed over the event: the entity changed since, or
    -- the change could not be read or put back.
    stale INTEGER NOT NULL DEFAULT 0 CHECK (stale IN (0, 1)),
    -- The id a client made for an undo, on the change that undo settled,
    -- or on the undo's own event when it found nothing to undo.
    undo_request TEXT CHECK (undo_request IS NULL OR length(undo_request) = 36),
    -- The change's event a write is part of: undo takes it back with
    -- that change, never on its own.
    part_of TEXT CHECK (part_of IS NULL OR length(part_of) = 36),
    PRIMARY KEY (user_id, id)
) STRICT;

INSERT INTO events_new (
    rowid, id, user_id, at, actor, client, model, operation, entity,
    before_state, after_state, undone_by, stale, undo_request, part_of
)
SELECT
    rowid,
    id,
    user_id,
    at,
    actor,
    client,
    model,
    operation,
    entity,
    before_state,
    after_state,
    undone_by,
    stale,
    undo_request,
    part_of
FROM events;

DROP TABLE events;
ALTER TABLE events_new RENAME TO events;

CREATE INDEX events_user ON events (user_id);

CREATE INDEX events_user_entity ON events (user_id, entity);

CREATE INDEX events_user_part_of ON events (user_id, part_of)
WHERE part_of IS NOT NULL;

CREATE UNIQUE INDEX events_user_undo_request ON events (user_id, undo_request)
WHERE undo_request IS NOT NULL;
