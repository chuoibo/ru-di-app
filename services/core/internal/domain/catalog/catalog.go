// Package catalog is app.places.catalog's CATEGORIES list for GET /places.
package catalog

// Category is one row of CATEGORIES.
type Category struct {
	ID    string
	Label string
}

// Categories is CATEGORIES, in declaration order.
var Categories = []Category{
	{"quan-an-local", "Quán ăn local"},
	{"cafe", "Cafe"},
	{"vui-choi", "Vui chơi"},
	{"di-choi-dem", "Đi chơi đêm"},
}
