package store

import (
	"context"
	"time"

	"github.com/forumline/forum-server/forum/model"
)

// ListChannelFollows returns category IDs the user follows.
func (s *Store) ListChannelFollows(ctx context.Context, userID string) ([]string, error) {
	rows, err := s.DB.Query(ctx,
		"SELECT category_id FROM channel_follows WHERE user_id = $1", userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var ids []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	if ids == nil {
		ids = []string{}
	}
	return ids, nil
}

// AddChannelFollow adds a channel follow.
func (s *Store) AddChannelFollow(ctx context.Context, userID, categoryID string) error {
	_, err := s.DB.Exec(ctx,
		`INSERT INTO channel_follows (user_id, category_id)
		 VALUES ($1, $2)
		 ON CONFLICT (user_id, category_id) DO NOTHING`,
		userID, categoryID)
	return err
}

// RemoveChannelFollow removes a channel follow.
func (s *Store) RemoveChannelFollow(ctx context.Context, userID, categoryID string) error {
	_, err := s.DB.Exec(ctx,
		"DELETE FROM channel_follows WHERE user_id = $1 AND category_id = $2",
		userID, categoryID)
	return err
}

// ListNotificationPrefs returns a user's notification preferences.
func (s *Store) ListNotificationPrefs(ctx context.Context, userID string) ([]model.NotificationPreference, error) {
	rows, err := s.DB.Query(ctx,
		"SELECT category, enabled FROM notification_preferences WHERE user_id = $1", userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var prefs []model.NotificationPreference
	for rows.Next() {
		var p model.NotificationPreference
		if err := rows.Scan(&p.Category, &p.Enabled); err != nil {
			return nil, err
		}
		prefs = append(prefs, p)
	}
	if prefs == nil {
		prefs = []model.NotificationPreference{}
	}
	return prefs, nil
}

// UpsertNotificationPref creates or updates a notification preference.
func (s *Store) UpsertNotificationPref(ctx context.Context, userID, category string, enabled bool) error {
	_, err := s.DB.Exec(ctx,
		`INSERT INTO notification_preferences (user_id, category, enabled, updated_at)
		 VALUES ($1, $2, $3, $4)
		 ON CONFLICT (user_id, category)
		 DO UPDATE SET enabled = $3, updated_at = $4`,
		userID, category, enabled, time.Now().UTC())
	return err
}

// GetAdminStats returns admin dashboard statistics.
func (s *Store) GetAdminStats(ctx context.Context) (model.AdminStats, error) {
	var stats model.AdminStats
	if err := s.DB.QueryRow(ctx, `SELECT COUNT(*) FROM profiles`).Scan(&stats.TotalUsers); err != nil {
		return stats, err
	}
	if err := s.DB.QueryRow(ctx, `SELECT COUNT(*) FROM threads`).Scan(&stats.TotalThreads); err != nil {
		return stats, err
	}
	if err := s.DB.QueryRow(ctx, `SELECT COUNT(*) FROM posts`).Scan(&stats.TotalPosts); err != nil {
		return stats, err
	}
	return stats, nil
}
