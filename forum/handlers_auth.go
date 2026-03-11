package forum

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"regexp"
	"strings"
	"time"
)

var (
	emailRegex    = regexp.MustCompile(`^[^\s@]+@[^\s@]+\.[^\s@]+$`)
	usernameRegex = regexp.MustCompile(`^[a-zA-Z0-9_-]+$`)
)

// HandleSignup handles POST /api/auth/signup.
func (h *Handlers) HandleSignup(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "Method not allowed"})
		return
	}

	var body struct {
		Email    string `json:"email"`
		Password string `json:"password"`
		Username string `json:"username"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
		return
	}

	body.Email = strings.TrimSpace(body.Email)
	body.Username = strings.TrimSpace(body.Username)

	// Validate email
	if !emailRegex.MatchString(body.Email) {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "Invalid email address"})
		return
	}

	// Validate password
	if len(body.Password) < 6 {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "Password must be at least 6 characters"})
		return
	}

	// Validate username
	if len(body.Username) < 3 || len(body.Username) > 30 {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "Username must be 3-30 characters"})
		return
	}
	if !usernameRegex.MatchString(body.Username) {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "Username can only contain letters, numbers, underscores, and hyphens"})
		return
	}

	ctx := r.Context()

	// Check email uniqueness via GoTrue admin API
	gotrueURL := h.Config.GoTrueURL
	serviceKey := h.Config.GoTrueServiceRoleKey

	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, gotrueURL+"/admin/users", nil)
	req.Header.Set("Authorization", "Bearer "+serviceKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "auth service unavailable"})
		return
	}
	defer func() { _ = resp.Body.Close() }()

	var usersResp struct {
		Users []struct {
			Email string `json:"email"`
		} `json:"users"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&usersResp); err == nil {
		for _, u := range usersResp.Users {
			if strings.EqualFold(u.Email, body.Email) {
				writeJSON(w, http.StatusConflict, map[string]string{"error": "An account with this email already exists"})
				return
			}
		}
	}

	// Check username uniqueness
	var exists bool
	err = h.Pool.QueryRow(ctx,
		"SELECT EXISTS(SELECT 1 FROM profiles WHERE username = $1)", body.Username,
	).Scan(&exists)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "database error"})
		return
	}
	if exists {
		writeJSON(w, http.StatusConflict, map[string]string{"error": "Username already taken"})
		return
	}

	// Create user via GoTrue signup endpoint
	payload, _ := json.Marshal(map[string]interface{}{
		"email":    body.Email,
		"password": body.Password,
		"data": map[string]string{
			"username":     body.Username,
			"display_name": body.Username,
		},
	})

	signupReq, _ := http.NewRequestWithContext(ctx, http.MethodPost, gotrueURL+"/signup", bytes.NewReader(payload))
	signupReq.Header.Set("Content-Type", "application/json")
	signupReq.Header.Set("apikey", serviceKey)
	signupResp, err := http.DefaultClient.Do(signupReq)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "auth service unavailable"})
		return
	}
	defer func() { _ = signupResp.Body.Close() }()

	signupBody, _ := io.ReadAll(signupResp.Body)

	if signupResp.StatusCode != 200 {
		var gotrueErr struct {
			ErrorDescription string `json:"error_description"`
			Msg              string `json:"msg"`
		}
		if err := json.Unmarshal(signupBody, &gotrueErr); err != nil {
			log.Printf("failed to unmarshal GoTrue error: %v", err)
		}
		errMsg := "Signup failed"
		if gotrueErr.ErrorDescription != "" {
			errMsg = gotrueErr.ErrorDescription
		} else if gotrueErr.Msg != "" {
			errMsg = gotrueErr.Msg
		}
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": errMsg})
		return
	}

	var gotrueResp struct {
		AccessToken  string `json:"access_token"`
		RefreshToken string `json:"refresh_token"`
		User         struct {
			ID    string `json:"id"`
			Email string `json:"email"`
		} `json:"user"`
	}
	if err := json.Unmarshal(signupBody, &gotrueResp); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to parse auth response"})
		return
	}

	if gotrueResp.User.ID == "" || gotrueResp.AccessToken == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "Signup failed"})
		return
	}

	// Create profile
	_, err = h.Pool.Exec(ctx,
		`INSERT INTO profiles (id, username, display_name)
		 VALUES ($1, $2, $3)
		 ON CONFLICT (id) DO UPDATE SET username = $2, display_name = $3`,
		gotrueResp.User.ID, body.Username, body.Username)
	if err != nil {
		// Rollback: delete auth user
		deleteGoTrueUser(gotrueURL, serviceKey, gotrueResp.User.ID)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "Failed to create profile"})
		return
	}

	writeJSON(w, http.StatusCreated, map[string]interface{}{
		"user": map[string]string{
			"id":    gotrueResp.User.ID,
			"email": gotrueResp.User.Email,
		},
		"session": map[string]string{
			"access_token":  gotrueResp.AccessToken,
			"refresh_token": gotrueResp.RefreshToken,
		},
	})
}

func deleteGoTrueUser(gotrueURL, serviceKey, userID string) {
	if gotrueURL == "" || serviceKey == "" {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	req, _ := http.NewRequestWithContext(ctx, http.MethodDelete, gotrueURL+"/admin/users/"+userID, nil)
	req.Header.Set("Authorization", "Bearer "+serviceKey)
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		log.Printf("deleteGoTrueUser request failed: %v", err)
	} else {
		defer func() { _ = resp.Body.Close() }()
	}
}

// gotrueAdminCreateUser creates a user via GoTrue admin API.
func gotrueAdminCreateUser(gotrueURL, serviceKey string, payload map[string]interface{}) (userID string, err error) {
	body, _ := json.Marshal(payload)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	req, _ := http.NewRequestWithContext(ctx, http.MethodPost, gotrueURL+"/admin/users", bytes.NewReader(body))
	req.Header.Set("Authorization", "Bearer "+serviceKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("admin create user request failed: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	respBody, _ := io.ReadAll(resp.Body)
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", fmt.Errorf("admin create user failed (%d): %s", resp.StatusCode, string(respBody))
	}

	var result struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(respBody, &result); err != nil {
		return "", fmt.Errorf("failed to parse admin create user response: %w", err)
	}
	return result.ID, nil
}

// gotrueAdminGenerateLink generates a magic link via GoTrue admin API.
func gotrueAdminGenerateLink(gotrueURL, serviceKey, email string) (hashedToken string, err error) {
	payload, _ := json.Marshal(map[string]interface{}{
		"type":  "magiclink",
		"email": email,
	})
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	req, _ := http.NewRequestWithContext(ctx, http.MethodPost, gotrueURL+"/admin/generate_link", bytes.NewReader(payload))
	req.Header.Set("Authorization", "Bearer "+serviceKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("generate link request failed: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	respBody, _ := io.ReadAll(resp.Body)
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", fmt.Errorf("generate link failed (%d): %s", resp.StatusCode, string(respBody))
	}

	var result struct {
		HashedToken string `json:"hashed_token"`
	}
	if err := json.Unmarshal(respBody, &result); err != nil {
		return "", fmt.Errorf("failed to parse generate link response: %w", err)
	}
	return result.HashedToken, nil
}

// gotrueVerifyOTP verifies an OTP hash and returns session tokens.
func gotrueVerifyOTP(gotrueURL, serviceKey, tokenHash string) (accessToken, refreshToken string, err error) {
	payload, _ := json.Marshal(map[string]string{
		"token_hash": tokenHash,
		"type":       "magiclink",
	})
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	req, _ := http.NewRequestWithContext(ctx, http.MethodPost, gotrueURL+"/verify", bytes.NewReader(payload))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("apikey", serviceKey)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", "", fmt.Errorf("verify OTP request failed: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	respBody, _ := io.ReadAll(resp.Body)
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", "", fmt.Errorf("verify OTP failed (%d): %s", resp.StatusCode, string(respBody))
	}

	var result struct {
		AccessToken  string `json:"access_token"`
		RefreshToken string `json:"refresh_token"`
	}
	if err := json.Unmarshal(respBody, &result); err != nil {
		return "", "", fmt.Errorf("failed to parse verify OTP response: %w", err)
	}
	if result.AccessToken == "" {
		return "", "", fmt.Errorf("no session returned from verify OTP")
	}
	return result.AccessToken, result.RefreshToken, nil
}

// gotrueAdminGetUser retrieves a user by ID via GoTrue admin API.
func gotrueAdminGetUser(gotrueURL, serviceKey, userID string) (email string, err error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, gotrueURL+"/admin/users/"+userID, nil)
	req.Header.Set("Authorization", "Bearer "+serviceKey)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("get user request failed: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	var result struct {
		Email string `json:"email"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", err
	}
	return result.Email, nil
}

// gotrueListedUser represents a user returned from GoTrue admin list.
type gotrueListedUser struct {
	ID           string                 `json:"id"`
	Email        string                 `json:"email"`
	UserMetadata map[string]interface{} `json:"user_metadata"`
}

// gotrueAdminListUsers retrieves all users from GoTrue admin API.
func gotrueAdminListUsers(gotrueURL, serviceKey string) ([]gotrueListedUser, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	adminReq, _ := http.NewRequestWithContext(ctx, http.MethodGet, gotrueURL+"/admin/users", nil)
	adminReq.Header.Set("Authorization", "Bearer "+serviceKey)
	adminReq.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(adminReq)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()

	var result struct {
		Users []gotrueListedUser `json:"users"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}
	return result.Users, nil
}

// gotrueGetUserByToken retrieves a user's email by their access token.
func gotrueGetUserByToken(gotrueURL, token string) (email string, err error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, gotrueURL+"/user", nil)
	req.Header.Set("Authorization", "Bearer "+token)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", err
	}
	defer func() { _ = resp.Body.Close() }()

	var result struct {
		Email string `json:"email"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		log.Printf("failed to decode GoTrue user response: %v", err)
	}
	return result.Email, nil
}

// gotrueAdminSignOut revokes a user's session.
func gotrueAdminSignOut(gotrueURL, serviceKey, token string) {
	if gotrueURL == "" || serviceKey == "" || token == "" {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	reqBody, _ := json.Marshal(map[string]string{})
	adminReq, _ := http.NewRequestWithContext(ctx, http.MethodPost, gotrueURL+"/logout", bytes.NewReader(reqBody))
	adminReq.Header.Set("Authorization", "Bearer "+token)
	adminReq.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(adminReq)
	if err != nil {
		log.Printf("gotrueAdminSignOut request failed: %v", err)
	} else {
		defer func() { _ = resp.Body.Close() }()
	}
}

// afterAuth generates a Supabase session for a user and returns a redirect URL with tokens.
func (h *Handlers) afterAuth(userID string) string {
	email, err := gotrueAdminGetUser(h.Config.GoTrueURL, h.Config.GoTrueServiceRoleKey, userID)
	if err != nil || email == "" {
		return ""
	}

	hashedToken, err := gotrueAdminGenerateLink(h.Config.GoTrueURL, h.Config.GoTrueServiceRoleKey, email)
	if err != nil || hashedToken == "" {
		return ""
	}

	accessToken, refreshToken, err := gotrueVerifyOTP(h.Config.GoTrueURL, h.Config.GoTrueServiceRoleKey, hashedToken)
	if err != nil {
		return ""
	}

	return fmt.Sprintf("%s/#access_token=%s&refresh_token=%s&type=bearer",
		h.Config.SiteURL, accessToken, refreshToken)
}
