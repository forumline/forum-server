package forum

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	shared "github.com/forumline/forumline/shared-go"
)

// HandleChatStream handles GET /api/channels/{slug}/stream (SSE).
// Streams new chat messages for a specific channel, enriched with author profile.
func (h *Handlers) HandleChatStream(w http.ResponseWriter, r *http.Request) {
	slug := chi.URLParam(r, "slug")

	// Look up channel_id for filtering
	var channelID string
	err := h.Pool.QueryRow(r.Context(),
		`SELECT id FROM chat_channels WHERE slug = $1`, slug).Scan(&channelID)
	if err != nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "channel not found"})
		return
	}

	client := &shared.SSEClient{
		Channel: "chat_message_changes",
		Filter:  map[string]string{"channel_id": channelID},
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
			// Enrich the pg_notify payload with author profile
			var raw map[string]interface{}
			if err := json.Unmarshal(data, &raw); err != nil {
				continue
			}

			authorID, _ := raw["author_id"].(string)
			if authorID != "" {
				row := h.Pool.QueryRow(ctx,
					`SELECT `+profileColumns+` FROM profiles WHERE id = $1`, authorID)
				p, err := scanProfile(row.Scan)
				if err == nil {
					raw["author"] = p
				}
			}

			enriched, _ := json.Marshal(raw)
			if _, err := fmt.Fprintf(w, "data: %s\n\n", enriched); err != nil {
				return
			}
			flusher.Flush()
		}
	}
}

// HandlePostStream handles GET /api/threads/{id}/stream (SSE).
// Streams new posts for a specific thread, enriched with author profile.
func (h *Handlers) HandlePostStream(w http.ResponseWriter, r *http.Request) {
	threadID := chi.URLParam(r, "id")

	client := &shared.SSEClient{
		Channel: "post_changes",
		Filter:  map[string]string{"thread_id": threadID},
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
			if err := json.Unmarshal(data, &raw); err != nil {
				continue
			}

			authorID, _ := raw["author_id"].(string)
			if authorID != "" {
				row := h.Pool.QueryRow(ctx,
					`SELECT `+profileColumns+` FROM profiles WHERE id = $1`, authorID)
				p, err := scanProfile(row.Scan)
				if err == nil {
					raw["author"] = p
				}
			}

			enriched, _ := json.Marshal(raw)
			if _, err := fmt.Fprintf(w, "data: %s\n\n", enriched); err != nil {
				return
			}
			flusher.Flush()
		}
	}
}

// HandleVoicePresenceStream handles GET /api/voice-presence/stream (SSE).
// Streams voice presence changes (join/leave/update).
func (h *Handlers) HandleVoicePresenceStream(w http.ResponseWriter, r *http.Request) {
	client := &shared.SSEClient{
		Channel: "voice_presence_changes",
		Filter:  map[string]string{}, // no filter — all changes
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
			// Enrich with profile data for non-DELETE events
			var raw map[string]interface{}
			if err := json.Unmarshal(data, &raw); err != nil {
				continue
			}

			userID, _ := raw["user_id"].(string)
			if userID != "" {
				row := h.Pool.QueryRow(ctx,
					`SELECT `+profileColumns+` FROM profiles WHERE id = $1`, userID)
				p, err := scanProfile(row.Scan)
				if err == nil {
					raw["profile"] = p
				}
			}

			enriched, _ := json.Marshal(raw)
			if _, err := fmt.Fprintf(w, "data: %s\n\n", enriched); err != nil {
				return
			}
			flusher.Flush()
		}
	}
}
