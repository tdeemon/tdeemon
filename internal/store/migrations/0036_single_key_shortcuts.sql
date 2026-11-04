-- A user may turn off the web app's shortcuts of one key (WCAG 2.1.4,
-- for speech input): 1 is off. Every user has them on, as before.

ALTER TABLE users ADD COLUMN no_single_key_shortcuts INTEGER NOT NULL DEFAULT 0 CHECK (
    no_single_key_shortcuts IN (0, 1)
);
