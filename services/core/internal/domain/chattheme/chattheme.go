// Package chattheme is the Go port of app/domain/chat_theme.py (ADR-0021
// §2.4): five closed slugs, never a free colour.
//
// PATCH /contexts/{context_id} refuses any other value with 422
// theme_unknown. The comparison is exact: no case folding, no trimming, no
// normalisation, as Python's `value in THEMES` on a str.
package chattheme

import "slices"

// DefaultTheme is DEFAULT_THEME.
const DefaultTheme = "mac-dinh"

var themes = [...]string{"mac-dinh", "hoang-hon", "bien-dem", "rung-thong", "ruc-ro"}

// Themes is THEMES, in the Python order.
func Themes() []string { return slices.Clone(themes[:]) }

// IsTheme is is_theme for a str.
func IsTheme(value string) bool {
	return slices.Contains(themes[:], value)
}
