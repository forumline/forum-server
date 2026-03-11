package forum

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"strings"
	"time"

	shared "github.com/forumline/forumline/shared-go"
	"github.com/livekit/protocol/auth"
	"github.com/livekit/protocol/livekit"
	lksdk "github.com/livekit/server-sdk-go/v2"
)

// HandleLiveKitToken handles POST /api/livekit — generates an access token for joining a room.
func (h *Handlers) HandleLiveKitToken(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "Method not allowed"})
		return
	}

	var body struct {
		RoomName        string `json:"roomName"`
		ParticipantName string `json:"participantName"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
		return
	}
	if body.RoomName == "" || body.ParticipantName == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "roomName and participantName are required"})
		return
	}

	// Authenticate
	userID, err := h.authenticateFromHeader(r)
	if err != nil {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "Missing authorization token"})
		return
	}

	apiKey := h.Config.LiveKitAPIKey
	apiSecret := h.Config.LiveKitAPISecret
	livekitURL := h.Config.LiveKitURL
	if apiKey == "" || apiSecret == "" || livekitURL == "" {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "LiveKit not configured"})
		return
	}

	// Remove user from any existing rooms (enforce one room at a time, clean up ghosts)
	httpHost := strings.Replace(strings.Replace(livekitURL, "wss://", "https://", 1), "ws://", "http://", 1)
	roomClient := lksdk.NewRoomServiceClient(httpHost, apiKey, apiSecret)

	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	rooms, err := roomClient.ListRooms(ctx, &livekit.ListRoomsRequest{})
	if err == nil && rooms != nil {
		for _, room := range rooms.Rooms {
			_, removeErr := roomClient.RemoveParticipant(ctx, &livekit.RoomParticipantIdentity{
				Room:     room.Name,
				Identity: userID,
			})
			_ = removeErr // Not in this room — ignore
		}
	}

	// Generate access token
	at := auth.NewAccessToken(apiKey, apiSecret)
	grant := &auth.VideoGrant{
		Room:         body.RoomName,
		RoomJoin:     true,
		CanPublish:   boolPtr(true),
		CanSubscribe: boolPtr(true),
	}
	at.SetVideoGrant(grant).
		SetIdentity(userID).
		SetName(body.ParticipantName).
		SetValidFor(6 * time.Hour)

	token, err := at.ToJWT()
	if err != nil {
		log.Printf("[LiveKit] Failed to generate token: %v", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "Failed to generate token"})
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"token": token})
}

// HandleLiveKitParticipants handles GET /api/livekit — lists participants.
func (h *Handlers) HandleLiveKitParticipants(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "Method not allowed"})
		return
	}

	_ = shared.UserIDFromContext(r.Context()) // not required for this endpoint

	apiKey := h.Config.LiveKitAPIKey
	apiSecret := h.Config.LiveKitAPISecret
	livekitURL := h.Config.LiveKitURL
	if apiKey == "" || apiSecret == "" || livekitURL == "" {
		writeJSON(w, http.StatusOK, map[string]interface{}{"participants": []interface{}{}})
		return
	}

	httpHost := strings.Replace(strings.Replace(livekitURL, "wss://", "https://", 1), "ws://", "http://", 1)
	roomClient := lksdk.NewRoomServiceClient(httpHost, apiKey, apiSecret)

	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	roomName := r.URL.Query().Get("room")
	if roomName != "" {
		// Single room participants
		resp, err := roomClient.ListParticipants(ctx, &livekit.ListParticipantsRequest{Room: roomName})
		if err != nil {
			writeJSON(w, http.StatusOK, map[string]interface{}{"participants": []interface{}{}})
			return
		}

		type participant struct {
			Identity string `json:"identity"`
			Name     string `json:"name"`
		}
		var participants []participant
		for _, p := range resp.Participants {
			name := p.Name
			if name == "" {
				name = p.Identity
			}
			participants = append(participants, participant{Identity: p.Identity, Name: name})
		}
		if participants == nil {
			participants = []participant{}
		}
		writeJSON(w, http.StatusOK, map[string]interface{}{"participants": participants})
		return
	}

	// All rooms' participants
	roomsResp, err := roomClient.ListRooms(ctx, &livekit.ListRoomsRequest{})
	if err != nil {
		writeJSON(w, http.StatusOK, map[string]interface{}{"rooms": map[string]interface{}{}})
		return
	}

	type roomInfo struct {
		Count      int      `json:"count"`
		Names      []string `json:"names"`
		Identities []string `json:"identities"`
	}
	result := make(map[string]roomInfo)

	for _, room := range roomsResp.Rooms {
		pResp, err := roomClient.ListParticipants(ctx, &livekit.ListParticipantsRequest{Room: room.Name})
		if err != nil || len(pResp.Participants) == 0 {
			continue
		}
		info := roomInfo{Count: len(pResp.Participants)}
		for _, p := range pResp.Participants {
			name := p.Name
			if name == "" {
				name = p.Identity
			}
			info.Names = append(info.Names, name)
			info.Identities = append(info.Identities, p.Identity)
		}
		result[room.Name] = info
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{"rooms": result})
}

func boolPtr(b bool) *bool {
	return &b
}
