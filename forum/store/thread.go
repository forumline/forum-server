package store

import (
	"context"
	"fmt"
	"time"

	"github.com/forumline/forum-server/forum/model"
)

// ListThreads returns threads ordered by pinned + last_post_at.
func (s *Store) ListThreads(ctx context.Context, limit int) ([]model.Thread, error) {
	rows, err := s.DB.Query(ctx,
		threadWithJoinsQuery+` ORDER BY t.is_pinned DESC, t.last_post_at DESC NULLS LAST LIMIT $1`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanThreadRows(rows)
}

// GetThread returns a single thread by ID.
func (s *Store) GetThread(ctx context.Context, id string) (*model.Thread, error) {
	row := s.DB.QueryRow(ctx, threadWithJoinsQuery+` WHERE t.id = $1`, id)
	t, err := scanThreadWithAuthor(row.Scan)
	if err != nil {
		return nil, err
	}
	return &t, nil
}

// ListThreadsByCategory returns threads for a category slug.
func (s *Store) ListThreadsByCategory(ctx context.Context, slug string) ([]model.Thread, error) {
	rows, err := s.DB.Query(ctx,
		threadWithJoinsQuery+` WHERE c.slug = $1 ORDER BY t.is_pinned DESC, t.last_post_at DESC NULLS LAST`, slug)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanThreadRows(rows)
}

// ListUserThreads returns threads authored by a user.
func (s *Store) ListUserThreads(ctx context.Context, userID string) ([]model.Thread, error) {
	rows, err := s.DB.Query(ctx,
		threadWithJoinsQuery+` WHERE t.author_id = $1 ORDER BY t.created_at DESC LIMIT 10`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanThreadRows(rows)
}

// SearchThreads searches threads by title.
func (s *Store) SearchThreads(ctx context.Context, pattern string) ([]model.Thread, error) {
	rows, err := s.DB.Query(ctx,
		threadWithJoinsQuery+` WHERE t.title ILIKE $1 ORDER BY t.created_at DESC LIMIT 20`, pattern)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanThreadRows(rows)
}

// CreateThread inserts a new thread and returns its ID.
func (s *Store) CreateThread(ctx context.Context, categoryID, authorID, title, slug string, content, imageURL *string) (string, error) {
	var id string
	now := time.Now()
	err := s.DB.QueryRow(ctx,
		`INSERT INTO threads (category_id, author_id, title, slug, content, image_url, post_count, last_post_at, created_at, updated_at)
		 VALUES ($1, $2, $3, $4, $5, $6, 1, $7, $7, $7)
		 RETURNING id`,
		categoryID, authorID, title, slug, content, imageURL, now).Scan(&id)
	return id, err
}

// GetThreadOwnership returns the thread author ID and whether the given user is an admin.
func (s *Store) GetThreadOwnership(ctx context.Context, threadID, userID string) (authorID string, isAdmin bool, err error) {
	err = s.DB.QueryRow(ctx,
		`SELECT t.author_id, COALESCE(p.is_admin, false)
		 FROM threads t LEFT JOIN profiles p ON p.id = $2
		 WHERE t.id = $1`, threadID, userID).Scan(&authorID, &isAdmin)
	return
}

// UpdateThread performs a dynamic update on a thread.
func (s *Store) UpdateThread(ctx context.Context, threadID string, setClauses []string, args []interface{}) error {
	query := "UPDATE threads SET "
	for i, clause := range setClauses {
		if i > 0 {
			query += ", "
		}
		query += clause
	}
	query += " WHERE id = $1"
	_, err := s.DB.Exec(ctx, query, args...)
	return err
}

// UpdateThreadStats updates a thread's last_post_at and increments post_count.
func (s *Store) UpdateThreadStats(ctx context.Context, threadID string, now time.Time) error {
	_, err := s.DB.Exec(ctx,
		`UPDATE threads SET last_post_at = $2, post_count = post_count + 1, updated_at = $2 WHERE id = $1`,
		threadID, now)
	return err
}

// scanThreadRows scans multiple thread rows into a slice.
func scanThreadRows(rows interface{ Next() bool; Scan(dest ...interface{}) error }) ([]model.Thread, error) {
	var threads []model.Thread
	for rows.Next() {
		t, err := scanThreadWithAuthor(rows.Scan)
		if err != nil {
			return nil, err
		}
		threads = append(threads, t)
	}
	if threads == nil {
		threads = []model.Thread{}
	}
	return threads, nil
}

// SetClause returns a SQL SET clause like "col = $N".
func SetClause(col string, argIdx int) string {
	return col + " = $" + fmt.Sprint(argIdx)
}
