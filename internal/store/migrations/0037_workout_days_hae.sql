-- The days of workouts get a second source: Health Auto Export's
-- workouts (Apple Health's, which Strong and a watch write), beside
-- Strong's export. A day of a workout from either source is a training
-- day, unless the check-in's toggle says otherwise (plan.TypeOf). The
-- table is rebuilt for its CHECK of sources; no view or trigger uses it.
CREATE TABLE workout_days_new (
    user_id TEXT NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    source TEXT NOT NULL CHECK (source IN ('strong', 'hae')),
    day TEXT NOT NULL CHECK (day GLOB '[0-9][0-9][0-9][0-9]-[0-1][0-9]-[0-3][0-9]'),
    PRIMARY KEY (user_id, source, day)
) STRICT;

INSERT INTO workout_days_new (user_id, source, day)
SELECT
    user_id,
    source,
    day
FROM workout_days;

DROP TABLE workout_days;
ALTER TABLE workout_days_new RENAME TO workout_days;
