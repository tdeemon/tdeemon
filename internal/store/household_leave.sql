-- Takes the user :user out of their household (story 408): what
-- LeaveHousehold does, but its event, and what an admin runs to take a
-- member out by hand (docs/DESIGN.md). Every member keeps every food
-- and recipe they have, as it is.

-- The other members of the household, who stay.
CREATE TEMP TABLE leave_mates AS
SELECT m.user_id
FROM household_members AS m
INNER JOIN household_members AS me ON m.household_id = me.household_id
WHERE me.user_id = :user AND m.user_id != :user;

-- The member's seats of the household go: their foods and recipes are
-- ordinary ones of theirs. A seat of an explicit share (story 403)
-- stays, and so does a recipe they left.
DELETE FROM food_shares
WHERE user_id = :user AND household = 1 AND left_at IS NULL;

-- A recipe they left with their household (a share with it stopped)
-- stays theirs, left: its group's version moves on, so an undo of that
-- stop, of before the leave, never gives it back to them with the
-- household's.
UPDATE food_shares SET
    version = (
        SELECT max(v.version) + 1 FROM food_shares AS v
        WHERE v.share_id = food_shares.share_id
    )
WHERE
    left_at IS NULL
    AND share_id IN (
        SELECT l.share_id FROM food_shares AS l
        WHERE l.user_id = :user AND l.household = 1 AND l.left_at IS NOT NULL
    );

-- A group the others are in by the household's seats alone, with
-- someone outside them in it (the member who leaves, by an explicit
-- share, or a user outside the household): the others' seats move to a
-- group of their own, a new one each, and the group outside stays as
-- the explicit shares made it.
CREATE TEMP TABLE leave_moves AS
SELECT
    g.share_id AS old_id,
    lower(
        hex(randomblob(4)) || '-' || hex(randomblob(2)) || '-4'
        || substr(hex(randomblob(2)), 2) || '-'
        || substr('89AB', 1 + abs(random()) % 4, 1)
        || substr(hex(randomblob(2)), 2) || '-' || hex(randomblob(6))
    ) AS new_id
FROM (
    SELECT DISTINCT s.share_id
    FROM food_shares AS s
    WHERE
        s.left_at IS NULL
        AND s.user_id IN (SELECT lm.user_id FROM leave_mates AS lm)
        AND NOT EXISTS (
            SELECT 1 FROM food_shares AS e
            WHERE
                e.share_id = s.share_id
                AND e.left_at IS NULL
                AND e.household = 0
                AND e.user_id IN (SELECT lm.user_id FROM leave_mates AS lm)
        )
        AND EXISTS (
            SELECT 1 FROM food_shares AS o
            WHERE
                o.share_id = s.share_id
                AND o.left_at IS NULL
                AND o.user_id NOT IN (SELECT lm.user_id FROM leave_mates AS lm)
        )
) AS g;

UPDATE food_shares SET
    share_id = (
        SELECT mv.new_id FROM leave_moves AS mv
        WHERE mv.old_id = food_shares.share_id
    )
WHERE
    left_at IS NULL
    AND share_id IN (SELECT mv.old_id FROM leave_moves AS mv)
    AND user_id IN (SELECT lm.user_id FROM leave_mates AS lm);

-- A seat of the household's left alone in its group goes: of one member
-- left, which dissolves the household, every seat of theirs.
DELETE FROM food_shares
WHERE
    household = 1
    AND left_at IS NULL
    AND user_id IN (SELECT lm.user_id FROM leave_mates AS lm)
    AND NOT EXISTS (
        SELECT 1 FROM food_shares AS o
        WHERE
            o.share_id = food_shares.share_id
            AND o.user_id != food_shares.user_id
            AND o.left_at IS NULL
    );

-- The member's invites waiting go: they were the household's.
DELETE FROM household_invites
WHERE user_id = :user;

DELETE FROM household_members
WHERE user_id = :user;

-- A household of one member is dissolved.
DELETE FROM household_members
WHERE household_id IN (
    SELECT hm.household_id FROM household_members AS hm
    GROUP BY hm.household_id
    HAVING count(*) < 2
);

DELETE FROM households
WHERE id NOT IN (SELECT hm.household_id FROM household_members AS hm);

DROP TABLE leave_moves;

DROP TABLE leave_mates;
