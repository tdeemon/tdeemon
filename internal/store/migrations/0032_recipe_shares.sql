-- Shared recipes (story 403): a recipe shared with other users of the
-- instance is one recipe each of them has. Every member keeps a recipe
-- of their own (their food, recipes row and ingredients), so a search, a
-- meal's item and its foreign key stay the user's; a share group ties
-- those recipes together, and a save of one writes the shared fields
-- into the others. An ingredient may be a food of another member's
-- (recipe_items.food_user_id); when its owner's account goes, so does
-- the food, and the ingredient keeps its values.

-- The members of a share group, one row each: their seat in it (a
-- number, which the group's events and the members see of one another
-- rather than a user's or a food's id), their recipe (food_id), the
-- group's version (one more with each save, share or stop, the same in
-- every live row) and when they left (NULL while a member). A member who
-- left keeps their recipe for their meals, but picks it no more.
CREATE TABLE recipe_shares (
    share_id TEXT NOT NULL CHECK (length(share_id) = 36),
    seat INTEGER NOT NULL CHECK (seat >= 1),
    user_id TEXT NOT NULL,
    food_id TEXT NOT NULL,
    version INTEGER NOT NULL DEFAULT 1 CHECK (version >= 1),
    left_at TEXT,
    PRIMARY KEY (share_id, user_id),
    UNIQUE (share_id, seat),
    UNIQUE (user_id, food_id),
    FOREIGN KEY (user_id, food_id) REFERENCES recipes (user_id, food_id) ON DELETE CASCADE
) STRICT;

-- The ingredients, rebuilt so an ingredient's food may be another
-- member's: food_user_id is its owner (the recipe's user for their own
-- foods), and the food going (its owner's account deleted) leaves both
-- NULL, the ingredient keeping its values.
CREATE TABLE recipe_items_new (
    recipe_id TEXT NOT NULL,
    user_id TEXT NOT NULL,
    -- The ingredient's place in the recipe, from 0.
    position INTEGER NOT NULL CHECK (position >= 0),
    food_user_id TEXT,
    food_id TEXT,
    name TEXT NOT NULL CHECK (name <> ''),
    brand TEXT NOT NULL DEFAULT '',
    quantity_amount REAL NOT NULL DEFAULT 0 CHECK (quantity_amount >= 0),
    quantity_unit TEXT NOT NULL DEFAULT '' CHECK (quantity_unit IN ('', 'g', 'ml')),
    quantity_servings REAL NOT NULL DEFAULT 0 CHECK (quantity_servings >= 0),
    quantity_serving TEXT NOT NULL DEFAULT '',
    quantity_portions REAL NOT NULL DEFAULT 0 CHECK (quantity_portions >= 0),
    amount REAL NOT NULL CHECK (amount > 0),
    unit TEXT NOT NULL CHECK (unit IN ('g', 'ml', 'portion')),
    kcal REAL NOT NULL CHECK (kcal >= 0),
    protein REAL NOT NULL CHECK (protein >= 0),
    carbs REAL NOT NULL CHECK (carbs >= 0),
    fat REAL NOT NULL CHECK (fat >= 0),
    fiber REAL CHECK (fiber >= 0),
    saturated_fat REAL CHECK (saturated_fat >= 0),
    sugars REAL CHECK (sugars >= 0),
    salt REAL CHECK (salt >= 0),
    polyols REAL CHECK (polyols >= 0),
    alcohol REAL CHECK (alcohol >= 0),
    source TEXT NOT NULL CHECK (
        source IN ('label', 'estimate', 'reference', 'user', 'dish', 'recipe', 'off')
    ),
    note TEXT NOT NULL DEFAULT '',
    portion_grams REAL CHECK (portion_grams > 0),
    PRIMARY KEY (user_id, recipe_id, position),
    CHECK ((food_user_id IS NULL) = (food_id IS NULL)),
    FOREIGN KEY (user_id, recipe_id) REFERENCES recipes (user_id, food_id) ON DELETE CASCADE,
    -- Foods are archived, never deleted, while a recipe uses them, but
    -- for their owner's account's deletion.
    FOREIGN KEY (food_user_id, food_id) REFERENCES foods (user_id, id) ON DELETE SET NULL
) STRICT;

INSERT INTO recipe_items_new (
    rowid, recipe_id, user_id, position, food_user_id, food_id, name, brand,
    quantity_amount, quantity_unit, quantity_servings, quantity_serving,
    quantity_portions, amount, unit, kcal, protein, carbs, fat, fiber,
    saturated_fat, sugars, salt, polyols, alcohol, source, note, portion_grams
)
SELECT
    rowid,
    recipe_id,
    user_id,
    position,
    user_id AS food_user_id,
    food_id,
    name,
    brand,
    quantity_amount,
    quantity_unit,
    quantity_servings,
    quantity_serving,
    quantity_portions,
    amount,
    unit,
    kcal,
    protein,
    carbs,
    fat,
    fiber,
    saturated_fat,
    sugars,
    salt,
    polyols,
    alcohol,
    source,
    note,
    portion_grams
FROM recipe_items;

DROP TABLE recipe_items;
ALTER TABLE recipe_items_new RENAME TO recipe_items;

CREATE INDEX recipe_items_food ON recipe_items (food_user_id, food_id);

-- The events, rebuilt for a share and its stop.
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
        'recipe.share', 'recipe.unshare'
    )),
    -- What changed: a food's (a recipe's too, which a share or its stop
    -- names by the user's own recipe), a meal's or a proposal's
    -- id, a day, a day and a metric, a phase's start, a target's first
    -- day, "profile", "plan", "limits", "notes", "autopilot", a review's
    -- day ("review/YYYY-MM-DD") or a source of workouts, or for an undo
    -- the event it took back.
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
