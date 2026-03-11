package forum

import (
	"context"
	"fmt"
	"net/http"
	"strings"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"

	shared "github.com/forumline/forumline/shared-go"
)

func (h *Handlers) r2Client() (*minio.Client, error) {
	endpoint := fmt.Sprintf("%s.r2.cloudflarestorage.com", h.Config.R2AccountID)
	return minio.New(endpoint, &minio.Options{
		Creds:  credentials.NewStaticV4(h.Config.R2AccessKeyID, h.Config.R2SecretAccessKey, ""),
		Secure: true,
	})
}

// HandleAvatarUpload accepts a multipart file upload and stores it in R2.
// POST /api/avatars/upload
// Form fields: file (required), path (optional, defaults to user/{userId}/custom.png)
func (h *Handlers) HandleAvatarUpload(w http.ResponseWriter, r *http.Request) {
	userID := shared.UserIDFromContext(r.Context())

	// 5 MB max
	r.Body = http.MaxBytesReader(w, r.Body, 5<<20)
	if err := r.ParseMultipartForm(5 << 20); err != nil {
		http.Error(w, "File too large (max 5MB)", http.StatusBadRequest)
		return
	}

	file, header, err := r.FormFile("file")
	if err != nil {
		http.Error(w, "Missing file field", http.StatusBadRequest)
		return
	}
	defer func() { _ = file.Close() }()

	// Determine content type
	contentType := header.Header.Get("Content-Type")
	if contentType == "" {
		contentType = "image/png"
	}

	// Only allow image types
	if !strings.HasPrefix(contentType, "image/") {
		http.Error(w, "Only image files allowed", http.StatusBadRequest)
		return
	}

	// Determine path — allow caller to specify
	path := r.FormValue("path")
	if path == "" {
		path = fmt.Sprintf("user/%s/custom.png", userID)
	}
	// Security: allow user/{userID}/ for user avatars, thread/ for thread images (any authenticated user can upload)
	userPrefix := fmt.Sprintf("user/%s/", userID)
	if !strings.HasPrefix(path, userPrefix) && !strings.HasPrefix(path, "thread/") {
		http.Error(w, "Invalid path", http.StatusForbidden)
		return
	}

	client, err := h.r2Client()
	if err != nil {
		http.Error(w, "Storage unavailable", http.StatusInternalServerError)
		return
	}

	_, err = client.PutObject(context.Background(), h.Config.R2BucketName, path, file, header.Size, minio.PutObjectOptions{
		ContentType: contentType,
	})
	if err != nil {
		http.Error(w, "Upload failed", http.StatusInternalServerError)
		return
	}

	publicURL := fmt.Sprintf("%s/%s", strings.TrimRight(h.Config.R2PublicURL, "/"), path)
	writeJSON(w, http.StatusOK, map[string]string{"url": publicURL})
}
