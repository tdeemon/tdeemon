-- Strong's export is no source of workouts any more: Apple Health holds
-- every workout, Strong's and a watch's, and Health Auto Export sends them
-- (source 'hae'). The days Strong's export gave stay training days, as
-- days of Apple Health's, without a day twice. The table is rebuilt for
-- its CHECK of sources; no view or trigger uses it.
CREATE TABLE workout_days_new (
    user_id TEXT NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    source TEXT NOT NULL CHECK (source IN ('hae')),
    day TEXT NOT NULL CHECK (day GLOB '[0-9][0-9][0-9][0-9]-[0-1][0-9]-[0-3][0-9]'),
    PRIMARY KEY (user_id, source, day)
) STRICT;

INSERT INTO workout_days_new (user_id, source, day)
SELECT DISTINCT
    user_id,
    'hae' AS source,
    day
FROM workout_days;

DROP TABLE workout_days;
ALTER TABLE workout_days_new RENAME TO workout_days;
