package store

import (
	"context"
	"time"

	"github.com/forumline/forum-server/forum/model"
)

// ListBookmarks returns a user's bookmarks with full thread data.
func (s *Store) ListBookmarks(ctx context.Context, userID string) ([]model.Bookmark, error) {
	rows, err := s.DB.Query(ctx,
		`SELECT b.id, b.created_at,
		        t.id, t.category_id, t.author_id, t.title, t.slug, t.content, t.image_url,
		        t.is_pinned, t.is_locked, t.view_count, t.post_count,
		        COALESCE(t.last_post_at, t.created_at), t.created_at, t.updated_at,
		        p.id, p.username, p.display_name, p.avatar_url, p.bio, p.website,
		        p.is_admin, p.forumline_id, p.created_at, p.updated_at,
		        c.id, c.name, c.slug, c.description, c.sort_order, c.created_at
		 FROM bookmarks b
		 JOIN threads t ON t.id = b.thread_id
		 JOIN profiles p ON p.id = t.author_id
		 JOIN categories c ON c.id = t.category_id
		 WHERE b.user_id = $1
		 ORDER BY b.created_at DESC`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var bookmarks []model.Bookmark
	for rows.Next() {
		var bm model.Bookmark
		var bmCreatedAt time.Time

		var t model.Thread
		var lastPostAt, tCreatedAt, tUpdatedAt time.Time
		var authorCreatedAt, authorUpdatedAt time.Time
		var catCreatedAt time.Time

		err := rows.Scan(
			&bm.ID, &bmCreatedAt,
			&t.ID, &t.CategoryID, &t.AuthorID, &t.Title, &t.Slug, &t.Content, &t.ImageURL,
			&t.IsPinned, &t.IsLocked, &t.ViewCount, &t.PostCount,
			&lastPostAt, &tCreatedAt, &tUpdatedAt,
			&t.Author.ID, &t.Author.Username, &t.Author.DisplayName, &t.Author.AvatarURL,
			&t.Author.Bio, &t.Author.Website, &t.Author.IsAdmin, &t.Author.ForumlineID,
			&authorCreatedAt, &authorUpdatedAt,
			&t.Category.ID, &t.Category.Name, &t.Category.Slug, &t.Category.Description,
			&t.Category.SortOrder, &catCreatedAt,
		)
		if err != nil {
			return nil, err
		}

		bm.CreatedAt = bmCreatedAt.Format(time.RFC3339)
		t.CreatedAt = tCreatedAt.Format(time.RFC3339)
		t.UpdatedAt = tUpdatedAt.Format(time.RFC3339)
		if !lastPostAt.IsZero() {
			s := lastPostAt.Format(time.RFC3339)
			t.LastPostAt = &s
		}
		t.Author.CreatedAt = authorCreatedAt.Format(time.RFC3339)
		t.Author.UpdatedAt = authorUpdatedAt.Format(time.RFC3339)
		t.Category.CreatedAt = catCreatedAt.Format(time.RFC3339)
		bm.Thread = t

		bookmarks = append(bookmarks, bm)
	}
	if bookmarks == nil {
		bookmarks = []model.Bookmark{}
	}
	return bookmarks, nil
}

// GetBookmarkStatus returns the bookmark ID if it exists.
func (s *Store) GetBookmarkStatus(ctx context.Context, userID, threadID string) (*string, error) {
	var id *string
	err := s.DB.QueryRow(ctx,
		`SELECT id FROM bookmarks WHERE user_id = $1 AND thread_id = $2`,
		userID, threadID).Scan(&id)
	if err != nil {
		return nil, err
	}
	return id, nil
}

// AddBookmark adds a bookmark.
func (s *Store) AddBookmark(ctx context.Context, userID, threadID string) error {
	_, err := s.DB.Exec(ctx,
		`INSERT INTO bookmarks (user_id, thread_id) VALUES ($1, $2) ON CONFLICT DO NOTHING`,
		userID, threadID)
	return err
}

// RemoveBookmark removes a bookmark by thread ID.
func (s *Store) RemoveBookmark(ctx context.Context, userID, threadID string) error {
	_, err := s.DB.Exec(ctx,
		`DELETE FROM bookmarks WHERE user_id = $1 AND thread_id = $2`, userID, threadID)
	return err
}

// RemoveBookmarkByID removes a bookmark by bookmark ID.
func (s *Store) RemoveBookmarkByID(ctx context.Context, userID, bookmarkID string) error {
	_, err := s.DB.Exec(ctx,
		`DELETE FROM bookmarks WHERE id = $1 AND user_id = $2`, bookmarkID, userID)
	return err
}
