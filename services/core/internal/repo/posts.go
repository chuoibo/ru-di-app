package repo

import (
	"context"
	"errors"
	"sort"
	"strconv"
	"time"

	"github.com/jackc/pgx/v5"
)

// Post is PostRecord.
type Post struct {
	ID        string
	AuthorID  string
	Audience  string
	ContextID *string
	Body      string
	ImageURL  *string
	CreatedAt time.Time
}

// PostInput is create_post's keyword arguments.
type PostInput struct {
	AuthorID  string
	Audience  string
	ContextID *string
	Body      string
	ImageURL  *string
	Now       time.Time
}

var postAudiences = map[string]bool{"only_me": true, "friends": true, "group": true, "public": true}

const postColumns = `posts.id, posts.author_id, posts.audience, posts.context_id, posts.body, posts.image_url, posts.created_at`

func scanPost(row pgx.Row) (*Post, error) {
	var p Post
	err := row.Scan(&p.ID, &p.AuthorID, &p.Audience, &p.ContextID, &p.Body, &p.ImageURL, &p.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	p.CreatedAt = p.CreatedAt.UTC()
	return &p, nil
}

// CreatePost is create_post: `PostAudience(audience)` refuses an unknown
// audience before any statement (ErrUnknownPostAudience, a ValueError), then
// one INSERT with a client-side uuid4 and the caller's clock. The record
// carries the values handed in. The CHECKs and foreign keys refuse in
// PostgreSQL and that *pgconn.PgError is returned as is.
func (r Repository) CreatePost(ctx context.Context, in PostInput) (Post, error) {
	if !postAudiences[in.Audience] {
		return Post{}, ErrUnknownPostAudience
	}
	id, err := newUUID()
	if err != nil {
		return Post{}, err
	}
	created := pythonInstant(in.Now)
	if _, err := r.Q.Exec(ctx,
		`INSERT INTO posts (id, author_id, audience, context_id, body, image_url, created_at)
		 VALUES ($1::UUID, $2::UUID, $3, $4::UUID, $5::VARCHAR, $6::VARCHAR, $7::TIMESTAMP WITH TIME ZONE)`,
		id, in.AuthorID, in.Audience, in.ContextID, in.Body, in.ImageURL, created); err != nil {
		return Post{}, err
	}
	return Post{ID: id, AuthorID: in.AuthorID, Audience: in.Audience, ContextID: in.ContextID, Body: in.Body,
		ImageURL: in.ImageURL, CreatedAt: created}, nil
}

// GetPost is get_post: `session.get(Post, id)`, whoever may read it.
//
// SQLAlchemy note: a second session.get of the same id in one session answers
// from the identity map without a statement; Go reads again.
func (r Repository) GetPost(ctx context.Context, postID string) (*Post, error) {
	return scanPost(r.Q.QueryRow(ctx,
		`SELECT posts.id AS posts_id, posts.author_id AS posts_author_id, posts.audience AS posts_audience,
		        posts.context_id AS posts_context_id, posts.body AS posts_body,
		        posts.image_url AS posts_image_url, posts.created_at AS posts_created_at
		   FROM posts
		  WHERE posts.id = $1::UUID`, postID))
}

// readableBy is `_readable_by(reader_id)` as SQLAlchemy renders it, with
// SQLAlchemy's precedence (no parentheses around the AND arms) and its bind
// sharing: the `blocked` subquery is one object used by two arms, so both
// copies bind the same three parameters, here the same $n.
func readableBy(bind func(any) string, readerID string) string {
	author := bind(readerID)
	blockedState, blockedRequester, blockedAddressee := bind("blocked"), bind(readerID), bind(readerID)
	blocked := `NOT (EXISTS (SELECT friend_requests.id FROM friend_requests
	                   WHERE friend_requests.state = ` + blockedState + `
	                     AND (friend_requests.requester_id = ` + blockedRequester + `::UUID AND friend_requests.addressee_id = posts.author_id
	                          OR friend_requests.addressee_id = ` + blockedAddressee + `::UUID AND friend_requests.requester_id = posts.author_id)))`
	public := bind("public")
	friends := bind("friends")
	acceptedState, acceptedRequester, acceptedAddressee := bind("accepted"), bind(readerID), bind(readerID)
	group := bind("group")
	member, active := bind(readerID), bind("active")
	return `posts.author_id = ` + author + `::UUID
	     OR ` + blocked + ` AND posts.audience = ` + public + `
	     OR ` + blocked + ` AND posts.audience = ` + friends + `
	        AND (EXISTS (SELECT friend_requests.id FROM friend_requests
	                      WHERE friend_requests.state = ` + acceptedState + `
	                        AND (friend_requests.requester_id = ` + acceptedRequester + `::UUID AND friend_requests.addressee_id = posts.author_id
	                             OR friend_requests.addressee_id = ` + acceptedAddressee + `::UUID AND friend_requests.requester_id = posts.author_id)))
	     OR posts.audience = ` + group + ` AND posts.context_id IS NOT NULL
	        AND (EXISTS (SELECT memberships.id FROM memberships
	                      WHERE memberships.context_id = posts.context_id AND memberships.person_id = ` + member + `::UUID
	                        AND memberships.state = ` + active + ` AND memberships.left_at IS NULL))`
}

func (r Repository) posts(ctx context.Context, sql string, args []any) ([]Post, error) {
	rows, err := r.Q.Query(ctx, sql, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Post{}
	for rows.Next() {
		p, err := scanPost(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, *p)
	}
	return out, rows.Err()
}

func bindArgs(args *[]any) func(any) string {
	return func(value any) string {
		*args = append(*args, value)
		return "$" + strconv.Itoa(len(*args))
	}
}

// ListPostsVisibleTo is list_posts_visible_to: every post `_readable_by` the
// reader, newest first by (created_at DESC, id DESC), LIMIT as given (a
// negative limit fails in PostgreSQL, as in Python).
func (r Repository) ListPostsVisibleTo(ctx context.Context, readerID string, limit int) ([]Post, error) {
	var args []any
	bind := bindArgs(&args)
	where := readableBy(bind, readerID)
	return r.posts(ctx, `SELECT `+postColumns+` FROM posts WHERE `+where+`
		ORDER BY posts.created_at DESC, posts.id DESC LIMIT `+bind(limit)+`::INTEGER`, args)
}

// ListPersonPostsVisibleTo is list_person_posts_visible_to: the same with the
// author fixed.
func (r Repository) ListPersonPostsVisibleTo(ctx context.Context, personID, readerID string, limit int) ([]Post, error) {
	var args []any
	bind := bindArgs(&args)
	author := bind(personID)
	where := readableBy(bind, readerID)
	return r.posts(ctx, `SELECT `+postColumns+` FROM posts WHERE posts.author_id = `+author+`::UUID AND (`+where+`)
		ORDER BY posts.created_at DESC, posts.id DESC LIMIT `+bind(limit)+`::INTEGER`, args)
}

// PostKindCount is one entry of a post's reactions-per-kind dict.
type PostKindCount struct {
	Kind  string
	Count int64
}

// PostReactionTally is one entry of PostSocialCounts.reactions.
type PostReactionTally struct {
	PostID string
	Kinds  []PostKindCount
}

// PostCommentTally is one entry of PostSocialCounts.comments.
type PostCommentTally struct {
	PostID string
	Count  int64
}

// PostViewerKinds is one entry of PostSocialCounts.mine: the viewer's kinds as
// a set, held sorted because a frozenset has no order.
type PostViewerKinds struct {
	PostID string
	Kinds  []string
}

// PostSocialCounts is PostSocialCounts. Every dict is a slice in the dict's
// insertion order, which is the order PostgreSQL returned the rows in.
type PostSocialCounts struct {
	Reactions []PostReactionTally
	Comments  []PostCommentTally
	Mine      []PostViewerKinds
}

// PostSocialCounts is post_social_counts: no ids, no statement; otherwise
// three reads in Python's order (reactions grouped by post and kind, comments
// grouped by post, the viewer's own reactions), none with ORDER BY. The IN
// list has one parameter per id as passed, duplicates included.
func (r Repository) PostSocialCounts(ctx context.Context, postIDs []string, viewerID string) (PostSocialCounts, error) {
	out := PostSocialCounts{Reactions: []PostReactionTally{}, Comments: []PostCommentTally{}, Mine: []PostViewerKinds{}}
	if len(postIDs) == 0 {
		return out, nil
	}
	in := uuidPlaceholders(1, len(postIDs))
	rows, err := r.Q.Query(ctx,
		`SELECT post_reactions.post_id, post_reactions.kind, count(post_reactions.id) AS count_1
		   FROM post_reactions
		  WHERE post_reactions.post_id IN (`+in+`)
		  GROUP BY post_reactions.post_id, post_reactions.kind`, uuidArgs(postIDs)...)
	if err != nil {
		return out, err
	}
	reactionAt := map[string]int{}
	for rows.Next() {
		var post, kind string
		var total int64
		if err := rows.Scan(&post, &kind, &total); err != nil {
			rows.Close()
			return out, err
		}
		i, seen := reactionAt[post]
		if !seen {
			i = len(out.Reactions)
			reactionAt[post] = i
			out.Reactions = append(out.Reactions, PostReactionTally{PostID: post})
		}
		tally := &out.Reactions[i]
		replaced := false
		for k := range tally.Kinds {
			if tally.Kinds[k].Kind == kind {
				tally.Kinds[k].Count, replaced = total, true
			}
		}
		if !replaced {
			tally.Kinds = append(tally.Kinds, PostKindCount{Kind: kind, Count: total})
		}
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return out, err
	}

	rows, err = r.Q.Query(ctx,
		`SELECT post_comments.post_id, count(post_comments.id) AS count_1
		   FROM post_comments
		  WHERE post_comments.post_id IN (`+in+`)
		  GROUP BY post_comments.post_id`, uuidArgs(postIDs)...)
	if err != nil {
		return out, err
	}
	commentAt := map[string]int{}
	for rows.Next() {
		var post string
		var total int64
		if err := rows.Scan(&post, &total); err != nil {
			rows.Close()
			return out, err
		}
		if i, seen := commentAt[post]; seen {
			out.Comments[i].Count = total
			continue
		}
		commentAt[post] = len(out.Comments)
		out.Comments = append(out.Comments, PostCommentTally{PostID: post, Count: total})
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return out, err
	}

	rows, err = r.Q.Query(ctx,
		`SELECT post_reactions.post_id, post_reactions.kind
		   FROM post_reactions
		  WHERE post_reactions.post_id IN (`+in+`) AND post_reactions.person_id = $`+strconv.Itoa(len(postIDs)+1)+`::UUID`,
		append(uuidArgs(postIDs), viewerID)...)
	if err != nil {
		return out, err
	}
	mineAt := map[string]int{}
	for rows.Next() {
		var post, kind string
		if err := rows.Scan(&post, &kind); err != nil {
			rows.Close()
			return out, err
		}
		i, seen := mineAt[post]
		if !seen {
			i = len(out.Mine)
			mineAt[post] = i
			out.Mine = append(out.Mine, PostViewerKinds{PostID: post})
		}
		if !contains(out.Mine[i].Kinds, kind) {
			out.Mine[i].Kinds = append(out.Mine[i].Kinds, kind)
		}
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return out, err
	}
	for i := range out.Mine {
		sort.Strings(out.Mine[i].Kinds)
	}
	return out, nil
}

func contains(values []string, value string) bool {
	for _, v := range values {
		if v == value {
			return true
		}
	}
	return false
}

// PostReaction is PostReactionRecord.
type PostReaction struct {
	ID        string
	PostID    string
	PersonID  string
	Kind      string
	CreatedAt time.Time
}

// reactionSavepoint is the name SQLAlchemy gives the first savepoint of a
// connection (`sa_savepoint_1`); begin_nested in add_post_reaction is the
// first one a W2 request opens. A second begin_nested on the same Python
// session would be `sa_savepoint_2`.
const reactionSavepoint = "sa_savepoint_1"

func (r Repository) postReaction(ctx context.Context, postID, personID, kind string) (*PostReaction, error) {
	var p PostReaction
	err := r.Q.QueryRow(ctx,
		`SELECT post_reactions.id, post_reactions.post_id, post_reactions.person_id, post_reactions.kind,
		        post_reactions.created_at
		   FROM post_reactions
		  WHERE post_reactions.post_id = $1::UUID AND post_reactions.person_id = $2::UUID
		    AND post_reactions.kind = $3::VARCHAR`, postID, personID, kind).
		Scan(&p.ID, &p.PostID, &p.PersonID, &p.Kind, &p.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	p.CreatedAt = p.CreatedAt.UTC()
	return &p, nil
}

// AddPostReaction is add_post_reaction: the INSERT inside a savepoint
// (SAVEPOINT, INSERT, RELEASE SAVEPOINT). When the INSERT fails the savepoint
// is rolled back first; a unique violation on uq_post_reactions_one_per_kind
// then reads the row that is already there and answers it, and any other
// failure (the kind CHECK, a foreign key, a kind longer than varchar(16)) is
// returned as is, with the transaction still usable.
func (r Repository) AddPostReaction(ctx context.Context, postID, personID, kind string, now time.Time) (PostReaction, error) {
	id, err := newUUID()
	if err != nil {
		return PostReaction{}, err
	}
	created := pythonInstant(now)
	if _, err := r.Q.Exec(ctx, `SAVEPOINT `+reactionSavepoint); err != nil {
		return PostReaction{}, err
	}
	_, insertErr := r.Q.Exec(ctx,
		`INSERT INTO post_reactions (id, post_id, person_id, kind, created_at)
		 VALUES ($1::UUID, $2::UUID, $3::UUID, $4::VARCHAR, $5::TIMESTAMP WITH TIME ZONE)`,
		id, postID, personID, kind, created)
	if insertErr == nil {
		if _, err := r.Q.Exec(ctx, `RELEASE SAVEPOINT `+reactionSavepoint); err != nil {
			return PostReaction{}, err
		}
		return PostReaction{ID: id, PostID: postID, PersonID: personID, Kind: kind, CreatedAt: created}, nil
	}
	if _, err := r.Q.Exec(ctx, `ROLLBACK TO SAVEPOINT `+reactionSavepoint); err != nil {
		return PostReaction{}, err
	}
	pg := integrityViolation(insertErr)
	if pg == nil || pg.ConstraintName != "uq_post_reactions_one_per_kind" {
		return PostReaction{}, insertErr
	}
	existing, err := r.postReaction(ctx, postID, personID, kind)
	if err != nil {
		return PostReaction{}, err
	}
	if existing == nil {
		return PostReaction{}, ErrReactionVanished
	}
	return *existing, nil
}

// RemovePostReaction is remove_post_reaction: the person's row of that kind,
// then one DELETE by primary key; false and no DELETE when there is none.
func (r Repository) RemovePostReaction(ctx context.Context, postID, personID, kind string) (bool, error) {
	existing, err := r.postReaction(ctx, postID, personID, kind)
	if err != nil || existing == nil {
		return false, err
	}
	if _, err := r.Q.Exec(ctx, `DELETE FROM post_reactions WHERE post_reactions.id = $1::UUID`, existing.ID); err != nil {
		return false, err
	}
	return true, nil
}

// PostComment is PostCommentRecord.
type PostComment struct {
	ID                string
	PostID            string
	AuthorID          string
	AuthorDisplayName string
	Body              string
	CreatedAt         time.Time
}

// CreatePostComment is create_post_comment: one INSERT with a client-side
// uuid4 and the caller's clock, then the author's name.
func (r Repository) CreatePostComment(ctx context.Context, postID, authorID, body string, now time.Time) (PostComment, error) {
	id, err := newUUID()
	if err != nil {
		return PostComment{}, err
	}
	created := pythonInstant(now)
	if _, err := r.Q.Exec(ctx,
		`INSERT INTO post_comments (id, post_id, author_id, body, created_at)
		 VALUES ($1::UUID, $2::UUID, $3::UUID, $4::VARCHAR, $5::TIMESTAMP WITH TIME ZONE)`,
		id, postID, authorID, body, created); err != nil {
		return PostComment{}, err
	}
	names, err := r.displayNames(ctx, []string{authorID})
	if err != nil {
		return PostComment{}, err
	}
	return PostComment{ID: id, PostID: postID, AuthorID: authorID, AuthorDisplayName: names[authorID],
		Body: body, CreatedAt: created}, nil
}

// postCommentByID is `session.get(PostComment, id)`, labelled table_column.
func (r Repository) postCommentByID(ctx context.Context, commentID string) (*PostComment, error) {
	var c PostComment
	err := r.Q.QueryRow(ctx,
		`SELECT post_comments.id AS post_comments_id, post_comments.post_id AS post_comments_post_id,
		        post_comments.author_id AS post_comments_author_id, post_comments.body AS post_comments_body,
		        post_comments.created_at AS post_comments_created_at
		   FROM post_comments
		  WHERE post_comments.id = $1::UUID`, commentID).
		Scan(&c.ID, &c.PostID, &c.AuthorID, &c.Body, &c.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	c.CreatedAt = c.CreatedAt.UTC()
	return &c, nil
}

// GetPostComment is get_post_comment: the comment by id, then its author's
// name.
func (r Repository) GetPostComment(ctx context.Context, commentID string) (*PostComment, error) {
	c, err := r.postCommentByID(ctx, commentID)
	if err != nil || c == nil {
		return nil, err
	}
	names, err := r.displayNames(ctx, []string{c.AuthorID})
	if err != nil {
		return nil, err
	}
	c.AuthorDisplayName = names[c.AuthorID]
	return c, nil
}

// DeletePostComment is delete_post_comment: `session.get`, then one DELETE by
// primary key; false and no DELETE when there is no row.
func (r Repository) DeletePostComment(ctx context.Context, commentID string) (bool, error) {
	c, err := r.postCommentByID(ctx, commentID)
	if err != nil || c == nil {
		return false, err
	}
	if _, err := r.Q.Exec(ctx, `DELETE FROM post_comments WHERE post_comments.id = $1::UUID`, c.ID); err != nil {
		return false, err
	}
	return true, nil
}

// PostCommentCursor is list_post_comments' `after` tuple.
type PostCommentCursor struct {
	CreatedAt time.Time
	ID        string
}

// ListPostComments is list_post_comments: oldest first by (created_at, id),
// strictly after the cursor when one is given, LIMIT as given (the service
// passes limit + 1; a negative limit fails in PostgreSQL), then one
// `_display_names` statement for the distinct authors (none without rows).
func (r Repository) ListPostComments(ctx context.Context, postID string, limit int, after *PostCommentCursor) ([]PostComment, error) {
	args := []any{postID}
	bind := bindArgs(&args)
	sql := `SELECT post_comments.id, post_comments.post_id, post_comments.author_id, post_comments.body,
	               post_comments.created_at
	          FROM post_comments
	         WHERE post_comments.post_id = $1::UUID`
	if after != nil {
		sql += ` AND (post_comments.created_at, post_comments.id) > (` +
			bind(after.CreatedAt) + `::TIMESTAMP WITH TIME ZONE, ` + bind(after.ID) + `::UUID)`
	}
	sql += ` ORDER BY post_comments.created_at, post_comments.id LIMIT ` + bind(limit) + `::INTEGER`
	rows, err := r.Q.Query(ctx, sql, args...)
	if err != nil {
		return nil, err
	}
	out := []PostComment{}
	for rows.Next() {
		var c PostComment
		if err := rows.Scan(&c.ID, &c.PostID, &c.AuthorID, &c.Body, &c.CreatedAt); err != nil {
			rows.Close()
			return nil, err
		}
		c.CreatedAt = c.CreatedAt.UTC()
		out = append(out, c)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return nil, err
	}
	authors := make([]string, len(out))
	for i, c := range out {
		authors[i] = c.AuthorID
	}
	names, err := r.displayNames(ctx, authors)
	if err != nil {
		return nil, err
	}
	for i := range out {
		out[i].AuthorDisplayName = names[out[i].AuthorID]
	}
	return out, nil
}
