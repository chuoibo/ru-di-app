package ingest

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
)

// SchemaVersion is the contract this package reads.
const SchemaVersion = "place.v1"

// Reject codes. They are machine-readable on purpose: a rejected row lands in
// `ingest_reject` with one of these, and "how many rows did we drop and why"
// has to be answerable with a GROUP BY rather than by reading prose.
const (
	RejectBadJSON         = "json_hong"
	RejectSchemaVersion   = "schema_la"
	RejectNoPlaceID       = "thieu_place_id"
	RejectNoName          = "thieu_ten"
	RejectUnknownKind     = "loai_khong_biet"
	RejectDish            = "loai_la_mon_an"
	RejectNoPosts         = "thieu_posts"
	RejectPostNoPlatform  = "post_thieu_platform"
	RejectPostBadPlatform = "post_platform_la"
	RejectPostNoID        = "post_thieu_id"
	RejectUnknownProvince = "province_khong_biet"
	RejectGeoHalfPoint    = "geo_nua_cap"
	RejectGeoNoPrecision  = "geo_thieu_precision"
	RejectGeoBadPrecision = "geo_precision_la"
	RejectGeoBadSource    = "geo_nguon_la"
	RejectFrameNoSize     = "frame_thieu_kich_thuoc"
	RejectFrameBadType    = "frame_content_type_la"
	RejectFrameNoKey      = "frame_thieu_storage_key"
	RejectVerbatimQuote   = "lot_trich_nguyen_van"
)

// The feed's seven kinds. `mon_an` is a dish rather than a place and is
// dropped: it has no address, cannot be visited, and would sit in the
// catalogue as something nobody can go to.
var feedKinds = map[string]bool{
	"quan_an": true, "cafe": true, "khu_am_thuc": true, "diem_tham_quan": true,
	"trai_nghiem": true, "giai_tri": true, "mon_an": true,
}

// GeoPrecisions is the vocabulary the catalogue column accepts. `suy_luan`
// stands apart from the four real levels: it means a model produced the point
// with no address behind it, and a screen must be able to tell that apart from
// a ward centroid. It is currently never produced, and is here anyway -- a
// vocabulary that has to grow later grows by migration.
var GeoPrecisions = map[string]bool{
	"rooftop": true, "street": true, "ward_centroid": true,
	"province_centroid": true, "suy_luan": true, "none": true,
}

var platforms = map[string]bool{"tiktok": true, "threads": true}

// Only these two survive the image sanitiser, whatever arrives.
var storedImageTypes = map[string]bool{"image/jpeg": true, "image/png": true}

// An OSM object reference, not a web address: `way/40219` stays correct
// when osm.org moves its paths, and a URL can be built from it at any time.
var osmRef = regexp.MustCompile(`^osm:(node|way|relation)/\d+$`)

// PostRef is one platform post a place was built from. The pair (Platform,
// PostID) is the only identifier in the whole feed the platform itself issued,
// which is why identity is anchored here and not on the feed's own row key.
type PostRef struct {
	Platform  string `json:"platform"`
	PostID    string `json:"post_id"`
	SourceURL string `json:"source_url"`
}

// Frame is one selected video frame.
type Frame struct {
	StorageKey  string   `json:"storage_key"`
	ContentType string   `json:"content_type"`
	ByteSize    int64    `json:"byte_size"`
	Width       int      `json:"width"`
	Height      int      `json:"height"`
	FileExists  bool     `json:"file_exists"`
	PostID      string   `json:"post_id"`
	Platform    string   `json:"platform"`
	SourceURL   string   `json:"source_url"`
	Giay        *float64 `json:"giay"`
	Diem        *float64 `json:"diem"`
	ChuThe      string   `json:"chu_the"`
	MoTa        string   `json:"mo_ta"`
}

// Geo is the coordinate block, absent until the feed has geocoded a row.
type Geo struct {
	Lat        *float64 `json:"lat"`
	Lng        *float64 `json:"lng"`
	Precision  string   `json:"precision"`
	Evidence   string   `json:"evidence"`
	Nguon      []string `json:"nguon"`
	NguonToaDo string   `json:"nguon_toa_do"`
	TruyVan    string   `json:"truy_van"`
	TaLoaiBo   string   `json:"ta_loai_bo"`
}

// Record is one line of the feed, in the shape this side reads it.
//
// Fields the catalogue does not project into a column are deliberately absent
// here rather than mapped to something approximate. Nothing is lost by that:
// the whole payload is kept verbatim in `ingest_place_raw`, so a field that
// earns a column later is projected by replaying what was already received.
type Record struct {
	SchemaVersion string   `json:"schema_version"`
	PlaceID       string   `json:"place_id"`
	Source        string   `json:"source"`
	TenChuan      string   `json:"ten_chuan"`
	TenBienThe    []string `json:"ten_bien_the"`
	Loai          string   `json:"loai"`
	Category      []string `json:"category"`
	ProvinceCode  *int16   `json:"province_code"`
	ProvinceName  string   `json:"province_name"`
	TinhRaw       string   `json:"tinh_raw"`
	KhuVucRaw     string   `json:"khu_vuc_raw"`
	DiaChi        string   `json:"dia_chi"`
	DiaChiDayDu   string   `json:"dia_chi_day_du"`
	DiemXepHang   *float64 `json:"diem_xep_hang_llm"`
	DoTin         *float64 `json:"do_tin"`
	SoBai         int      `json:"so_bai"`
	Posts         []PostRef
	Frames        []Frame
	Geo           *Geo
	TrungLapVoi   []string        `json:"trung_lap_voi"`
	CreatedAt     string          `json:"created_at"`
	UpdatedAt     string          `json:"updated_at"`
	Review        json.RawMessage `json:"review"`
	TomLuoc       json.RawMessage `json:"tom_luoc"`
}

// knownKeys is every top-level key this side expects. An unfamiliar key is not
// an error -- the feed's shape moves with its prompt versions and a reader that
// refuses an unfamiliar field would stop on a change that does not concern it.
// It is counted instead, so drift is visible rather than invisible.
var knownKeys = map[string]bool{
	"schema_version": true, "place_id": true, "source": true, "ten_chuan": true,
	"ten_bien_the": true, "ten_khac": true, "loai": true, "loai_hinh": true,
	"category": true, "province_code": true, "province_name": true,
	"province_match": true, "tinh_raw": true, "khu_vuc_raw": true,
	"dia_chi": true, "dia_chi_xac_nhan": true, "dia_chi_nguon": true,
	"dia_chi_day_du": true, "dia_chi_khac": true, "geo": true,
	"diem_xep_hang_llm": true, "do_tin": true, "so_bai": true, "review": true,
	"tom_luoc": true, "posts": true, "frames": true, "trung_lap_voi": true,
	"trung_toa_do_voi": true,
	"created_at":       true, "updated_at": true,
}

// knownGeoKeys is the evidence bag's vocabulary.
//
// Watched separately from the top level because this is where the feed is
// still growing: `ward_code`, `ta_loai_bo` and `truy_van` all arrived after
// the first delivery. A field here is present only when that step produced
// something, which is a different statement from producing nothing -- so an
// absent key is never drift, and an unrecognised one always is.
var knownGeoKeys = map[string]bool{
	"lat": true, "lng": true, "precision": true, "evidence": true,
	"nguon": true, "nguon_toa_do": true, "truy_van": true, "ta_loai_bo": true,
	"ward_code": true, "ward_name": true, "ward_nguon": true,
}

// Reject is one reason one line did not become a catalogue row.
type Reject struct {
	Code   string
	Detail string
}

func (r Reject) String() string { return r.Code + ": " + r.Detail }

// Parse reads one NDJSON line.
//
// It returns at most one Reject, because a line that fails is not worth
// enumerating every way in which it fails: the first refusal is what the
// operator acts on. Unknown top-level keys are returned separately -- they do
// not stop the row, they are a drift signal.
func Parse(line []byte) (*Record, []string, *Reject) {
	var loose map[string]json.RawMessage
	if err := json.Unmarshal(line, &loose); err != nil {
		return nil, nil, &Reject{RejectBadJSON, err.Error()}
	}
	var unknown []string
	for key := range loose {
		if !knownKeys[key] {
			unknown = append(unknown, key)
		}
	}

	var rec Record
	if err := json.Unmarshal(line, &rec); err != nil {
		return nil, unknown, &Reject{RejectBadJSON, err.Error()}
	}
	// `posts`, `frames` and `geo` are unmarshalled by name rather than by tag so
	// that a malformed one of them is a reject with its own code instead of a
	// bare "cannot unmarshal" on the whole line.
	if raw, ok := loose["posts"]; ok {
		if err := json.Unmarshal(raw, &rec.Posts); err != nil {
			return nil, unknown, &Reject{RejectNoPosts, "posts: " + err.Error()}
		}
	}
	if raw, ok := loose["frames"]; ok {
		if err := json.Unmarshal(raw, &rec.Frames); err != nil {
			return nil, unknown, &Reject{RejectFrameNoSize, "frames: " + err.Error()}
		}
	}
	if raw, ok := loose["geo"]; ok && string(raw) != "null" {
		if err := json.Unmarshal(raw, &rec.Geo); err != nil {
			return nil, unknown, &Reject{RejectGeoNoPrecision, "geo: " + err.Error()}
		}
		var looseGeo map[string]json.RawMessage
		if json.Unmarshal(raw, &looseGeo) == nil {
			for key := range looseGeo {
				if !knownGeoKeys[key] {
					unknown = append(unknown, "geo."+key)
				}
			}
		}
	}

	if key, found := findForbiddenKey(line); found {
		return nil, unknown, &Reject{RejectVerbatimQuote, key}
	}
	if reject := rec.validate(); reject != nil {
		return nil, unknown, reject
	}
	return &rec, unknown, nil
}

// forbiddenKeys carry other people's words verbatim.
//
// The decision was to keep the feed's written-up prose and not the quotations
// it was written from: those are sentences real people posted without knowing
// this catalogue exists. The feed already excludes them, and this check exists
// anyway -- a promise upstream is not a guarantee downstream, and a later
// prompt change could start including them again with no announcement.
var forbiddenKeys = map[string]bool{
	"bang_chung": true, "trich_dan": true, "nhan_xet_lap_lai": true,
}

// findForbiddenKey walks the payload at any depth.
func findForbiddenKey(line []byte) (string, bool) {
	var payload any
	if err := json.Unmarshal(line, &payload); err != nil {
		return "", false
	}
	return walkForbidden(payload)
}

func walkForbidden(node any) (string, bool) {
	switch value := node.(type) {
	case map[string]any:
		for key, child := range value {
			if forbiddenKeys[key] {
				return key, true
			}
			if found, ok := walkForbidden(child); ok {
				return found, true
			}
		}
	case []any:
		for _, child := range value {
			if found, ok := walkForbidden(child); ok {
				return found, true
			}
		}
	}
	return "", false
}

func (r *Record) validate() *Reject {
	if r.SchemaVersion != SchemaVersion {
		return &Reject{RejectSchemaVersion, fmt.Sprintf(
			"expected %s, got %q", SchemaVersion, r.SchemaVersion)}
	}
	if strings.TrimSpace(r.PlaceID) == "" {
		return &Reject{RejectNoPlaceID, "blank"}
	}
	if strings.TrimSpace(r.TenChuan) == "" {
		return &Reject{RejectNoName, r.PlaceID}
	}
	if !feedKinds[r.Loai] {
		return &Reject{RejectUnknownKind, r.Loai}
	}
	// A dish is not somewhere to go. Dropping it here rather than mapping it to
	// a category keeps the catalogue answerable to "can I visit this".
	if r.Loai == "mon_an" {
		return &Reject{RejectDish, r.TenChuan}
	}
	if len(r.Posts) == 0 {
		return &Reject{RejectNoPosts, r.PlaceID}
	}
	for i, post := range r.Posts {
		if post.Platform == "" {
			return &Reject{RejectPostNoPlatform, fmt.Sprintf("posts[%d]", i)}
		}
		if !platforms[post.Platform] {
			return &Reject{RejectPostBadPlatform, post.Platform}
		}
		if strings.TrimSpace(post.PostID) == "" {
			return &Reject{RejectPostNoID, fmt.Sprintf("posts[%d]", i)}
		}
	}
	if reject := r.validateGeo(); reject != nil {
		return reject
	}
	for i, frame := range r.Frames {
		if !frame.FileExists {
			continue // announced as missing by the feed; skipped, not a reject
		}
		if strings.TrimSpace(frame.StorageKey) == "" {
			return &Reject{RejectFrameNoKey, fmt.Sprintf("frames[%d]", i)}
		}
		// The stored image is whatever the sanitiser re-encodes it to, but a
		// frame that cannot state its own type and size cannot be written: the
		// photograph table requires both, and a zero would be a lie.
		if !storedImageTypes[frame.ContentType] && frame.ContentType != "image/webp" {
			return &Reject{RejectFrameBadType, frame.ContentType}
		}
		if frame.Width <= 0 || frame.Height <= 0 || frame.ByteSize <= 0 {
			return &Reject{RejectFrameNoSize, fmt.Sprintf(
				"frames[%d]: %dx%d, %d bytes",
				i, frame.Width, frame.Height, frame.ByteSize)}
		}
	}
	return nil
}

func (r *Record) validateGeo() *Reject {
	if r.Geo == nil {
		return nil
	}
	hasLat, hasLng := r.Geo.Lat != nil, r.Geo.Lng != nil
	if hasLat != hasLng {
		return &Reject{RejectGeoHalfPoint, r.PlaceID}
	}
	if r.Geo.Precision != "" && !GeoPrecisions[r.Geo.Precision] {
		return &Reject{RejectGeoBadPrecision, r.Geo.Precision}
	}
	// The rule the catalogue column enforces, applied before the write so the
	// row is rejected with a reason instead of by a constraint violation.
	if hasLat && r.Geo.Precision == "" {
		return &Reject{RejectGeoNoPrecision, r.PlaceID}
	}
	for _, source := range r.Geo.Nguon {
		if strings.HasPrefix(source, "http://") || strings.HasPrefix(source, "https://") {
			continue
		}
		if osmRef.MatchString(source) {
			continue
		}
		return &Reject{RejectGeoBadSource, source}
	}
	return nil
}

// HasPoint reports whether this row carries coordinates at all.
func (r *Record) HasPoint() bool {
	return r.Geo != nil && r.Geo.Lat != nil && r.Geo.Lng != nil
}

// unmappable are the precisions whose coordinates exist but must not be drawn.
//
// A province centroid is the middle of a province. The feed produces one when
// the address was a landmark rather than a location -- "in front of the high
// school", "down the alley by the pagoda, then right" -- which is a real way
// people describe places and nothing a geocoder can resolve. Nearly half of
// the located rows are like that, and all of them share one point.
//
// Drawing them would put several thousand pins on one spot in the middle of a
// city, each claiming to be a restaurant that is not there. `suy_luan` is the
// same problem from the other direction: a model's guess with no address
// behind it. Both are better shown as "we do not know where this is".
var unmappable = map[string]bool{"province_centroid": true, "suy_luan": true, "none": true}

// MappablePoint reports whether this row's coordinates are worth drawing.
//
// Deliberately narrower than HasPoint: having a latitude and being somewhere
// findable are different claims, and the map may only make the second one.
func (r *Record) MappablePoint() bool {
	return r.HasPoint() && !unmappable[r.Geo.Precision]
}

// DuplicateCoordinatesAreSuspect reports whether two rows sharing this row's
// point is a sign of a duplicated place rather than ordinary geography.
//
// Only at rooftop. Seventeen places on one street legitimately share that
// street's representative point, and every place in a ward shares its centroid
// -- that is what those levels mean. Sharing a building is the only one worth
// looking at.
func (r *Record) DuplicateCoordinatesAreSuspect() bool {
	return r.HasPoint() && r.Geo.Precision == "rooftop"
}

// Category maps the feed's seven kinds onto the four the client knows.
//
// The original is never discarded -- it is written to `places.source_kind` --
// because this mapping loses real distinctions: a museum and a cooking class
// both land in `vui-choi`, and the day the client learns a fifth category the
// answer has to be recoverable without asking the feed again.
func (r *Record) CategoryFor() string {
	switch r.Loai {
	case "cafe":
		return "cafe"
	case "quan_an", "khu_am_thuc":
		return "quan-an-local"
	case "giai_tri":
		if r.isNightlife() {
			return "di-choi-dem"
		}
		return "vui-choi"
	default:
		return "vui-choi"
	}
}

// nightlifeWords are the tags that make a `giai_tri` row an evening out.
//
// Theatre is deliberately absent. A play starts at eight and a bar opens at
// eight, and sorting them together would put "kịch nói" under a heading that
// promises something else entirely.
var nightlifeWords = []string{
	"bar", "pub", "club", "karaoke", "beer", "bia", "cocktail", "lounge",
}

func (r *Record) isNightlife() bool {
	for _, tag := range r.Category {
		lowered := strings.ToLower(tag)
		for _, word := range nightlifeWords {
			if strings.Contains(lowered, word) {
				return true
			}
		}
	}
	return false
}
