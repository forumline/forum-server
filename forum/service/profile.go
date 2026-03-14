package service

import (
	"context"
	"time"

	"github.com/forumline/forum-server/forum/model"
	"github.com/forumline/forum-server/forum/store"
)

// ProfileService handles profile business logic.
type ProfileService struct {
	Store *store.Store
}

// NewProfileService creates a new ProfileService.
func NewProfileService(s *store.Store) *ProfileService {
	return &ProfileService{Store: s}
}

// Get returns a profile by ID.
func (ps *ProfileService) Get(ctx context.Context, id string) (*model.Profile, error) {
	p, err := ps.Store.GetProfile(ctx, id)
	if err != nil {
		return nil, &NotFoundError{Msg: "profile not found"}
	}
	return p, nil
}

// GetByUsername returns a profile by username.
func (ps *ProfileService) GetByUsername(ctx context.Context, username string) (*model.Profile, error) {
	p, err := ps.Store.GetProfileByUsername(ctx, username)
	if err != nil {
		return nil, &NotFoundError{Msg: "profile not found"}
	}
	return p, nil
}

// GetBatch returns profiles for the given IDs.
func (ps *ProfileService) GetBatch(ctx context.Context, ids []string) ([]model.Profile, error) {
	if len(ids) == 0 {
		return []model.Profile{}, nil
	}
	return ps.Store.GetProfilesByIDs(ctx, ids)
}

// UpdateProfileInput holds optional fields for updating a profile.
type UpdateProfileInput struct {
	Username    *string
	DisplayName *string
	AvatarURL   *string
	Bio         *string
	Website     *string
}

// Upsert creates or updates a profile.
func (ps *ProfileService) Upsert(ctx context.Context, userID, profileID string, input UpdateProfileInput) error {
	if userID != profileID {
		return &ForbiddenError{Msg: "can only update own profile"}
	}

	if input.Username != nil {
		return ps.Store.UpsertProfileFull(ctx, userID, *input.Username, input.DisplayName, input.AvatarURL)
	}

	// Partial update
	now := time.Now()
	setClauses := []string{"updated_at = $2"}
	args := []interface{}{userID, now}
	argIdx := 3

	if input.DisplayName != nil {
		setClauses = append(setClauses, store.SetClause("display_name", argIdx))
		args = append(args, *input.DisplayName)
		argIdx++
	}
	if input.Bio != nil {
		setClauses = append(setClauses, store.SetClause("bio", argIdx))
		args = append(args, *input.Bio)
		argIdx++
	}
	if input.Website != nil {
		setClauses = append(setClauses, store.SetClause("website", argIdx))
		args = append(args, *input.Website)
		argIdx++
	}
	if input.AvatarURL != nil {
		setClauses = append(setClauses, store.SetClause("avatar_url", argIdx))
		args = append(args, *input.AvatarURL)
	}

	return ps.Store.UpdateProfilePartial(ctx, userID, setClauses, args)
}

// ClearForumlineID removes the forumline_id from a profile.
func (ps *ProfileService) ClearForumlineID(ctx context.Context, userID, profileID string) error {
	if userID != profileID {
		return &ForbiddenError{Msg: "can only update own profile"}
	}
	return ps.Store.ClearForumlineID(ctx, userID)
}
