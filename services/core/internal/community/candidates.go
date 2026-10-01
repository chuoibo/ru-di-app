package community

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5"
)

// discoveryRanking caches only public ranking inputs. Every returned page
// still reads through current ACLs; this cache never grants access to a post.
func (h *Handler) discoveryRanking(ctx context.Context, tx pgx.Tx, person, mode string, personalized bool) ([]string, error) {
	h.candidateMu.Lock()
	if time.Now().After(h.candidateUntil) {
		rows, err := tx.Query(ctx, `WITH eligible AS MATERIALIZED (
 SELECT p.id,p.author_id,c.topics,c.published_at FROM community_posts c JOIN posts p ON p.id=c.post_id JOIN people a ON a.id=p.author_id
 WHERE c.deleted_at IS NULL AND a.deleted_at IS NULL AND p.audience='public' AND c.published_revision IS NOT NULL AND c.published_at IS NOT NULL ORDER BY c.published_at DESC,p.id DESC LIMIT 500
), likes AS (
 SELECT post_id,count(DISTINCT person_id) AS total FROM post_reactions WHERE post_id IN (SELECT id FROM eligible) AND created_at>statement_timestamp()-interval '48 hours' GROUP BY post_id
), comments AS (
 SELECT post_id,count(DISTINCT author_id) AS total FROM post_comments WHERE post_id IN (SELECT id FROM eligible) AND created_at>statement_timestamp()-interval '48 hours' GROUP BY post_id
), saves AS (
 SELECT post_id,count(*) AS total FROM community_feedback WHERE kind='saved' AND post_id IN (SELECT id FROM eligible) GROUP BY post_id
) SELECT e.id,e.author_id,e.topics,e.published_at,COALESCE(l.total,0),COALESCE(c.total,0),COALESCE(s.total,0)
 FROM eligible e LEFT JOIN likes l ON l.post_id=e.id LEFT JOIN comments c ON c.post_id=e.id LEFT JOIN saves s ON s.post_id=e.id`)
		if err != nil {
			h.candidateMu.Unlock()
			return nil, err
		}
		items := []candidate{}
		for rows.Next() {
			var c candidate
			if err = rows.Scan(&c.ID, &c.Author, &c.Topics, &c.Created, &c.Likes, &c.Comments, &c.Saves); err != nil {
				break
			}
			items = append(items, c)
		}
		rows.Close()
		if err == nil {
			err = rows.Err()
		}
		if err != nil {
			h.candidateMu.Unlock()
			return nil, err
		}
		h.candidates = items
		// Non-personalized discovery shares the ranking of this one-second
		// public candidate snapshot. Author/topic following is still ranked per
		// reader, and all page content is rechecked against live permissions.
		h.commonRanking = rank(append([]candidate(nil), items...), "trending", false, time.Now())
		h.candidateUntil = time.Now().Add(time.Second)
	}
	if mode != "following" && (!personalized || mode == "trending") {
		ids := banSaoXepHang(h.commonRanking)
		h.candidateMu.Unlock()
		return ids, nil
	}
	items := append([]candidate(nil), h.candidates...)
	h.candidateMu.Unlock()
	people, topics := map[string]bool{}, map[string]bool{}
	rows, err := tx.Query(ctx, `SELECT kind,target FROM community_follows WHERE person_id=$1`, person)
	if err != nil {
		return nil, err
	}
	for rows.Next() {
		var kind, target string
		if err = rows.Scan(&kind, &target); err != nil {
			break
		}
		if kind == "person" {
			people[target] = true
		} else {
			topics[target] = true
		}
	}
	rows.Close()
	if err == nil {
		err = rows.Err()
	}
	if err != nil {
		return nil, err
	}
	weights, views := map[string]int{}, map[string]int{}
	if personalized && mode == "for_you" {
		rows, err = tx.Query(ctx, `SELECT i.post_id,c.topics,count(*)::integer FROM community_interactions i JOIN community_posts c ON c.post_id=i.post_id JOIN posts p ON p.id=c.post_id WHERE i.person_id=$1 AND i.kind IN('view','complete') AND i.created_at>clock_timestamp()-interval '90 days' AND p.audience='public' AND c.deleted_at IS NULL AND (`+readableSQL+`) GROUP BY i.post_id,c.topics`, person)
		if err != nil {
			return nil, err
		}
		for rows.Next() {
			var id string
			var tags []string
			var n int
			if err = rows.Scan(&id, &tags, &n); err != nil {
				break
			}
			views[id] = n
			for _, tag := range tags {
				weights[tag] += n
			}
		}
		rows.Close()
		if err == nil {
			err = rows.Err()
		}
		if err != nil {
			return nil, err
		}
	}
	out := make([]candidate, 0, len(items))
	for _, c := range items {
		c.Followed = people[c.Author]
		c.Views = views[c.ID]
		for _, tag := range c.Topics {
			c.Followed = c.Followed || topics[tag]
			c.Affinity += weights[tag]
		}
		if mode != "following" || c.Followed {
			out = append(out, c)
		}
	}
	return rank(out, mode, personalized, time.Now()), nil
}

// banSaoXepHang copies the shared ranking for one reader. Never nil, even when
// no public post exists yet: the snapshot row stores the ids in
// community_feeds.post_ids, which is NOT NULL, and pgx writes a nil slice as
// NULL.
func banSaoXepHang(ids []string) []string {
	return append(make([]string, 0, len(ids)), ids...)
}
