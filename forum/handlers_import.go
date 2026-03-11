package forum

import (
	"encoding/json"
	"net/http"

	shared "github.com/forumline/forumline/shared-go"
)

// HandleImport imports forum data from a forumline export file.
// POST /api/admin/import
// Body: the JSON export file from a hosted forum's export endpoint.
// Requires admin authentication.
func (h *Handlers) HandleImport(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
		return
	}

	// Verify the user is an admin
	userID := shared.UserIDFromContext(r.Context())
	if userID == "" {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "authentication required"})
		return
	}

	var isAdmin bool
	if err := h.Pool.QueryRow(r.Context(), "SELECT is_admin FROM profiles WHERE id = $1", userID).Scan(&isAdmin); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "database error"})
		return
	}
	if !isAdmin {
		writeJSON(w, http.StatusForbidden, map[string]string{"error": "admin access required"})
		return
	}

	var data ExportData
	if err := json.NewDecoder(r.Body).Decode(&data); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid export file"})
		return
	}

	if data.ForumlineVersion != "1" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "unsupported export version"})
		return
	}

	if err := Import(r.Context(), h.Pool, &data); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "import failed: " + err.Error()})
		return
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"ok":      true,
		"forum":   data.Forum,
		"message": "import complete",
	})
}
