package forum

import (
	"encoding/json"
	"net/http"

	shared "github.com/forumline/forumline/shared-go"
)

// HandleImport imports forum data from a forumline export file.
// POST /api/admin/import
func (h *Handlers) HandleImport(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
		return
	}

	userID := shared.UserIDFromContext(r.Context())
	if userID == "" {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "authentication required"})
		return
	}

	if err := h.AdminSvc.VerifyAdmin(r.Context(), userID); err != nil {
		writeServiceError(w, err)
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

	if err := Import(r.Context(), h.Store.DB, &data); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "import failed: " + err.Error()})
		return
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"ok":      true,
		"forum":   data.Forum,
		"message": "import complete",
	})
}
