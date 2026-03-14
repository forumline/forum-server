package store

import (
	"context"
	"time"

	"github.com/forumline/forum-server/forum/model"
)

// ListPostsByThread returns posts for a thread ordered by creation time.
func (s *Store) ListPostsByThread(ctx context.Context, threadID string) ([]model.Post, error) {
	rows, err := s.DB.Query(ctx,
		postWithJoinsQuery+` WHERE po.thread_id = $1 ORDER BY po.created_at ASC`, threadID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanPostRows(rows)
}

// ListUserPosts returns posts authored by a user.
func (s *Store) ListUserPosts(ctx context.Context, userID string) ([]model.Post, error) {
	rows, err := s.DB.Query(ctx,
		postWithJoinsQuery+` WHERE po.author_id = $1 ORDER BY po.created_at DESC LIMIT 20`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanPostRows(rows)
}

// SearchPosts searches posts by content.
func (s *Store) SearchPosts(ctx context.Context, pattern string) ([]model.Post, error) {
	rows, err := s.DB.Query(ctx,
		postWithJoinsQuery+` WHERE po.content ILIKE $1 ORDER BY po.created_at DESC LIMIT 20`, pattern)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanPostRows(rows)
}

// CreatePost inserts a new post and returns its ID.
func (s *Store) CreatePost(ctx context.Context, threadID, authorID, content string, replyToID *string) (string, error) {
	var id string
	now := time.Now()
	err := s.DB.QueryRow(ctx,
		`INSERT INTO posts (thread_id, author_id, content, reply_to_id, created_at, updated_at)
		 VALUES ($1, $2, $3, $4, $5, $5)
		 RETURNING id`,
		threadID, authorID, content, replyToID, now).Scan(&id)
	return id, err
}

// GetPostAuthor returns the author_id of a post.
func (s *Store) GetPostAuthor(ctx context.Context, postID string) (string, error) {
	var authorID string
	err := s.DB.QueryRow(ctx,
		`SELECT author_id FROM posts WHERE id = $1`, postID).Scan(&authorID)
	return authorID, err
}

// scanPostRows scans multiple post rows into a slice.
func scanPostRows(rows interface{ Next() bool; Scan(dest ...interface{}) error }) ([]model.Post, error) {
	var posts []model.Post
	for rows.Next() {
		p, err := scanPostWithAuthor(rows.Scan)
		if err != nil {
			return nil, err
		}
		posts = append(posts, p)
	}
	if posts == nil {
		posts = []model.Post{}
	}
	return posts, nil
}
