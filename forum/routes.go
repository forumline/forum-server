package forum

import (
	"github.com/go-chi/chi/v5"

	"github.com/forumline/forum-server/forum/service"
	"github.com/forumline/forum-server/forum/store"
	shared "github.com/forumline/forumline/shared-go"
)

func NewRouter(pool shared.DB, sseHub *shared.SSEHub, cfg *Config) *chi.Mux {
	r := chi.NewRouter()

	// Create layers
	s := store.New(pool)

	notifSvc := service.NewNotificationService(s, &service.NotificationConfig{
		ForumlineURL:          cfg.ForumlineURL,
		ForumlineClientID:     cfg.ForumlineClientID,
		ForumlineClientSecret: cfg.ForumlineClientSecret,
	})
	threadSvc := service.NewThreadService(s)
	postSvc := service.NewPostService(s, notifSvc)
	profileSvc := service.NewProfileService(s)
	chatSvc := service.NewChatService(s)
	adminSvc := service.NewAdminService(s)

	h := &Handlers{
		SSEHub:          sseHub,
		Config:          cfg,
		Store:           s,
		ThreadSvc:       threadSvc,
		PostSvc:         postSvc,
		ProfileSvc:      profileSvc,
		ChatSvc:         chatSvc,
		AdminSvc:        adminSvc,
		NotificationSvc: notifSvc,
	}

	// Channel follows (authenticated)
	r.Group(func(r chi.Router) {
		r.Use(shared.AuthMiddleware)
		r.Get("/api/channel-follows", h.HandleChannelFollows)
		r.Post("/api/channel-follows", h.HandleChannelFollows)
		r.Delete("/api/channel-follows", h.HandleChannelFollows)
	})

	// Notification preferences (authenticated)
	r.Group(func(r chi.Router) {
		r.Use(shared.AuthMiddleware)
		r.Get("/api/notification-preferences", h.HandleNotificationPreferences)
		r.Put("/api/notification-preferences", h.HandleNotificationPreferences)
	})

	// Forumline OAuth
	r.Get("/api/forumline/auth", h.HandleForumlineAuth)
	r.Post("/api/forumline/auth", h.HandleForumlineAuth)
	r.Get("/api/forumline/auth/callback", h.HandleForumlineCallback)
	r.Get("/api/forumline/auth/forumline-token", h.HandleForumlineToken)
	r.Get("/api/forumline/auth/session", h.HandleForumlineSession)
	r.Delete("/api/forumline/auth/session", h.HandleForumlineSession)

	// Forumline notifications
	r.Get("/api/forumline/notifications", h.HandleNotifications)
	r.Post("/api/forumline/notifications/read", h.HandleNotificationRead)
	r.Get("/api/forumline/unread", h.HandleUnread)
	r.Get("/api/forumline/notifications/stream", h.HandleNotificationStream)

	// LiveKit
	r.Post("/api/livekit", h.HandleLiveKitToken)
	r.Get("/api/livekit", h.HandleLiveKitParticipants)

	// ================================================================
	// Data endpoints (Phase B)
	// ================================================================

	// Forum config (public)
	r.Get("/api/config", h.HandleConfig)

	// Static/config (public)
	r.Get("/api/categories", h.HandleCategories)
	r.Get("/api/categories/{slug}", h.HandleCategoryBySlug)
	r.Get("/api/channels", h.HandleChannels)
	r.Get("/api/voice-rooms", h.HandleVoiceRooms)

	// Threads (public reads)
	r.Get("/api/threads", h.HandleThreads)
	r.Get("/api/threads/{id}", h.HandleThread)
	r.Get("/api/categories/{slug}/threads", h.HandleThreadsByCategory)
	r.Get("/api/users/{id}/threads", h.HandleUserThreads)
	r.Get("/api/search/threads", h.HandleSearchThreads)

	// Posts (public reads + stream)
	r.Get("/api/threads/{id}/posts", h.HandlePosts)
	r.Get("/api/threads/{id}/stream", h.HandlePostStream)
	r.Get("/api/users/{id}/posts", h.HandleUserPosts)
	r.Get("/api/search/posts", h.HandleSearchPosts)

	// Profiles (public reads)
	r.Get("/api/profiles/batch", h.HandleProfilesBatch)
	r.Get("/api/profiles/by-username/{username}", h.HandleProfileByUsername)
	r.Get("/api/profiles/{id}", h.HandleProfile)

	// Chat messages (public read)
	r.Get("/api/channels/{slug}/messages", h.HandleChatMessages)

	// Voice presence (public read)
	r.Get("/api/voice-presence", h.HandleVoicePresence)

	// Authenticated data endpoints
	r.Group(func(r chi.Router) {
		r.Use(shared.AuthMiddleware)

		// Thread writes
		r.Post("/api/threads", h.HandleCreateThread)
		r.Patch("/api/threads/{id}", h.HandleUpdateThread)

		// Post writes
		r.Post("/api/posts", h.HandleCreatePost)

		// Chat writes & stream
		r.Post("/api/channels/{slug}/messages", h.HandleSendChatMessage)
		r.Post("/api/channels/_by-id/{id}/messages", h.HandleSendChatMessageByID)
		r.Get("/api/channels/{slug}/stream", h.HandleChatStream)

		// Bookmarks
		r.Get("/api/bookmarks", h.HandleBookmarks)
		r.Get("/api/bookmarks/{threadId}/status", h.HandleBookmarkStatus)
		r.Post("/api/bookmarks", h.HandleAddBookmark)
		r.Delete("/api/bookmarks/{threadId}", h.HandleRemoveBookmark)
		r.Delete("/api/bookmarks/by-id/{id}", h.HandleRemoveBookmarkByID)

		// Notifications (data provider)
		r.Get("/api/notifications", h.HandleNotificationsData)
		r.Post("/api/notifications/read-all", h.HandleMarkAllNotificationsRead)

		// Profile writes
		r.Put("/api/profiles/{id}", h.HandleUpsertProfile)
		r.Delete("/api/profiles/{id}/forumline-id", h.HandleClearForumlineID)

		// Voice presence writes & stream
		r.Put("/api/voice-presence", h.HandleSetVoicePresence)
		r.Delete("/api/voice-presence", h.HandleClearVoicePresence)
		r.Get("/api/voice-presence/stream", h.HandleVoicePresenceStream)

		// Voice P2P signaling
		r.Post("/api/voice-signal", h.HandleVoiceSignal)
		r.Get("/api/voice-signal/stream", h.HandleVoiceSignalStream)

		// Avatars
		r.Post("/api/avatars/upload", h.HandleAvatarUpload)

		// Admin
		r.Get("/api/admin/stats", h.HandleAdminStats)
		r.Get("/api/admin/users", h.HandleAdminUsers)
		r.Post("/api/admin/import", h.HandleImport)
	})

	// Forumline manifest (discovery)
	r.Get("/.well-known/forumline-manifest.json", h.HandleManifest)

	return r
}
