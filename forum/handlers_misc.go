package forum

import (
	"encoding/json"
	"net/http"
	"time"

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
		"hosted_mode": h.Config.GoTrueURL == "",
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
		rows, err := h.Pool.Query(r.Context(),
			"SELECT category_id FROM channel_follows WHERE user_id = $1", userID)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}
		defer rows.Close()

		var ids []string
		for rows.Next() {
			var id string
			if err := rows.Scan(&id); err != nil {
				writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
				return
			}
			ids = append(ids, id)
		}
		if ids == nil {
			ids = []string{}
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

		_, err := h.Pool.Exec(r.Context(),
			`INSERT INTO channel_follows (user_id, category_id)
			 VALUES ($1, $2)
			 ON CONFLICT (user_id, category_id) DO NOTHING`,
			userID, body.CategoryID)
		if err != nil {
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

		_, err := h.Pool.Exec(r.Context(),
			"DELETE FROM channel_follows WHERE user_id = $1 AND category_id = $2",
			userID, body.CategoryID)
		if err != nil {
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
		rows, err := h.Pool.Query(r.Context(),
			"SELECT category, enabled FROM notification_preferences WHERE user_id = $1", userID)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}
		defer rows.Close()

		type pref struct {
			Category string `json:"category"`
			Enabled  bool   `json:"enabled"`
		}
		var prefs []pref
		for rows.Next() {
			var p pref
			if err := rows.Scan(&p.Category, &p.Enabled); err != nil {
				writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
				return
			}
			prefs = append(prefs, p)
		}
		if prefs == nil {
			prefs = []pref{}
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

		_, err := h.Pool.Exec(r.Context(),
			`INSERT INTO notification_preferences (user_id, category, enabled, updated_at)
			 VALUES ($1, $2, $3, $4)
			 ON CONFLICT (user_id, category)
			 DO UPDATE SET enabled = $3, updated_at = $4`,
			userID, body.Category, body.Enabled, time.Now().UTC())
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}
		writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
	}
}
