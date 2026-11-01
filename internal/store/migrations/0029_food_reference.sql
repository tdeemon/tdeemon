-- A food's values may come from a standard table (USDA, a national
-- food composition table): source 'reference', no estimate. The three
-- tables that keep a source are rebuilt for their CHECK, keeping the
-- rowid; foods' full-text trigger goes and comes back with it.
DROP TRIGGER foods_fts_delete;

CREATE TABLE foods_new (
    -- A UUIDv7, chosen by the client that first saved the food. Ids are
    -- per user (the primary key is the pair), so another user's food
    -- under the same id is no row of this user's, and nothing tells it
    -- is there.
    id TEXT NOT NULL CHECK (length(id) = 36),
    user_id TEXT NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    name TEXT NOT NULL CHECK (name <> ''),
    brand TEXT NOT NULL DEFAULT '',
    -- name and brand as food.Key compares them: the same name and brand
    -- are the same food (see foods_user_name_brand).
    name_key TEXT NOT NULL CHECK (name_key <> ''),
    brand_key TEXT NOT NULL DEFAULT '',
    -- What the nutrients refer to: 100 g, 100 ml or one portion.
    basis TEXT NOT NULL CHECK (basis IN ('100g', '100ml', 'portion')),
    kcal REAL NOT NULL CHECK (kcal >= 0),
    protein REAL NOT NULL CHECK (protein >= 0),
    carbs REAL NOT NULL CHECK (carbs >= 0),
    fat REAL NOT NULL CHECK (fat >= 0),
    -- Optional nutrients, in grams like the macronutrients; NULL when
    -- unknown.
    fiber REAL CHECK (fiber >= 0),
    saturated_fat REAL CHECK (saturated_fat >= 0),
    sugars REAL CHECK (sugars >= 0),
    salt REAL CHECK (salt >= 0),
    polyols REAL CHECK (polyols >= 0),
    alcohol REAL CHECK (alcohol >= 0),
    -- The weight of one portion of a food per portion, when known.
    portion_g REAL CHECK (portion_g IS NULL OR (basis = 'portion' AND portion_g > 0)),
    source TEXT NOT NULL CHECK (
        source IN ('label', 'estimate', 'reference', 'user', 'dish', 'recipe', 'off')
    ),
    note TEXT NOT NULL DEFAULT '',
    archived INTEGER NOT NULL DEFAULT 0 CHECK (archived IN (0, 1)),
    created_at TEXT NOT NULL,
    updated_at TEXT NOT NULL,
    -- The child tables' foreign keys name the user as well, so a child
    -- row can never belong to another user's food.
    PRIMARY KEY (user_id, id)
) STRICT;

INSERT INTO foods_new (
    rowid,
    id,
    user_id,
    name,
    brand,
    name_key,
    brand_key,
    basis,
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
    portion_g,
    source,
    note,
    archived,
    created_at,
    updated_at
)
SELECT
    rowid,
    id,
    user_id,
    name,
    brand,
    name_key,
    brand_key,
    basis,
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
    portion_g,
    source,
    note,
    archived,
    created_at,
    updated_at
FROM foods;

DROP TABLE foods;
ALTER TABLE foods_new RENAME TO foods;

CREATE UNIQUE INDEX foods_user_name_brand ON foods (user_id, name_key, brand_key);

CREATE TABLE meal_items_new (
    -- A UUIDv7 the client chose for a new item, or the store did; per
    -- user.
    id TEXT NOT NULL CHECK (length(id) = 36),
    meal_id TEXT NOT NULL,
    user_id TEXT NOT NULL,
    -- The item's place in the meal, from 0.
    position INTEGER NOT NULL CHECK (position >= 0),
    -- NULL for a one-off estimate.
    food_id TEXT,
    -- The food's name (with its brand) when logged, or the estimate's.
    name TEXT NOT NULL CHECK (name <> ''),
    -- The quantity as the client gave it: one of an amount with its unit,
    -- a number of a named serving, or a number of portions.
    quantity_amount REAL NOT NULL DEFAULT 0 CHECK (quantity_amount >= 0),
    quantity_unit TEXT NOT NULL DEFAULT '' CHECK (quantity_unit IN ('', 'g', 'ml')),
    quantity_servings REAL NOT NULL DEFAULT 0 CHECK (quantity_servings >= 0),
    quantity_serving TEXT NOT NULL DEFAULT '',
    quantity_portions REAL NOT NULL DEFAULT 0 CHECK (quantity_portions >= 0),
    -- The quantity in the food's unit, and the item's own nutrients.
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
    -- As foods.source.
    source TEXT NOT NULL CHECK (
        source IN ('label', 'estimate', 'reference', 'user', 'dish', 'recipe', 'off')
    ),
    note TEXT NOT NULL DEFAULT '', sent_food TEXT CHECK (
        sent_food IS NULL OR (length(sent_food) = 64 AND food_id IS NOT NULL)
    ), brand TEXT NOT NULL DEFAULT '',
    PRIMARY KEY (user_id, id),
    FOREIGN KEY (user_id, meal_id) REFERENCES meals (user_id, id) ON DELETE CASCADE,
    -- Foods are archived, never deleted, while an item uses them.
    FOREIGN KEY (user_id, food_id) REFERENCES foods (user_id, id)
) STRICT;

INSERT INTO meal_items_new (
    rowid,
    id,
    meal_id,
    user_id,
    position,
    food_id,
    name,
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
    sent_food,
    brand
)
SELECT
    rowid,
    id,
    meal_id,
    user_id,
    position,
    food_id,
    name,
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
    sent_food,
    brand
FROM meal_items;

DROP TABLE meal_items;
ALTER TABLE meal_items_new RENAME TO meal_items;

CREATE UNIQUE INDEX meal_items_meal_position ON meal_items (user_id, meal_id, position);

CREATE INDEX meal_items_user_food ON meal_items (user_id, food_id);

CREATE TABLE recipe_items_new (
    recipe_id TEXT NOT NULL,
    user_id TEXT NOT NULL,
    -- The ingredient's place in the recipe, from 0.
    position INTEGER NOT NULL CHECK (position >= 0),
    food_id TEXT NOT NULL,
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
    note TEXT NOT NULL DEFAULT '', portion_grams REAL CHECK (portion_grams > 0),
    PRIMARY KEY (user_id, recipe_id, position),
    FOREIGN KEY (user_id, recipe_id) REFERENCES recipes (user_id, food_id) ON DELETE CASCADE,
    -- Foods are archived, never deleted, while a recipe uses them.
    FOREIGN KEY (user_id, food_id) REFERENCES foods (user_id, id)
) STRICT;

INSERT INTO recipe_items_new (
    rowid,
    recipe_id,
    user_id,
    position,
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
)
SELECT
    rowid,
    recipe_id,
    user_id,
    position,
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

CREATE INDEX recipe_items_user_food ON recipe_items (user_id, food_id);

CREATE TRIGGER foods_fts_delete AFTER DELETE ON foods
BEGIN
    DELETE FROM foods_fts
    WHERE food_id = old.id AND user_id = old.user_id;
END;
