-- On its own, outside sqlfluff (see .sqlfluffignore): its SQLite dialect
-- cannot parse the arguments of an FTS5 table.
--
-- Search over the catalog's names and brands, as foods_fts does over a
-- user's foods (a đ indexed as d and dj too, searchText in foods.go);
-- no aliases, which are a user's own words. The store keeps it in step
-- with catalog_foods in the same transactions, and the trigger removes a
-- row's words however the row goes.
CREATE VIRTUAL TABLE catalog_fts USING fts5 (
    catalog_id UNINDEXED,
    name,
    brand,
    tokenize = 'unicode61 remove_diacritics 2'
);

INSERT INTO catalog_fts (catalog_fts, rank) VALUES ('secure-delete', 1);

CREATE TRIGGER catalog_fts_delete AFTER DELETE ON catalog_foods
BEGIN
    DELETE FROM catalog_fts WHERE catalog_id = old.id;
END;

-- The rows 0030 made, each text with a đ in it as it is, with d and with
-- dj in its place.
INSERT INTO catalog_fts (catalog_id, name, brand)
SELECT
    id,
    CASE
        WHEN instr(name, 'đ') > 0 OR instr(name, 'Đ') > 0
            THEN
                name || char(10)
                || replace(replace(name, 'đ', 'd'), 'Đ', 'D') || char(10)
                || replace(replace(name, 'đ', 'dj'), 'Đ', 'Dj')
        ELSE name
    END,
    CASE
        WHEN instr(brand, 'đ') > 0 OR instr(brand, 'Đ') > 0
            THEN
                brand || char(10)
                || replace(replace(brand, 'đ', 'd'), 'Đ', 'D') || char(10)
                || replace(replace(brand, 'đ', 'dj'), 'Đ', 'Dj')
        ELSE brand
    END
FROM catalog_foods;
