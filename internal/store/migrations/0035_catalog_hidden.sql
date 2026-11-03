-- A row of the catalog is hidden by an admin's save alone: an admin's
-- food of the row saved of no declared values (an estimate) hides it
-- when no food that names it is declared, and a declared save shows it
-- again. Before, a row was hidden while every food that named it was of
-- no declared values, any user's, so a user who is no admin could hide
-- what every user finds. The rows hidden so are hidden still.

ALTER TABLE catalog_foods ADD COLUMN hidden INTEGER NOT NULL DEFAULT 0 CHECK (
    hidden IN (0, 1)
);

UPDATE catalog_foods AS c SET hidden = 1
WHERE
    EXISTS (
        SELECT 1 FROM foods AS l
        WHERE l.catalog_id = c.id
    )
    AND NOT EXISTS (
        SELECT 1 FROM foods AS l
        WHERE l.catalog_id = c.id AND l.source IN ('label', 'off', 'reference')
    );
