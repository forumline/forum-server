package store

import (
	"context"
	"time"

	"github.com/forumline/forum-server/forum/model"
)

// ListCategories returns all categories ordered by sort_order.
func (s *Store) ListCategories(ctx context.Context) ([]model.Category, error) {
	rows, err := s.DB.Query(ctx,
		`SELECT id, name, slug, description, sort_order, created_at
		 FROM categories ORDER BY sort_order`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var categories []model.Category
	for rows.Next() {
		var c model.Category
		var createdAt time.Time
		if err := rows.Scan(&c.ID, &c.Name, &c.Slug, &c.Description, &c.SortOrder, &createdAt); err != nil {
			return nil, err
		}
		c.CreatedAt = createdAt.Format(time.RFC3339)
		categories = append(categories, c)
	}
	if categories == nil {
		categories = []model.Category{}
	}
	return categories, nil
}

// GetCategoryBySlug returns a category by its slug.
func (s *Store) GetCategoryBySlug(ctx context.Context, slug string) (*model.Category, error) {
	var c model.Category
	var createdAt time.Time
	err := s.DB.QueryRow(ctx,
		`SELECT id, name, slug, description, sort_order, created_at
		 FROM categories WHERE slug = $1`, slug).
		Scan(&c.ID, &c.Name, &c.Slug, &c.Description, &c.SortOrder, &createdAt)
	if err != nil {
		return nil, err
	}
	c.CreatedAt = createdAt.Format(time.RFC3339)
	return &c, nil
}
