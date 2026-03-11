package forum

import (
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	shared "github.com/forumline/forumline/shared-go"
)

// ============================================================================
// Static / Config endpoints (public)
// ============================================================================

// HandleCategories handles GET /api/categories
func (h *Handlers) HandleCategories(w http.ResponseWriter, r *http.Request) {
	rows, err := h.Pool.Query(r.Context(),
		`SELECT id, name, slug, description, sort_order, created_at
		 FROM categories ORDER BY sort_order`)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	defer rows.Close()

	type category struct {
		ID          string  `json:"id"`
		Name        string  `json:"name"`
		Slug        string  `json:"slug"`
		Description *string `json:"description"`
		SortOrder   int     `json:"sort_order"`
		CreatedAt   string  `json:"created_at"`
	}

	var categories []category
	for rows.Next() {
		var c category
		var createdAt time.Time
		if err := rows.Scan(&c.ID, &c.Name, &c.Slug, &c.Description, &c.SortOrder, &createdAt); err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}
		c.CreatedAt = createdAt.Format(time.RFC3339)
		categories = append(categories, c)
	}
	if categories == nil {
		categories = []category{}
	}
	writeJSON(w, http.StatusOK, categories)
}

// HandleCategoryBySlug handles GET /api/categories/{slug}
func (h *Handlers) HandleCategoryBySlug(w http.ResponseWriter, r *http.Request) {
	slug := chi.URLParam(r, "slug")

	type category struct {
		ID          string  `json:"id"`
		Name        string  `json:"name"`
		Slug        string  `json:"slug"`
		Description *string `json:"description"`
		SortOrder   int     `json:"sort_order"`
		CreatedAt   string  `json:"created_at"`
	}

	var c category
	var createdAt time.Time
	err := h.Pool.QueryRow(r.Context(),
		`SELECT id, name, slug, description, sort_order, created_at
		 FROM categories WHERE slug = $1`, slug).
		Scan(&c.ID, &c.Name, &c.Slug, &c.Description, &c.SortOrder, &createdAt)
	if err != nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "category not found"})
		return
	}
	c.CreatedAt = createdAt.Format(time.RFC3339)
	writeJSON(w, http.StatusOK, c)
}

// HandleChannels handles GET /api/channels
func (h *Handlers) HandleChannels(w http.ResponseWriter, r *http.Request) {
	rows, err := h.Pool.Query(r.Context(),
		`SELECT id, name, slug, description, created_at
		 FROM chat_channels ORDER BY name`)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	defer rows.Close()

	type channel struct {
		ID          string  `json:"id"`
		Name        string  `json:"name"`
		Slug        string  `json:"slug"`
		Description *string `json:"description"`
		CreatedAt   string  `json:"created_at"`
	}

	var channels []channel
	for rows.Next() {
		var c channel
		var createdAt time.Time
		if err := rows.Scan(&c.ID, &c.Name, &c.Slug, &c.Description, &createdAt); err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}
		c.CreatedAt = createdAt.Format(time.RFC3339)
		channels = append(channels, c)
	}
	if channels == nil {
		channels = []channel{}
	}
	writeJSON(w, http.StatusOK, channels)
}

// HandleVoiceRooms handles GET /api/voice-rooms
func (h *Handlers) HandleVoiceRooms(w http.ResponseWriter, r *http.Request) {
	rows, err := h.Pool.Query(r.Context(),
		`SELECT id, name, slug, created_at
		 FROM voice_rooms ORDER BY name`)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	defer rows.Close()

	type voiceRoom struct {
		ID        string `json:"id"`
		Name      string `json:"name"`
		Slug      string `json:"slug"`
		CreatedAt string `json:"created_at"`
	}

	var rooms []voiceRoom
	for rows.Next() {
		var room voiceRoom
		var createdAt time.Time
		if err := rows.Scan(&room.ID, &room.Name, &room.Slug, &createdAt); err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}
		room.CreatedAt = createdAt.Format(time.RFC3339)
		rooms = append(rooms, room)
	}
	if rooms == nil {
		rooms = []voiceRoom{}
	}
	writeJSON(w, http.StatusOK, rooms)
}

// ============================================================================
// Threads
// ============================================================================

type profileJSON struct {
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

type categoryJSON struct {
	ID          string  `json:"id"`
	Name        string  `json:"name"`
	Slug        string  `json:"slug"`
	Description *string `json:"description"`
	SortOrder   int     `json:"sort_order"`
	CreatedAt   string  `json:"created_at"`
}

type threadJSON struct {
	ID         string       `json:"id"`
	CategoryID string       `json:"category_id"`
	AuthorID   string       `json:"author_id"`
	Title      string       `json:"title"`
	Slug       string       `json:"slug"`
	Content    *string      `json:"content"`
	ImageURL   *string      `json:"image_url"`
	IsPinned   bool         `json:"is_pinned"`
	IsLocked   bool         `json:"is_locked"`
	ViewCount  int          `json:"view_count"`
	PostCount  int          `json:"post_count"`
	LastPostAt *string      `json:"last_post_at"`
	CreatedAt  string       `json:"created_at"`
	UpdatedAt  string       `json:"updated_at"`
	Author     profileJSON  `json:"author"`
	Category   categoryJSON `json:"category"`
}

func scanThreadWithAuthor(scan func(dest ...interface{}) error) (threadJSON, error) {
	var t threadJSON
	var lastPostAt, createdAt, updatedAt time.Time
	var hasLastPost bool
	var authorCreatedAt, authorUpdatedAt time.Time
	var catCreatedAt time.Time

	err := scan(
		&t.ID, &t.CategoryID, &t.AuthorID, &t.Title, &t.Slug, &t.Content, &t.ImageURL,
		&t.IsPinned, &t.IsLocked, &t.ViewCount, &t.PostCount, &lastPostAt, &createdAt, &updatedAt,
		// author
		&t.Author.ID, &t.Author.Username, &t.Author.DisplayName, &t.Author.AvatarURL,
		&t.Author.Bio, &t.Author.Website, &t.Author.IsAdmin, &t.Author.ForumlineID,
		&authorCreatedAt, &authorUpdatedAt,
		// category
		&t.Category.ID, &t.Category.Name, &t.Category.Slug, &t.Category.Description,
		&t.Category.SortOrder, &catCreatedAt,
	)
	if err != nil {
		return t, err
	}

	t.CreatedAt = createdAt.Format(time.RFC3339)
	t.UpdatedAt = updatedAt.Format(time.RFC3339)
	hasLastPost = !lastPostAt.IsZero()
	if hasLastPost {
		s := lastPostAt.Format(time.RFC3339)
		t.LastPostAt = &s
	}
	t.Author.CreatedAt = authorCreatedAt.Format(time.RFC3339)
	t.Author.UpdatedAt = authorUpdatedAt.Format(time.RFC3339)
	t.Category.CreatedAt = catCreatedAt.Format(time.RFC3339)

	return t, nil
}

const threadWithJoinsQuery = `
SELECT t.id, t.category_id, t.author_id, t.title, t.slug, t.content, t.image_url,
       t.is_pinned, t.is_locked, t.view_count, t.post_count,
       COALESCE(t.last_post_at, t.created_at), t.created_at, t.updated_at,
       p.id, p.username, p.display_name, p.avatar_url, p.bio, p.website,
       p.is_admin, p.forumline_id, p.created_at, p.updated_at,
       c.id, c.name, c.slug, c.description, c.sort_order, c.created_at
FROM threads t
JOIN profiles p ON p.id = t.author_id
JOIN categories c ON c.id = t.category_id`

// HandleThreads handles GET /api/threads
func (h *Handlers) HandleThreads(w http.ResponseWriter, r *http.Request) {
	limit := 20
	if l := r.URL.Query().Get("limit"); l != "" {
		if n := parseInt(l, 20); n > 0 && n <= 100 {
			limit = n
		}
	}

	rows, err := h.Pool.Query(r.Context(),
		threadWithJoinsQuery+` ORDER BY t.is_pinned DESC, t.last_post_at DESC NULLS LAST LIMIT $1`, limit)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	defer rows.Close()

	var threads []threadJSON
	for rows.Next() {
		t, err := scanThreadWithAuthor(rows.Scan)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}
		threads = append(threads, t)
	}
	if threads == nil {
		threads = []threadJSON{}
	}
	writeJSON(w, http.StatusOK, threads)
}

// HandleThread handles GET /api/threads/{id}
func (h *Handlers) HandleThread(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")

	row := h.Pool.QueryRow(r.Context(), threadWithJoinsQuery+` WHERE t.id = $1`, id)
	t, err := scanThreadWithAuthor(row.Scan)
	if err != nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "thread not found"})
		return
	}
	writeJSON(w, http.StatusOK, t)
}

// HandleThreadsByCategory handles GET /api/categories/{slug}/threads
func (h *Handlers) HandleThreadsByCategory(w http.ResponseWriter, r *http.Request) {
	slug := chi.URLParam(r, "slug")

	rows, err := h.Pool.Query(r.Context(),
		threadWithJoinsQuery+` WHERE c.slug = $1 ORDER BY t.is_pinned DESC, t.last_post_at DESC NULLS LAST`, slug)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	defer rows.Close()

	var threads []threadJSON
	for rows.Next() {
		t, err := scanThreadWithAuthor(rows.Scan)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}
		threads = append(threads, t)
	}
	if threads == nil {
		threads = []threadJSON{}
	}
	writeJSON(w, http.StatusOK, threads)
}

// HandleUserThreads handles GET /api/users/{id}/threads
func (h *Handlers) HandleUserThreads(w http.ResponseWriter, r *http.Request) {
	userID := chi.URLParam(r, "id")

	rows, err := h.Pool.Query(r.Context(),
		threadWithJoinsQuery+` WHERE t.author_id = $1 ORDER BY t.created_at DESC LIMIT 10`, userID)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	defer rows.Close()

	var threads []threadJSON
	for rows.Next() {
		t, err := scanThreadWithAuthor(rows.Scan)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}
		threads = append(threads, t)
	}
	if threads == nil {
		threads = []threadJSON{}
	}
	writeJSON(w, http.StatusOK, threads)
}

// HandleSearchThreads handles GET /api/search/threads?q=
func (h *Handlers) HandleSearchThreads(w http.ResponseWriter, r *http.Request) {
	q := strings.TrimSpace(r.URL.Query().Get("q"))
	if q == "" {
		writeJSON(w, http.StatusOK, []threadJSON{})
		return
	}
	pattern := "%" + q + "%"

	rows, err := h.Pool.Query(r.Context(),
		threadWithJoinsQuery+` WHERE t.title ILIKE $1 ORDER BY t.created_at DESC LIMIT 20`, pattern)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	defer rows.Close()

	var threads []threadJSON
	for rows.Next() {
		t, err := scanThreadWithAuthor(rows.Scan)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}
		threads = append(threads, t)
	}
	if threads == nil {
		threads = []threadJSON{}
	}
	writeJSON(w, http.StatusOK, threads)
}

// ============================================================================
// Posts
// ============================================================================

type postJSON struct {
	ID        string      `json:"id"`
	ThreadID  string      `json:"thread_id"`
	AuthorID  string      `json:"author_id"`
	Content   string      `json:"content"`
	ReplyToID *string     `json:"reply_to_id"`
	CreatedAt string      `json:"created_at"`
	UpdatedAt string      `json:"updated_at"`
	Author    profileJSON `json:"author"`
}

const postWithJoinsQuery = `
SELECT po.id, po.thread_id, po.author_id, po.content, po.reply_to_id, po.created_at, po.updated_at,
       p.id, p.username, p.display_name, p.avatar_url, p.bio, p.website,
       p.is_admin, p.forumline_id, p.created_at, p.updated_at
FROM posts po
JOIN profiles p ON p.id = po.author_id`

func scanPostWithAuthor(scan func(dest ...interface{}) error) (postJSON, error) {
	var po postJSON
	var createdAt, updatedAt time.Time
	var authorCreatedAt, authorUpdatedAt time.Time

	err := scan(
		&po.ID, &po.ThreadID, &po.AuthorID, &po.Content, &po.ReplyToID, &createdAt, &updatedAt,
		&po.Author.ID, &po.Author.Username, &po.Author.DisplayName, &po.Author.AvatarURL,
		&po.Author.Bio, &po.Author.Website, &po.Author.IsAdmin, &po.Author.ForumlineID,
		&authorCreatedAt, &authorUpdatedAt,
	)
	if err != nil {
		return po, err
	}

	po.CreatedAt = createdAt.Format(time.RFC3339)
	po.UpdatedAt = updatedAt.Format(time.RFC3339)
	po.Author.CreatedAt = authorCreatedAt.Format(time.RFC3339)
	po.Author.UpdatedAt = authorUpdatedAt.Format(time.RFC3339)
	return po, nil
}

// HandlePosts handles GET /api/threads/{id}/posts
func (h *Handlers) HandlePosts(w http.ResponseWriter, r *http.Request) {
	threadID := chi.URLParam(r, "id")

	rows, err := h.Pool.Query(r.Context(),
		postWithJoinsQuery+` WHERE po.thread_id = $1 ORDER BY po.created_at ASC`, threadID)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	defer rows.Close()

	var posts []postJSON
	for rows.Next() {
		p, err := scanPostWithAuthor(rows.Scan)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}
		posts = append(posts, p)
	}
	if posts == nil {
		posts = []postJSON{}
	}
	writeJSON(w, http.StatusOK, posts)
}

// HandleUserPosts handles GET /api/users/{id}/posts
func (h *Handlers) HandleUserPosts(w http.ResponseWriter, r *http.Request) {
	userID := chi.URLParam(r, "id")

	rows, err := h.Pool.Query(r.Context(),
		postWithJoinsQuery+` WHERE po.author_id = $1 ORDER BY po.created_at DESC LIMIT 20`, userID)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	defer rows.Close()

	var posts []postJSON
	for rows.Next() {
		p, err := scanPostWithAuthor(rows.Scan)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}
		posts = append(posts, p)
	}
	if posts == nil {
		posts = []postJSON{}
	}
	writeJSON(w, http.StatusOK, posts)
}

// HandleSearchPosts handles GET /api/search/posts?q=
func (h *Handlers) HandleSearchPosts(w http.ResponseWriter, r *http.Request) {
	q := strings.TrimSpace(r.URL.Query().Get("q"))
	if q == "" {
		writeJSON(w, http.StatusOK, []postJSON{})
		return
	}
	pattern := "%" + q + "%"

	rows, err := h.Pool.Query(r.Context(),
		postWithJoinsQuery+` WHERE po.content ILIKE $1 ORDER BY po.created_at DESC LIMIT 20`, pattern)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	defer rows.Close()

	var posts []postJSON
	for rows.Next() {
		p, err := scanPostWithAuthor(rows.Scan)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}
		posts = append(posts, p)
	}
	if posts == nil {
		posts = []postJSON{}
	}
	writeJSON(w, http.StatusOK, posts)
}

// ============================================================================
// Profiles
// ============================================================================

func scanProfile(scan func(dest ...interface{}) error) (profileJSON, error) {
	var p profileJSON
	var createdAt, updatedAt time.Time
	err := scan(&p.ID, &p.Username, &p.DisplayName, &p.AvatarURL, &p.Bio, &p.Website,
		&p.IsAdmin, &p.ForumlineID, &createdAt, &updatedAt)
	if err != nil {
		return p, err
	}
	p.CreatedAt = createdAt.Format(time.RFC3339)
	p.UpdatedAt = updatedAt.Format(time.RFC3339)
	return p, nil
}

const profileColumns = `id, username, display_name, avatar_url, bio, website, is_admin, forumline_id, created_at, updated_at`

// HandleProfile handles GET /api/profiles/{id}
func (h *Handlers) HandleProfile(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	row := h.Pool.QueryRow(r.Context(),
		`SELECT `+profileColumns+` FROM profiles WHERE id = $1`, id)
	p, err := scanProfile(row.Scan)
	if err != nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "profile not found"})
		return
	}
	writeJSON(w, http.StatusOK, p)
}

// HandleProfileByUsername handles GET /api/profiles/by-username/{username}
func (h *Handlers) HandleProfileByUsername(w http.ResponseWriter, r *http.Request) {
	username := chi.URLParam(r, "username")
	row := h.Pool.QueryRow(r.Context(),
		`SELECT `+profileColumns+` FROM profiles WHERE username = $1`, username)
	p, err := scanProfile(row.Scan)
	if err != nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "profile not found"})
		return
	}
	writeJSON(w, http.StatusOK, p)
}

// HandleProfilesBatch handles GET /api/profiles/batch?ids=id1,id2,...
func (h *Handlers) HandleProfilesBatch(w http.ResponseWriter, r *http.Request) {
	idsParam := r.URL.Query().Get("ids")
	if idsParam == "" {
		writeJSON(w, http.StatusOK, []profileJSON{})
		return
	}
	ids := strings.Split(idsParam, ",")

	rows, err := h.Pool.Query(r.Context(),
		`SELECT `+profileColumns+` FROM profiles WHERE id = ANY($1)`, ids)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	defer rows.Close()

	var profiles []profileJSON
	for rows.Next() {
		p, err := scanProfile(rows.Scan)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}
		profiles = append(profiles, p)
	}
	if profiles == nil {
		profiles = []profileJSON{}
	}
	writeJSON(w, http.StatusOK, profiles)
}

// ============================================================================
// Chat Messages
// ============================================================================

type chatMessageJSON struct {
	ID        string      `json:"id"`
	ChannelID string      `json:"channel_id"`
	AuthorID  string      `json:"author_id"`
	Content   string      `json:"content"`
	CreatedAt string      `json:"created_at"`
	Author    profileJSON `json:"author"`
}

// HandleChatMessages handles GET /api/channels/{slug}/messages
func (h *Handlers) HandleChatMessages(w http.ResponseWriter, r *http.Request) {
	slug := chi.URLParam(r, "slug")

	rows, err := h.Pool.Query(r.Context(),
		`SELECT m.id, m.channel_id, m.author_id, m.content, m.created_at,
		        p.id, p.username, p.display_name, p.avatar_url, p.bio, p.website,
		        p.is_admin, p.forumline_id, p.created_at, p.updated_at
		 FROM chat_messages m
		 JOIN chat_channels ch ON ch.id = m.channel_id
		 JOIN profiles p ON p.id = m.author_id
		 WHERE ch.slug = $1
		 ORDER BY m.created_at ASC
		 LIMIT 100`, slug)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	defer rows.Close()

	var messages []chatMessageJSON
	for rows.Next() {
		var m chatMessageJSON
		var msgCreatedAt, authorCreatedAt, authorUpdatedAt time.Time
		err := rows.Scan(
			&m.ID, &m.ChannelID, &m.AuthorID, &m.Content, &msgCreatedAt,
			&m.Author.ID, &m.Author.Username, &m.Author.DisplayName, &m.Author.AvatarURL,
			&m.Author.Bio, &m.Author.Website, &m.Author.IsAdmin, &m.Author.ForumlineID,
			&authorCreatedAt, &authorUpdatedAt,
		)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}
		m.CreatedAt = msgCreatedAt.Format(time.RFC3339)
		m.Author.CreatedAt = authorCreatedAt.Format(time.RFC3339)
		m.Author.UpdatedAt = authorUpdatedAt.Format(time.RFC3339)
		messages = append(messages, m)
	}
	if messages == nil {
		messages = []chatMessageJSON{}
	}
	writeJSON(w, http.StatusOK, messages)
}

// ============================================================================
// Voice Presence
// ============================================================================

type voicePresenceJSON struct {
	ID       string      `json:"id"`
	UserID   string      `json:"user_id"`
	RoomSlug string      `json:"room_slug"`
	JoinedAt string      `json:"joined_at"`
	Profile  profileJSON `json:"profile"`
}

// HandleVoicePresence handles GET /api/voice-presence
func (h *Handlers) HandleVoicePresence(w http.ResponseWriter, r *http.Request) {
	rows, err := h.Pool.Query(r.Context(),
		`SELECT vp.id, vp.user_id, vp.room_slug, vp.joined_at,
		        p.id, p.username, p.display_name, p.avatar_url, p.bio, p.website,
		        p.is_admin, p.forumline_id, p.created_at, p.updated_at
		 FROM voice_presence vp
		 JOIN profiles p ON p.id = vp.user_id`)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	defer rows.Close()

	var presence []voicePresenceJSON
	for rows.Next() {
		var vp voicePresenceJSON
		var joinedAt, authorCreatedAt, authorUpdatedAt time.Time
		err := rows.Scan(
			&vp.ID, &vp.UserID, &vp.RoomSlug, &joinedAt,
			&vp.Profile.ID, &vp.Profile.Username, &vp.Profile.DisplayName, &vp.Profile.AvatarURL,
			&vp.Profile.Bio, &vp.Profile.Website, &vp.Profile.IsAdmin, &vp.Profile.ForumlineID,
			&authorCreatedAt, &authorUpdatedAt,
		)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}
		vp.JoinedAt = joinedAt.Format(time.RFC3339)
		vp.Profile.CreatedAt = authorCreatedAt.Format(time.RFC3339)
		vp.Profile.UpdatedAt = authorUpdatedAt.Format(time.RFC3339)
		presence = append(presence, vp)
	}
	if presence == nil {
		presence = []voicePresenceJSON{}
	}
	writeJSON(w, http.StatusOK, presence)
}

// ============================================================================
// Bookmarks
// ============================================================================

type bookmarkJSON struct {
	ID        string     `json:"id"`
	CreatedAt string     `json:"created_at"`
	Thread    threadJSON `json:"thread"`
}

// HandleBookmarks handles GET /api/bookmarks
func (h *Handlers) HandleBookmarks(w http.ResponseWriter, r *http.Request) {
	userID := shared.UserIDFromContext(r.Context())

	rows, err := h.Pool.Query(r.Context(),
		`SELECT b.id, b.created_at,
		        t.id, t.category_id, t.author_id, t.title, t.slug, t.content, t.image_url,
		        t.is_pinned, t.is_locked, t.view_count, t.post_count,
		        COALESCE(t.last_post_at, t.created_at), t.created_at, t.updated_at,
		        p.id, p.username, p.display_name, p.avatar_url, p.bio, p.website,
		        p.is_admin, p.forumline_id, p.created_at, p.updated_at,
		        c.id, c.name, c.slug, c.description, c.sort_order, c.created_at
		 FROM bookmarks b
		 JOIN threads t ON t.id = b.thread_id
		 JOIN profiles p ON p.id = t.author_id
		 JOIN categories c ON c.id = t.category_id
		 WHERE b.user_id = $1
		 ORDER BY b.created_at DESC`, userID)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	defer rows.Close()

	var bookmarks []bookmarkJSON
	for rows.Next() {
		var bm bookmarkJSON
		var bmCreatedAt time.Time

		// We need to scan bookmark fields then thread+joins
		var t threadJSON
		var lastPostAt, tCreatedAt, tUpdatedAt time.Time
		var authorCreatedAt, authorUpdatedAt time.Time
		var catCreatedAt time.Time

		err := rows.Scan(
			&bm.ID, &bmCreatedAt,
			&t.ID, &t.CategoryID, &t.AuthorID, &t.Title, &t.Slug, &t.Content, &t.ImageURL,
			&t.IsPinned, &t.IsLocked, &t.ViewCount, &t.PostCount,
			&lastPostAt, &tCreatedAt, &tUpdatedAt,
			&t.Author.ID, &t.Author.Username, &t.Author.DisplayName, &t.Author.AvatarURL,
			&t.Author.Bio, &t.Author.Website, &t.Author.IsAdmin, &t.Author.ForumlineID,
			&authorCreatedAt, &authorUpdatedAt,
			&t.Category.ID, &t.Category.Name, &t.Category.Slug, &t.Category.Description,
			&t.Category.SortOrder, &catCreatedAt,
		)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}

		bm.CreatedAt = bmCreatedAt.Format(time.RFC3339)
		t.CreatedAt = tCreatedAt.Format(time.RFC3339)
		t.UpdatedAt = tUpdatedAt.Format(time.RFC3339)
		if !lastPostAt.IsZero() {
			s := lastPostAt.Format(time.RFC3339)
			t.LastPostAt = &s
		}
		t.Author.CreatedAt = authorCreatedAt.Format(time.RFC3339)
		t.Author.UpdatedAt = authorUpdatedAt.Format(time.RFC3339)
		t.Category.CreatedAt = catCreatedAt.Format(time.RFC3339)
		bm.Thread = t

		bookmarks = append(bookmarks, bm)
	}
	if bookmarks == nil {
		bookmarks = []bookmarkJSON{}
	}
	writeJSON(w, http.StatusOK, bookmarks)
}

// HandleBookmarkStatus handles GET /api/bookmarks/{threadId}/status
func (h *Handlers) HandleBookmarkStatus(w http.ResponseWriter, r *http.Request) {
	userID := shared.UserIDFromContext(r.Context())
	threadID := chi.URLParam(r, "threadId")

	var id *string
	err := h.Pool.QueryRow(r.Context(),
		`SELECT id FROM bookmarks WHERE user_id = $1 AND thread_id = $2`,
		userID, threadID).Scan(&id)

	if err != nil || id == nil {
		writeJSON(w, http.StatusOK, map[string]interface{}{"bookmarked": false})
		return
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{"bookmarked": true, "id": *id})
}

// ============================================================================
// Notifications (data read — extends existing handlers)
// ============================================================================

// HandleNotificationsData handles GET /api/notifications (data provider version)
func (h *Handlers) HandleNotificationsData(w http.ResponseWriter, r *http.Request) {
	userID := shared.UserIDFromContext(r.Context())

	rows, err := h.Pool.Query(r.Context(),
		`SELECT id, user_id, type, title, message, link, read, created_at
		 FROM notifications
		 WHERE user_id = $1
		 ORDER BY created_at DESC
		 LIMIT 20`, userID)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	defer rows.Close()

	type notification struct {
		ID        string  `json:"id"`
		UserID    string  `json:"user_id"`
		Type      string  `json:"type"`
		Title     string  `json:"title"`
		Message   string  `json:"message"`
		Link      *string `json:"link"`
		Read      bool    `json:"read"`
		CreatedAt string  `json:"created_at"`
	}

	var notifications []notification
	for rows.Next() {
		var n notification
		var createdAt time.Time
		if err := rows.Scan(&n.ID, &n.UserID, &n.Type, &n.Title, &n.Message, &n.Link, &n.Read, &createdAt); err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}
		n.CreatedAt = createdAt.Format(time.RFC3339)
		notifications = append(notifications, n)
	}
	if notifications == nil {
		notifications = []notification{}
	}
	writeJSON(w, http.StatusOK, notifications)
}

// ============================================================================
// Admin
// ============================================================================

// HandleAdminStats handles GET /api/admin/stats
func (h *Handlers) HandleAdminStats(w http.ResponseWriter, r *http.Request) {
	userID := shared.UserIDFromContext(r.Context())

	// Verify admin
	var isAdmin bool
	err := h.Pool.QueryRow(r.Context(), `SELECT is_admin FROM profiles WHERE id = $1`, userID).Scan(&isAdmin)
	if err != nil || !isAdmin {
		writeJSON(w, http.StatusForbidden, map[string]string{"error": "admin access required"})
		return
	}

	var totalUsers, totalThreads, totalPosts int
	if err := h.Pool.QueryRow(r.Context(), `SELECT COUNT(*) FROM profiles`).Scan(&totalUsers); err != nil {
		log.Printf("count profiles error: %v", err)
	}
	if err := h.Pool.QueryRow(r.Context(), `SELECT COUNT(*) FROM threads`).Scan(&totalThreads); err != nil {
		log.Printf("count threads error: %v", err)
	}
	if err := h.Pool.QueryRow(r.Context(), `SELECT COUNT(*) FROM posts`).Scan(&totalPosts); err != nil {
		log.Printf("count posts error: %v", err)
	}

	writeJSON(w, http.StatusOK, map[string]int{
		"totalUsers":   totalUsers,
		"totalThreads": totalThreads,
		"totalPosts":   totalPosts,
	})
}

// HandleAdminUsers handles GET /api/admin/users
func (h *Handlers) HandleAdminUsers(w http.ResponseWriter, r *http.Request) {
	userID := shared.UserIDFromContext(r.Context())

	var isAdmin bool
	err := h.Pool.QueryRow(r.Context(), `SELECT is_admin FROM profiles WHERE id = $1`, userID).Scan(&isAdmin)
	if err != nil || !isAdmin {
		writeJSON(w, http.StatusForbidden, map[string]string{"error": "admin access required"})
		return
	}

	rows, err := h.Pool.Query(r.Context(),
		`SELECT `+profileColumns+` FROM profiles ORDER BY created_at DESC LIMIT 50`)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	defer rows.Close()

	var profiles []profileJSON
	for rows.Next() {
		p, err := scanProfile(rows.Scan)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}
		profiles = append(profiles, p)
	}
	if profiles == nil {
		profiles = []profileJSON{}
	}
	writeJSON(w, http.StatusOK, profiles)
}

// ============================================================================
// Helpers
// ============================================================================

func parseInt(s string, defaultVal int) int {
	var n int
	for _, c := range s {
		if c < '0' || c > '9' {
			return defaultVal
		}
		n = n*10 + int(c-'0')
	}
	if n == 0 {
		return defaultVal
	}
	return n
}
