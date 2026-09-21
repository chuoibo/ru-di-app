package normalize

import (
	"fmt"
	"regexp"
	"sort"
)

// byteaHex is a bytea value as row_to_json renders it: "\\x<hex>".
var byteaHex = regexp.MustCompile(`\\\\x([0-9a-f]*)`)

// OrderKey is text with every value this binder has already numbered replaced
// by its placeholder and every other value two stacks cannot share masked to
// its kind, the way the database lane masks rows to order them. Two rows that
// are equal once masked but name different values the binder already knows
// (two shares of one person on two known items, say) get different keys, and
// those keys are the same on both stacks. Timestamps stay masked: their ranks
// only exist once the binder is frozen.
func (b *Binder) OrderKey(text string) string {
	text = b.replaceNamed(text)
	text = MaskCursors(text)
	text = tokenRun.ReplaceAllStringFunc(text, func(run string) string {
		if len(run) != tokenLen {
			return run
		}
		if n, ok := b.tokens[run]; ok {
			return fmt.Sprintf("<token43#%d>", n)
		}
		return "<token43>"
	})
	text = uuid4.ReplaceAllStringFunc(text, func(id string) string {
		if n, ok := b.uuids[id]; ok {
			return fmt.Sprintf("<uuid#%d>", n)
		}
		return "<uuid>"
	})
	text = timestamp.ReplaceAllString(text, "<ts>")
	text = byteaHex.ReplaceAllStringFunc(text, func(value string) string {
		if n, ok := b.digests[value[len(`\\x`):]]; ok {
			return fmt.Sprintf(`\\x<digest#%d>`, n)
		}
		return `\\x<hex>`
	})
	return hexRun.ReplaceAllStringFunc(text, func(run string) string {
		switch len(run) {
		case keyLen:
			if n, ok := b.keys[run]; ok {
				return fmt.Sprintf("<hex32#%d>", n)
			}
			return "<hex32>"
		case digestLen:
			if n, ok := b.digests[run]; ok {
				return fmt.Sprintf("<digest#%d>", n)
			}
		}
		return run
	})
}

// ObserveGroups observes texts that appeared together, such as one step's
// database rows grouped by relation and kind, in an order both stacks share.
//
// Within a group, a text whose OrderKey no other pending text of the group has
// is observed first, in key order. Observing it can number values that tell
// other texts apart (the item ids two shares point to), so keys are computed
// again until a pass observes nothing. Texts still tied after that differ only
// in values nothing else names; they are observed in the order given.
func (b *Binder) ObserveGroups(groups [][]string) error {
	pending := make([][]string, len(groups))
	for i, group := range groups {
		pending[i] = append([]string(nil), group...)
	}
	for {
		observed := false
		for g, texts := range pending {
			if len(texts) == 0 {
				continue
			}
			keys := make([]string, len(texts))
			count := map[string]int{}
			for i, text := range texts {
				keys[i] = b.OrderKey(text)
				count[keys[i]]++
			}
			order := make([]int, len(texts))
			for i := range order {
				order[i] = i
			}
			sort.SliceStable(order, func(i, j int) bool { return keys[order[i]] < keys[order[j]] })
			var tied []string
			for _, i := range order {
				if count[keys[i]] == 1 {
					if err := b.Observe(texts[i]); err != nil {
						return err
					}
					observed = true
				}
			}
			for i, text := range texts {
				if count[keys[i]] > 1 {
					tied = append(tied, text)
				}
			}
			pending[g] = tied
		}
		if !observed {
			break
		}
	}
	for _, texts := range pending {
		for _, text := range texts {
			if err := b.Observe(text); err != nil {
				return err
			}
		}
	}
	return nil
}
