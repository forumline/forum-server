package forum

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/golang-jwt/jwt/v5"
	shared "github.com/forumline/forumline/shared-go"
)

// HandleNotifications handles GET /api/forumline/notifications.
func (h *Handlers) HandleNotifications(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "Method not allowed"})
		return
	}

	userID, err := h.authenticateFromHeader(r)
	if err != nil {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "Unauthorized"})
		return
	}

	notifications, err := h.Store.ListForumlineNotifications(r.Context(), userID, 50, h.Config.Domain)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, notifications)
}

// HandleNotificationRead handles POST /api/forumline/notifications/read.
func (h *Handlers) HandleNotificationRead(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "Method not allowed"})
		return
	}

	userID, err := h.authenticateFromHeader(r)
	if err != nil {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "Unauthorized"})
		return
	}

	var body struct {
		ID string `json:"id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.ID == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "Notification ID required"})
		return
	}

	if err := h.Store.MarkNotificationRead(r.Context(), body.ID, userID); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"success": true})
}

// HandleUnread handles GET /api/forumline/unread.
func (h *Handlers) HandleUnread(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "Method not allowed"})
		return
	}

	userID, err := h.authenticateFromHeader(r)
	if err != nil {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "Unauthorized"})
		return
	}

	notifCount, chatMentionCount, err := h.Store.CountUnread(r.Context(), userID)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	writeJSON(w, http.StatusOK, map[string]int{
		"notifications": notifCount,
		"chat_mentions": chatMentionCount,
		"dms":           0,
	})
}

// HandleNotificationStream handles GET /api/forumline/notifications/stream (SSE).
func (h *Handlers) HandleNotificationStream(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "Method not allowed"})
		return
	}

	userID, err := h.authenticateFromHeader(r)
	if err != nil {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "Unauthorized"})
		return
	}

	client := &shared.SSEClient{
		Channel: "notification_changes",
		Filter:  map[string]string{"user_id": userID},
		Send:    make(chan []byte, 32),
		Done:    make(chan struct{}),
	}

	h.SSEHub.Register(client)
	defer func() {
		h.SSEHub.Unregister(client)
		close(client.Done)
	}()

	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "streaming unsupported", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no")
	w.WriteHeader(http.StatusOK)

	if _, err := fmt.Fprint(w, ":connected\n\n"); err != nil {
		return
	}
	flusher.Flush()

	heartbeat := time.NewTicker(30 * time.Second)
	defer heartbeat.Stop()

	ctx := r.Context()
	for {
		select {
		case <-ctx.Done():
			return
		case <-heartbeat.C:
			if _, err := fmt.Fprint(w, ":heartbeat\n\n"); err != nil {
				return
			}
			flusher.Flush()
		case data := <-client.Send:
			var raw map[string]interface{}
			if err := json.Unmarshal(data, &raw); err == nil {
				event := map[string]interface{}{
					"id":           raw["id"],
					"type":         raw["type"],
					"title":        raw["title"],
					"body":         raw["message"],
					"timestamp":    raw["created_at"],
					"read":         raw["read"],
					"link":         raw["link"],
					"forum_domain": h.Config.Domain,
				}
				if event["link"] == nil {
					event["link"] = "/"
				}
				eventJSON, _ := json.Marshal(event)
				if _, err := fmt.Fprintf(w, "data: %s\n\n", eventJSON); err != nil {
					return
				}
			} else {
				if _, err := fmt.Fprintf(w, "data: %s\n\n", data); err != nil {
					return
				}
			}
			flusher.Flush()
		}
	}
}

// authenticateFromHeader extracts and validates the JWT from the Authorization header.
func (h *Handlers) authenticateFromHeader(r *http.Request) (string, error) {
	auth := r.Header.Get("Authorization")
	if auth == "" {
		token := r.URL.Query().Get("access_token")
		if token != "" {
			return h.validateTokenWithFallback(token)
		}
		return "", fmt.Errorf("missing authorization")
	}
	if len(auth) < 8 || auth[:7] != "Bearer " {
		return "", fmt.Errorf("invalid authorization header")
	}
	token := auth[7:]
	return h.validateTokenWithFallback(token)
}

// validateTokenWithFallback tries JWT_SECRET first, then ForumlineJWTSecret.
func (h *Handlers) validateTokenWithFallback(token string) (string, error) {
	claims, err := shared.ValidateJWT(token)
	if err == nil {
		return claims.Subject, nil
	}
	if h.Config.ForumlineJWTSecret != "" {
		parsed, parseErr := jwt.ParseWithClaims(token, &jwt.RegisteredClaims{}, func(t *jwt.Token) (interface{}, error) {
			if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
				return nil, fmt.Errorf("unexpected signing method")
			}
			return []byte(h.Config.ForumlineJWTSecret), nil
		})
		if parseErr == nil && parsed.Valid {
			if rc, ok := parsed.Claims.(*jwt.RegisteredClaims); ok && rc.Subject != "" {
				return rc.Subject, nil
			}
		}
	}
	return "", err
}
