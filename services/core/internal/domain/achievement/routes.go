// Package achievement decides journey routes from verified product facts.
// It never trusts a phone or an AI model to award an ending.
package achievement

import "sort"

// Facts is a count-only snapshot. All fields are recomputed from server rows.
type Facts struct {
	Checkins                  int  `json:"checkins"`
	DistinctPlacesInOneOuting int  `json:"distinct_places_in_one_outing"`
	DistinctDestinations      int  `json:"distinct_destinations"`
	OutingsWithDestinations   int  `json:"outings_with_destinations"`
	PhotoDays                 int  `json:"photo_days"`
	StoryCount                int  `json:"story_count"`
	StoryDays                 int  `json:"story_days"`
	SharedOutings             int  `json:"shared_outings"`
	LargestSharedParty        int  `json:"largest_shared_party"`
	RepeatedCompanionOutings  int  `json:"repeated_companion_outings"`
	PhotoAtCheckedPlace       bool `json:"photo_at_checked_place"`
	PhotoInSharedGroup        bool `json:"photo_in_shared_group"`
}

type Requirement struct {
	Label string `json:"label"`
	Have  int    `json:"have"`
	Need  int    `json:"need"`
}

type Choice struct {
	ID           string        `json:"id"`
	RouteID      string        `json:"route_id"`
	Title        string        `json:"title"`
	Description  string        `json:"description"`
	Reward       string        `json:"reward"`
	FeatureID    string        `json:"feature_id"`
	Eligible     bool          `json:"eligible"`
	Earned       bool          `json:"earned"`
	Requirements []Requirement `json:"requirements"`
}

// Chapter is a temporary story scene opened by the person's recent route
// decisions. It points only to a server-offered goal and never awards a badge.
type Chapter struct {
	ID             string `json:"id"`
	Title          string `json:"title"`
	Line           string `json:"line"`
	RouteID        string `json:"route_id"`
	TargetEndingID string `json:"target_ending_id"`
}

// FeatureForBadge is the product entitlement name; render workers check the
// earned badge row before accepting an optional creative template.
func FeatureForBadge(id string) string {
	switch id {
	case "many_turns":
		return "route_trail"
	case "open_map":
		return "map_album"
	case "photos_remain":
		return "instax_album"
	case "storyteller":
		return "photo_story"
	case "again_together":
		return "duo_album"
	case "full_house":
		return "group_intro"
	case "map_becomes_page":
		return "map_photo"
	case "shared_memory":
		return "shared_album"
	case "whole_journey":
		return "journey_finale"
	default:
		return ""
	}
}

func UnlockedFeatures(earned map[string]bool) []string {
	var out []string
	for _, id := range []string{"many_turns", "open_map", "photos_remain", "storyteller", "again_together", "full_house", "map_becomes_page", "shared_memory", "whole_journey"} {
		if earned[id] {
			out = append(out, FeatureForBadge(id))
		}
	}
	return out
}

type Route struct {
	ID    string `json:"id"`
	Title string `json:"title"`
	Blurb string `json:"blurb"`
}

var Routes = []Route{
	{ID: "dau_chan", Title: "Dấu chân", Blurb: "Mỗi nơi bạn đến để lại một lối rẽ."},
	{ID: "ky_niem", Title: "Kỷ niệm", Blurb: "Giữ một ngày ở lại lâu hơn bằng ảnh và lời kể."},
	{ID: "dong_hanh", Title: "Đồng hành", Blurb: "Những chuyến đi có người cùng nhớ."},
	{ID: "nga_re", Title: "Ngã rẽ", Blurb: "Những câu chuyện gặp nhau ở một trang mới."},
}

func requirement(label string, have, need int) Requirement {
	return Requirement{Label: label, Have: have, Need: need}
}

func flag(label string, yes bool) Requirement {
	if yes {
		return requirement(label, 1, 1)
	}
	return requirement(label, 0, 1)
}

func oneFrom(earned map[string]bool, ids ...string) bool {
	for _, id := range ids {
		if earned[id] {
			return true
		}
	}
	return false
}

func appendChoice(out []Choice, earned map[string]bool, id, routeID, title, description, reward string, req ...Requirement) []Choice {
	ready := true
	for _, r := range req {
		if r.Have < r.Need {
			ready = false
		}
	}
	return append(out, Choice{ID: id, RouteID: routeID, Title: title, Description: description,
		Reward: reward, FeatureID: FeatureForBadge(id), Eligible: ready, Earned: earned[id], Requirements: req})
}

// recentTransition reads the last two chosen story routes. A branch selection
// itself is skipped so it does not erase the two decisions that opened it.
func recentTransition(history []string) (string, string) {
	var last, before string
	for _, id := range history {
		if id == "nga_re" {
			continue
		}
		if last == "" {
			last = id
		} else {
			before = id
			break
		}
	}
	return before, last
}

// Candidates exposes six opening endings. Compound endings also require an
// ordered, replayable route decision. Verified evidence remains the sole award
// condition; an earned ending stays visible after later choices.
func Candidates(f Facts, earned map[string]bool, history ...string) []Choice {
	out := make([]Choice, 0, 9)
	from, to := recentTransition(history)
	out = appendChoice(out, earned, "many_turns", "dau_chan", "Một ngày nhiều ngã", "Hai nơi trong một chuyến đi.", "Mẫu video lộ trình",
		requirement("Nơi khác nhau trong một chuyến", f.DistinctPlacesInOneOuting, 2))
	out = appendChoice(out, earned, "open_map", "dau_chan", "Bản đồ mở", "Hai điểm đến trong hai chuyến đi.", "Khung hồ sơ và album bản đồ",
		requirement("Điểm đến", f.DistinctDestinations, 2), requirement("Chuyến có điểm đến", f.OutingsWithDestinations, 2))
	out = appendChoice(out, earned, "photos_remain", "ky_niem", "Những tấm ảnh còn đây", "Ảnh kỷ niệm ở ba ngày khác nhau.", "Album kiểu ảnh in",
		requirement("Ngày có ảnh kỷ niệm", f.PhotoDays, 3))
	out = appendChoice(out, earned, "storyteller", "ky_niem", "Chuyện mình kể", "Ba bài kể đủ dài ở ba ngày khác nhau.", "Mẫu video ảnh và chữ",
		requirement("Ngày có bài kể", f.StoryDays, 3))
	out = appendChoice(out, earned, "again_together", "dong_hanh", "Hẹn rồi lại hẹn", "Cùng một người check-in ở hai chuyến đi.", "Bố cục album đôi",
		requirement("Chuyến với cùng bạn", f.RepeatedCompanionOutings, 2))
	out = appendChoice(out, earned, "full_house", "dong_hanh", "Đủ mặt hôm nay", "Hai chuyến có bạn đồng hành, một chuyến có ít nhất ba người.", "Mẫu mở đầu video nhóm",
		requirement("Chuyến có bạn đồng hành", f.SharedOutings, 2), requirement("Người cùng một chuyến", f.LargestSharedParty, 3))
	if oneFrom(earned, "many_turns", "open_map") && oneFrom(earned, "photos_remain", "storyteller") && ((from == "dau_chan" && to == "ky_niem") || earned["map_becomes_page"]) {
		out = appendChoice(out, earned, "map_becomes_page", "nga_re", "Bản đồ thành trang", "Dấu chân, kỷ niệm và ảnh ở nơi bạn từng check-in.", "Video bản đồ và ảnh",
			flag("Kết Dấu chân", true), flag("Kết Kỷ niệm", true), flag("Ảnh ở nơi đã check-in", f.PhotoAtCheckedPlace))
	}
	if oneFrom(earned, "again_together", "full_house") && oneFrom(earned, "photos_remain", "storyteller") && ((from == "ky_niem" && to == "dong_hanh") || earned["shared_memory"]) {
		out = appendChoice(out, earned, "shared_memory", "nga_re", "Kỷ niệm chung", "Đồng hành, kỷ niệm và ảnh trong nhóm có chuyến chung.", "Album kỷ niệm chung",
			flag("Kết Đồng hành", true), flag("Kết Kỷ niệm", true), flag("Ảnh của nhóm có chuyến chung", f.PhotoInSharedGroup))
	}
	if oneFrom(earned, "many_turns", "open_map") && oneFrom(earned, "photos_remain", "storyteller") && oneFrom(earned, "again_together", "full_house") && oneFrom(earned, "map_becomes_page", "shared_memory") {
		out = appendChoice(out, earned, "whole_journey", "nga_re", "Hành trình của mình", "Một kết mỗi tuyến và một ngã rẽ.", "Dấu ấn hồ sơ và video tổng kết")
	}
	return out
}

// Chapters writes the next scene from the chosen sequence and progress. The
// six transitions deliberately lead to different scenes; the two converging
// paths become branch endings when their evidence is complete.
func Chapters(f Facts, earned map[string]bool, history ...string) []Chapter {
	from, to := recentTransition(history)
	if from == "" || to == "" {
		return []Chapter{}
	}
	type scene struct{ title, line, route, ending string }
	scenes := map[string]scene{
		"dau_chan:ky_niem":   {"Tấm ảnh dẫn về nơi cũ", "Bạn đi từ dấu chân sang kỷ niệm. Một khung ảnh ở nơi từng check-in có thể nối hai trang sổ.", "ky_niem", "photos_remain"},
		"ky_niem:dau_chan":   {"Ký ức gọi một con đường", "Từ một tấm ảnh, bạn chọn quay ra đường. Hai điểm đến sẽ mở một bản đồ khác.", "dau_chan", "open_map"},
		"ky_niem:dong_hanh":  {"Bức thư cho người cùng đi", "Kỷ niệm vừa có thêm một người kể. Hãy giữ tấm ảnh của một chuyến đi chung.", "dong_hanh", "again_together"},
		"dong_hanh:ky_niem":  {"Một ngày được kể lại", "Sau những lần cùng đi, bạn trở về trang ảnh. Ba ngày kỷ niệm có thể giữ câu chuyện ở lại.", "ky_niem", "photos_remain"},
		"dau_chan:dong_hanh": {"Con đường có thêm bước chân", "Bạn chọn rủ người đi cùng. Hai chuyến gặp lại nhau có thể thành một lời hẹn dài.", "dong_hanh", "again_together"},
		"dong_hanh:dau_chan": {"Theo bạn qua thành phố khác", "Từ nhóm bạn, bạn mở bản đồ. Hai điểm đến ở hai chuyến đi sẽ in một dấu mới.", "dau_chan", "open_map"},
	}
	id := from + ":" + to
	s, ok := scenes[id]
	if !ok {
		return []Chapter{}
	}
	if id == "dau_chan:ky_niem" && oneFrom(earned, "many_turns", "open_map") && oneFrom(earned, "photos_remain", "storyteller") {
		s.route, s.ending = "nga_re", "map_becomes_page"
		if !f.PhotoAtCheckedPlace {
			s.line = "Hai kết đã gặp nhau. Chụp một kỷ niệm ở nơi bạn từng check-in để hoàn tất trang bản đồ."
		}
	}
	if id == "ky_niem:dong_hanh" && oneFrom(earned, "again_together", "full_house") && oneFrom(earned, "photos_remain", "storyteller") {
		s.route, s.ending = "nga_re", "shared_memory"
		if !f.PhotoInSharedGroup {
			s.line = "Hai kết đã gặp nhau. Giữ một tấm ảnh trong nhóm có chuyến đi chung để mở trang kỷ niệm."
		}
	}
	return []Chapter{{ID: id, Title: s.title, Line: s.line, RouteID: s.route, TargetEndingID: s.ending}}
}

// SuggestedChoices is deterministic when Nếp is unavailable. It promotes the
// chosen route and near goals while still allowing every valid route.
func SuggestedChoices(f Facts, earned map[string]bool, selectedRoute string, history ...string) []Choice {
	choices := Candidates(f, earned, history...)
	visited := map[string]int{}
	for _, routeID := range history {
		visited[routeID]++
	}
	sort.SliceStable(choices, func(i, j int) bool {
		score := func(c Choice) int {
			v := 0
			if c.RouteID == selectedRoute {
				v += 100
			}
			if !c.Earned {
				v += 40
			}
			if c.Eligible {
				v += 20
			}
			if c.RouteID != selectedRoute {
				v -= 10 * visited[c.RouteID]
			}
			for _, r := range c.Requirements {
				if r.Need > 0 {
					v += 15 * min(r.Have, r.Need) / r.Need
				}
			}
			return v
		}
		return score(choices[i]) > score(choices[j])
	})
	return choices
}

func OpeningBadges(f Facts) []string {
	var out []string
	if f.Checkins > 0 {
		out = append(out, "first_checkin")
	}
	if f.PhotoDays > 0 {
		out = append(out, "first_photo")
	}
	if f.StoryCount > 0 {
		out = append(out, "first_story")
	}
	if f.SharedOutings > 0 {
		out = append(out, "first_together")
	}
	return out
}
