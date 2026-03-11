package forum

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"time"

	shared "github.com/forumline/forumline/shared-go"
)

// ExportData is the portable format for importing/exporting forum data.
type ExportData struct {
	ForumlineVersion string    `json:"forumline_version"`
	ExportedAt       time.Time `json:"exported_at"`
	Forum            ForumMeta `json:"forum"`

	Categories              []json.RawMessage `json:"categories"`
	Profiles                []json.RawMessage `json:"profiles"`
	Threads                 []json.RawMessage `json:"threads"`
	Posts                   []json.RawMessage `json:"posts"`
	ChatChannels            []json.RawMessage `json:"chat_channels"`
	ChatMessages            []json.RawMessage `json:"chat_messages"`
	VoiceRooms              []json.RawMessage `json:"voice_rooms"`
	Bookmarks               []json.RawMessage `json:"bookmarks"`
	Notifications           []json.RawMessage `json:"notifications"`
	ChannelFollows          []json.RawMessage `json:"channel_follows"`
	NotificationPreferences []json.RawMessage `json:"notification_preferences"`
}

// ForumMeta contains metadata about the forum being imported/exported.
type ForumMeta struct {
	Slug        string `json:"slug"`
	Name        string `json:"name"`
	Domain      string `json:"domain"`
	Description string `json:"description"`
}

// Import loads ExportData into a forum database.
// Import is additive — it uses ON CONFLICT DO NOTHING to avoid duplicating
// data if run multiple times. It preserves original UUIDs and timestamps.
func Import(ctx context.Context, db shared.DB, data *ExportData) error {
	steps := []struct {
		name  string
		items []json.RawMessage
		query string
	}{
		{"categories", data.Categories, `
			INSERT INTO categories (id, name, slug, description, sort_order, created_at)
			SELECT id, name, slug, description, sort_order, created_at
			FROM json_populate_record(null::categories, $1::json)
			ON CONFLICT DO NOTHING`},
		{"chat_channels", data.ChatChannels, `
			INSERT INTO chat_channels (id, name, slug, description, created_at)
			SELECT id, name, slug, description, created_at
			FROM json_populate_record(null::chat_channels, $1::json)
			ON CONFLICT DO NOTHING`},
		{"voice_rooms", data.VoiceRooms, `
			INSERT INTO voice_rooms (id, name, slug, created_at)
			SELECT id, name, slug, created_at
			FROM json_populate_record(null::voice_rooms, $1::json)
			ON CONFLICT DO NOTHING`},
		{"profiles", data.Profiles, `
			INSERT INTO profiles (id, username, display_name, avatar_url, bio, website, is_admin, forumline_id, created_at, updated_at)
			SELECT id, username, display_name, avatar_url, bio, website, is_admin, forumline_id, created_at, updated_at
			FROM json_populate_record(null::profiles, $1::json)
			ON CONFLICT DO NOTHING`},
		{"threads", data.Threads, `
			INSERT INTO threads (id, category_id, author_id, title, slug, content, image_url, is_pinned, is_locked, view_count, post_count, last_post_at, created_at, updated_at)
			SELECT id, category_id, author_id, title, slug, content, image_url, is_pinned, is_locked, view_count, post_count, last_post_at, created_at, updated_at
			FROM json_populate_record(null::threads, $1::json)
			ON CONFLICT DO NOTHING`},
		{"posts", data.Posts, `
			INSERT INTO posts (id, thread_id, author_id, content, reply_to_id, created_at, updated_at)
			SELECT id, thread_id, author_id, content, reply_to_id, created_at, updated_at
			FROM json_populate_record(null::posts, $1::json)
			ON CONFLICT DO NOTHING`},
		{"chat_messages", data.ChatMessages, `
			INSERT INTO chat_messages (id, channel_id, author_id, content, created_at)
			SELECT id, channel_id, author_id, content, created_at
			FROM json_populate_record(null::chat_messages, $1::json)
			ON CONFLICT DO NOTHING`},
		{"bookmarks", data.Bookmarks, `
			INSERT INTO bookmarks (id, user_id, thread_id, created_at)
			SELECT id, user_id, thread_id, created_at
			FROM json_populate_record(null::bookmarks, $1::json)
			ON CONFLICT DO NOTHING`},
		{"notifications", data.Notifications, `
			INSERT INTO notifications (id, user_id, type, title, message, link, read, created_at)
			SELECT id, user_id, type, title, message, link, read, created_at
			FROM json_populate_record(null::notifications, $1::json)
			ON CONFLICT DO NOTHING`},
		{"channel_follows", data.ChannelFollows, `
			INSERT INTO channel_follows (id, user_id, category_id, created_at)
			SELECT id, user_id, category_id, created_at
			FROM json_populate_record(null::channel_follows, $1::json)
			ON CONFLICT DO NOTHING`},
		{"notification_preferences", data.NotificationPreferences, `
			INSERT INTO notification_preferences (id, user_id, category, enabled, updated_at)
			SELECT id, user_id, category, enabled, updated_at
			FROM json_populate_record(null::notification_preferences, $1::json)
			ON CONFLICT DO NOTHING`},
	}

	for _, step := range steps {
		for i, item := range step.items {
			_, err := db.Exec(ctx, step.query, string(item))
			if err != nil {
				return fmt.Errorf("import %s row %d: %w", step.name, i, err)
			}
		}
		log.Printf("imported %d %s", len(step.items), step.name)
	}

	return nil
}
