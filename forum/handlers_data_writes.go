package forum

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"regexp"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/golang-jwt/jwt/v5"
	shared "github.com/forumline/forumline/shared-go"
)

var mentionRe = regexp.MustCompile(`@(\w+)`)

// ============================================================================
// Thread writes
// ============================================================================

// HandleCreateThread handles POST /api/threads
func (h *Handlers) HandleCreateThread(w http.ResponseWriter, r *http.Request) {
	userID := shared.UserIDFromContext(r.Context())

	var body struct {
		CategoryID string  `json:"category_id"`
		Title      string  `json:"title"`
		Slug       string  `json:"slug"`
		Content    *string `json:"content"`
		ImageURL   *string `json:"image_url"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
		return
	}
	if body.CategoryID == "" || body.Title == "" || body.Slug == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "category_id, title, and slug are required"})
		return
	}

	var id string
	now := time.Now()
	err := h.Pool.QueryRow(r.Context(),
		`INSERT INTO threads (category_id, author_id, title, slug, content, image_url, post_count, last_post_at, created_at, updated_at)
		 VALUES ($1, $2, $3, $4, $5, $6, 1, $7, $7, $7)
		 RETURNING id`,
		body.CategoryID, userID, body.Title, body.Slug, body.Content, body.ImageURL, now).Scan(&id)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusCreated, map[string]string{"id": id})
}

// HandleUpdateThread handles PATCH /api/threads/{id}
func (h *Handlers) HandleUpdateThread(w http.ResponseWriter, r *http.Request) {
	userID := shared.UserIDFromContext(r.Context())
	threadID := chi.URLParam(r, "id")

	// Verify ownership or admin
	var authorID string
	var isAdmin bool
	err := h.Pool.QueryRow(r.Context(),
		`SELECT t.author_id, COALESCE(p.is_admin, false)
		 FROM threads t LEFT JOIN profiles p ON p.id = $2
		 WHERE t.id = $1`, threadID, userID).Scan(&authorID, &isAdmin)
	if err != nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "thread not found"})
		return
	}
	if authorID != userID && !isAdmin {
		writeJSON(w, http.StatusForbidden, map[string]string{"error": "not authorized"})
		return
	}

	var body struct {
		ImageURL   *string `json:"image_url,omitempty"`
		LastPostAt *string `json:"last_post_at,omitempty"`
		PostCount  *int    `json:"post_count,omitempty"`
		IsPinned   *bool   `json:"is_pinned,omitempty"`
		IsLocked   *bool   `json:"is_locked,omitempty"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
		return
	}

	// Build dynamic UPDATE
	setClauses := []string{"updated_at = NOW()"}
	args := []interface{}{threadID}
	argIdx := 2

	if body.ImageURL != nil {
		setClauses = append(setClauses, setClause("image_url", argIdx))
		args = append(args, *body.ImageURL)
		argIdx++
	}
	if body.LastPostAt != nil {
		setClauses = append(setClauses, setClause("last_post_at", argIdx))
		args = append(args, *body.LastPostAt)
		argIdx++
	}
	if body.PostCount != nil {
		setClauses = append(setClauses, setClause("post_count", argIdx))
		args = append(args, *body.PostCount)
		argIdx++
	}
	if body.IsPinned != nil {
		setClauses = append(setClauses, setClause("is_pinned", argIdx))
		args = append(args, *body.IsPinned)
		argIdx++
	}
	if body.IsLocked != nil {
		setClauses = append(setClauses, setClause("is_locked", argIdx))
		args = append(args, *body.IsLocked)
	}

	query := "UPDATE threads SET "
	for i, clause := range setClauses {
		if i > 0 {
			query += ", "
		}
		query += clause
	}
	query += " WHERE id = $1"

	_, err = h.Pool.Exec(r.Context(), query, args...)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"success": true})
}

// ============================================================================
// Post writes
// ============================================================================

// HandleCreatePost handles POST /api/posts
func (h *Handlers) HandleCreatePost(w http.ResponseWriter, r *http.Request) {
	userID := shared.UserIDFromContext(r.Context())

	var body struct {
		ThreadID  string  `json:"thread_id"`
		Content   string  `json:"content"`
		ReplyToID *string `json:"reply_to_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
		return
	}
	if body.ThreadID == "" || body.Content == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "thread_id and content are required"})
		return
	}

	var id string
	now := time.Now()
	err := h.Pool.QueryRow(r.Context(),
		`INSERT INTO posts (thread_id, author_id, content, reply_to_id, created_at, updated_at)
		 VALUES ($1, $2, $3, $4, $5, $5)
		 RETURNING id`,
		body.ThreadID, userID, body.Content, body.ReplyToID, now).Scan(&id)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	// Update thread's last_post_at and post_count
	if _, err := h.Pool.Exec(r.Context(),
		`UPDATE threads SET last_post_at = $2, post_count = post_count + 1, updated_at = $2 WHERE id = $1`,
		body.ThreadID, now); err != nil {
		log.Printf("update thread stats error: %v", err)
	}

	// Generate notifications (best-effort, don't fail the request).
	// Uses background context because the request context is cancelled after response.
	go h.generatePostNotifications(body.ThreadID, id, userID, body.Content, body.ReplyToID) //nolint:gosec // intentional background goroutine

	writeJSON(w, http.StatusCreated, map[string]string{"id": id})
}

// forumlinePushItem holds a notification to be pushed to forumline.
type forumlinePushItem struct {
	ForumlineUserID string `json:"forumline_user_id"`
	Type            string `json:"type"`
	Title           string `json:"title"`
	Body            string `json:"body"`
	Link            string `json:"link"`
}

// generatePostNotifications creates notification rows for @mentions and thread reply notifications.
// After inserting locally, it batches and pushes to forumline for users with a forumline_id.
func (h *Handlers) generatePostNotifications(threadID, postID, authorID, content string, replyToID *string) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Look up author username
	var authorUsername string
	_ = h.Pool.QueryRow(ctx,
		`SELECT username FROM profiles WHERE id = $1`, authorID).Scan(&authorUsername)
	if authorUsername == "" {
		authorUsername = "Someone"
	}

	// Look up thread title and OP author
	var threadTitle string
	var threadAuthorID string
	_ = h.Pool.QueryRow(ctx,
		`SELECT t.title, p.author_id FROM threads t
		 JOIN posts p ON p.thread_id = t.id
		 WHERE t.id = $1 ORDER BY p.created_at ASC LIMIT 1`,
		threadID).Scan(&threadTitle, &threadAuthorID)

	threadLink := fmt.Sprintf("/t/%s", threadID)
	notified := map[string]bool{authorID: true} // don't notify the post author

	// Collect forumline push items
	var pushItems []forumlinePushItem

	// helper: insert local notification and queue forumline push
	notifyUser := func(userID, notifType, title, body, link string) {
		_, err := h.Pool.Exec(ctx,
			`INSERT INTO notifications (user_id, type, title, message, link)
			 VALUES ($1, $2, $3, $4, $5)`,
			userID, notifType, title, body, link)
		if err != nil {
			log.Printf("[notifications] failed to insert for %s: %v", userID, err)
			return
		}

		// Look up forumline_id for push
		var forumlineID *string
		_ = h.Pool.QueryRow(ctx,
			`SELECT forumline_id FROM profiles WHERE id = $1`, userID).Scan(&forumlineID)
		if forumlineID != nil && *forumlineID != "" {
			pushItems = append(pushItems, forumlinePushItem{
				ForumlineUserID: *forumlineID,
				Type:            notifType,
				Title:           title,
				Body:            body,
				Link:            link,
			})
		}
	}

	// 1. Notify thread author about the reply (unless they're the one replying)
	if threadAuthorID != "" && !notified[threadAuthorID] {
		notified[threadAuthorID] = true
		notifyUser(threadAuthorID, "reply",
			fmt.Sprintf("<strong>%s</strong> replied in \"%s\"", authorUsername, threadTitle),
			truncate(content, 200), threadLink)
	}

	// 2. If this is a reply to a specific post, notify that post's author
	if replyToID != nil && *replyToID != "" {
		var replyAuthorID string
		_ = h.Pool.QueryRow(ctx,
			`SELECT author_id FROM posts WHERE id = $1`, *replyToID).Scan(&replyAuthorID)
		if replyAuthorID != "" && !notified[replyAuthorID] {
			notified[replyAuthorID] = true
			notifyUser(replyAuthorID, "reply",
				fmt.Sprintf("<strong>%s</strong> replied to your post in \"%s\"", authorUsername, threadTitle),
				truncate(content, 200), threadLink)
		}
	}

	// 3. Notify @mentioned users
	matches := mentionRe.FindAllStringSubmatch(content, -1)
	for _, match := range matches {
		username := strings.ToLower(match[1])
		var mentionedUserID string
		_ = h.Pool.QueryRow(ctx,
			`SELECT id FROM profiles WHERE lower(username) = $1`, username).Scan(&mentionedUserID)
		if mentionedUserID != "" && !notified[mentionedUserID] {
			notified[mentionedUserID] = true
			notifyUser(mentionedUserID, "mention",
				fmt.Sprintf("<strong>%s</strong> mentioned you in \"%s\"", authorUsername, threadTitle),
				truncate(content, 200), threadLink)
		}
	}

	// Push batch to forumline
	if len(pushItems) > 0 {
		h.pushToForumline(pushItems)
	}
}

// pushToForumline sends a batch of notifications to the forumline API webhook.
func (h *Handlers) pushToForumline(items []forumlinePushItem) {
	webhookBase := h.Config.ForumlineWebhookURL
	if webhookBase == "" {
		webhookBase = h.Config.ForumlineURL
	}
	if webhookBase == "" || h.Config.ForumlineJWTSecret == "" {
		return
	}

	// Sign a JWT: sub=forum domain, iss="forum"
	now := time.Now()
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.RegisteredClaims{
		Subject:   h.Config.Domain,
		Issuer:    "forum",
		IssuedAt:  jwt.NewNumericDate(now),
		ExpiresAt: jwt.NewNumericDate(now.Add(time.Minute)),
	})
	tokenStr, err := token.SignedString([]byte(h.Config.ForumlineJWTSecret))
	if err != nil {
		log.Printf("[notifications] failed to sign forumline token: %v", err)
		return
	}

	var endpoint string
	var payload []byte
	if len(items) == 1 {
		endpoint = webhookBase + "/api/webhooks/notification"
		payload, _ = json.Marshal(items[0])
	} else {
		endpoint = webhookBase + "/api/webhooks/notifications"
		payload, _ = json.Marshal(items)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, "POST", endpoint, bytes.NewReader(payload))
	if err != nil {
		log.Printf("[notifications] failed to create forumline request: %v", err)
		return
	}
	req.Header.Set("Authorization", "Bearer "+tokenStr)
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		log.Printf("[notifications] push to forumline failed: %v", err)
		return
	}
	_ = resp.Body.Close()

	if resp.StatusCode >= 300 {
		log.Printf("[notifications] forumline webhook returned HTTP %d", resp.StatusCode)
	}
}

func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}

// ============================================================================
// Chat writes
// ============================================================================

// HandleSendChatMessage handles POST /api/channels/{slug}/messages
func (h *Handlers) HandleSendChatMessage(w http.ResponseWriter, r *http.Request) {
	userID := shared.UserIDFromContext(r.Context())
	slug := chi.URLParam(r, "slug")

	var body struct {
		Content string `json:"content"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
		return
	}
	if body.Content == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "content is required"})
		return
	}

	// Look up channel by slug
	var channelID string
	err := h.Pool.QueryRow(r.Context(),
		`SELECT id FROM chat_channels WHERE slug = $1`, slug).Scan(&channelID)
	if err != nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "channel not found"})
		return
	}

	_, err = h.Pool.Exec(r.Context(),
		`INSERT INTO chat_messages (channel_id, author_id, content) VALUES ($1, $2, $3)`,
		channelID, userID, body.Content)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusCreated, map[string]bool{"success": true})
}

// HandleSendChatMessageByID handles POST /api/channels/_by-id/{id}/messages
func (h *Handlers) HandleSendChatMessageByID(w http.ResponseWriter, r *http.Request) {
	userID := shared.UserIDFromContext(r.Context())
	channelID := chi.URLParam(r, "id")

	var body struct {
		Content string `json:"content"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
		return
	}
	if body.Content == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "content is required"})
		return
	}

	_, err := h.Pool.Exec(r.Context(),
		`INSERT INTO chat_messages (channel_id, author_id, content) VALUES ($1, $2, $3)`,
		channelID, userID, body.Content)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusCreated, map[string]bool{"success": true})
}

// ============================================================================
// Bookmark writes
// ============================================================================

// HandleAddBookmark handles POST /api/bookmarks
func (h *Handlers) HandleAddBookmark(w http.ResponseWriter, r *http.Request) {
	userID := shared.UserIDFromContext(r.Context())

	var body struct {
		ThreadID string `json:"thread_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.ThreadID == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "thread_id is required"})
		return
	}

	_, err := h.Pool.Exec(r.Context(),
		`INSERT INTO bookmarks (user_id, thread_id) VALUES ($1, $2) ON CONFLICT DO NOTHING`,
		userID, body.ThreadID)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusCreated, map[string]bool{"success": true})
}

// HandleRemoveBookmark handles DELETE /api/bookmarks/{threadId}
func (h *Handlers) HandleRemoveBookmark(w http.ResponseWriter, r *http.Request) {
	userID := shared.UserIDFromContext(r.Context())
	threadID := chi.URLParam(r, "threadId")

	_, err := h.Pool.Exec(r.Context(),
		`DELETE FROM bookmarks WHERE user_id = $1 AND thread_id = $2`, userID, threadID)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"success": true})
}

// HandleRemoveBookmarkByID handles DELETE /api/bookmarks/by-id/{id}
func (h *Handlers) HandleRemoveBookmarkByID(w http.ResponseWriter, r *http.Request) {
	userID := shared.UserIDFromContext(r.Context())
	bookmarkID := chi.URLParam(r, "id")

	_, err := h.Pool.Exec(r.Context(),
		`DELETE FROM bookmarks WHERE id = $1 AND user_id = $2`, bookmarkID, userID)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"success": true})
}

// ============================================================================
// Notification writes
// ============================================================================

// HandleMarkAllNotificationsRead handles POST /api/notifications/read-all
func (h *Handlers) HandleMarkAllNotificationsRead(w http.ResponseWriter, r *http.Request) {
	userID := shared.UserIDFromContext(r.Context())

	_, err := h.Pool.Exec(r.Context(),
		`UPDATE notifications SET read = true WHERE user_id = $1 AND read = false`, userID)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"success": true})
}

// ============================================================================
// Profile writes
// ============================================================================

// HandleUpsertProfile handles PUT /api/profiles/{id}
func (h *Handlers) HandleUpsertProfile(w http.ResponseWriter, r *http.Request) {
	userID := shared.UserIDFromContext(r.Context())
	profileID := chi.URLParam(r, "id")

	if userID != profileID {
		writeJSON(w, http.StatusForbidden, map[string]string{"error": "can only update own profile"})
		return
	}

	var body struct {
		Username    *string `json:"username"`
		DisplayName *string `json:"display_name"`
		AvatarURL   *string `json:"avatar_url"`
		Bio         *string `json:"bio"`
		Website     *string `json:"website"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
		return
	}

	now := time.Now()
	if body.Username != nil {
		// Upsert with username (profile creation or full update)
		_, err := h.Pool.Exec(r.Context(),
			`INSERT INTO profiles (id, username, display_name, avatar_url, created_at, updated_at)
			 VALUES ($1, $2, $3, $4, $5, $5)
			 ON CONFLICT (id) DO UPDATE SET
			   username = EXCLUDED.username,
			   display_name = EXCLUDED.display_name,
			   avatar_url = EXCLUDED.avatar_url,
			   updated_at = EXCLUDED.updated_at`,
			userID, *body.Username, body.DisplayName, body.AvatarURL, now)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}
	} else {
		// Partial update
		setClauses := []string{"updated_at = $2"}
		args := []interface{}{userID, now}
		argIdx := 3

		if body.DisplayName != nil {
			setClauses = append(setClauses, setClause("display_name", argIdx))
			args = append(args, *body.DisplayName)
			argIdx++
		}
		if body.Bio != nil {
			setClauses = append(setClauses, setClause("bio", argIdx))
			args = append(args, *body.Bio)
			argIdx++
		}
		if body.Website != nil {
			setClauses = append(setClauses, setClause("website", argIdx))
			args = append(args, *body.Website)
			argIdx++
		}
		if body.AvatarURL != nil {
			setClauses = append(setClauses, setClause("avatar_url", argIdx))
			args = append(args, *body.AvatarURL)
		}

		query := "UPDATE profiles SET "
		for i, clause := range setClauses {
			if i > 0 {
				query += ", "
			}
			query += clause
		}
		query += " WHERE id = $1"

		_, err := h.Pool.Exec(r.Context(), query, args...)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}
	}

	writeJSON(w, http.StatusOK, map[string]bool{"success": true})
}

// HandleClearForumlineID handles DELETE /api/profiles/{id}/forumline-id
func (h *Handlers) HandleClearForumlineID(w http.ResponseWriter, r *http.Request) {
	userID := shared.UserIDFromContext(r.Context())
	profileID := chi.URLParam(r, "id")

	if userID != profileID {
		writeJSON(w, http.StatusForbidden, map[string]string{"error": "can only update own profile"})
		return
	}

	_, err := h.Pool.Exec(r.Context(),
		`UPDATE profiles SET forumline_id = NULL, updated_at = NOW() WHERE id = $1`, userID)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"success": true})
}

// ============================================================================
// Voice presence writes
// ============================================================================

// HandleSetVoicePresence handles PUT /api/voice-presence
func (h *Handlers) HandleSetVoicePresence(w http.ResponseWriter, r *http.Request) {
	userID := shared.UserIDFromContext(r.Context())

	var body struct {
		RoomSlug string `json:"room_slug"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.RoomSlug == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "room_slug is required"})
		return
	}

	now := time.Now()
	_, err := h.Pool.Exec(r.Context(),
		`INSERT INTO voice_presence (user_id, room_slug, joined_at)
		 VALUES ($1, $2, $3)
		 ON CONFLICT (user_id) DO UPDATE SET room_slug = $2, joined_at = $3`,
		userID, body.RoomSlug, now)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"success": true})
}

// HandleClearVoicePresence handles DELETE /api/voice-presence
func (h *Handlers) HandleClearVoicePresence(w http.ResponseWriter, r *http.Request) {
	userID := shared.UserIDFromContext(r.Context())

	_, err := h.Pool.Exec(r.Context(),
		`DELETE FROM voice_presence WHERE user_id = $1`, userID)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"success": true})
}

// ============================================================================
// Helpers
// ============================================================================

func setClause(col string, argIdx int) string {
	return col + " = $" + itoa(argIdx)
}

func itoa(n int) string {
	if n < 10 {
		// #nosec G115 -- n is always 0-9
		return string(rune('0' + n))
	}
	// #nosec G115 -- n%10 is always 0-9
	return itoa(n/10) + string(rune('0'+n%10))
}
