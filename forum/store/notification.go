package store

import (
	"context"
	"time"

	"github.com/forumline/forum-server/forum/model"
)

// ListNotifications returns notifications for a user (data provider format).
func (s *Store) ListNotifications(ctx context.Context, userID string, limit int) ([]model.Notification, error) {
	rows, err := s.DB.Query(ctx,
		`SELECT id, user_id, type, title, message, link, read, created_at
		 FROM notifications
		 WHERE user_id = $1
		 ORDER BY created_at DESC
		 LIMIT $2`, userID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var notifications []model.Notification
	for rows.Next() {
		var n model.Notification
		var createdAt time.Time
		if err := rows.Scan(&n.ID, &n.UserID, &n.Type, &n.Title, &n.Message, &n.Link, &n.Read, &createdAt); err != nil {
			return nil, err
		}
		n.CreatedAt = createdAt.Format(time.RFC3339)
		notifications = append(notifications, n)
	}
	if notifications == nil {
		notifications = []model.Notification{}
	}
	return notifications, nil
}

// ListForumlineNotifications returns notifications in the forumline protocol format.
func (s *Store) ListForumlineNotifications(ctx context.Context, userID string, limit int, forumDomain string) ([]model.ForumlineNotification, error) {
	rows, err := s.DB.Query(ctx,
		`SELECT id, type, title, message, link, read, created_at
		 FROM notifications
		 WHERE user_id = $1
		 ORDER BY created_at DESC
		 LIMIT $2`, userID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var notifications []model.ForumlineNotification
	for rows.Next() {
		var n model.ForumlineNotification
		var message string
		var link *string
		var createdAt time.Time
		if err := rows.Scan(&n.ID, &n.Type, &n.Title, &message, &link, &n.Read, &createdAt); err != nil {
			return nil, err
		}
		n.Body = message
		if link != nil {
			n.Link = *link
		} else {
			n.Link = "/"
		}
		n.Timestamp = createdAt.Format(time.RFC3339)
		n.ForumDomain = forumDomain
		notifications = append(notifications, n)
	}
	if notifications == nil {
		notifications = []model.ForumlineNotification{}
	}
	return notifications, nil
}

// MarkNotificationRead marks a single notification as read.
func (s *Store) MarkNotificationRead(ctx context.Context, notifID, userID string) error {
	_, err := s.DB.Exec(ctx,
		"UPDATE notifications SET read = true WHERE id = $1 AND user_id = $2",
		notifID, userID)
	return err
}

// MarkAllNotificationsRead marks all notifications as read for a user.
func (s *Store) MarkAllNotificationsRead(ctx context.Context, userID string) error {
	_, err := s.DB.Exec(ctx,
		`UPDATE notifications SET read = true WHERE user_id = $1 AND read = false`, userID)
	return err
}

// InsertNotification inserts a notification.
func (s *Store) InsertNotification(ctx context.Context, userID, notifType, title, message, link string) error {
	_, err := s.DB.Exec(ctx,
		`INSERT INTO notifications (user_id, type, title, message, link)
		 VALUES ($1, $2, $3, $4, $5)`,
		userID, notifType, title, message, link)
	return err
}

// CountUnread returns unread notification and chat_mention counts.
func (s *Store) CountUnread(ctx context.Context, userID string) (notifs int, chatMentions int, err error) {
	err = s.DB.QueryRow(ctx,
		`SELECT COUNT(*) FROM notifications
		 WHERE user_id = $1 AND read = false AND type != 'chat_mention'`,
		userID).Scan(&notifs)
	if err != nil {
		return
	}
	err = s.DB.QueryRow(ctx,
		`SELECT COUNT(*) FROM notifications
		 WHERE user_id = $1 AND read = false AND type = 'chat_mention'`,
		userID).Scan(&chatMentions)
	return
}

// GetThreadTitleAndAuthor returns the thread title and OP author ID.
func (s *Store) GetThreadTitleAndAuthor(ctx context.Context, threadID string) (title string, authorID string, err error) {
	err = s.DB.QueryRow(ctx,
		`SELECT t.title, p.author_id FROM threads t
		 JOIN posts p ON p.thread_id = t.id
		 WHERE t.id = $1 ORDER BY p.created_at ASC LIMIT 1`,
		threadID).Scan(&title, &authorID)
	return
}
