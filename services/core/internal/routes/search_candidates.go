package routes

import (
	"sort"
	"strings"
	"unicode"

	"golang.org/x/text/unicode/norm"

	"mobile/services/core/internal/domain/taste"
	"mobile/services/core/internal/repo"
	"mobile/services/core/internal/service"
)

// maxSearchCandidates is how many places a search prompt may list.
//
// The prompt carries one line per catalogue row before the person's sentence.
// That was harmless at twelve seed rows; on the fed catalogue (~9,300 rows and
// growing) it is a prompt of millions of tokens per search -- slow, costly,
// and past what a model reads. Retrieval narrows first, the model chooses
// among what is left.
const maxSearchCandidates = 120

// foldVietnamese lowers, strips combining marks and maps đ to d, so «pho»
// matches «Phở» the way the Explore filter on the phone already does.
func foldVietnamese(text string) string {
	var b strings.Builder
	for _, r := range norm.NFD.String(strings.ToLower(text)) {
		switch {
		case unicode.Is(unicode.Mn, r):
		case r == 'đ':
			b.WriteRune('d')
		default:
			b.WriteRune(r)
		}
	}
	return b.String()
}

// searchCandidates keeps the rows a search prompt will list.
//
// A catalogue at or under the cap is returned unchanged, so small catalogues
// (the seed the parity harness replays) build the very prompt they built
// before. Above it, rows are ranked by how many of the query's words appear
// in their name, category, kinds, traits or address (accent-insensitive),
// then by the group's taste score, then by catalogue position; the top rows
// are kept in catalogue order, the order the prompt always listed them in.
//
// Lexical recall misses purely semantic asks («chỗ nào lãng mạn»); for those
// every row scores zero on words and the taste score decides, which is the
// same order the list screen ranks by.
func searchCandidates(rows []repo.Place, query string, group taste.Profile) []repo.Place {
	if len(rows) <= maxSearchCandidates {
		return rows
	}
	var words []string
	for _, word := range strings.Fields(foldVietnamese(query)) {
		if len([]rune(word)) >= 2 {
			words = append(words, word)
		}
	}
	type ranked struct {
		row     repo.Place
		at      int // position in the catalogue's own order
		lexical int
		taste   int64
	}
	all := make([]ranked, len(rows))
	for i, row := range rows {
		address := ""
		if row.Address != nil {
			address = *row.Address
		}
		hay := foldVietnamese(strings.Join(append(append([]string{row.Name, row.Category, address},
			row.Kinds...), row.Traits...), " "))
		hits := 0
		for _, word := range words {
			if strings.Contains(hay, word) {
				hits++
			}
		}
		all[i] = ranked{row: row, at: i, lexical: hits}
		if group.Known() {
			all[i].taste = service.ScoreOrZero(service.PlaceRow(row), group)
		}
	}
	sort.SliceStable(all, func(i, j int) bool {
		if all[i].lexical != all[j].lexical {
			return all[i].lexical > all[j].lexical
		}
		if all[i].taste != all[j].taste {
			return all[i].taste > all[j].taste
		}
		return all[i].at < all[j].at
	})
	top := all[:maxSearchCandidates]
	// Back to the catalogue's order (the database's ORDER BY id, under its
	// collation), not Go's byte order.
	sort.Slice(top, func(i, j int) bool { return top[i].at < top[j].at })
	kept := make([]repo.Place, len(top))
	for i, r := range top {
		kept[i] = r.row
	}
	return kept
}
