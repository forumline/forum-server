package forum

import (
	"encoding/json"
	"errors"
	"log"
	"net/http"

	"github.com/forumline/forum-server/forum/service"
	"github.com/forumline/forum-server/forum/store"
	shared "github.com/forumline/forumline/shared-go"
)

// Handlers holds dependencies for all forum API handlers.
type Handlers struct {
	SSEHub          *shared.SSEHub
	Config          *Config
	Store           *store.Store
	ThreadSvc       *service.ThreadService
	PostSvc         *service.PostService
	ProfileSvc      *service.ProfileService
	ChatSvc         *service.ChatService
	AdminSvc        *service.AdminService
	NotificationSvc *service.NotificationService
}

// Config holds environment-driven configuration for the forum server.
type Config struct {
	GoTrueURL                string
	GoTrueServiceRoleKey     string
	SiteURL                  string
	Domain                   string
	ForumName                string
	IconURL                  string
	ForumlineURL             string
	ForumlineClientID        string
	ForumlineClientSecret    string
	ForumlineJWTSecret       string
	ForumlineGoTrueURL       string
	ForumlineServiceRoleKey  string
	LiveKitURL               string
	LiveKitAPIKey            string
	LiveKitAPISecret         string
	R2AccountID              string
	R2AccessKeyID            string
	R2SecretAccessKey        string
	R2BucketName             string
	R2PublicURL              string
}

func writeJSON(w http.ResponseWriter, status int, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(v); err != nil {
		log.Printf("json encode error: %v", err)
	}
}

func parseCookies(r *http.Request) map[string]string {
	cookies := make(map[string]string)
	for _, c := range r.Cookies() {
		cookies[c.Name] = c.Value
	}
	return cookies
}

// writeServiceError maps service-layer errors to HTTP status codes.
func writeServiceError(w http.ResponseWriter, err error) {
	var validationErr *service.ValidationError
	var notFoundErr *service.NotFoundError
	var forbiddenErr *service.ForbiddenError
	var conflictErr *service.ConflictError

	switch {
	case errors.As(err, &validationErr):
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": validationErr.Msg})
	case errors.As(err, &notFoundErr):
		writeJSON(w, http.StatusNotFound, map[string]string{"error": notFoundErr.Msg})
	case errors.As(err, &forbiddenErr):
		writeJSON(w, http.StatusForbidden, map[string]string{"error": forbiddenErr.Msg})
	case errors.As(err, &conflictErr):
		writeJSON(w, http.StatusConflict, map[string]string{"error": conflictErr.Msg})
	default:
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
	}
}
