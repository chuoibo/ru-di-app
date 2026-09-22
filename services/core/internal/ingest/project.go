package ingest

import (
	"encoding/json"
	"strings"
)

// Projection is one feed row in the shape the catalogue stores.
//
// Built from a Record and nothing else, so the same delivery always projects
// the same way and a corrected rule can be replayed over what was landed.
type Projection struct {
	ID           string
	SourceRef    string
	Name         string
	Category     string
	SourceKind   string
	Kinds        []string
	Address      *string
	ProvinceCode *int16
	Lat          *float64
	Lng          *float64
	GeoPrecision *string
	GeoEvidence  *string
	Description  *string
	Reviews      json.RawMessage
	Confidence   *float64
	EvidencePost int
	SourceUpdate *string
	Posts        []PostRef
}

// PlaceID is the catalogue id minted for a feed row.
//
// Derived from the feed's own key so that projecting the same row twice is the
// same row. The feed's key is a hash of a name a model chose, so a rename
// produces a new id and a second catalogue row -- which is what the post table
// and `superseded_by` exist to reconcile. Deriving it means the reconciliation
// is the only place that has to handle renames, instead of every load.
func PlaceID(sourceRef string) string {
	return "vnl-" + strings.TrimPrefix(sourceRef, "plc_")
}

// Project turns one validated feed row into catalogue columns.
//
// Everything the catalogue does not take a column for stays in the landed
// payload rather than being squeezed into an approximate column.
func Project(rec *Record) Projection {
	rec.CapPrecision()

	projection := Projection{
		ID:           PlaceID(rec.PlaceID),
		SourceRef:    rec.PlaceID,
		Name:         rec.TenChuan,
		Category:     rec.CategoryFor(),
		SourceKind:   rec.Loai,
		Kinds:        rec.Category,
		ProvinceCode: rec.ProvinceCode,
		Confidence:   rec.DoTin,
		EvidencePost: rec.SoBai,
		Posts:        rec.Posts,
	}
	if rec.UpdatedAt != "" {
		projection.SourceUpdate = &rec.UpdatedAt
	}

	// `dia_chi` is the address the feed actually holds. `dia_chi_day_du` is not
	// used, despite being called "full address": on the rows that were read it
	// holds an area -- "Phường Tân Mai, Biên Hòa, Đồng Nai" -- and putting that
	// in a column called `address` is how a screen comes to show a ward where a
	// street should be. Null here means the screen says there is no address,
	// which is true.
	if address := strings.TrimSpace(rec.DiaChi); address != "" {
		projection.Address = &address
	}

	if prose := strings.TrimSpace(reviewString(rec.Review, "mo_ta_tong_quan")); prose != "" {
		projection.Description = &prose
	}
	projection.Reviews = rec.Review

	projection.setPoint(rec)
	return projection
}

// setPoint decides what coordinates, if any, the catalogue keeps.
func (p *Projection) setPoint(rec *Record) {
	if !rec.HasPoint() {
		return
	}
	// A regional speciality keeps no coordinates at all.
	//
	// Its precision does not protect it -- two of these resolved to `street`,
	// and one of those landed on an insurance office. Relying on every future
	// reader to check the kind before drawing a pin is the kind of rule that
	// holds until somebody writes a second screen. Leaving the columns empty
	// means no reader can draw it, which is the same guarantee without the
	// vigilance.
	if rec.Loai == "mon_an" {
		return
	}
	p.Lat, p.Lng = rec.Geo.Lat, rec.Geo.Lng
	precision := rec.Geo.Precision
	p.GeoPrecision = &precision
	if evidence := strings.TrimSpace(rec.Geo.Evidence); evidence != "" {
		p.GeoEvidence = &evidence
	}
}

// reviewString pulls one string out of the feed's prose block.
func reviewString(raw json.RawMessage, key string) string {
	if len(raw) == 0 {
		return ""
	}
	var fields map[string]json.RawMessage
	if json.Unmarshal(raw, &fields) != nil {
		return ""
	}
	var value string
	if json.Unmarshal(fields[key], &value) != nil {
		return ""
	}
	return value
}

// Rating is deliberately absent from Projection.
//
// The feed carries `diem_xep_hang_llm`, a score a model gave after reading the
// posts. It is not a rating: nobody rated anything. ADR-0017 removed invented
// stars from this catalogue once already, and a number that looks like 5.8 out
// of 10 would put them back under a different name. The score stays in the
// landed payload, where it can inform ranking without ever being shown as
// something people said.
