package model

// Profile represents a user profile.
type Profile struct {
	ID          string  `json:"id"`
	Username    string  `json:"username"`
	DisplayName *string `json:"display_name"`
	AvatarURL   *string `json:"avatar_url"`
	Bio         *string `json:"bio"`
	Website     *string `json:"website"`
	IsAdmin     bool    `json:"is_admin"`
	ForumlineID *string `json:"forumline_id"`
	CreatedAt   string  `json:"created_at"`
	UpdatedAt   string  `json:"updated_at"`
}

// Category represents a forum category.
type Category struct {
	ID          string  `json:"id"`
	Name        string  `json:"name"`
	Slug        string  `json:"slug"`
	Description *string `json:"description"`
	SortOrder   int     `json:"sort_order"`
	CreatedAt   string  `json:"created_at"`
}

// Thread represents a forum thread with author and category.
type Thread struct {
	ID         string   `json:"id"`
	CategoryID string   `json:"category_id"`
	AuthorID   string   `json:"author_id"`
	Title      string   `json:"title"`
	Slug       string   `json:"slug"`
	Content    *string  `json:"content"`
	ImageURL   *string  `json:"image_url"`
	IsPinned   bool     `json:"is_pinned"`
	IsLocked   bool     `json:"is_locked"`
	ViewCount  int      `json:"view_count"`
	PostCount  int      `json:"post_count"`
	LastPostAt *string  `json:"last_post_at"`
	CreatedAt  string   `json:"created_at"`
	UpdatedAt  string   `json:"updated_at"`
	Author     Profile  `json:"author"`
	Category   Category `json:"category"`
}

// Post represents a forum post with author.
type Post struct {
	ID        string  `json:"id"`
	ThreadID  string  `json:"thread_id"`
	AuthorID  string  `json:"author_id"`
	Content   string  `json:"content"`
	ReplyToID *string `json:"reply_to_id"`
	CreatedAt string  `json:"created_at"`
	UpdatedAt string  `json:"updated_at"`
	Author    Profile `json:"author"`
}

// Channel represents a chat channel.
type Channel struct {
	ID          string  `json:"id"`
	Name        string  `json:"name"`
	Slug        string  `json:"slug"`
	Description *string `json:"description"`
	CreatedAt   string  `json:"created_at"`
}

// VoiceRoom represents a voice room.
type VoiceRoom struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	Slug      string `json:"slug"`
	CreatedAt string `json:"created_at"`
}

// ChatMessage represents a chat message with author.
type ChatMessage struct {
	ID        string  `json:"id"`
	ChannelID string  `json:"channel_id"`
	AuthorID  string  `json:"author_id"`
	Content   string  `json:"content"`
	CreatedAt string  `json:"created_at"`
	Author    Profile `json:"author"`
}

// VoicePresence represents a user's voice presence with profile.
type VoicePresence struct {
	ID       string  `json:"id"`
	UserID   string  `json:"user_id"`
	RoomSlug string  `json:"room_slug"`
	JoinedAt string  `json:"joined_at"`
	Profile  Profile `json:"profile"`
}

// Bookmark represents a bookmarked thread.
type Bookmark struct {
	ID        string `json:"id"`
	CreatedAt string `json:"created_at"`
	Thread    Thread `json:"thread"`
}

// Notification represents a notification.
type Notification struct {
	ID        string  `json:"id"`
	UserID    string  `json:"user_id"`
	Type      string  `json:"type"`
	Title     string  `json:"title"`
	Message   string  `json:"message"`
	Link      *string `json:"link"`
	Read      bool    `json:"read"`
	CreatedAt string  `json:"created_at"`
}

// ForumlineNotification represents a notification in the forumline protocol format.
type ForumlineNotification struct {
	ID          string `json:"id"`
	Type        string `json:"type"`
	Title       string `json:"title"`
	Body        string `json:"body"`
	Link        string `json:"link"`
	Read        bool   `json:"read"`
	Timestamp   string `json:"timestamp"`
	ForumDomain string `json:"forum_domain"`
}

// NotificationPreference represents a user's notification preference.
type NotificationPreference struct {
	Category string `json:"category"`
	Enabled  bool   `json:"enabled"`
}

// ForumlinePushItem holds a notification to be pushed to forumline.
type ForumlinePushItem struct {
	ForumlineUserID string `json:"forumline_user_id"`
	Type            string `json:"type"`
	Title           string `json:"title"`
	Body            string `json:"body"`
	Link            string `json:"link"`
}

// ForumlineIdentity represents a Forumline identity.
type ForumlineIdentity struct {
	ForumlineID string `json:"forumline_id"`
	Username    string `json:"username"`
	DisplayName string `json:"display_name"`
	AvatarURL   string `json:"avatar_url"`
	Bio         string `json:"bio,omitempty"`
}

// AdminStats holds admin dashboard statistics.
type AdminStats struct {
	TotalUsers   int `json:"totalUsers"`
	TotalThreads int `json:"totalThreads"`
	TotalPosts   int `json:"totalPosts"`
}
