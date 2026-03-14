package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/forumline/forum-server/forum"
	shared "github.com/forumline/forumline/shared-go"
)

func main() {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Auth uses FORUMLINE_JWT_SECRET for all JWT validation.
	// Set JWT_SECRET to match so shared.ValidateJWT works transparently.
	if jwtSecret := os.Getenv("FORUMLINE_JWT_SECRET"); jwtSecret != "" {
		if os.Getenv("JWT_SECRET") == "" {
			if err := os.Setenv("JWT_SECRET", jwtSecret); err != nil {
				log.Fatalf("failed to set JWT_SECRET: %v", err)
			}
		}
	}

	// Database
	pool, err := shared.NewDBPool(ctx)
	if err != nil {
		log.Fatalf("failed to connect to database: %v", err)
	}
	defer pool.Close()

	// SSE hub for LISTEN/NOTIFY — uses direct connection (bypasses PgBouncer)
	// to support LISTEN/NOTIFY which doesn't work in transaction pooling mode.
	listenDSN := os.Getenv("DATABASE_URL_DIRECT")
	if listenDSN == "" {
		// Fall back to the regular DATABASE_URL (works when not using PgBouncer)
		listenDSN = os.Getenv("DATABASE_URL")
	}
	sseHub := shared.NewSSEHub(listenDSN)
	sseHub.Listen(ctx, "notification_changes")
	sseHub.Listen(ctx, "chat_message_changes")
	sseHub.Listen(ctx, "voice_presence_changes")
	sseHub.Listen(ctx, "post_changes")
	sseHub.Listen(ctx, "voice_signal_changes")
	sseHub.StartListening(ctx)

	// Config
	domain := os.Getenv("FORUM_DOMAIN")
	if domain == "" {
		domain = "demo.forumline.net"
	}
	siteURL := os.Getenv("VITE_SITE_URL")
	if siteURL == "" {
		siteURL = "https://" + domain
	}

	cfg := &forum.Config{
		SiteURL:               siteURL,
		Domain:                domain,
		ForumName:             os.Getenv("FORUM_NAME"),
		IconURL:               os.Getenv("FORUM_ICON_URL"),
		ForumlineURL:          os.Getenv("FORUMLINE_APP_URL"),
		ForumlineClientID:     os.Getenv("FORUMLINE_CLIENT_ID"),
		ForumlineClientSecret: os.Getenv("FORUMLINE_CLIENT_SECRET"),
		ForumlineJWTSecret:    os.Getenv("FORUMLINE_JWT_SECRET"),
		LiveKitURL:            os.Getenv("LIVEKIT_URL"),
		LiveKitAPIKey:         os.Getenv("LIVEKIT_API_KEY"),
		LiveKitAPISecret:      os.Getenv("LIVEKIT_API_SECRET"),
		R2AccountID:           os.Getenv("R2_ACCOUNT_ID"),
		R2AccessKeyID:         os.Getenv("R2_ACCESS_KEY_ID"),
		R2SecretAccessKey:     os.Getenv("R2_SECRET_ACCESS_KEY"),
		R2BucketName:          os.Getenv("R2_BUCKET_NAME"),
		R2PublicURL:           os.Getenv("R2_PUBLIC_URL"),
	}

	// Router
	router := forum.NewRouter(pool, sseHub, cfg)

	// Wrap with global middleware
	var handler http.Handler = router
	handler = shared.CORSMiddleware(handler)
	handler = shared.SecurityHeaders(handler)

	// Static file serving (SPA fallback)
	handler = spaHandler(handler)

	port := os.Getenv("PORT")
	if port == "" {
		port = "3000"
	}

	srv := &http.Server{
		Addr:              ":" + port,
		Handler:           handler,
		ReadHeaderTimeout: 10 * time.Second,
	}

	// Graceful shutdown
	go func() {
		sigCh := make(chan os.Signal, 1)
		signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
		<-sigCh
		log.Println("shutting down...")
		cancel()
		shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer shutdownCancel()
		if err := srv.Shutdown(shutdownCtx); err != nil {
			log.Printf("shutdown error: %v", err)
		}
	}()

	// #nosec G706 -- port is from trusted env var
	log.Printf("forum server listening on http://localhost:%s", port)
	if err := srv.ListenAndServe(); err != http.ErrServerClosed {
		log.Fatalf("server error: %v", err)
	}
}

// spaHandler serves static files from ./dist and falls back to index.html
// for navigation routes (paths without file extensions).
func spaHandler(apiHandler http.Handler) http.Handler {
	distDir := "./dist"
	fileServer := http.FileServer(http.Dir(distDir))

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// API routes go to the router
		if strings.HasPrefix(r.URL.Path, "/api/") || strings.HasPrefix(r.URL.Path, "/.well-known/") {
			apiHandler.ServeHTTP(w, r)
			return
		}

		// Try to serve the static file
		path := filepath.Join(distDir, r.URL.Path)
		if info, err := os.Stat(path); err == nil && !info.IsDir() { // #nosec G703 -- path is cleaned by http.Dir
			// Hashed assets can be cached forever; index.html must not be cached
			if filepath.Base(r.URL.Path) == "index.html" {
				w.Header().Set("Cache-Control", "no-cache, no-store, must-revalidate")
			}
			fileServer.ServeHTTP(w, r)
			return
		}

		// If path has a file extension, it's a missing static file — 404
		if filepath.Ext(r.URL.Path) != "" {
			http.NotFound(w, r)
			return
		}

		// SPA fallback — serve index.html (no-cache so deploys take effect immediately)
		w.Header().Set("Cache-Control", "no-cache, no-store, must-revalidate")
		http.ServeFile(w, r, filepath.Join(distDir, "index.html"))
	})
}
