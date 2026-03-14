package forum

import (
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"
	shared "github.com/forumline/forumline/shared-go"
)

// ============================================================================
// Static / Config endpoints (public)
// ============================================================================

// HandleCategories handles GET /api/categories
func (h *Handlers) HandleCategories(w http.ResponseWriter, r *http.Request) {
	categories, err := h.Store.ListCategories(r.Context())
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, categories)
}

// HandleCategoryBySlug handles GET /api/categories/{slug}
func (h *Handlers) HandleCategoryBySlug(w http.ResponseWriter, r *http.Request) {
	slug := chi.URLParam(r, "slug")
	cat, err := h.Store.GetCategoryBySlug(r.Context(), slug)
	if err != nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "category not found"})
		return
	}
	writeJSON(w, http.StatusOK, cat)
}

// HandleChannels handles GET /api/channels
func (h *Handlers) HandleChannels(w http.ResponseWriter, r *http.Request) {
	channels, err := h.Store.ListChannels(r.Context())
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, channels)
}

// HandleVoiceRooms handles GET /api/voice-rooms
func (h *Handlers) HandleVoiceRooms(w http.ResponseWriter, r *http.Request) {
	rooms, err := h.Store.ListVoiceRooms(r.Context())
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, rooms)
}

// ============================================================================
// Threads
// ============================================================================

// HandleThreads handles GET /api/threads
func (h *Handlers) HandleThreads(w http.ResponseWriter, r *http.Request) {
	limit := 20
	if l := r.URL.Query().Get("limit"); l != "" {
		if n := parseInt(l, 20); n > 0 && n <= 100 {
			limit = n
		}
	}

	threads, err := h.ThreadSvc.List(r.Context(), limit)
	if err != nil {
		writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, threads)
}

// HandleThread handles GET /api/threads/{id}
func (h *Handlers) HandleThread(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	t, err := h.ThreadSvc.Get(r.Context(), id)
	if err != nil {
		writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, t)
}

// HandleThreadsByCategory handles GET /api/categories/{slug}/threads
func (h *Handlers) HandleThreadsByCategory(w http.ResponseWriter, r *http.Request) {
	slug := chi.URLParam(r, "slug")
	threads, err := h.ThreadSvc.ListByCategory(r.Context(), slug)
	if err != nil {
		writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, threads)
}

// HandleUserThreads handles GET /api/users/{id}/threads
func (h *Handlers) HandleUserThreads(w http.ResponseWriter, r *http.Request) {
	userID := chi.URLParam(r, "id")
	threads, err := h.ThreadSvc.ListByUser(r.Context(), userID)
	if err != nil {
		writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, threads)
}

// HandleSearchThreads handles GET /api/search/threads?q=
func (h *Handlers) HandleSearchThreads(w http.ResponseWriter, r *http.Request) {
	q := strings.TrimSpace(r.URL.Query().Get("q"))
	threads, err := h.ThreadSvc.Search(r.Context(), q)
	if err != nil {
		writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, threads)
}

// ============================================================================
// Posts
// ============================================================================

// HandlePosts handles GET /api/threads/{id}/posts
func (h *Handlers) HandlePosts(w http.ResponseWriter, r *http.Request) {
	threadID := chi.URLParam(r, "id")
	posts, err := h.PostSvc.ListByThread(r.Context(), threadID)
	if err != nil {
		writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, posts)
}

// HandleUserPosts handles GET /api/users/{id}/posts
func (h *Handlers) HandleUserPosts(w http.ResponseWriter, r *http.Request) {
	userID := chi.URLParam(r, "id")
	posts, err := h.PostSvc.ListByUser(r.Context(), userID)
	if err != nil {
		writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, posts)
}

// HandleSearchPosts handles GET /api/search/posts?q=
func (h *Handlers) HandleSearchPosts(w http.ResponseWriter, r *http.Request) {
	q := strings.TrimSpace(r.URL.Query().Get("q"))
	posts, err := h.PostSvc.Search(r.Context(), q)
	if err != nil {
		writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, posts)
}

// ============================================================================
// Profiles
// ============================================================================

// HandleProfile handles GET /api/profiles/{id}
func (h *Handlers) HandleProfile(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	p, err := h.ProfileSvc.Get(r.Context(), id)
	if err != nil {
		writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, p)
}

// HandleProfileByUsername handles GET /api/profiles/by-username/{username}
func (h *Handlers) HandleProfileByUsername(w http.ResponseWriter, r *http.Request) {
	username := chi.URLParam(r, "username")
	p, err := h.ProfileSvc.GetByUsername(r.Context(), username)
	if err != nil {
		writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, p)
}

// HandleProfilesBatch handles GET /api/profiles/batch?ids=id1,id2,...
func (h *Handlers) HandleProfilesBatch(w http.ResponseWriter, r *http.Request) {
	idsParam := r.URL.Query().Get("ids")
	if idsParam == "" {
		writeJSON(w, http.StatusOK, []interface{}{})
		return
	}
	ids := strings.Split(idsParam, ",")
	profiles, err := h.ProfileSvc.GetBatch(r.Context(), ids)
	if err != nil {
		writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, profiles)
}

// ============================================================================
// Chat Messages
// ============================================================================

// HandleChatMessages handles GET /api/channels/{slug}/messages
func (h *Handlers) HandleChatMessages(w http.ResponseWriter, r *http.Request) {
	slug := chi.URLParam(r, "slug")
	messages, err := h.ChatSvc.ListMessages(r.Context(), slug)
	if err != nil {
		writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, messages)
}

// ============================================================================
// Voice Presence
// ============================================================================

// HandleVoicePresence handles GET /api/voice-presence
func (h *Handlers) HandleVoicePresence(w http.ResponseWriter, r *http.Request) {
	presence, err := h.Store.ListVoicePresence(r.Context())
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, presence)
}

// ============================================================================
// Bookmarks
// ============================================================================

// HandleBookmarks handles GET /api/bookmarks
func (h *Handlers) HandleBookmarks(w http.ResponseWriter, r *http.Request) {
	userID := shared.UserIDFromContext(r.Context())
	bookmarks, err := h.Store.ListBookmarks(r.Context(), userID)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, bookmarks)
}

// HandleBookmarkStatus handles GET /api/bookmarks/{threadId}/status
func (h *Handlers) HandleBookmarkStatus(w http.ResponseWriter, r *http.Request) {
	userID := shared.UserIDFromContext(r.Context())
	threadID := chi.URLParam(r, "threadId")

	id, err := h.Store.GetBookmarkStatus(r.Context(), userID, threadID)
	if err != nil || id == nil {
		writeJSON(w, http.StatusOK, map[string]interface{}{"bookmarked": false})
		return
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{"bookmarked": true, "id": *id})
}

// ============================================================================
// Notifications (data read)
// ============================================================================

// HandleNotificationsData handles GET /api/notifications (data provider version)
func (h *Handlers) HandleNotificationsData(w http.ResponseWriter, r *http.Request) {
	userID := shared.UserIDFromContext(r.Context())
	notifications, err := h.Store.ListNotifications(r.Context(), userID, 20)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, notifications)
}

// ============================================================================
// Admin
// ============================================================================

// HandleAdminStats handles GET /api/admin/stats
func (h *Handlers) HandleAdminStats(w http.ResponseWriter, r *http.Request) {
	userID := shared.UserIDFromContext(r.Context())
	stats, err := h.AdminSvc.GetStats(r.Context(), userID)
	if err != nil {
		writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, stats)
}

// HandleAdminUsers handles GET /api/admin/users
func (h *Handlers) HandleAdminUsers(w http.ResponseWriter, r *http.Request) {
	userID := shared.UserIDFromContext(r.Context())
	profiles, err := h.AdminSvc.ListUsers(r.Context(), userID)
	if err != nil {
		writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, profiles)
}

// ============================================================================
// Helpers
// ============================================================================

func parseInt(s string, defaultVal int) int {
	var n int
	for _, c := range s {
		if c < '0' || c > '9' {
			return defaultVal
		}
		n = n*10 + int(c-'0')
	}
	if n == 0 {
		return defaultVal
	}
	return n
}
