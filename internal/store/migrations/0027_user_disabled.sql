-- When the admin deactivated the user: they cannot sign in, their
-- sessions went and their API tokens are refused, and the scheduler
-- skips them; their data stay. NULL for an active user.
ALTER TABLE users ADD COLUMN disabled_at TEXT CHECK (disabled_at <> '');
