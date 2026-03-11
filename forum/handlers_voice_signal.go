package forum

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	shared "github.com/forumline/forumline/shared-go"
)

// HandleVoiceSignal handles POST /api/voice-signal — relays WebRTC signaling
// (SDP offers/answers and ICE candidates) between peers via Postgres NOTIFY.
func (h *Handlers) HandleVoiceSignal(w http.ResponseWriter, r *http.Request) {
	senderID := shared.UserIDFromContext(r.Context())

	var body struct {
		TargetUserID string          `json:"target_user_id"`
		Type         string          `json:"type"` // "offer", "answer", "ice-candidate"
		RoomSlug     string          `json:"room_slug"`
		Payload      json.RawMessage `json:"payload"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
		return
	}

	validTypes := map[string]bool{"offer": true, "answer": true, "ice-candidate": true, "escalate": true}
	if !validTypes[body.Type] || body.TargetUserID == "" || body.RoomSlug == "" || len(body.Payload) == 0 {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "target_user_id, type, room_slug, and payload are required"})
		return
	}

	// Build signal payload and fire pg_notify directly (no table needed — signals are transient)
	signal := map[string]interface{}{
		"sender_user_id": senderID,
		"target_user_id": body.TargetUserID,
		"type":           body.Type,
		"room_slug":      body.RoomSlug,
		"payload":        body.Payload,
	}
	signalJSON, err := json.Marshal(signal)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to encode signal"})
		return
	}

	_, err = h.Pool.Exec(r.Context(),
		"SELECT pg_notify('voice_signal_changes', $1)", string(signalJSON))
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to send signal"})
		return
	}

	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

// HandleVoiceSignalStream handles GET /api/voice-signal/stream (SSE).
// Streams WebRTC signals targeted at the authenticated user.
func (h *Handlers) HandleVoiceSignalStream(w http.ResponseWriter, r *http.Request) {
	userID := shared.UserIDFromContext(r.Context())

	client := &shared.SSEClient{
		Channel: "voice_signal_changes",
		Filter:  map[string]string{"target_user_id": userID},
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
			if _, err := fmt.Fprintf(w, "data: %s\n\n", data); err != nil {
				return
			}
			flusher.Flush()
		}
	}
}
