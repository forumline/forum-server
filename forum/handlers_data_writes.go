package forum

import (
	"encoding/json"
	"log"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	shared "github.com/forumline/forumline/shared-go"
)

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

	writeJSON(w, http.StatusCreated, map[string]string{"id": id})
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
