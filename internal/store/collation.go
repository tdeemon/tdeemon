package store

import (
	"sync"

	"golang.org/x/text/collate"
	"golang.org/x/text/language"
	"modernc.org/sqlite"

	"github.com/zarnautovic/tdeemon/internal/domain/user"
)

// A list of names in order is in the order of the user's language: in
// Croatian, Č and Ć after C, Đ after D, Š after S and Ž after Z; in
// English, each beside its plain letter. Each language the app speaks
// (user.Locales) has a collation of its own in SQLite, names_<locale>,
// which every connection has.

// namesCollation returns the collation of names in locale's order, or
// English's for a locale the app does not speak.
func namesCollation(locale string) string {
	for _, l := range user.Locales() {
		if l == locale {
			return "names_" + l
		}
	}
	return "names_en"
}

func init() {
	for _, l := range user.Locales() {
		c := &collator{c: collate.New(language.Make(l))}
		sqlite.MustRegisterCollationUtf8("names_"+l, c.compare)
	}
}

// collator compares by a language's collation. A collate.Collator is no
// goroutine's to share, and the connections compare at once.
type collator struct {
	mu sync.Mutex
	c  *collate.Collator
}

func (c *collator) compare(a, b string) int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.c.CompareString(a, b)
}
