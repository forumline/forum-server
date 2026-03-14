package service

import (
	"context"

	"github.com/forumline/forum-server/forum/model"
	"github.com/forumline/forum-server/forum/store"
)

// AdminService handles admin business logic.
type AdminService struct {
	Store *store.Store
}

// NewAdminService creates a new AdminService.
func NewAdminService(s *store.Store) *AdminService {
	return &AdminService{Store: s}
}

// GetStats returns admin dashboard statistics after verifying admin access.
func (as *AdminService) GetStats(ctx context.Context, userID string) (*model.AdminStats, error) {
	isAdmin, err := as.Store.IsAdmin(ctx, userID)
	if err != nil || !isAdmin {
		return nil, &ForbiddenError{Msg: "admin access required"}
	}

	stats, err := as.Store.GetAdminStats(ctx)
	if err != nil {
		return nil, err
	}
	return &stats, nil
}

// ListUsers returns all users after verifying admin access.
func (as *AdminService) ListUsers(ctx context.Context, userID string) ([]model.Profile, error) {
	isAdmin, err := as.Store.IsAdmin(ctx, userID)
	if err != nil || !isAdmin {
		return nil, &ForbiddenError{Msg: "admin access required"}
	}

	return as.Store.ListProfiles(ctx, 50)
}

// VerifyAdmin returns an error if the user is not an admin.
func (as *AdminService) VerifyAdmin(ctx context.Context, userID string) error {
	isAdmin, err := as.Store.IsAdmin(ctx, userID)
	if err != nil || !isAdmin {
		return &ForbiddenError{Msg: "admin access required"}
	}
	return nil
}
