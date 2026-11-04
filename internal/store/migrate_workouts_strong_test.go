package store

import (
	"reflect"
	"testing"
)

// 0038 keeps the days Strong's export gave, as days of Apple Health's,
// without a day twice where both had it.
func TestMigrationWithoutStrongKeepsItsDays(t *testing.T) {
	t.Parallel()

	db := openEmpty(t)
	if err := db.migrate(t.Context(), upTo(t, "0037_workout_days_hae.sql"), discard()); err != nil {
		t.Fatalf("migrate to 0037: %v", err)
	}
	const user = "0199f0a0-0000-7000-8000-000000000001"
	exec := func(q string, args ...any) {
		t.Helper()
		if _, err := db.write.ExecContext(t.Context(), q, args...); err != nil {
			t.Fatalf("%s: %v", q, err)
		}
	}
	exec(`INSERT INTO users (id, email, name, role, locale, timezone, day_start_hour, created_at, updated_at)
		VALUES (?, 'ana@example.com', 'Ana', 'user', 'en', 'UTC', 4, '2026-10-21T08:00:00Z', '2026-10-21T08:00:00Z')`, user)
	exec(`INSERT INTO workout_days (user_id, source, day) VALUES
		(?, 'strong', '2026-10-01'), (?, 'strong', '2026-10-03'), (?, 'hae', '2026-10-03'), (?, 'hae', '2026-10-05')`, user, user, user, user)

	if err := db.migrate(t.Context(), upTo(t, "0038_workout_days_without_strong.sql"), discard()); err != nil {
		t.Fatalf("migrate to 0038: %v", err)
	}
	rows, err := db.read.QueryContext(t.Context(), `SELECT source, day FROM workout_days WHERE user_id = ? ORDER BY day`, user)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = rows.Close() }()
	var got []string
	for rows.Next() {
		var source, d string
		if err := rows.Scan(&source, &d); err != nil {
			t.Fatal(err)
		}
		got = append(got, source+" "+d)
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	if want := []string{"hae 2026-10-01", "hae 2026-10-03", "hae 2026-10-05"}; !reflect.DeepEqual(got, want) {
		t.Errorf("days after the migration = %v, want %v", got, want)
	}
	if _, err := db.write.ExecContext(t.Context(), `INSERT INTO workout_days (user_id, source, day) VALUES (?, 'strong', '2026-10-07')`, user); err == nil {
		t.Error("a day of Strong's went in after the migration")
	}
}
