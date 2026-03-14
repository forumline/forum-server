package store

import (
	"time"

	"github.com/forumline/forum-server/forum/model"
	shared "github.com/forumline/forumline/shared-go"
)

// Store provides data access methods for the forum database.
// DB is the shared.DB interface — in single-tenant mode this is a *pgxpool.Pool,
// in multi-tenant mode this is a *TenantPool that sets search_path per-request.
type Store struct {
	DB shared.DB
}

// New creates a new Store.
func New(db shared.DB) *Store {
	return &Store{DB: db}
}

// profileColumns is the column list for scanning profiles.
const profileColumns = `id, username, display_name, avatar_url, bio, website, is_admin, forumline_id, created_at, updated_at`

// scanProfile scans a profile row into a model.Profile.
func scanProfile(scan func(dest ...interface{}) error) (model.Profile, error) {
	var p model.Profile
	var createdAt, updatedAt time.Time
	err := scan(&p.ID, &p.Username, &p.DisplayName, &p.AvatarURL, &p.Bio, &p.Website,
		&p.IsAdmin, &p.ForumlineID, &createdAt, &updatedAt)
	if err != nil {
		return p, err
	}
	p.CreatedAt = createdAt.Format(time.RFC3339)
	p.UpdatedAt = updatedAt.Format(time.RFC3339)
	return p, nil
}

// threadWithJoinsQuery is the base SQL for fetching threads with author and category.
const threadWithJoinsQuery = `
SELECT t.id, t.category_id, t.author_id, t.title, t.slug, t.content, t.image_url,
       t.is_pinned, t.is_locked, t.view_count, t.post_count,
       COALESCE(t.last_post_at, t.created_at), t.created_at, t.updated_at,
       p.id, p.username, p.display_name, p.avatar_url, p.bio, p.website,
       p.is_admin, p.forumline_id, p.created_at, p.updated_at,
       c.id, c.name, c.slug, c.description, c.sort_order, c.created_at
FROM threads t
JOIN profiles p ON p.id = t.author_id
JOIN categories c ON c.id = t.category_id`

// scanThreadWithAuthor scans a thread row (with joined author and category).
func scanThreadWithAuthor(scan func(dest ...interface{}) error) (model.Thread, error) {
	var t model.Thread
	var lastPostAt, createdAt, updatedAt time.Time
	var authorCreatedAt, authorUpdatedAt time.Time
	var catCreatedAt time.Time

	err := scan(
		&t.ID, &t.CategoryID, &t.AuthorID, &t.Title, &t.Slug, &t.Content, &t.ImageURL,
		&t.IsPinned, &t.IsLocked, &t.ViewCount, &t.PostCount, &lastPostAt, &createdAt, &updatedAt,
		&t.Author.ID, &t.Author.Username, &t.Author.DisplayName, &t.Author.AvatarURL,
		&t.Author.Bio, &t.Author.Website, &t.Author.IsAdmin, &t.Author.ForumlineID,
		&authorCreatedAt, &authorUpdatedAt,
		&t.Category.ID, &t.Category.Name, &t.Category.Slug, &t.Category.Description,
		&t.Category.SortOrder, &catCreatedAt,
	)
	if err != nil {
		return t, err
	}

	t.CreatedAt = createdAt.Format(time.RFC3339)
	t.UpdatedAt = updatedAt.Format(time.RFC3339)
	if !lastPostAt.IsZero() {
		s := lastPostAt.Format(time.RFC3339)
		t.LastPostAt = &s
	}
	t.Author.CreatedAt = authorCreatedAt.Format(time.RFC3339)
	t.Author.UpdatedAt = authorUpdatedAt.Format(time.RFC3339)
	t.Category.CreatedAt = catCreatedAt.Format(time.RFC3339)

	return t, nil
}

// postWithJoinsQuery is the base SQL for fetching posts with author.
const postWithJoinsQuery = `
SELECT po.id, po.thread_id, po.author_id, po.content, po.reply_to_id, po.created_at, po.updated_at,
       p.id, p.username, p.display_name, p.avatar_url, p.bio, p.website,
       p.is_admin, p.forumline_id, p.created_at, p.updated_at
FROM posts po
JOIN profiles p ON p.id = po.author_id`

// scanPostWithAuthor scans a post row (with joined author).
func scanPostWithAuthor(scan func(dest ...interface{}) error) (model.Post, error) {
	var po model.Post
	var createdAt, updatedAt time.Time
	var authorCreatedAt, authorUpdatedAt time.Time

	err := scan(
		&po.ID, &po.ThreadID, &po.AuthorID, &po.Content, &po.ReplyToID, &createdAt, &updatedAt,
		&po.Author.ID, &po.Author.Username, &po.Author.DisplayName, &po.Author.AvatarURL,
		&po.Author.Bio, &po.Author.Website, &po.Author.IsAdmin, &po.Author.ForumlineID,
		&authorCreatedAt, &authorUpdatedAt,
	)
	if err != nil {
		return po, err
	}

	po.CreatedAt = createdAt.Format(time.RFC3339)
	po.UpdatedAt = updatedAt.Format(time.RFC3339)
	po.Author.CreatedAt = authorCreatedAt.Format(time.RFC3339)
	po.Author.UpdatedAt = authorUpdatedAt.Format(time.RFC3339)
	return po, nil
}

// ProfileColumns returns the profile column list for use in SSE handlers.
func ProfileColumns() string {
	return profileColumns
}

// ScanProfile scans a profile row — exported for SSE handler use.
func ScanProfile(scan func(dest ...interface{}) error) (model.Profile, error) {
	return scanProfile(scan)
}
