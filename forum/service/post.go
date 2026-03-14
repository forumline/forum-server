package service

import (
	"context"
	"log"
	"time"

	"github.com/forumline/forum-server/forum/model"
	"github.com/forumline/forum-server/forum/store"
)

// PostService handles post business logic.
type PostService struct {
	Store           *store.Store
	NotificationSvc *NotificationService
}

// NewPostService creates a new PostService.
func NewPostService(s *store.Store, notifSvc *NotificationService) *PostService {
	return &PostService{Store: s, NotificationSvc: notifSvc}
}

// ListByThread returns posts for a thread.
func (ps *PostService) ListByThread(ctx context.Context, threadID string) ([]model.Post, error) {
	return ps.Store.ListPostsByThread(ctx, threadID)
}

// ListByUser returns posts authored by a user.
func (ps *PostService) ListByUser(ctx context.Context, userID string) ([]model.Post, error) {
	return ps.Store.ListUserPosts(ctx, userID)
}

// Search searches posts by content.
func (ps *PostService) Search(ctx context.Context, query string) ([]model.Post, error) {
	if query == "" {
		return []model.Post{}, nil
	}
	return ps.Store.SearchPosts(ctx, "%"+query+"%")
}

// CreatePostInput holds input for creating a post.
type CreatePostInput struct {
	ThreadID  string
	Content   string
	ReplyToID *string
}

// Create creates a new post, updates thread stats, and triggers notifications.
func (ps *PostService) Create(ctx context.Context, userID string, input CreatePostInput) (string, error) {
	if input.ThreadID == "" || input.Content == "" {
		return "", &ValidationError{Msg: "thread_id and content are required"}
	}

	id, err := ps.Store.CreatePost(ctx, input.ThreadID, userID, input.Content, input.ReplyToID)
	if err != nil {
		return "", err
	}

	// Update thread stats (best-effort)
	if err := ps.Store.UpdateThreadStats(ctx, input.ThreadID, time.Now()); err != nil {
		log.Printf("update thread stats error: %v", err)
	}

	// Generate notifications in background
	go ps.NotificationSvc.GeneratePostNotifications(input.ThreadID, id, userID, input.Content, input.ReplyToID) //nolint:gosec

	return id, nil
}
