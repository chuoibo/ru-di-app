package community

import (
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"net/http"
	"slices"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
)

func asDenial(err error, d **denial) bool { return errors.As(err, d) }

type pageCursor struct {
	At time.Time `json:"at"`
	ID string    `json:"id"`
}

func encodeCursor(v pageCursor) string {
	b, _ := json.Marshal(v)
	return base64.RawURLEncoding.EncodeToString(b)
}
func parseCursor(s string) (pageCursor, error) {
	// repo-guard: allow=long-number reason=synthetic-zero-uuid
	c := pageCursor{At: time.Date(1970, 1, 1, 0, 0, 0, 0, time.UTC), ID: "00000000-0000-0000-0000-000000000000"}
	if s == "" {
		return c, nil
	}
	if len(s) > 256 {
		return c, no(422, "invalid_cursor")
	}
	b, err := base64.RawURLEncoding.DecodeString(s)
	if err != nil || json.Unmarshal(b, &c) != nil || !validID(c.ID) {
		return c, no(422, "invalid_cursor")
	}
	return c, nil
}

type candidate struct {
	ID, Author                    string
	Created                       time.Time
	Topics                        []string
	Likes, Comments, Saves, Views int
	Followed                      bool
	Affinity                      int
	Score                         float64
}

// rank is version one: transparent, bounded, deterministic and diversity-aware.
func rank(items []candidate, mode string, personalized bool, now time.Time) []string {
	for i := range items {
		p := &items[i]
		hours := math.Max(0, now.Sub(p.Created).Hours())
		engagement := math.Log1p(float64(p.Likes*2 + p.Comments*3 + p.Saves*5))
		p.Score = engagement/math.Pow(hours+2, 0.8) + 1/(hours+2)
		if mode == "following" {
			p.Score = float64(p.Created.Unix())
		}
		if mode == "for_you" && personalized {
			p.Score += math.Log1p(float64(p.Affinity)) * 0.7
			if p.Followed {
				p.Score += 1.5
			}
			p.Score -= math.Min(float64(p.Views)*0.25, 2)
		}
	}
	sort.SliceStable(items, func(i, j int) bool {
		if items[i].Score == items[j].Score {
			return items[i].ID > items[j].ID
		}
		return items[i].Score > items[j].Score
	})
	out := []string{}
	recent := []string{}
	// Compact candidate indexes, not the large ranking structs, after a pick.
	// The ordering and exploration/diversity decisions remain identical.
	remaining := make([]int, len(items))
	for i := range remaining {
		remaining[i] = i
	}
	for len(remaining) > 0 {
		pick := 0
		if mode != "following" {
			// One exploration slot in five, favouring the newest unseen candidate.
			if len(out)%5 == 4 {
				for i, at := range remaining {
					if items[at].Views == 0 && (items[remaining[pick]].Views > 0 || items[at].Created.After(items[remaining[pick]].Created)) {
						pick = i
					}
				}
			}
			for i := range remaining {
				count := 0
				for _, a := range recent {
					if a == items[remaining[pick]].Author {
						count++
					}
				}
				if count < 2 {
					break
				}
				pick = i
			}
		}
		p := items[remaining[pick]]
		out = append(out, p.ID)
		recent = append(recent, p.Author)
		if len(recent) > 6 {
			recent = recent[1:]
		}
		remaining = append(remaining[:pick], remaining[pick+1:]...)
	}
	return out
}
func (h *Handler) feed(w http.ResponseWriter, r *http.Request) {
	tx, person, err := h.begin(r)
	if err != nil {
		fail(w, err)
		return
	}
	defer tx.Rollback(r.Context())
	ctx := r.Context()
	q := r.URL.Query()
	mode := q.Get("mode")
	if mode == "" {
		mode = "for_you"
	}
	// `hidden`: the posts this person marked «Không quan tâm», so they can take
	// one back (QA UI-141); like `mine` and `saved` it is a list, not a ranking.
	if !slices.Contains([]string{"for_you", "following", "trending", "saved", "mine", "hidden"}, mode) {
		fail(w, no(422, "invalid_feed"))
		return
	}
	var personalized bool
	err = tx.QueryRow(ctx, `SELECT COALESCE((SELECT personalized FROM community_preferences WHERE person_id=$1),false)`, person).Scan(&personalized)
	if err != nil {
		fail(w, err)
		return
	}
	snapshot := q.Get("after")
	offset := 0
	ids := []string{}
	if snapshot != "" {
		parts := strings.Split(snapshot, ":")
		if len(parts) != 2 || !validID(parts[0]) {
			fail(w, no(422, "invalid_cursor"))
			return
		}
		offset, err = strconv.Atoi(parts[1])
		if err != nil || offset < 0 || offset > 500 {
			fail(w, no(422, "invalid_cursor"))
			return
		}
		snapshot = parts[0]
		err = tx.QueryRow(ctx, `SELECT post_ids FROM community_feeds WHERE id=$1 AND person_id=$2 AND mode=$3 AND expires_at>clock_timestamp()`, snapshot, person, mode).Scan(&ids)
		if err != nil {
			fail(w, no(409, "feed_expired"))
			return
		}
	} else {
		topic := q.Get("topic")
		if topic != "" {
			v, e := normalizeTopics([]string{topic})
			if e != nil {
				fail(w, e)
				return
			}
			topic = v[0]
		}
		author := q.Get("author")
		if author != "" && !validID(author) {
			fail(w, no(422, "invalid_author"))
			return
		}
		items := []candidate{}
		ranked := false
		if topic == "" && author == "" && mode != "mine" && mode != "saved" && mode != "hidden" {
			ids, err = h.discoveryRanking(ctx, tx, person, mode, personalized)
			if err != nil {
				fail(w, err)
				return
			}
			ranked = true
		} else {
			rows, e := tx.Query(ctx, `WITH candidates AS (
  SELECT p.id,p.author_id,c.topics,COALESCE(c.published_at,p.created_at) AS created_at,
   EXISTS(SELECT 1 FROM community_follows f WHERE f.person_id=$1 AND ((f.kind='person' AND f.target=p.author_id::text) OR (f.kind='topic' AND f.target=ANY(c.topics)))) AS followed
  FROM community_posts c JOIN posts p ON p.id=c.post_id JOIN people a ON a.id=p.author_id
  WHERE c.deleted_at IS NULL AND a.deleted_at IS NULL AND (`+readableSQL+`)
   AND (($2='mine' AND p.author_id=$1) OR ($2='saved' AND EXISTS(SELECT 1 FROM community_feedback WHERE person_id=$1 AND post_id=p.id AND kind='saved')) OR ($2 NOT IN ('mine','saved') AND p.audience='public' AND c.published_revision IS NOT NULL))
   AND ($3='' OR $3=ANY(c.topics)) AND ($4='' OR p.author_id::text=$4)
   AND ($2='hidden')=EXISTS(SELECT 1 FROM community_feedback WHERE person_id=$1 AND post_id=p.id AND kind='hidden')
  ORDER BY COALESCE(c.published_at,p.created_at) DESC,p.id DESC LIMIT 500
 ), interest AS (
 SELECT unnest(cp.topics) AS topic,count(DISTINCT i.id)::integer AS weight FROM community_interactions i JOIN community_posts cp ON cp.post_id=i.post_id JOIN posts p ON p.id=cp.post_id WHERE $5 AND p.audience='public' AND cp.deleted_at IS NULL AND cp.published_revision IS NOT NULL AND (`+readableSQL+`) AND i.person_id=$1 AND i.kind IN ('view','complete') AND i.created_at>clock_timestamp()-interval '90 days' GROUP BY cp.topics
 ) SELECT c.id,c.author_id,c.created_at,c.topics,c.followed,
 (SELECT count(DISTINCT person_id) FROM post_reactions WHERE post_id=c.id AND created_at>clock_timestamp()-interval '48 hours'),
 (SELECT count(DISTINCT author_id) FROM post_comments WHERE post_id=c.id AND created_at>clock_timestamp()-interval '48 hours'),
 (SELECT count(*) FROM community_feedback WHERE post_id=c.id AND kind='saved'),
 (SELECT count(*) FROM community_interactions WHERE $5 AND person_id=$1 AND post_id=c.id AND kind='view' AND created_at>clock_timestamp()-interval '90 days'),
 COALESCE((SELECT sum(weight) FROM interest WHERE topic=ANY(c.topics)),0)
 FROM candidates c WHERE $2<>'following' OR followed`, person, mode, topic, author, personalized)
			if e != nil {
				fail(w, e)
				return
			}
			items = []candidate{}
			for rows.Next() {
				var c candidate
				if e = rows.Scan(&c.ID, &c.Author, &c.Created, &c.Topics, &c.Followed, &c.Likes, &c.Comments, &c.Saves, &c.Views, &c.Affinity); e != nil {
					break
				}
				items = append(items, c)
			}
			rows.Close()
			if e == nil {
				e = rows.Err()
			}
			if e != nil {
				fail(w, e)
				return
			}
		}
		if mode == "mine" || mode == "saved" || mode == "hidden" {
			for _, c := range items {
				ids = append(ids, c.ID)
			}
		} else if !ranked {
			ids = rank(items, mode, personalized, time.Now())
		}
		snapshot = uuid()
		if ids == nil {
			ids = []string{}
		}
		rankKey := fmt.Sprintf("%x", sha256.Sum256([]byte(strings.Join(ids, ":"))))
		// A refresh with the same ranking should not generate another row
		// version and WAL record. Renew only near expiry; ACLs stay live below.
		err = tx.QueryRow(ctx, `WITH existing AS MATERIALIZED (
 SELECT id FROM community_feeds WHERE person_id=$2 AND mode=$3 AND rank_key=$5 AND expires_at>statement_timestamp()+interval '1 minute'
), stored AS (
 INSERT INTO community_feeds(id,person_id,mode,post_ids,rank_key)
 SELECT $1,$2,$3,$4,$5 WHERE NOT EXISTS(SELECT 1 FROM existing)
 ON CONFLICT(person_id,mode,rank_key) WHERE rank_key IS NOT NULL DO UPDATE SET expires_at=EXCLUDED.expires_at RETURNING id
) SELECT id FROM existing UNION ALL SELECT id FROM stored LIMIT 1`, snapshot, person, mode, ids, rankKey).Scan(&snapshot)
		if err != nil {
			fail(w, err)
			return
		}
	}
	// Recheck feedback even when pagination uses an older ranking snapshot.
	rows, err := tx.Query(ctx, `SELECT post_id FROM community_feedback WHERE person_id=$1 AND kind='hidden' AND post_id=ANY($2::uuid[])`, person, ids)
	if err != nil {
		fail(w, err)
		return
	}
	hidden := map[string]bool{}
	for rows.Next() {
		var id string
		if err = rows.Scan(&id); err != nil {
			break
		}
		hidden[id] = true
	}
	rows.Close()
	if err == nil {
		err = rows.Err()
	}
	if err != nil {
		fail(w, err)
		return
	}
	posts := []Post{}
	for offset < len(ids) && len(posts) < 20 {
		end := min(offset+20-len(posts), len(ids))
		batch, e := readPosts(ctx, tx, person, ids[offset:end])
		offset = end
		if e != nil {
			fail(w, e)
			return
		}
		for _, p := range batch {
			if hidden[p.ID] != (mode == "hidden") {
				continue
			}
			// The author's own post stays in their feed while an edit waits for
			// review: it reads as their latest words under the «Đang chờ duyệt»
			// band, the way the post page shows it. It used to leave the
			// author's feed until the edit was approved (QA UI-142).
			if mode != "mine" && mode != "saved" && p.Audience != "public" {
				continue
			}
			p.Why = "Mới trong cộng đồng"
			if mode == "hidden" {
				p.Why = "Bạn đã chọn «Không quan tâm» cho bài này"
			} else if mode == "trending" {
				p.Why = "Được nhiều người tương tác gần đây"
			} else if mode == "following" || p.Following {
				p.Why = "Bạn đang theo dõi"
			} else if personalized {
				p.Why = "Theo chủ đề bạn thường xem và nội dung mới"
			}
			posts = append(posts, p)
		}
	}
	var next *string
	if offset < len(ids) {
		v := snapshot + ":" + strconv.Itoa(offset)
		next = &v
	}
	commit(w, r, tx, 200, map[string]any{"posts": posts, "next_cursor": next, "personalized": personalized, "rank_version": "community-v1"})
}
func (h *Handler) topics(w http.ResponseWriter, r *http.Request) {
	tx, person, err := h.begin(r)
	if err != nil {
		fail(w, err)
		return
	}
	defer tx.Rollback(r.Context())
	search := strings.ToLower(strings.TrimSpace(r.URL.Query().Get("q")))
	if len(search) > 100 {
		fail(w, no(422, "invalid_search"))
		return
	}
	rows, err := tx.Query(r.Context(), `SELECT topic,count(*) FROM community_posts c JOIN posts p ON p.id=c.post_id CROSS JOIN unnest(c.topics) topic WHERE p.audience='public' AND c.published_revision IS NOT NULL AND c.deleted_at IS NULL AND (`+readableSQL+`) AND strpos(topic,$2)>0 GROUP BY topic ORDER BY count(*) DESC,topic LIMIT 30`, person, search)
	if err != nil {
		fail(w, err)
		return
	}
	out := []map[string]any{}
	for rows.Next() {
		var s string
		var n int
		if err = rows.Scan(&s, &n); err != nil {
			break
		}
		out = append(out, map[string]any{"name": s, "posts": n})
	}
	rows.Close()
	if err == nil {
		err = rows.Err()
	}
	if err != nil {
		fail(w, err)
		return
	}
	commit(w, r, tx, 200, map[string]any{"topics": out})
}
func (h *Handler) preferences(w http.ResponseWriter, r *http.Request) {
	tx, person, err := h.begin(r)
	if err != nil {
		fail(w, err)
		return
	}
	defer tx.Rollback(r.Context())
	var in struct {
		Personalized bool `json:"personalized"`
		Asked        bool `json:"asked"`
	}
	if r.Method == "PUT" {
		if err = decode(w, r, &in); err != nil {
			fail(w, err)
			return
		}
		in.Asked = true
		_, err = tx.Exec(r.Context(), `INSERT INTO community_preferences VALUES($1,$2,true) ON CONFLICT(person_id) DO UPDATE SET personalized=$2,asked=true`, person, in.Personalized)
	} else {
		err = tx.QueryRow(r.Context(), `SELECT COALESCE((SELECT personalized FROM community_preferences WHERE person_id=$1),false),COALESCE((SELECT asked FROM community_preferences WHERE person_id=$1),false)`, person).Scan(&in.Personalized, &in.Asked)
	}
	if err != nil {
		fail(w, err)
		return
	}
	commit(w, r, tx, 200, in)
}
func (h *Handler) clearHistory(w http.ResponseWriter, r *http.Request) {
	tx, person, err := h.begin(r)
	if err != nil {
		fail(w, err)
		return
	}
	defer tx.Rollback(r.Context())
	// An exclusive preference lock orders deletion against in-flight events.
	_, err = tx.Exec(r.Context(), `INSERT INTO community_preferences(person_id,personalized,asked) VALUES($1,false,false) ON CONFLICT DO NOTHING`, person)
	if err == nil {
		_, err = tx.Exec(r.Context(), `SELECT person_id FROM community_preferences WHERE person_id=$1 FOR UPDATE`, person)
	}
	if err == nil {
		_, err = tx.Exec(r.Context(), `DELETE FROM community_interactions WHERE person_id=$1`, person)
	}
	if err == nil {
		_, err = tx.Exec(r.Context(), `DELETE FROM community_feeds WHERE person_id=$1`, person)
	}
	if err != nil {
		fail(w, err)
		return
	}
	commit(w, r, tx, 204, nil)
}
func (h *Handler) interactions(w http.ResponseWriter, r *http.Request) {
	var in struct {
		ID     string `json:"id"`
		PostID string `json:"post_id"`
		Kind   string `json:"kind"`
		Dwell  int    `json:"dwell_ms"`
	}
	if err := decode(w, r, &in); err != nil {
		fail(w, err)
		return
	}
	if !validID(in.ID) || !slices.Contains([]string{"impression", "view", "skip", "complete"}, in.Kind) || in.Dwell < 0 || in.Dwell > 180000 {
		fail(w, no(422, "invalid_interaction"))
		return
	}
	tx, person, err := h.begin(r)
	if err != nil {
		fail(w, err)
		return
	}
	defer tx.Rollback(r.Context())
	p, err := readPost(r.Context(), tx, person, in.PostID)
	if err != nil {
		fail(w, err)
		return
	}
	if p.Audience != "public" {
		fail(w, no(404, "post_not_found"))
		return
	}
	// Serialize consent changes and event writes on the preference row.
	var consent bool
	err = tx.QueryRow(r.Context(), `SELECT personalized FROM community_preferences WHERE person_id=$1 FOR SHARE`, person).Scan(&consent)
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		fail(w, err)
		return
	}
	if !consent {
		commit(w, r, tx, 204, nil)
		return
	}
	_, err = tx.Exec(r.Context(), `SELECT pg_advisory_xact_lock(hashtextextended($1,73413802))`, person+":"+in.PostID+":"+in.Kind)
	if err == nil {
		_, err = tx.Exec(r.Context(), `INSERT INTO community_interactions(id,person_id,post_id,kind,dwell_ms) SELECT $1,$2,$3,$4,$5 WHERE NOT EXISTS(SELECT 1 FROM community_interactions WHERE person_id=$2 AND post_id=$3 AND kind=$4 AND created_at>clock_timestamp()-interval '1 minute') ON CONFLICT DO NOTHING`, in.ID, person, in.PostID, in.Kind, in.Dwell)
	}
	if err != nil {
		fail(w, err)
		return
	}
	commit(w, r, tx, 204, nil)
}
