package service

import (
	"context"

	"github.com/forumline/forum-server/forum/model"
	"github.com/forumline/forum-server/forum/store"
)

// ChatService handles chat business logic.
type ChatService struct {
	Store *store.Store
}

// NewChatService creates a new ChatService.
func NewChatService(s *store.Store) *ChatService {
	return &ChatService{Store: s}
}

// ListMessages returns chat messages for a channel slug.
func (cs *ChatService) ListMessages(ctx context.Context, slug string) ([]model.ChatMessage, error) {
	return cs.Store.ListChatMessages(ctx, slug)
}

// SendMessage sends a chat message to a channel by slug.
func (cs *ChatService) SendMessage(ctx context.Context, userID, slug, content string) error {
	if content == "" {
		return &ValidationError{Msg: "content is required"}
	}

	channelID, err := cs.Store.GetChannelIDBySlug(ctx, slug)
	if err != nil {
		return &NotFoundError{Msg: "channel not found"}
	}

	return cs.Store.InsertChatMessage(ctx, channelID, userID, content)
}

// SendMessageByID sends a chat message to a channel by ID.
func (cs *ChatService) SendMessageByID(ctx context.Context, userID, channelID, content string) error {
	if content == "" {
		return &ValidationError{Msg: "content is required"}
	}
	return cs.Store.InsertChatMessage(ctx, channelID, userID, content)
}
