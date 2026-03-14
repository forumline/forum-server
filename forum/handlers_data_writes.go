package forum

import (
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/forumline/forum-server/forum/service"
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

	id, err := h.ThreadSvc.Create(r.Context(), userID, service.CreateThreadInput{
		CategoryID: body.CategoryID,
		Title:      body.Title,
		Slug:       body.Slug,
		Content:    body.Content,
		ImageURL:   body.ImageURL,
	})
	if err != nil {
		writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, map[string]string{"id": id})
}

// HandleUpdateThread handles PATCH /api/threads/{id}
func (h *Handlers) HandleUpdateThread(w http.ResponseWriter, r *http.Request) {
	userID := shared.UserIDFromContext(r.Context())
	threadID := chi.URLParam(r, "id")

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

	err := h.ThreadSvc.Update(r.Context(), userID, threadID, service.UpdateThreadInput{
		ImageURL:   body.ImageURL,
		LastPostAt: body.LastPostAt,
		PostCount:  body.PostCount,
		IsPinned:   body.IsPinned,
		IsLocked:   body.IsLocked,
	})
	if err != nil {
		writeServiceError(w, err)
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

	id, err := h.PostSvc.Create(r.Context(), userID, service.CreatePostInput{
		ThreadID:  body.ThreadID,
		Content:   body.Content,
		ReplyToID: body.ReplyToID,
	})
	if err != nil {
		writeServiceError(w, err)
		return
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

	if err := h.ChatSvc.SendMessage(r.Context(), userID, slug, body.Content); err != nil {
		writeServiceError(w, err)
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

	if err := h.ChatSvc.SendMessageByID(r.Context(), userID, channelID, body.Content); err != nil {
		writeServiceError(w, err)
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

	if err := h.Store.AddBookmark(r.Context(), userID, body.ThreadID); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusCreated, map[string]bool{"success": true})
}

// HandleRemoveBookmark handles DELETE /api/bookmarks/{threadId}
func (h *Handlers) HandleRemoveBookmark(w http.ResponseWriter, r *http.Request) {
	userID := shared.UserIDFromContext(r.Context())
	threadID := chi.URLParam(r, "threadId")

	if err := h.Store.RemoveBookmark(r.Context(), userID, threadID); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"success": true})
}

// HandleRemoveBookmarkByID handles DELETE /api/bookmarks/by-id/{id}
func (h *Handlers) HandleRemoveBookmarkByID(w http.ResponseWriter, r *http.Request) {
	userID := shared.UserIDFromContext(r.Context())
	bookmarkID := chi.URLParam(r, "id")

	if err := h.Store.RemoveBookmarkByID(r.Context(), userID, bookmarkID); err != nil {
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

	if err := h.Store.MarkAllNotificationsRead(r.Context(), userID); err != nil {
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

	err := h.ProfileSvc.Upsert(r.Context(), userID, profileID, service.UpdateProfileInput{
		Username:    body.Username,
		DisplayName: body.DisplayName,
		AvatarURL:   body.AvatarURL,
		Bio:         body.Bio,
		Website:     body.Website,
	})
	if err != nil {
		writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"success": true})
}

// HandleClearForumlineID handles DELETE /api/profiles/{id}/forumline-id
func (h *Handlers) HandleClearForumlineID(w http.ResponseWriter, r *http.Request) {
	userID := shared.UserIDFromContext(r.Context())
	profileID := chi.URLParam(r, "id")

	if err := h.ProfileSvc.ClearForumlineID(r.Context(), userID, profileID); err != nil {
		writeServiceError(w, err)
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

	if err := h.Store.SetVoicePresence(r.Context(), userID, body.RoomSlug); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"success": true})
}

// HandleClearVoicePresence handles DELETE /api/voice-presence
func (h *Handlers) HandleClearVoicePresence(w http.ResponseWriter, r *http.Request) {
	userID := shared.UserIDFromContext(r.Context())

	if err := h.Store.ClearVoicePresence(r.Context(), userID); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"success": true})
}
