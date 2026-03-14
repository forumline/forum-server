package store

import (
	"context"
	"time"

	"github.com/forumline/forum-server/forum/model"
)

// ListChannels returns all chat channels ordered by name.
func (s *Store) ListChannels(ctx context.Context) ([]model.Channel, error) {
	rows, err := s.DB.Query(ctx,
		`SELECT id, name, slug, description, created_at
		 FROM chat_channels ORDER BY name`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var channels []model.Channel
	for rows.Next() {
		var c model.Channel
		var createdAt time.Time
		if err := rows.Scan(&c.ID, &c.Name, &c.Slug, &c.Description, &createdAt); err != nil {
			return nil, err
		}
		c.CreatedAt = createdAt.Format(time.RFC3339)
		channels = append(channels, c)
	}
	if channels == nil {
		channels = []model.Channel{}
	}
	return channels, nil
}

// GetChannelIDBySlug returns the channel ID for a given slug.
func (s *Store) GetChannelIDBySlug(ctx context.Context, slug string) (string, error) {
	var id string
	err := s.DB.QueryRow(ctx,
		`SELECT id FROM chat_channels WHERE slug = $1`, slug).Scan(&id)
	return id, err
}

// ListChatMessages returns chat messages for a channel slug (with author profile).
func (s *Store) ListChatMessages(ctx context.Context, slug string) ([]model.ChatMessage, error) {
	rows, err := s.DB.Query(ctx,
		`SELECT m.id, m.channel_id, m.author_id, m.content, m.created_at,
		        p.id, p.username, p.display_name, p.avatar_url, p.bio, p.website,
		        p.is_admin, p.forumline_id, p.created_at, p.updated_at
		 FROM chat_messages m
		 JOIN chat_channels ch ON ch.id = m.channel_id
		 JOIN profiles p ON p.id = m.author_id
		 WHERE ch.slug = $1
		 ORDER BY m.created_at ASC
		 LIMIT 100`, slug)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var messages []model.ChatMessage
	for rows.Next() {
		var m model.ChatMessage
		var msgCreatedAt, authorCreatedAt, authorUpdatedAt time.Time
		err := rows.Scan(
			&m.ID, &m.ChannelID, &m.AuthorID, &m.Content, &msgCreatedAt,
			&m.Author.ID, &m.Author.Username, &m.Author.DisplayName, &m.Author.AvatarURL,
			&m.Author.Bio, &m.Author.Website, &m.Author.IsAdmin, &m.Author.ForumlineID,
			&authorCreatedAt, &authorUpdatedAt,
		)
		if err != nil {
			return nil, err
		}
		m.CreatedAt = msgCreatedAt.Format(time.RFC3339)
		m.Author.CreatedAt = authorCreatedAt.Format(time.RFC3339)
		m.Author.UpdatedAt = authorUpdatedAt.Format(time.RFC3339)
		messages = append(messages, m)
	}
	if messages == nil {
		messages = []model.ChatMessage{}
	}
	return messages, nil
}

// InsertChatMessage inserts a chat message.
func (s *Store) InsertChatMessage(ctx context.Context, channelID, authorID, content string) error {
	_, err := s.DB.Exec(ctx,
		`INSERT INTO chat_messages (channel_id, author_id, content) VALUES ($1, $2, $3)`,
		channelID, authorID, content)
	return err
}
