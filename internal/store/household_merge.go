package store

import (
	"cmp"
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"maps"
	"reflect"
	"slices"
	"time"
	"uuid"

	"github.com/tdeemon/tdeemon/internal/domain/food"
	"github.com/tdeemon/tdeemon/internal/domain/meal"
	"github.com/tdeemon/tdeemon/internal/domain/recipe"
)

// The merge of two kitchens (story 408), when a user (U) accepts an
// invite into a household: the inviting side's foods (H, every member's
// of the inviter's household, or the inviter's alone) and U's are
// matched, each match becomes one food, and each food without one is
// copied to the side that lacks it. After it, every food of every member
// has a copy at every member, all of one share group, as a household
// keeps them (household_foods.go).
//
//  1. Matching: a food of H's and one of U's of the same kind (a recipe,
//     or a food that is none) match by the same name and brand
//     (food.Key), else the same barcode, else the same catalog row; each
//     matches one food at most, an earlier rule before a later one, and
//     among several at one rule the most recently saved first. A recipe
//     a member of H's and U share already (story 403) is one before any.
//  2. A match takes H's shared fields: U's row keeps its id (its meals
//     and recipes name it), its note, aliases and archived; the barcodes
//     are both sides', but one another food has, and as many as a food
//     may have (food.MaxBarcodes), H's first.
//  3. A food of one side without a match is copied to each member of the
//     other: its shared fields, no note or aliases, archived when every
//     member who has it has it archived (withdrawn, it comes so).
//  4. A recipe's ingredients are H's in a match; each member's recipe
//     names each ingredient's food by their own copy of it.
//  5. A recipe of U's in a share group with users outside keeps it, the
//     household joining it; matched with one of H's, U's leaves that
//     group (its other members keep theirs) for H's.
//  6. A name or a barcode left clashing goes from U's food: the name gets
//     a number (" (2)", the lowest free), the barcode is dropped.
//
// A recipe a member left the share of (notWithdrawn) is no food of the
// kitchen's: it stays theirs, for their meals, but its name and barcodes
// clash all the same, and it gives way as U's foods do; one of a group
// the merge brings them into again is their copy, brought back.
//
// The merge is planned first (planMerge), and the plan, of which its
// report and fingerprint are made, then written (apply): a preview is
// the same merge, rolled back.

// MergeCounts is a count of foods that are no recipes, and of recipes.
type MergeCounts struct {
	Foods   int `json:"foods"`
	Recipes int `json:"recipes"`
}

// MergeReport is what a merge does: how many foods of the invited
// user's become one with the household's (Merged), how many of the
// household's come to them (Received), how many of theirs go to the
// household (Given); and each food renamed, and each barcode dropped
// from a food, by its name.
type MergeReport struct {
	Merged   MergeCounts    `json:"merged"`
	Received MergeCounts    `json:"received"`
	Given    MergeCounts    `json:"given"`
	Renamed  []MergeRename  `json:"renamed,omitempty"`
	Dropped  []MergeBarcode `json:"dropped,omitempty"`
}

// MergeRename is a food a merge renames: its name before and after.
type MergeRename struct {
	From string `json:"from"`
	To   string `json:"to"`
}

// MergeBarcode is a barcode a merge drops from a food, which another
// food has: the food, by its name after the merge, and the barcode.
type MergeBarcode struct {
	Food string `json:"food"`
	Code string `json:"code"`
}

// kitchenFood is a food of one side's kitchen: its copies, by member,
// and the one whose shared fields it has (rep).
type kitchenFood struct {
	rep     shareMember
	food    food.Food
	recipe  *recipe.Recipe
	saved   string
	catalog string
	// group is the live share group of its copies, or "".
	group  string
	copies map[uuid.UUID]uuid.UUID
}

// leftFood is a recipe its member left the share of (group).
type leftFood struct {
	member shareMember
	group  string
	food   food.Food
	// changed tells a name or barcodes the merge changed.
	changed bool
}

// mergeItem is a food of the merged kitchen: H's food h, U's food u, or
// both, matched; its shared fields after the merge (food), and each
// member's copy of it.
type mergeItem struct {
	h, u *kitchenFood
	food food.Food
	// group is the share group of its copies, "" for a new one; leave is
	// U's group u leaves for it.
	group, leave string
	copies       map[uuid.UUID]uuid.UUID
	// back are the members whose copy is a recipe they left, brought
	// back; fresh the members whose copy the merge makes.
	back, fresh map[uuid.UUID]bool
	// own is each member's food as it was, but a fresh copy's.
	own map[uuid.UUID]food.Food
}

// base is the item's food of the side whose shared fields it takes.
func (it *mergeItem) base() *kitchenFood {
	if it.h != nil {
		return it.h
	}
	return it.u
}

// catalog is the catalog's row every copy of the item names, or "".
func (it *mergeItem) catalog() string { return it.base().catalog }

// mergePlan is a merge, planned: the members of side H and the user U
// who joins it, the merged kitchen's foods, and the recipes left of the
// members', as the merge leaves them.
type mergePlan struct {
	side    []uuid.UUID
	invited uuid.UUID
	items   []*mergeItem
	left    []*leftFood
	report  MergeReport
}

// members are the household's members after the merge, the side's
// first.
func (p *mergePlan) members() []uuid.UUID { return append(slices.Clone(p.side), p.invited) }

// mergeKitchens merges the kitchen of invited into side's, in tx, and
// returns what it did, with its fingerprint.
func mergeKitchens(ctx context.Context, tx *sql.Tx, side []uuid.UUID, invited uuid.UUID, now time.Time) (MergePreview, error) {
	slices.SortFunc(side[1:], uuid.UUID.Compare)
	hs, hLeft, err := readKitchen(ctx, tx, side)
	if err != nil {
		return MergePreview{}, err
	}
	us, uLeft, err := readKitchen(ctx, tx, []uuid.UUID{invited})
	if err != nil {
		return MergePreview{}, err
	}
	p := &mergePlan{side: side, invited: invited}
	p.plan(hs, hLeft, us, uLeft)
	fp, err := p.fingerprint()
	if err != nil {
		return MergePreview{}, err
	}
	if err := p.apply(ctx, tx, now); err != nil {
		return MergePreview{}, err
	}
	return MergePreview{Report: p.report, Fingerprint: fp}, nil
}

// kitchenRow is a food of a member's as readKitchen lists it.
type kitchenRow struct {
	member         shareMember
	saved, catalog string
	group          string
	left           bool
}

// readKitchen reads the kitchen of members: their foods, a food's copies
// (one share group's) one food, and the recipes they left apart.
func readKitchen(ctx context.Context, tx *sql.Tx, members []uuid.UUID) ([]*kitchenFood, []*leftFood, error) {
	var foods []*kitchenFood
	var left []*leftFood
	byKey := map[string]*kitchenFood{}
	for _, m := range members {
		rows, err := kitchenRows(ctx, tx, m)
		if err != nil {
			return nil, nil, err
		}
		for _, r := range rows {
			if r.left {
				f, err := foodInTx(ctx, tx, m, r.member.Food)
				if err != nil {
					return nil, nil, err
				}
				left = append(left, &leftFood{member: r.member, group: r.group, food: f})
				continue
			}
			key := r.group
			if key == "" {
				key = "own/" + r.member.User.String() + "/" + r.member.Food.String()
			}
			kf := byKey[key]
			if kf == nil {
				kf = &kitchenFood{group: r.group, copies: map[uuid.UUID]uuid.UUID{}}
				byKey[key] = kf
				foods = append(foods, kf)
			}
			kf.copies[m] = r.member.Food
			// The first member's copy, the inviter's, else the latest saved.
			if kf.rep.User == uuid.Nil() || (kf.rep.User != members[0] && r.saved > kf.saved) {
				kf.rep, kf.saved, kf.catalog = r.member, r.saved, r.catalog
			}
		}
	}
	for _, kf := range foods {
		if err := readRep(ctx, tx, kf); err != nil {
			return nil, nil, err
		}
	}
	return foods, left, nil
}

// readRep reads kf's food whole, and its recipe.
func readRep(ctx context.Context, tx *sql.Tx, kf *kitchenFood) error {
	var err error
	if kf.food, err = foodInTx(ctx, tx, kf.rep.User, kf.rep.Food); err != nil || kf.food.Source != food.Recipe {
		return err
	}
	r, err := recipeInTx(ctx, tx, kf.rep.User, kf.rep.Food)
	if err != nil {
		return err
	}
	kf.recipe = &r
	return nil
}

// kitchenRows lists userID's foods, by id.
func kitchenRows(ctx context.Context, tx *sql.Tx, userID uuid.UUID) ([]kitchenRow, error) {
	rows, err := tx.QueryContext(ctx, `SELECT f.id, f.updated_at, coalesce(f.catalog_id, ''), coalesce(s.share_id, ''),
			s.left_at IS NOT NULL
		FROM foods AS f LEFT JOIN food_shares AS s ON s.user_id = f.user_id AND s.food_id = f.id
		WHERE f.user_id = ? ORDER BY f.id`, userID.String())
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	var out []kitchenRow
	for rows.Next() {
		r := kitchenRow{member: shareMember{User: userID}}
		var id string
		if err := rows.Scan(&id, &r.saved, &r.catalog, &r.group, &r.left); err != nil {
			return nil, err
		}
		if r.member.Food, err = uuid.Parse(id); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

// plan plans the merge of H's foods hs and U's us, and the recipes their
// members left (hLeft, uLeft): the items, their names, barcodes, groups
// and copies, and the report.
func (p *mergePlan) plan(hs []*kitchenFood, hLeft []*leftFood, us []*kitchenFood, uLeft []*leftFood) {
	p.items, uLeft = match(hs, us, uLeft, p.invited)
	p.left = slices.Concat(hLeft, uLeft)
	for _, it := range p.items {
		it.food = it.base().food
		switch {
		case it.h != nil:
			it.group = it.h.group
			if it.u != nil && it.u.group != "" && it.u.group != it.group {
				it.leave = it.u.group
			}
		default:
			it.group = it.u.group
		}
	}
	p.bringBack()
	p.resolveNames()
	p.resolveBarcodes()
	for _, it := range p.items {
		c := &p.report.Given
		switch {
		case it.h != nil && it.u != nil:
			c = &p.report.Merged
		case it.h != nil:
			c = &p.report.Received
		}
		if it.food.Source == food.Recipe {
			c.Recipes++
		} else {
			c.Foods++
		}
	}
}

// match pairs H's foods hs with U's us, and U's recipes left uLeft of a
// group of one of H's (which are U's copies of it, brought back), as the
// merge's rule 1 says, and returns the merged kitchen's items, and the
// recipes left of U's that are none.
func match(hs, us []*kitchenFood, uLeft []*leftFood, invited uuid.UUID) ([]*mergeItem, []*leftFood) {
	var items []*mergeItem
	pairs := map[*kitchenFood]*kitchenFood{}
	paired := map[*kitchenFood]bool{}
	pair := func(h, u *kitchenFood, back bool) {
		pairs[h], paired[u] = u, true
		it := &mergeItem{h: h, u: u, copies: maps.Clone(h.copies), back: map[uuid.UUID]bool{}}
		it.copies[invited] = u.rep.Food
		if back {
			it.back[invited] = true
		}
		items = append(items, it)
	}
	stays := matchGroups(hs, us, uLeft, pairs, pair)
	for _, rule := range []func(*kitchenFood) []string{nameKeys, barcodeKeys, catalogKeys} {
		for _, c := range candidates(hs, us, pairs, paired, rule) {
			if pairs[c[0]] == nil && !paired[c[1]] {
				pair(c[0], c[1], false)
			}
		}
	}
	for _, h := range hs {
		if pairs[h] == nil {
			items = append(items, &mergeItem{h: h, copies: maps.Clone(h.copies), back: map[uuid.UUID]bool{}})
		}
	}
	for _, u := range us {
		if !paired[u] {
			items = append(items, &mergeItem{u: u, copies: maps.Clone(u.copies), back: map[uuid.UUID]bool{}})
		}
	}
	return items, stays
}

// matchGroups pairs the foods of H's and U's of one share group (403),
// which are one already: U's live recipe, or one U left, which comes
// back. It returns the recipes U left that are none.
func matchGroups(hs, us []*kitchenFood, uLeft []*leftFood, pairs map[*kitchenFood]*kitchenFood,
	pair func(h, u *kitchenFood, back bool),
) []*leftFood {
	byGroup := map[string]*kitchenFood{}
	for _, h := range hs {
		if h.group != "" {
			byGroup[h.group] = h
		}
	}
	for _, u := range us {
		if h := byGroup[u.group]; u.group != "" && h != nil {
			pair(h, u, false)
		}
	}
	var stays []*leftFood
	for _, lf := range uLeft {
		if h := byGroup[lf.group]; h != nil && pairs[h] == nil {
			pair(h, &kitchenFood{rep: lf.member, food: lf.food, group: lf.group, copies: map[uuid.UUID]uuid.UUID{lf.member.User: lf.member.Food}}, true)
			continue
		}
		stays = append(stays, lf)
	}
	return stays
}

// candidates returns the pairs of a food of H's and one of U's, both
// unpaired and of one kind, that a rule's keys of match, the most
// recently saved first.
func candidates(hs, us []*kitchenFood, pairs map[*kitchenFood]*kitchenFood, paired map[*kitchenFood]bool,
	keys func(*kitchenFood) []string,
) [][2]*kitchenFood {
	byKey := map[string][]*kitchenFood{}
	for _, h := range hs {
		if pairs[h] == nil {
			for _, k := range keys(h) {
				byKey[k] = append(byKey[k], h)
			}
		}
	}
	var out [][2]*kitchenFood
	seen := map[[2]*kitchenFood]bool{}
	for _, u := range us {
		if paired[u] {
			continue
		}
		for _, k := range keys(u) {
			for _, h := range byKey[k] {
				c := [2]*kitchenFood{h, u}
				if !seen[c] && (h.food.Source == food.Recipe) == (u.food.Source == food.Recipe) {
					seen[c] = true
					out = append(out, c)
				}
			}
		}
	}
	slices.SortStableFunc(out, func(a, b [2]*kitchenFood) int {
		return cmp.Or(cmp.Compare(b[0].saved, a[0].saved), cmp.Compare(b[1].saved, a[1].saved),
			a[0].rep.Food.Compare(b[0].rep.Food), a[1].rep.Food.Compare(b[1].rep.Food))
	})
	return out
}

func nameKeys(kf *kitchenFood) []string {
	return []string{food.Key(kf.food.Name) + "\x00" + food.Key(kf.food.Brand)}
}

func barcodeKeys(kf *kitchenFood) []string { return kf.food.Barcodes }

func catalogKeys(kf *kitchenFood) []string {
	if kf.catalog == "" {
		return nil
	}
	return []string{kf.catalog}
}

// bringBack makes a member's recipe left of an item's group their copy
// of it, when they have none: a group they come back to.
func (p *mergePlan) bringBack() {
	byGroup := map[string]*mergeItem{}
	for _, it := range p.items {
		if it.group != "" {
			byGroup[it.group] = it
		}
	}
	p.left = slices.DeleteFunc(p.left, func(lf *leftFood) bool {
		it := byGroup[lf.group]
		if it == nil {
			return false
		}
		if _, has := it.copies[lf.member.User]; has {
			return false
		}
		it.copies[lf.member.User] = lf.member.Food
		it.back[lf.member.User] = true
		return true
	})
}

// nameKey is a food's name and brand as food.Key compares them.
type nameKey struct{ name, brand string }

func keyOf(f food.Food) nameKey { return nameKey{food.Key(f.Name), food.Key(f.Brand)} }

// resolveNames gives every food of the merged kitchen a name and brand
// no other has, and every recipe left one no food has, renaming the one
// rule 6 says: of H's foods none, but one of the same name as another
// (which no household has); U's food, of a name one of H's has or a
// recipe left of a member of H's; a recipe left, of an item's name.
func (p *mergePlan) resolveNames() {
	reserved := map[nameKey]bool{}
	for _, it := range p.items {
		reserved[keyOf(it.food)] = true
	}
	hLeft := map[nameKey]bool{}
	for _, lf := range p.left {
		reserved[keyOf(lf.food)] = true
		if lf.member.User != p.invited {
			hLeft[keyOf(lf.food)] = true
		}
	}
	taken := map[nameKey]bool{}
	rename := func(f *food.Food) {
		from := f.Name
		name := []rune(f.Name)
		for n := 2; ; n++ {
			suffix := fmt.Sprintf(" (%d)", n)
			f.Name = string(name[:min(len(name), food.MaxNameLen-len(suffix))]) + suffix
			if k := keyOf(*f); !reserved[k] && !taken[k] {
				break
			}
		}
		reserved[keyOf(*f)] = true
		p.report.Renamed = append(p.report.Renamed, MergeRename{From: from, To: f.Name})
	}
	for _, theirs := range []bool{true, false} {
		for _, it := range p.items {
			if (it.h != nil) != theirs {
				continue
			}
			if k := keyOf(it.food); taken[k] || (!theirs && hLeft[k]) {
				rename(&it.food)
			}
			taken[keyOf(it.food)] = true
		}
	}
	for _, lf := range p.left {
		if taken[keyOf(lf.food)] {
			rename(&lf.food)
			lf.changed = true
		}
	}
}

// resolveBarcodes gives every food of the merged kitchen its barcodes:
// H's food's, and of a match U's food's too, but one another food of the
// kitchen has, or, for one of U's alone, a recipe left of a member of
// H's; a recipe left loses one an item has. Each barcode U's foods lose
// is in the report.
func (p *mergePlan) resolveBarcodes() {
	claimed := map[string]bool{}
	// H's barcodes first: they stay where they are.
	for _, it := range p.items {
		if it.h != nil {
			it.food.Barcodes = slices.DeleteFunc(slices.Clone(it.food.Barcodes), func(c string) bool { return claimed[c] })
			for _, c := range it.food.Barcodes {
				claimed[c] = true
			}
		}
	}
	hLeft := map[string]bool{}
	for _, lf := range p.left {
		if lf.member.User != p.invited {
			for _, c := range lf.food.Barcodes {
				hLeft[c] = true
			}
		}
	}
	for _, it := range p.items {
		if it.u != nil {
			p.addBarcodes(it, claimed, hLeft)
		}
		if len(it.food.Barcodes) == 0 {
			// As a food read has none.
			it.food.Barcodes = nil
		}
	}
	for _, lf := range p.left {
		p.trimBarcodes(lf, claimed)
	}
}

// addBarcodes gives item it U's food's barcodes, but the ones claimed by
// another food, or, for one of U's alone, of a recipe left of a member of
// H's (hLeft), and the ones past food.MaxBarcodes, which it drops.
func (p *mergePlan) addBarcodes(it *mergeItem, claimed, hLeft map[string]bool) {
	var codes []string
	if it.h != nil {
		codes = it.food.Barcodes
	}
	for _, c := range it.u.food.Barcodes {
		switch {
		case slices.Contains(codes, c):
		case claimed[c] || (it.h == nil && hLeft[c]) || len(codes) == food.MaxBarcodes:
			p.report.Dropped = append(p.report.Dropped, MergeBarcode{Food: it.food.Name, Code: c})
		default:
			claimed[c] = true
			codes = append(codes, c)
		}
	}
	it.food.Barcodes = codes
}

// trimBarcodes takes the barcodes claimed by a food of the kitchen from
// the recipe left lf, U's dropped in the report.
func (p *mergePlan) trimBarcodes(lf *leftFood, claimed map[string]bool) {
	var kept []string
	for _, c := range lf.food.Barcodes {
		switch {
		case !claimed[c]:
			kept = append(kept, c)
		case lf.member.User == p.invited:
			p.report.Dropped = append(p.report.Dropped, MergeBarcode{Food: lf.food.Name, Code: c})
		}
	}
	if len(kept) != len(lf.food.Barcodes) {
		lf.food.Barcodes, lf.changed = kept, true
	}
}

// planView is the plan as its fingerprint is made of it: what the merge
// writes, but the ids of the copies it makes.
type planView struct {
	Side    []uuid.UUID `json:"side"`
	Invited uuid.UUID   `json:"invited"`
	Items   []itemView  `json:"items"`
	Left    []food.Food `json:"left"`
	Report  MergeReport `json:"report"`
}

type itemView struct {
	Food    food.Food               `json:"food"`
	Owner   uuid.UUID               `json:"owner"`
	Recipe  *recipe.Recipe          `json:"recipe,omitempty"`
	Copies  map[uuid.UUID]uuid.UUID `json:"copies"`
	Back    map[uuid.UUID]bool      `json:"back,omitempty"`
	Group   string                  `json:"group"`
	Leave   string                  `json:"leave,omitempty"`
	Catalog string                  `json:"catalog,omitempty"`
}

// fingerprint is the hash of the plan: the same merge, the same one.
func (p *mergePlan) fingerprint() (string, error) {
	v := planView{Side: p.side, Invited: p.invited, Report: p.report}
	for _, it := range p.items {
		iv := itemView{
			Food: it.food, Owner: it.base().rep.User, Copies: it.copies, Back: it.back,
			Group: it.group, Leave: it.leave, Catalog: it.catalog(),
		}
		// A member's own words are theirs to change meanwhile.
		iv.Food.ID, iv.Food.Note, iv.Food.Aliases, iv.Food.Archived = uuid.Nil(), "", nil, false
		if r := it.base().recipe; r != nil {
			dish := *r
			dish.Food = food.Food{}
			iv.Recipe = &dish
		}
		v.Items = append(v.Items, iv)
	}
	slices.SortFunc(v.Items, func(a, b itemView) int {
		return cmp.Or(cmp.Compare(food.Key(a.Food.Name), food.Key(b.Food.Name)), cmp.Compare(food.Key(a.Food.Brand), food.Key(b.Food.Brand)))
	})
	for _, lf := range p.left {
		if lf.changed {
			v.Left = append(v.Left, lf.food)
		}
	}
	b, err := json.Marshal(v)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:]), nil
}

// apply writes the plan: each member's copy of each food, the recipes
// left renamed or of fewer barcodes, the share groups, and each member's
// recipes.
func (p *mergePlan) apply(ctx context.Context, tx *sql.Tx, now time.Time) error {
	members := p.members()
	for _, it := range p.items {
		it.fresh, it.own = map[uuid.UUID]bool{}, map[uuid.UUID]food.Food{}
		for _, m := range members {
			if _, has := it.copies[m]; !has {
				it.copies[m], it.fresh[m] = uuid.NewV7(), true
				continue
			}
			own, err := foodInTx(ctx, tx, m, it.copies[m])
			if err != nil {
				return err
			}
			it.own[m] = own
		}
	}
	if err := p.writeFoods(ctx, tx, members, now); err != nil {
		return err
	}
	for _, it := range p.items {
		if err := it.seat(ctx, tx, members, p.invited); err != nil {
			return err
		}
	}
	return p.writeRecipes(ctx, tx, members)
}

// copyOf is the food member m's copy of it is after the merge: its
// shared fields, and m's own note, aliases and archived; a fresh copy
// archived when every copy before it is (rule 3).
func (it *mergeItem) copyOf(m uuid.UUID) food.Food {
	f := it.food
	f.ID, f.CopyOf = it.copies[m], uuid.Nil()
	own := it.own[m]
	f.Note, f.Aliases, f.Archived = own.Note, own.Aliases, own.Archived
	if it.fresh[m] {
		f.Archived = it.withdrawn()
	}
	return f
}

// withdrawn reports whether every member's food of it before the merge
// is archived.
func (it *mergeItem) withdrawn() bool {
	for m := range it.own {
		if !it.own[m].Archived {
			return false
		}
	}
	return len(it.own) > 0
}

// foodWrite is a row of a member's the merge writes.
type foodWrite struct {
	user  uuid.UUID
	food  food.Food
	fresh bool
}

// writeFoods writes every copy the merge makes or changes, and the
// recipes left it renames or takes barcodes from, each copy of the
// catalog's row of its item. The rows that change give their names and
// barcodes up first, as one may take another's.
func (p *mergePlan) writeFoods(ctx context.Context, tx *sql.Tx, members []uuid.UUID, now time.Time) error {
	writes := p.foodWrites(members)
	for i := range writes {
		w := &writes[i]
		if w.fresh {
			continue
		}
		if _, err := tx.ExecContext(ctx, "UPDATE foods SET name_key = 'merge/' || id WHERE user_id = ? AND id = ?",
			w.user.String(), w.food.ID.String()); err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, "DELETE FROM food_barcodes WHERE user_id = ? AND food_id = ?",
			w.user.String(), w.food.ID.String()); err != nil {
			return err
		}
	}
	for i := range writes {
		w := &writes[i]
		write := updateFood
		if w.fresh {
			write = insertFood
		}
		if err := write(ctx, tx, w.user, w.food, food.Key(w.food.Name), food.Key(w.food.Brand), now); err != nil {
			return err
		}
		if err := replaceFoodChildren(ctx, tx, w.user, w.food); err != nil {
			return err
		}
	}
	for _, it := range p.items {
		link := sql.NullString{String: it.catalog(), Valid: it.catalog() != ""}
		for _, m := range members {
			if _, err := tx.ExecContext(ctx, "UPDATE foods SET catalog_id = ?1 WHERE user_id = ?2 AND id = ?3 AND catalog_id IS NOT ?1",
				link, m.String(), it.copies[m].String()); err != nil {
				return err
			}
		}
	}
	return nil
}

// foodWrites lists the rows writeFoods writes: each copy made, or not as
// the merge leaves it, and each recipe left it changes.
func (p *mergePlan) foodWrites(members []uuid.UUID) []foodWrite {
	var writes []foodWrite
	for _, it := range p.items {
		for _, m := range members {
			if f := it.copyOf(m); it.fresh[m] || !reflect.DeepEqual(f, it.own[m]) {
				writes = append(writes, foodWrite{user: m, food: f, fresh: it.fresh[m]})
			}
		}
	}
	for _, lf := range p.left {
		if lf.changed {
			writes = append(writes, foodWrite{user: lf.member.User, food: lf.food})
		}
	}
	return writes
}

// seat ties every member's copy of it into its group, made when it has
// none: U's food leaves the group outside it was in (rule 5), a recipe
// left comes back, and a member without a seat gets one, the
// household's. The groups' versions move on.
func (it *mergeItem) seat(ctx context.Context, tx *sql.Tx, members []uuid.UUID, invited uuid.UUID) error {
	if it.leave != "" {
		if _, err := tx.ExecContext(ctx, "DELETE FROM food_shares WHERE share_id = ? AND user_id = ?", it.leave, invited.String()); err != nil {
			return err
		}
		if _, err := bumpShare(ctx, tx, it.leave); err != nil {
			return err
		}
	}
	if it.group == "" {
		it.group = uuid.NewV7().String()
	}
	for _, m := range members {
		if it.back[m] {
			if _, err := tx.ExecContext(ctx, "UPDATE food_shares SET left_at = NULL, household = 1 WHERE share_id = ? AND user_id = ?",
				it.group, m.String()); err != nil {
				return err
			}
			continue
		}
		live, err := liveIn(ctx, tx, it.group, m)
		if err != nil {
			return err
		}
		if !live {
			if _, err := insertSeat(ctx, tx, it.group, m, it.copies[m], true); err != nil {
				return err
			}
		}
	}
	_, err := bumpShare(ctx, tx, it.group)
	return err
}

// writeRecipes writes each member's recipe of each recipe of the merged
// kitchen: the base's dish, each ingredient of a food of the kitchen's
// of the member's own copy of it, and checks no recipe of U's is made
// of itself now.
func (p *mergePlan) writeRecipes(ctx context.Context, tx *sql.Tx, members []uuid.UUID) error {
	of := map[shareMember]*mergeItem{}
	for _, it := range p.items {
		for m, id := range it.copies {
			of[shareMember{User: m, Food: id}] = it
		}
	}
	for _, it := range p.items {
		b := it.base()
		if b.recipe == nil {
			continue
		}
		for _, m := range members {
			r := *b.recipe
			r.Food = it.copyOf(m)
			r.Items = ownedItems(b.recipe.Items, b.rep.User, m, of)
			if !it.fresh[m] {
				had, err := recipeInTx(ctx, tx, m, r.Food.ID)
				if err != nil && !errors.Is(err, ErrNotFound) {
					return err
				}
				if err == nil && reflect.DeepEqual(had, r) {
					continue
				}
			}
			if err := writeRecipe(ctx, tx, m, r); err != nil {
				return err
			}
		}
	}
	// A recipe of U's made of a recipe of H's that is made of it.
	for _, it := range p.items {
		if b := it.base(); b.recipe != nil {
			if err := checkCycle(ctx, tx, p.invited, it.copies[p.invited], ownedItems(b.recipe.Items, b.rep.User, p.invited, of)); err != nil {
				return err
			}
		}
	}
	return nil
}

// ownedItems is items, the ingredients of owner's recipe, as member m's
// recipe has them: each of a food of the merged kitchen's of m's copy of
// it, any other of its owner's (meal.Item.FoodUser), none for m's own.
func ownedItems(items []meal.Item, owner, m uuid.UUID, of map[shareMember]*mergeItem) []meal.Item {
	out := slices.Clone(items)
	for i := range out {
		it := &out[i]
		if it.FoodID == uuid.Nil() {
			continue
		}
		who := it.FoodUser
		if who == uuid.Nil() {
			who = owner
		}
		if item := of[shareMember{User: who, Food: it.FoodID}]; item != nil {
			it.FoodID, it.FoodUser = item.copies[m], uuid.Nil()
			continue
		}
		it.FoodUser = who
		if who == m {
			it.FoodUser = uuid.Nil()
		}
	}
	return out
}
