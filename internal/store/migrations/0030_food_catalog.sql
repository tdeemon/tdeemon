-- The instance's catalog of foods: what every user's declared foods (a
-- label's, Open Food Facts', a food table's) come to, one row a product,
-- for every user's search to find. It is no user's: no row names one,
-- and a user's deletion leaves it. A user's food that went into it, or
-- was copied from it, names its row in foods.catalog_id; the store keeps
-- a row in step with the latest save of a food that names it, which
-- wins over the ones before.
CREATE TABLE catalog_foods (
    -- A UUIDv7 the server makes, no user's food's.
    id TEXT PRIMARY KEY CHECK (length(id) = 36),
    name TEXT NOT NULL CHECK (name <> ''),
    brand TEXT NOT NULL DEFAULT '',
    -- As foods': the same name and brand are one product.
    name_key TEXT NOT NULL CHECK (name_key <> ''),
    brand_key TEXT NOT NULL DEFAULT '',
    basis TEXT NOT NULL CHECK (basis IN ('100g', '100ml', 'portion')),
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
    portion_g REAL CHECK (portion_g IS NULL OR (basis = 'portion' AND portion_g > 0)),
    -- Only declared values: never an estimate, a user's word or a dish.
    source TEXT NOT NULL CHECK (source IN ('label', 'reference', 'off')),
    created_at TEXT NOT NULL,
    updated_at TEXT NOT NULL,
    -- One more with each save; an undo puts back the version before
    -- with the row, so a save's undo tells whether another came since.
    version INTEGER NOT NULL DEFAULT 1 CHECK (version >= 1),
    -- For a row this migration made, until a save of it: the event of
    -- the save before the catalog it was made of, whose undo puts it
    -- back (catalog_saves records the saves after).
    legacy_save TEXT CHECK (legacy_save IS NULL OR length(legacy_save) = 36)
) STRICT;

CREATE UNIQUE INDEX catalog_foods_name_brand ON catalog_foods (
    name_key, brand_key
);

CREATE TABLE catalog_barcodes (
    -- A barcode is one product's.
    code TEXT PRIMARY KEY CHECK (
        length(code) IN (8, 13, 14) AND code NOT GLOB '*[^0-9]*'
    ),
    catalog_id TEXT NOT NULL REFERENCES catalog_foods (id) ON DELETE CASCADE
) STRICT;

CREATE INDEX catalog_barcodes_food ON catalog_barcodes (catalog_id);

CREATE TABLE catalog_servings (
    catalog_id TEXT NOT NULL REFERENCES catalog_foods (id) ON DELETE CASCADE,
    name TEXT NOT NULL CHECK (name <> ''),
    name_key TEXT NOT NULL CHECK (name_key <> ''),
    amount REAL NOT NULL CHECK (amount > 0),
    PRIMARY KEY (catalog_id, name_key)
) STRICT;

-- What a user's save of a food did to the catalog, for its undo: the
-- row as it was (NULL when the save made it), the version the save left
-- it at, and the row the food named before. The user's, with the event.
CREATE TABLE catalog_saves (
    user_id TEXT NOT NULL,
    event_id TEXT NOT NULL,
    catalog_id TEXT NOT NULL REFERENCES catalog_foods (id) ON DELETE CASCADE,
    version INTEGER NOT NULL CHECK (version >= 1),
    before_state TEXT CHECK (before_state IS NULL OR json_valid(before_state)),
    link_before TEXT CHECK (link_before IS NULL OR length(link_before) = 36),
    PRIMARY KEY (user_id, event_id),
    FOREIGN KEY (user_id, event_id) REFERENCES events (user_id, id) ON DELETE CASCADE
) STRICT;

-- The catalog's row the food went into, or was copied from; NULL for
-- none. The row may have changed since: a food keeps its own values.
ALTER TABLE foods ADD COLUMN catalog_id TEXT REFERENCES catalog_foods (
    id
) ON DELETE SET NULL;

CREATE INDEX foods_catalog ON foods (catalog_id)
WHERE catalog_id IS NOT NULL;

-- The declared foods saved before it, in use: of the foods of one name
-- and brand, every user's, the one saved last, under a new UUIDv7 (its
-- time this one's, the rest random).
CREATE TEMP TABLE catalog_pick AS
WITH declared AS (
    SELECT
        f.user_id,
        f.id,
        f.name_key,
        f.brand_key,
        row_number() OVER (
            PARTITION BY f.name_key, f.brand_key
            ORDER BY f.updated_at DESC, f.id DESC
        ) AS n
    FROM foods AS f
    WHERE f.archived = 0 AND f.source IN ('label', 'reference', 'off')
),

stamp AS (
    SELECT printf('%012x', cast(unixepoch('subsec') * 1000 AS INTEGER)) AS ms
)

SELECT
    d.user_id,
    d.id AS food_id,
    d.name_key,
    d.brand_key,
    lower(
        substr(s.ms, 1, 8) || '-' || substr(s.ms, 9, 4) || '-7'
        || substr(hex(randomblob(2)), 2, 3) || '-'
        || substr('89ab', 1 + abs(random()) % 4, 1)
        || substr(hex(randomblob(2)), 2, 3) || '-' || hex(randomblob(6))
    ) AS catalog_id
FROM declared AS d CROSS JOIN stamp AS s
WHERE d.n = 1;

INSERT INTO catalog_foods (
    id, name, brand, name_key, brand_key, basis, kcal, protein, carbs, fat,
    fiber, saturated_fat, sugars, salt, polyols, alcohol, portion_g, source,
    created_at, updated_at, legacy_save
)
SELECT
    p.catalog_id,
    f.name,
    f.brand,
    f.name_key,
    f.brand_key,
    f.basis,
    f.kcal,
    f.protein,
    f.carbs,
    f.fat,
    f.fiber,
    f.saturated_fat,
    f.sugars,
    f.salt,
    f.polyols,
    f.alcohol,
    f.portion_g,
    f.source,
    f.updated_at AS created_at,
    f.updated_at,
    (
        SELECT e.id FROM events AS e
        WHERE
            e.user_id = p.user_id AND e.entity = p.food_id
            AND e.operation = 'food.save' AND e.undone_by IS NULL
        ORDER BY e.id DESC LIMIT 1
    ) AS legacy_save
FROM catalog_pick AS p
INNER JOIN foods AS f ON p.user_id = f.user_id AND p.food_id = f.id;

INSERT INTO catalog_servings (catalog_id, name, name_key, amount)
SELECT
    p.catalog_id,
    s.name,
    s.name_key,
    s.amount
FROM catalog_pick AS p
INNER JOIN food_servings AS s ON p.user_id = s.user_id AND p.food_id = s.food_id;

-- Every declared food in use names the row of its name and brand.
UPDATE foods SET
    catalog_id = (
        SELECT c.id FROM catalog_foods AS c
        WHERE c.name_key = foods.name_key AND c.brand_key = foods.brand_key
    )
WHERE archived = 0 AND source IN ('label', 'reference', 'off');

-- Their barcodes go to their rows, in the order of their foods' saves,
-- so the newest have the highest rowids, which the store keeps; a
-- barcode stays the product's that had it first, as the store keeps it:
-- of two names, the row of the food saved first.
INSERT OR IGNORE INTO catalog_barcodes (code, catalog_id)
SELECT
    b.code,
    f.catalog_id
FROM food_barcodes AS b
INNER JOIN foods AS f ON b.user_id = f.user_id AND b.food_id = f.id
WHERE f.catalog_id IS NOT NULL
ORDER BY f.updated_at, f.id, b.rowid;

-- A row keeps the newest ten, as a food does.
DELETE FROM catalog_barcodes
WHERE rowid NOT IN (
    SELECT ranked.r FROM (
        SELECT
            b.rowid AS r,
            row_number() OVER (
                PARTITION BY b.catalog_id ORDER BY b.rowid DESC
            ) AS n
        FROM catalog_barcodes AS b
    ) AS ranked
    WHERE ranked.n <= 10
);
