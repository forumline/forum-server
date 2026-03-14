package store

import (
	"context"
	"encoding/json"
	"time"

	"github.com/forumline/forum-server/forum/model"
)

// ListVoiceRooms returns all voice rooms ordered by name.
func (s *Store) ListVoiceRooms(ctx context.Context) ([]model.VoiceRoom, error) {
	rows, err := s.DB.Query(ctx,
		`SELECT id, name, slug, created_at
		 FROM voice_rooms ORDER BY name`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var rooms []model.VoiceRoom
	for rows.Next() {
		var room model.VoiceRoom
		var createdAt time.Time
		if err := rows.Scan(&room.ID, &room.Name, &room.Slug, &createdAt); err != nil {
			return nil, err
		}
		room.CreatedAt = createdAt.Format(time.RFC3339)
		rooms = append(rooms, room)
	}
	if rooms == nil {
		rooms = []model.VoiceRoom{}
	}
	return rooms, nil
}

// ListVoicePresence returns all voice presence entries with profile.
func (s *Store) ListVoicePresence(ctx context.Context) ([]model.VoicePresence, error) {
	rows, err := s.DB.Query(ctx,
		`SELECT vp.id, vp.user_id, vp.room_slug, vp.joined_at,
		        p.id, p.username, p.display_name, p.avatar_url, p.bio, p.website,
		        p.is_admin, p.forumline_id, p.created_at, p.updated_at
		 FROM voice_presence vp
		 JOIN profiles p ON p.id = vp.user_id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var presence []model.VoicePresence
	for rows.Next() {
		var vp model.VoicePresence
		var joinedAt, authorCreatedAt, authorUpdatedAt time.Time
		err := rows.Scan(
			&vp.ID, &vp.UserID, &vp.RoomSlug, &joinedAt,
			&vp.Profile.ID, &vp.Profile.Username, &vp.Profile.DisplayName, &vp.Profile.AvatarURL,
			&vp.Profile.Bio, &vp.Profile.Website, &vp.Profile.IsAdmin, &vp.Profile.ForumlineID,
			&authorCreatedAt, &authorUpdatedAt,
		)
		if err != nil {
			return nil, err
		}
		vp.JoinedAt = joinedAt.Format(time.RFC3339)
		vp.Profile.CreatedAt = authorCreatedAt.Format(time.RFC3339)
		vp.Profile.UpdatedAt = authorUpdatedAt.Format(time.RFC3339)
		presence = append(presence, vp)
	}
	if presence == nil {
		presence = []model.VoicePresence{}
	}
	return presence, nil
}

// SetVoicePresence sets a user's voice presence.
func (s *Store) SetVoicePresence(ctx context.Context, userID, roomSlug string) error {
	now := time.Now()
	_, err := s.DB.Exec(ctx,
		`INSERT INTO voice_presence (user_id, room_slug, joined_at)
		 VALUES ($1, $2, $3)
		 ON CONFLICT (user_id) DO UPDATE SET room_slug = $2, joined_at = $3`,
		userID, roomSlug, now)
	return err
}

// ClearVoicePresence removes a user's voice presence.
func (s *Store) ClearVoicePresence(ctx context.Context, userID string) error {
	_, err := s.DB.Exec(ctx,
		`DELETE FROM voice_presence WHERE user_id = $1`, userID)
	return err
}

// SendVoiceSignal sends a voice signal via pg_notify.
func (s *Store) SendVoiceSignal(ctx context.Context, signal map[string]interface{}) error {
	signalJSON, err := json.Marshal(signal)
	if err != nil {
		return err
	}
	_, err = s.DB.Exec(ctx,
		"SELECT pg_notify('voice_signal_changes', $1)", string(signalJSON))
	return err
}
