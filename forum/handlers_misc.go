package forum

import (
	"encoding/json"
	"net/http"

	shared "github.com/forumline/forumline/shared-go"
)

// HandleConfig serves /api/config for frontend discovery of forum name and mode.
func (h *Handlers) HandleConfig(w http.ResponseWriter, r *http.Request) {
	name := h.Config.ForumName
	if name == "" {
		name = h.Config.Domain
	}
	resp := map[string]interface{}{
		"name":        name,
		"hosted_mode": true,
		"icon_url":    h.Config.IconURL,
	}
	if h.Config.LiveKitURL != "" {
		resp["livekit_url"] = h.Config.LiveKitURL
	}
	writeJSON(w, http.StatusOK, resp)
}

// HandleManifest serves /.well-known/forumline-manifest.json for forum discovery.
func (h *Handlers) HandleManifest(w http.ResponseWriter, r *http.Request) {
	name := h.Config.ForumName
	if name == "" {
		name = h.Config.Domain
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"forumline_version": "1",
		"name":              name,
		"domain":            h.Config.Domain,
		"icon_url":          h.Config.IconURL,
		"api_base":          h.Config.SiteURL + "/api/forumline",
		"web_base":          h.Config.SiteURL,
		"capabilities":      []string{"threads", "chat", "voice", "notifications"},
	})
}

// HandleChannelFollows handles GET/POST/DELETE /api/channel-follows.
func (h *Handlers) HandleChannelFollows(w http.ResponseWriter, r *http.Request) {
	userID := shared.UserIDFromContext(r.Context())

	switch r.Method {
	case http.MethodGet:
		ids, err := h.Store.ListChannelFollows(r.Context(), userID)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}
		writeJSON(w, http.StatusOK, ids)

	case http.MethodPost:
		var body struct {
			CategoryID string `json:"category_id"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.CategoryID == "" {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "Missing category_id"})
			return
		}
		if err := h.Store.AddChannelFollow(r.Context(), userID, body.CategoryID); err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}
		writeJSON(w, http.StatusOK, map[string]bool{"ok": true})

	case http.MethodDelete:
		var body struct {
			CategoryID string `json:"category_id"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.CategoryID == "" {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "Missing category_id"})
			return
		}
		if err := h.Store.RemoveChannelFollow(r.Context(), userID, body.CategoryID); err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}
		writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
	}
}

// HandleNotificationPreferences handles GET/PUT /api/notification-preferences.
func (h *Handlers) HandleNotificationPreferences(w http.ResponseWriter, r *http.Request) {
	userID := shared.UserIDFromContext(r.Context())

	switch r.Method {
	case http.MethodGet:
		prefs, err := h.Store.ListNotificationPrefs(r.Context(), userID)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}
		writeJSON(w, http.StatusOK, prefs)

	case http.MethodPut:
		var body struct {
			Category string `json:"category"`
			Enabled  bool   `json:"enabled"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "Invalid request body"})
			return
		}

		validCategories := map[string]bool{
			"reply": true, "mention": true, "chat_mention": true, "dm": true, "new_thread": true,
		}
		if !validCategories[body.Category] {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "Invalid category or enabled value"})
			return
		}

		if err := h.Store.UpsertNotificationPref(r.Context(), userID, body.Category, body.Enabled); err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}
		writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
	}
}
