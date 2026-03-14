package forum

import (
	"encoding/json"
	"log"
	"net/http"

	shared "github.com/forumline/forumline/shared-go"
)

// Handlers holds dependencies for all forum API handlers.
// Pool is the shared.DB interface — in single-tenant mode this is a *pgxpool.Pool,
// in multi-tenant mode this is a *TenantPool that sets search_path per-request.
type Handlers struct {
	Pool   shared.DB
	SSEHub *shared.SSEHub
	Config *Config
}

// Config holds environment-driven configuration for the forum server.
type Config struct {
	GoTrueURL            string
	GoTrueServiceRoleKey       string
	SiteURL              string
	Domain               string
	ForumName            string
	IconURL              string
	ForumlineURL               string
	ForumlineWebhookURL        string // direct URL for webhook push (falls back to ForumlineURL)
	ForumlineClientID          string
	ForumlineClientSecret      string
	ForumlineJWTSecret         string
	ForumlineGoTrueURL       string
	ForumlineServiceRoleKey    string
	LiveKitURL           string
	LiveKitAPIKey        string
	LiveKitAPISecret     string
	R2AccountID          string
	R2AccessKeyID        string
	R2SecretAccessKey    string
	R2BucketName         string
	R2PublicURL          string
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
