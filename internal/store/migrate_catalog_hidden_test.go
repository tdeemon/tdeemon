package store

import "testing"

// 0035 keeps a row hidden that was: one whose foods are all of no
// declared values now (Ivo's apple made an estimate), while a row of a
// declared food stays shown.
func TestMigrationOfTheCatalogsHiddenRows(t *testing.T) {
	t.Parallel()

	db := openEmpty(t)
	if err := db.migrate(t.Context(), upTo(t, "0029_food_reference.sql"), discard()); err != nil {
		t.Fatalf("migrate to 0029: %v", err)
	}
	seedCatalog(t, db)
	if err := db.migrate(t.Context(), upTo(t, "0034_household_invites_by_address.sql"), discard()); err != nil {
		t.Fatalf("migrate to 0034: %v", err)
	}
	if _, err := db.write.ExecContext(t.Context(), "UPDATE foods SET source = 'estimate' WHERE id = ?", catalogSeeds[2].id); err != nil {
		t.Fatal(err)
	}
	if err := db.migrate(t.Context(), upTo(t, "9999"), discard()); err != nil {
		t.Fatalf("migrate from 0034: %v", err)
	}
	for name, want := range map[string]int{"jabuka": 1, "bijelo grožđe": 0} {
		var hidden int
		if err := db.write.QueryRowContext(t.Context(), "SELECT hidden FROM catalog_foods WHERE name_key = ?", name).Scan(&hidden); err != nil || hidden != want {
			t.Errorf("the %s's row hidden = %d, %v; want %d", name, hidden, err, want)
		}
	}
}
