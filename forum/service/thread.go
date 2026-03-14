package service

import (
	"context"

	"github.com/forumline/forum-server/forum/model"
	"github.com/forumline/forum-server/forum/store"
)

// ThreadService handles thread business logic.
type ThreadService struct {
	Store *store.Store
}

// NewThreadService creates a new ThreadService.
func NewThreadService(s *store.Store) *ThreadService {
	return &ThreadService{Store: s}
}

// List returns threads ordered by pinned + last_post_at.
func (ts *ThreadService) List(ctx context.Context, limit int) ([]model.Thread, error) {
	if limit <= 0 || limit > 100 {
		limit = 20
	}
	return ts.Store.ListThreads(ctx, limit)
}

// Get returns a single thread by ID.
func (ts *ThreadService) Get(ctx context.Context, id string) (*model.Thread, error) {
	t, err := ts.Store.GetThread(ctx, id)
	if err != nil {
		return nil, &NotFoundError{Msg: "thread not found"}
	}
	return t, nil
}

// ListByCategory returns threads for a category slug.
func (ts *ThreadService) ListByCategory(ctx context.Context, slug string) ([]model.Thread, error) {
	return ts.Store.ListThreadsByCategory(ctx, slug)
}

// ListByUser returns threads authored by a user.
func (ts *ThreadService) ListByUser(ctx context.Context, userID string) ([]model.Thread, error) {
	return ts.Store.ListUserThreads(ctx, userID)
}

// Search searches threads by title.
func (ts *ThreadService) Search(ctx context.Context, query string) ([]model.Thread, error) {
	if query == "" {
		return []model.Thread{}, nil
	}
	return ts.Store.SearchThreads(ctx, "%"+query+"%")
}

// CreateInput holds input for creating a thread.
type CreateThreadInput struct {
	CategoryID string
	Title      string
	Slug       string
	Content    *string
	ImageURL   *string
}

// Create creates a new thread.
func (ts *ThreadService) Create(ctx context.Context, userID string, input CreateThreadInput) (string, error) {
	if input.CategoryID == "" || input.Title == "" || input.Slug == "" {
		return "", &ValidationError{Msg: "category_id, title, and slug are required"}
	}
	return ts.Store.CreateThread(ctx, input.CategoryID, userID, input.Title, input.Slug, input.Content, input.ImageURL)
}

// UpdateThreadInput holds optional fields for updating a thread.
type UpdateThreadInput struct {
	ImageURL   *string
	LastPostAt *string
	PostCount  *int
	IsPinned   *bool
	IsLocked   *bool
}

// Update updates a thread, verifying ownership or admin.
func (ts *ThreadService) Update(ctx context.Context, userID, threadID string, input UpdateThreadInput) error {
	authorID, isAdmin, err := ts.Store.GetThreadOwnership(ctx, threadID, userID)
	if err != nil {
		return &NotFoundError{Msg: "thread not found"}
	}
	if authorID != userID && !isAdmin {
		return &ForbiddenError{Msg: "not authorized"}
	}

	setClauses := []string{"updated_at = NOW()"}
	args := []interface{}{threadID}
	argIdx := 2

	if input.ImageURL != nil {
		setClauses = append(setClauses, store.SetClause("image_url", argIdx))
		args = append(args, *input.ImageURL)
		argIdx++
	}
	if input.LastPostAt != nil {
		setClauses = append(setClauses, store.SetClause("last_post_at", argIdx))
		args = append(args, *input.LastPostAt)
		argIdx++
	}
	if input.PostCount != nil {
		setClauses = append(setClauses, store.SetClause("post_count", argIdx))
		args = append(args, *input.PostCount)
		argIdx++
	}
	if input.IsPinned != nil {
		setClauses = append(setClauses, store.SetClause("is_pinned", argIdx))
		args = append(args, *input.IsPinned)
		argIdx++
	}
	if input.IsLocked != nil {
		setClauses = append(setClauses, store.SetClause("is_locked", argIdx))
		args = append(args, *input.IsLocked)
	}

	return ts.Store.UpdateThread(ctx, threadID, setClauses, args)
}
