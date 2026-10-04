// Package diary defines the bounded, editable document produced after an outing.
// It knows neither HTTP nor persistence. Model output is data, never markup.
package diary

import (
	"errors"
	"strings"
	"time"
	"unicode/utf8"
)

const MaxPhotos = 40
const MaxPages = 24

type Photo struct {
	ID      string `json:"id"`
	URL     string `json:"url,omitempty"`
	Caption string `json:"caption"`
	Day     string `json:"day"`
}
type Source struct {
	Title    string   `json:"title"`
	StartsOn string   `json:"starts_on"`
	EndsOn   string   `json:"ends_on"`
	Kind     string   `json:"kind"`
	Photos   []Photo  `json:"photos"`
	Places   []string `json:"places"`
	// These excerpts are supplied by the device after a deliberate preview.
	Excerpts []string `json:"excerpts"`
}
type Page struct {
	Layout   string   `json:"layout"`
	Heading  string   `json:"heading"`
	Text     string   `json:"text"`
	PhotoIDs []string `json:"photo_ids"`
}
type Document struct {
	Title       string `json:"title"`
	Subtitle    string `json:"subtitle"`
	CoverID     string `json:"cover_id"`
	Pages       []Page `json:"pages"`
	AIGenerated bool   `json:"ai_generated"`
}

func SuggestedKind(start, end string) string {
	if end > start {
		return "trip"
	}
	return "moment"
}
func ValidKind(kind string) bool    { return kind == "trip" || kind == "moment" }
func textFits(s string, n int) bool { return utf8.ValidString(s) && utf8.RuneCountInString(s) <= n }

// Validate rejects unselected photos, executable layouts and oversized prose.
func Validate(d Document, allowed map[string]bool) error {
	if strings.TrimSpace(d.Title) == "" || !textFits(d.Title, 200) || !textFits(d.Subtitle, 500) || len(d.Pages) == 0 || len(d.Pages) > MaxPages {
		return errors.New("invalid_diary")
	}
	if d.CoverID != "" && !allowed[d.CoverID] {
		return errors.New("unknown_diary_photo")
	}
	for _, p := range d.Pages {
		if p.Layout != "photo" && p.Layout != "collage" && p.Layout != "note" {
			return errors.New("invalid_diary_layout")
		}
		if !textFits(p.Heading, 200) || !textFits(p.Text, 2000) || len(p.PhotoIDs) > 4 {
			return errors.New("invalid_diary_page")
		}
		for _, id := range p.PhotoIDs {
			if !allowed[id] {
				return errors.New("unknown_diary_photo")
			}
		}
	}
	return nil
}

// Compose is an honest non-AI fallback. Dates get readable labels, captions stay verbatim;
// an intended stop is never narrated as somewhere the group actually reached.
func Compose(s Source) Document {
	d := Document{Title: s.Title, Subtitle: "Một cuộc đi, những điều muốn giữ.", Pages: []Page{}}
	if s.Kind == "moment" {
		d.Subtitle = "Một ngày bình thường, một điều đáng nhớ."
	}
	for i := 0; i < len(s.Photos); i += 2 {
		p := Page{Layout: "photo", Heading: dayHeading(s.Photos[i].Day), Text: s.Photos[i].Caption, PhotoIDs: []string{s.Photos[i].ID}}
		if i+1 < len(s.Photos) {
			p.Layout = "collage"
			p.PhotoIDs = append(p.PhotoIDs, s.Photos[i+1].ID)
		}
		d.Pages = append(d.Pages, p)
	}
	if len(s.Photos) > 0 {
		d.CoverID = s.Photos[0].ID
	}
	if len(d.Pages) == 0 {
		d.Pages = append(d.Pages, Page{Layout: "note", Heading: "Điều mình muốn nhớ", Text: "", PhotoIDs: []string{}})
	}
	return d
}

// TenTrang is a stored page title as people read it: books saved before
// Compose wrote readable days kept the bare ISO date, which the Cộng đồng post
// built from a book printed verbatim (QA UI-154). Same rule as dayHeading.
func TenTrang(heading string) string { return dayHeading(heading) }

// dayHeading changes presentation only; source dates remain ISO on the wire.
func dayHeading(day string) string {
	date, err := time.Parse("2006-01-02", day)
	if err != nil {
		return day
	}
	return "Ngày " + date.Format("02/01/2006")
}
