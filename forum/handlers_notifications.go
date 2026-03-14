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

	rows, err := h.Pool.Query(r.Context(),
		`SELECT id, type, title, message, link, read, created_at
		 FROM notifications
		 WHERE user_id = $1
		 ORDER BY created_at DESC
		 LIMIT 50`, userID)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	defer rows.Close()

	type notification struct {
		ID          string `json:"id"`
		Type        string `json:"type"`
		Title       string `json:"title"`
		Body        string `json:"body"`
		Link        string `json:"link"`
		Read        bool   `json:"read"`
		Timestamp   string `json:"timestamp"`
		ForumDomain string `json:"forum_domain"`
	}

	var notifications []notification
	for rows.Next() {
		var n notification
		var message string
		var link *string
		var createdAt time.Time
		if err := rows.Scan(&n.ID, &n.Type, &n.Title, &message, &link, &n.Read, &createdAt); err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}
		n.Body = message
		if link != nil {
			n.Link = *link
		} else {
			n.Link = "/"
		}
		n.Timestamp = createdAt.Format(time.RFC3339)
		n.ForumDomain = h.Config.Domain
		notifications = append(notifications, n)
	}
	if notifications == nil {
		notifications = []notification{}
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

	_, err = h.Pool.Exec(r.Context(),
		"UPDATE notifications SET read = true WHERE id = $1 AND user_id = $2",
		body.ID, userID)
	if err != nil {
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

	var notifCount int
	err = h.Pool.QueryRow(r.Context(),
		`SELECT COUNT(*) FROM notifications
		 WHERE user_id = $1 AND read = false AND type != 'chat_mention'`,
		userID).Scan(&notifCount)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	var chatMentionCount int
	err = h.Pool.QueryRow(r.Context(),
		`SELECT COUNT(*) FROM notifications
		 WHERE user_id = $1 AND read = false AND type = 'chat_mention'`,
		userID).Scan(&chatMentionCount)
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
			// The pg_notify payload is raw JSON with notification fields.
			// Transform it to match the expected SSE format.
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
// Tries the forum's JWT_SECRET first, then falls back to ForumlineJWTSecret so
// the forumline-api can call notification endpoints on behalf of users.
func (h *Handlers) authenticateFromHeader(r *http.Request) (string, error) {
	auth := r.Header.Get("Authorization")
	if auth == "" {
		// Try query param (for EventSource/SSE)
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
