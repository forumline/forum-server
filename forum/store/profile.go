package store

import (
	"context"
	"time"

	"github.com/forumline/forum-server/forum/model"
)

// GetProfile returns a profile by ID.
func (s *Store) GetProfile(ctx context.Context, id string) (*model.Profile, error) {
	row := s.DB.QueryRow(ctx,
		`SELECT `+profileColumns+` FROM profiles WHERE id = $1`, id)
	p, err := scanProfile(row.Scan)
	if err != nil {
		return nil, err
	}
	return &p, nil
}

// GetProfileByUsername returns a profile by username.
func (s *Store) GetProfileByUsername(ctx context.Context, username string) (*model.Profile, error) {
	row := s.DB.QueryRow(ctx,
		`SELECT `+profileColumns+` FROM profiles WHERE username = $1`, username)
	p, err := scanProfile(row.Scan)
	if err != nil {
		return nil, err
	}
	return &p, nil
}

// GetProfilesByIDs returns profiles for the given IDs.
func (s *Store) GetProfilesByIDs(ctx context.Context, ids []string) ([]model.Profile, error) {
	rows, err := s.DB.Query(ctx,
		`SELECT `+profileColumns+` FROM profiles WHERE id = ANY($1)`, ids)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var profiles []model.Profile
	for rows.Next() {
		p, err := scanProfile(rows.Scan)
		if err != nil {
			return nil, err
		}
		profiles = append(profiles, p)
	}
	if profiles == nil {
		profiles = []model.Profile{}
	}
	return profiles, nil
}

// GetProfileByForumlineID returns a profile by forumline_id.
func (s *Store) GetProfileByForumlineID(ctx context.Context, forumlineID string) (*model.Profile, error) {
	var id string
	err := s.DB.QueryRow(ctx,
		"SELECT id FROM profiles WHERE forumline_id = $1", forumlineID).Scan(&id)
	if err != nil {
		return nil, err
	}
	return s.GetProfile(ctx, id)
}

// GetProfileIDByForumlineID returns just the profile ID for a forumline_id.
func (s *Store) GetProfileIDByForumlineID(ctx context.Context, forumlineID string) (string, error) {
	var id string
	err := s.DB.QueryRow(ctx,
		"SELECT id FROM profiles WHERE forumline_id = $1", forumlineID).Scan(&id)
	return id, err
}

// GetProfileIDByForumlineIDUnlinked returns the profile ID if forumline_id is null/empty.
func (s *Store) GetProfileIDByForumlineIDUnlinked(ctx context.Context, userID string) (string, error) {
	var id string
	err := s.DB.QueryRow(ctx,
		"SELECT id FROM profiles WHERE id = $1 AND (forumline_id IS NULL OR forumline_id = '')", userID).Scan(&id)
	return id, err
}

// UsernameExists checks if a username is taken.
func (s *Store) UsernameExists(ctx context.Context, username string) (bool, error) {
	var exists bool
	err := s.DB.QueryRow(ctx,
		"SELECT EXISTS(SELECT 1 FROM profiles WHERE username = $1)", username).Scan(&exists)
	return exists, err
}

// UpsertProfileFull creates or updates a profile with username.
func (s *Store) UpsertProfileFull(ctx context.Context, id, username string, displayName, avatarURL *string) error {
	now := time.Now()
	_, err := s.DB.Exec(ctx,
		`INSERT INTO profiles (id, username, display_name, avatar_url, created_at, updated_at)
		 VALUES ($1, $2, $3, $4, $5, $5)
		 ON CONFLICT (id) DO UPDATE SET
		   username = EXCLUDED.username,
		   display_name = EXCLUDED.display_name,
		   avatar_url = EXCLUDED.avatar_url,
		   updated_at = EXCLUDED.updated_at`,
		id, username, displayName, avatarURL, now)
	return err
}

// UpdateProfilePartial performs a dynamic partial update on a profile.
func (s *Store) UpdateProfilePartial(ctx context.Context, userID string, setClauses []string, args []interface{}) error {
	query := "UPDATE profiles SET "
	for i, clause := range setClauses {
		if i > 0 {
			query += ", "
		}
		query += clause
	}
	query += " WHERE id = $1"
	_, err := s.DB.Exec(ctx, query, args...)
	return err
}

// ClearForumlineID removes the forumline_id from a profile.
func (s *Store) ClearForumlineID(ctx context.Context, userID string) error {
	_, err := s.DB.Exec(ctx,
		`UPDATE profiles SET forumline_id = NULL, updated_at = NOW() WHERE id = $1`, userID)
	return err
}

// SetForumlineID sets the forumline_id on a profile.
func (s *Store) SetForumlineID(ctx context.Context, userID, forumlineID string) error {
	_, err := s.DB.Exec(ctx,
		"UPDATE profiles SET forumline_id = $1 WHERE id = $2",
		forumlineID, userID)
	return err
}

// EnsureProfileWithForumlineID creates or updates a profile with the forumline_id set.
func (s *Store) EnsureProfileWithForumlineID(ctx context.Context, userID string, identity *model.ForumlineIdentity) error {
	_, err := s.DB.Exec(ctx,
		`INSERT INTO profiles (id, username, display_name, forumline_id)
		 VALUES ($1, $2, $3, $4)
		 ON CONFLICT (id) DO UPDATE SET forumline_id = $4, display_name = $3`,
		userID, identity.Username, identity.DisplayName, identity.ForumlineID)
	return err
}

// GetForumlineID returns the forumline_id for a user.
func (s *Store) GetForumlineID(ctx context.Context, userID string) (*string, error) {
	var forumlineID *string
	err := s.DB.QueryRow(ctx,
		"SELECT forumline_id FROM profiles WHERE id = $1", userID).Scan(&forumlineID)
	return forumlineID, err
}

// IsAdmin returns whether a user is an admin.
func (s *Store) IsAdmin(ctx context.Context, userID string) (bool, error) {
	var isAdmin bool
	err := s.DB.QueryRow(ctx, `SELECT is_admin FROM profiles WHERE id = $1`, userID).Scan(&isAdmin)
	return isAdmin, err
}

// ListProfiles returns profiles ordered by created_at desc.
func (s *Store) ListProfiles(ctx context.Context, limit int) ([]model.Profile, error) {
	rows, err := s.DB.Query(ctx,
		`SELECT `+profileColumns+` FROM profiles ORDER BY created_at DESC LIMIT $1`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var profiles []model.Profile
	for rows.Next() {
		p, err := scanProfile(rows.Scan)
		if err != nil {
			return nil, err
		}
		profiles = append(profiles, p)
	}
	if profiles == nil {
		profiles = []model.Profile{}
	}
	return profiles, nil
}

// GetUserIDByUsername returns a user ID for a username (case-insensitive).
func (s *Store) GetUserIDByUsername(ctx context.Context, username string) (string, error) {
	var id string
	err := s.DB.QueryRow(ctx,
		`SELECT id FROM profiles WHERE lower(username) = $1`, username).Scan(&id)
	return id, err
}

// GetUsername returns the username for a user ID.
func (s *Store) GetUsername(ctx context.Context, userID string) (string, error) {
	var username string
	err := s.DB.QueryRow(ctx,
		`SELECT username FROM profiles WHERE id = $1`, userID).Scan(&username)
	return username, err
}

// UpdateDisplayName updates a profile's display_name.
func (s *Store) UpdateDisplayName(ctx context.Context, userID, displayName string) error {
	_, err := s.DB.Exec(ctx,
		"UPDATE profiles SET display_name = $1 WHERE id = $2",
		displayName, userID)
	return err
}

// UpdateDisplayNameAndAvatar updates display_name and avatar_url.
func (s *Store) UpdateDisplayNameAndAvatar(ctx context.Context, userID, displayName, avatarURL string) error {
	_, err := s.DB.Exec(ctx,
		"UPDATE profiles SET display_name = $1, avatar_url = $2, updated_at = now() WHERE id = $3",
		displayName, avatarURL, userID)
	return err
}

// CreateProfileSimple creates a basic profile with username.
func (s *Store) CreateProfileSimple(ctx context.Context, id, username, displayName string) error {
	_, err := s.DB.Exec(ctx,
		`INSERT INTO profiles (id, username, display_name)
		 VALUES ($1, $2, $3)
		 ON CONFLICT (id) DO UPDATE SET username = $2, display_name = $3`,
		id, username, displayName)
	return err
}

// CreateProfileHosted creates a profile for hosted mode (using forumline_id as profile ID).
func (s *Store) CreateProfileHosted(ctx context.Context, identity *model.ForumlineIdentity) error {
	_, err := s.DB.Exec(ctx,
		`INSERT INTO profiles (id, username, display_name, avatar_url, forumline_id)
		 VALUES ($1, $2, $3, $4, $5)
		 ON CONFLICT (id) DO UPDATE SET forumline_id = $5, display_name = $3`,
		identity.ForumlineID, identity.Username, identity.DisplayName, identity.AvatarURL, identity.ForumlineID)
	return err
}
