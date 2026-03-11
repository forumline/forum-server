package forum

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
	shared "github.com/forumline/forumline/shared-go"
)

// forumlineIdentity represents a Forumline identity.
type forumlineIdentity struct {
	ForumlineID string `json:"forumline_id"`
	Username    string `json:"username"`
	DisplayName string `json:"display_name"`
	AvatarURL   string `json:"avatar_url"`
	Bio         string `json:"bio,omitempty"`
}

// HandleForumlineAuth handles GET/POST /api/forumline/auth.
// Supports three flows:
// 1. link_token query param — "Connect from Settings" flow
// 2. forumline_token query param — server-side OAuth for iframe usage
// 3. No params — redirect to Forumline authorize page
func (h *Handlers) HandleForumlineAuth(w http.ResponseWriter, r *http.Request) {
	linkToken := r.URL.Query().Get("link_token")
	if linkToken != "" {
		h.handleLinkRedirect(w, r, linkToken)
		return
	}

	forumlineToken := r.URL.Query().Get("forumline_token")
	if forumlineToken != "" {
		h.handleServerSideAuth(w, r, forumlineToken)
		return
	}

	// Default: redirect to Forumline authorize page
	h.redirectToForumlineAuth(w, r, "")
}

// handleLinkRedirect handles "Connect from Settings" — verify session, set link cookie, redirect to Forumline.
func (h *Handlers) handleLinkRedirect(w http.ResponseWriter, r *http.Request, linkToken string) {
	// In hosted mode (no GoTrue), validate JWT directly
	if h.isHostedMode() {
		claims, err := shared.ValidateJWT(linkToken)
		if err != nil || claims.Subject == "" {
			http.Redirect(w, r, h.Config.SiteURL+"/settings?error=invalid_session", http.StatusFound)
			return
		}
		h.setLinkCookiesAndRedirect(w, r, claims.Subject)
		return
	}

	// Self-hosted mode: validate via GoTrue
	email, err := gotrueGetUserByToken(h.Config.GoTrueURL, linkToken)
	if err != nil || email == "" {
		// Try JWT validation as fallback
		claims, err := shared.ValidateJWT(linkToken)
		if err != nil || claims.Subject == "" {
			http.Redirect(w, r, h.Config.SiteURL+"/settings?error=invalid_session", http.StatusFound)
			return
		}
		h.setLinkCookiesAndRedirect(w, r, claims.Subject)
		return
	}

	// Get user ID from GoTrue
	req, _ := http.NewRequestWithContext(r.Context(), http.MethodGet, h.Config.GoTrueURL+"/user", nil)
	req.Header.Set("Authorization", "Bearer "+linkToken)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		http.Redirect(w, r, h.Config.SiteURL+"/settings?error=invalid_session", http.StatusFound)
		return
	}
	defer func() { _ = resp.Body.Close() }()

	var user struct {
		ID string `json:"id"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&user); err != nil {
		log.Printf("failed to decode GoTrue user response: %v", err)
	}
	if user.ID == "" {
		http.Redirect(w, r, h.Config.SiteURL+"/settings?error=invalid_session", http.StatusFound)
		return
	}

	h.setLinkCookiesAndRedirect(w, r, user.ID)
}

func (h *Handlers) setLinkCookiesAndRedirect(w http.ResponseWriter, r *http.Request, userID string) {
	state := randomHex(16)

	authURL, _ := url.Parse(h.Config.ForumlineURL + "/api/oauth/authorize")
	q := authURL.Query()
	q.Set("client_id", h.Config.ForumlineClientID)
	q.Set("redirect_uri", h.Config.SiteURL+"/api/forumline/auth/callback")
	q.Set("state", state)
	authURL.RawQuery = q.Encode()

	http.SetCookie(w, &http.Cookie{
		Name: "forumline_state", Value: state,
		Path: "/", HttpOnly: true, SameSite: http.SameSiteNoneMode, Secure: true, MaxAge: 600,
	})
	http.SetCookie(w, &http.Cookie{
		Name: "forumline_link_uid", Value: userID,
		Path: "/", HttpOnly: true, SameSite: http.SameSiteNoneMode, Secure: true, MaxAge: 600,
	})

	http.Redirect(w, r, authURL.String(), http.StatusFound)
}

// handleServerSideAuth does the entire OAuth exchange server-side (for iframe usage).
func (h *Handlers) handleServerSideAuth(w http.ResponseWriter, r *http.Request, forumlineToken string) {
	log.Println("[Forumline:Auth] Starting server-side auth with forumline_token")
	state := randomHex(16)
	redirectURI := h.Config.SiteURL + "/api/forumline/auth/callback"

	// Step 1: Call Forumline authorize endpoint server-side to get auth code
	authorizeURL, _ := url.Parse(h.Config.ForumlineURL + "/api/oauth/authorize")
	q := authorizeURL.Query()
	q.Set("client_id", h.Config.ForumlineClientID)
	q.Set("redirect_uri", redirectURI)
	q.Set("state", state)
	authorizeURL.RawQuery = q.Encode()

	payload, _ := json.Marshal(map[string]string{"access_token": forumlineToken})
	req, _ := http.NewRequestWithContext(r.Context(), http.MethodPost, authorizeURL.String(), strings.NewReader(string(payload)))
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{CheckRedirect: func(req *http.Request, via []*http.Request) error {
		return http.ErrUseLastResponse
	}}
	resp, err := client.Do(req)
	if err != nil {
		log.Printf("[Forumline:Auth] Forumline authorize request failed: %v", err)
		http.Redirect(w, r, h.Config.SiteURL+"/login?error=auth_failed", http.StatusFound)
		return
	}
	defer func() { _ = resp.Body.Close() }()

	location := resp.Header.Get("Location")
	if location == "" {
		log.Printf("[Forumline:Auth] No redirect from Forumline authorize. Status: %d", resp.StatusCode)
		http.Redirect(w, r, h.Config.SiteURL+"/login?error=auth_failed", http.StatusFound)
		return
	}

	callbackURL, err := url.Parse(location)
	if err != nil || callbackURL.Query().Get("code") == "" {
		log.Printf("[Forumline:Auth] No code in Forumline redirect: %s", location)
		http.Redirect(w, r, h.Config.SiteURL+"/login?error=auth_failed", http.StatusFound)
		return
	}
	code := callbackURL.Query().Get("code")

	// Step 2: Exchange code for identity token
	identity, identityToken, forumlineAccessToken, err := h.exchangeCodeForTokens(code, redirectURI)
	if err != nil {
		log.Printf("[Forumline:Auth] Token exchange failed: %v", err)
		http.Redirect(w, r, h.Config.SiteURL+"/login?error=auth_failed", http.StatusFound)
		return
	}

	// Step 3: Create or link local user
	var localUserID string
	if h.isHostedMode() {
		localUserID, err = h.createOrLinkUserHosted(r, identity)
	} else {
		localUserID, err = h.createOrLinkUser(r, identity, forumlineAccessToken)
	}
	if err != nil {
		log.Printf("[Forumline:Auth] createOrLinkUser failed: %v", err)
		if strings.Contains(err.Error(), "EMAIL_COLLISION") {
			http.Redirect(w, r, h.Config.SiteURL+"/login?error=email_exists", http.StatusFound)
		} else {
			http.Redirect(w, r, h.Config.SiteURL+"/login?error=auth_failed", http.StatusFound)
		}
		return
	}

	// Step 4: Set cookies
	h.setForumlineCookies(w, identityToken, localUserID, forumlineAccessToken)

	// Step 5: Generate session
	if h.isHostedMode() {
		accessToken, err := h.signHostedSession(localUserID)
		if err != nil {
			log.Printf("[Forumline:Auth] signHostedSession failed: %v", err)
			http.Redirect(w, r, h.Config.SiteURL+"/login?error=auth_failed", http.StatusFound)
			return
		}
		http.Redirect(w, r, fmt.Sprintf("%s/#access_token=%s&type=bearer", h.Config.SiteURL, accessToken), http.StatusFound)
		return
	}

	redirectURL := h.afterAuth(localUserID)
	if redirectURL != "" {
		http.Redirect(w, r, redirectURL, http.StatusFound)
		return
	}

	http.Redirect(w, r, h.Config.SiteURL+"/?forumline_auth=success", http.StatusFound)
}

// redirectToForumlineAuth redirects browser to the forumline OAuth authorize page.
func (h *Handlers) redirectToForumlineAuth(w http.ResponseWriter, r *http.Request, linkUID string) {
	state := randomHex(16)

	authURL, _ := url.Parse(h.Config.ForumlineURL + "/api/oauth/authorize")
	q := authURL.Query()
	q.Set("client_id", h.Config.ForumlineClientID)
	q.Set("redirect_uri", h.Config.SiteURL+"/api/forumline/auth/callback")
	q.Set("state", state)
	authURL.RawQuery = q.Encode()

	http.SetCookie(w, &http.Cookie{
		Name: "forumline_state", Value: state,
		Path: "/", HttpOnly: true, SameSite: http.SameSiteNoneMode, Secure: true, MaxAge: 600,
	})

	http.Redirect(w, r, authURL.String(), http.StatusFound)
}

// HandleForumlineCallback handles GET /api/forumline/auth/callback.
func (h *Handlers) HandleForumlineCallback(w http.ResponseWriter, r *http.Request) {
	cookies := parseCookies(r)
	linkUID := cookies["forumline_link_uid"]

	if linkUID != "" {
		h.handleLinkCallback(w, r, linkUID)
		return
	}

	h.handleNormalCallback(w, r)
}

// handleLinkCallback handles account linking flow (user clicked "Connect to Forumline" from Settings).
func (h *Handlers) handleLinkCallback(w http.ResponseWriter, r *http.Request, linkUID string) {
	cookies := parseCookies(r)
	code := r.URL.Query().Get("code")
	state := r.URL.Query().Get("state")

	if code == "" || state == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "Missing code or state parameter"})
		return
	}
	if cookies["forumline_state"] != state {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "State mismatch — possible CSRF attack"})
		return
	}

	identity, identityToken, forumlineAccessToken, err := h.exchangeCodeForTokens(code, h.Config.SiteURL+"/api/forumline/auth/callback")
	if err != nil {
		log.Printf("[Forumline:Link] Token exchange failed: %v", err)
		http.Redirect(w, r, h.Config.SiteURL+"/settings?error=link_failed", http.StatusFound)
		return
	}

	// Check that forumline_id isn't already linked to a different local account
	var existingID *string
	err = h.Pool.QueryRow(r.Context(),
		"SELECT id FROM profiles WHERE forumline_id = $1", identity.ForumlineID).Scan(&existingID)
	if err == nil && existingID != nil && *existingID != linkUID {
		log.Println("[Forumline:Link] forumline_id already linked to another account")
		http.Redirect(w, r, h.Config.SiteURL+"/settings?error=already_linked", http.StatusFound)
		return
	}

	// Link: update the user's profile with the forumline_id
	_, err = h.Pool.Exec(r.Context(),
		"UPDATE profiles SET forumline_id = $1 WHERE id = $2",
		identity.ForumlineID, linkUID)
	if err != nil {
		log.Printf("[Forumline:Link] Profile update failed: %v", err)
		http.Redirect(w, r, h.Config.SiteURL+"/settings?error=link_failed", http.StatusFound)
		return
	}

	// Set cookies and clear link cookie
	clearCookie(w, "forumline_state")
	clearCookie(w, "forumline_link_uid")
	h.setForumlineCookies(w, identityToken, linkUID, forumlineAccessToken)

	http.Redirect(w, r, h.Config.SiteURL+"/settings?forumline_linked=true", http.StatusFound)
}

// handleNormalCallback handles the normal OAuth callback (sign-in flow).
func (h *Handlers) handleNormalCallback(w http.ResponseWriter, r *http.Request) {
	cookies := parseCookies(r)
	code := r.URL.Query().Get("code")
	state := r.URL.Query().Get("state")

	if code == "" || state == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "Missing code or state parameter"})
		return
	}
	if cookies["forumline_state"] != state {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "State mismatch — possible CSRF attack"})
		return
	}

	identity, identityToken, forumlineAccessToken, err := h.exchangeCodeForTokens(code, h.Config.SiteURL+"/api/forumline/auth/callback")
	if err != nil {
		log.Printf("[Forumline:Callback] Token exchange failed: %v", err)
		http.Redirect(w, r, h.Config.SiteURL+"/login?error=auth_failed", http.StatusFound)
		return
	}

	var localUserID string
	if h.isHostedMode() {
		localUserID, err = h.createOrLinkUserHosted(r, identity)
	} else {
		localUserID, err = h.createOrLinkUser(r, identity, forumlineAccessToken)
	}
	if err != nil {
		log.Printf("[Forumline:Callback] createOrLinkUser failed: %v", err)
		if strings.Contains(err.Error(), "EMAIL_COLLISION") {
			http.Redirect(w, r, h.Config.SiteURL+"/login?error=email_exists", http.StatusFound)
		} else {
			http.Redirect(w, r, h.Config.SiteURL+"/login?error=auth_failed", http.StatusFound)
		}
		return
	}

	// Set cookies
	clearCookie(w, "forumline_state")
	h.setForumlineCookies(w, identityToken, localUserID, forumlineAccessToken)

	if h.isHostedMode() {
		// In hosted mode, sign our own session JWT and redirect with it
		accessToken, err := h.signHostedSession(localUserID)
		if err != nil {
			log.Printf("[Forumline:Callback] signHostedSession failed: %v", err)
			http.Redirect(w, r, h.Config.SiteURL+"/login?error=auth_failed", http.StatusFound)
			return
		}
		http.Redirect(w, r, fmt.Sprintf("%s/#access_token=%s&type=bearer", h.Config.SiteURL, accessToken), http.StatusFound)
		return
	}

	// Single-tenant: call afterAuth for GoTrue session
	redirectURL := h.afterAuth(localUserID)
	if redirectURL != "" {
		http.Redirect(w, r, redirectURL, http.StatusFound)
		return
	}

	http.Redirect(w, r, h.Config.SiteURL+"/?forumline_auth=success", http.StatusFound)
}

// HandleForumlineToken handles GET /api/forumline/auth/forumline-token.
func (h *Handlers) HandleForumlineToken(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "Method not allowed"})
		return
	}

	cookies := parseCookies(r)
	localUserID := cookies["forumline_user_id"]

	if localUserID != "" {
		// Verify user still has forumline_id
		var forumlineID *string
		if err := h.Pool.QueryRow(r.Context(),
			"SELECT forumline_id FROM profiles WHERE id = $1", localUserID).Scan(&forumlineID); err != nil {
			log.Printf("query forumline_id error: %v", err)
		}

		if forumlineID == nil || *forumlineID == "" {
			writeJSON(w, http.StatusOK, map[string]interface{}{"forumline_access_token": nil})
			return
		}
	}

	forumlineAccessToken := cookies["forumline_access_token"]
	writeJSON(w, http.StatusOK, map[string]interface{}{"forumline_access_token": nilIfEmpty(forumlineAccessToken)})
}

// HandleForumlineSession handles GET/DELETE /api/forumline/auth/session.
func (h *Handlers) HandleForumlineSession(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodDelete {
		h.handleDisconnect(w, r)
		return
	}

	if r.Method != http.MethodGet {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "Method not allowed"})
		return
	}

	h.handleSessionGet(w, r)
}

// handleDisconnect revokes Forumline session and clears cookies.
func (h *Handlers) handleDisconnect(w http.ResponseWriter, r *http.Request) {
	cookies := parseCookies(r)
	forumlineAccessToken := cookies["forumline_access_token"]

	if forumlineAccessToken != "" && h.Config.ForumlineGoTrueURL != "" {
		// Revoke Forumline session via GoTrue
		gotrueAdminSignOut(h.Config.ForumlineGoTrueURL, h.Config.ForumlineServiceRoleKey, forumlineAccessToken)
	}

	// Clear all Forumline cookies
	for _, name := range []string{"forumline_identity", "forumline_user_id", "forumline_access_token"} {
		http.SetCookie(w, &http.Cookie{
			Name: name, Value: "",
			Path: "/", HttpOnly: true, SameSite: http.SameSiteLaxMode, Secure: true, MaxAge: -1,
		})
	}
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

// handleSessionGet validates the forumline session and returns identity.
func (h *Handlers) handleSessionGet(w http.ResponseWriter, r *http.Request) {
	cookies := parseCookies(r)
	identityToken := cookies["forumline_identity"]
	localUserID := cookies["forumline_user_id"]

	if identityToken == "" || localUserID == "" {
		writeJSON(w, http.StatusOK, nil)
		return
	}

	// Verify JWT signature
	var payload map[string]interface{}
	if h.Config.ForumlineJWTSecret != "" {
		token, err := jwt.Parse(identityToken, func(t *jwt.Token) (interface{}, error) {
			if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
				return nil, fmt.Errorf("unexpected signing method")
			}
			return []byte(h.Config.ForumlineJWTSecret), nil
		})
		if err != nil || !token.Valid {
			h.clearForumlineCookies(w)
			writeJSON(w, http.StatusOK, nil)
			return
		}
		if claims, ok := token.Claims.(jwt.MapClaims); ok {
			payload = claims
		}
	} else {
		// No JWT secret configured — reject all tokens.
		// Unverified JWT parsing is unsafe; require FORUMLINE_JWT_SECRET to be set.
		log.Printf("[Forumline:Session] FORUMLINE_JWT_SECRET not configured, rejecting identity token")
		h.clearForumlineCookies(w)
		writeJSON(w, http.StatusOK, nil)
		return
	}

	if payload == nil || payload["identity"] == nil {
		h.clearForumlineCookies(w)
		writeJSON(w, http.StatusOK, nil)
		return
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"identity":      payload["identity"],
		"local_user_id": localUserID,
	})
}

// exchangeCodeForTokens exchanges an OAuth auth code for identity + tokens.
func (h *Handlers) exchangeCodeForTokens(code, redirectURI string) (*forumlineIdentity, string, string, error) {
	payload, _ := json.Marshal(map[string]string{
		"code":          code,
		"client_id":     h.Config.ForumlineClientID,
		"client_secret": h.Config.ForumlineClientSecret,
		"redirect_uri":  redirectURI,
	})

	tokenReq, err := http.NewRequestWithContext(context.Background(), http.MethodPost, h.Config.ForumlineURL+"/api/oauth/token", strings.NewReader(string(payload)))
	if err != nil {
		return nil, "", "", fmt.Errorf("create token request: %w", err)
	}
	tokenReq.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(tokenReq)
	if err != nil {
		return nil, "", "", fmt.Errorf("token request failed: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != 200 {
		return nil, "", "", fmt.Errorf("token exchange failed with status %d", resp.StatusCode)
	}

	var tokenData struct {
		Identity       *forumlineIdentity `json:"identity"`
		IdentityToken  string             `json:"identity_token"`
		HubAccessToken string             `json:"forumline_access_token"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&tokenData); err != nil {
		return nil, "", "", fmt.Errorf("failed to parse token response: %w", err)
	}

	if tokenData.Identity == nil || tokenData.Identity.ForumlineID == "" || tokenData.Identity.Username == "" {
		return nil, "", "", fmt.Errorf("invalid identity from Forumline")
	}

	return tokenData.Identity, tokenData.IdentityToken, tokenData.HubAccessToken, nil
}

// createOrLinkUser creates or links a local user from a Forumline identity.
func (h *Handlers) createOrLinkUser(r *http.Request, identity *forumlineIdentity, forumlineAccessToken string) (string, error) {
	ctx := r.Context()

	// 1. Check if a local profile already has this forumline_id
	var existingID string
	err := h.Pool.QueryRow(ctx,
		"SELECT id FROM profiles WHERE forumline_id = $1", identity.ForumlineID).Scan(&existingID)
	if err == nil && existingID != "" {
		// Update display info
		if _, err := h.Pool.Exec(ctx,
			"UPDATE profiles SET display_name = $1 WHERE id = $2",
			identity.DisplayName, existingID); err != nil {
			log.Printf("update display_name error: %v", err)
		}
		return existingID, nil
	}

	// 2. Get Forumline email and check for collision
	var forumlineEmail string
	if forumlineAccessToken != "" && h.Config.ForumlineGoTrueURL != "" {
		forumlineEmail, _ = gotrueGetUserByToken(h.Config.ForumlineGoTrueURL, forumlineAccessToken)
	}

	if forumlineEmail != "" {
		// Check if a local GoTrue user with this email already exists
		users, err := gotrueAdminListUsers(h.Config.GoTrueURL, h.Config.GoTrueServiceRoleKey)
		if err == nil {
			for _, u := range users {
				if strings.EqualFold(u.Email, forumlineEmail) {
					// User exists in GoTrue — check if they were created by a previous
					// Forumline login (has forumline_id in metadata) or are a local account.
					if meta, ok := u.UserMetadata["forumline_id"].(string); ok && meta == identity.ForumlineID {
						// This GoTrue user was created by a previous Forumline login.
						// Link the profile and return.
						h.ensureProfileWithForumlineID(ctx, u.ID, identity)
						return u.ID, nil
					}
					// Check if there's a profile without forumline_id that we can claim
					var profileID string
					err := h.Pool.QueryRow(ctx,
						"SELECT id FROM profiles WHERE id = $1 AND (forumline_id IS NULL OR forumline_id = '')", u.ID).Scan(&profileID)
					if err == nil && profileID != "" {
						h.ensureProfileWithForumlineID(ctx, profileID, identity)
						return profileID, nil
					}
					return "", fmt.Errorf("EMAIL_COLLISION: a local account with this email already exists, sign in locally and connect Forumline from Settings")
				}
			}
		}

		// Create new local user with Forumline email
		newUserID, err := gotrueAdminCreateUser(h.Config.GoTrueURL, h.Config.GoTrueServiceRoleKey, map[string]interface{}{
			"email":         forumlineEmail,
			"password":      randomHex(16),
			"email_confirm": true,
			"user_metadata": map[string]string{
				"username":     identity.Username,
				"display_name": identity.DisplayName,
				"forumline_id": identity.ForumlineID,
			},
		})
		if err != nil {
			return "", fmt.Errorf("failed to create local user: %w", err)
		}

		h.ensureProfileWithForumlineID(ctx, newUserID, identity)
		return newUserID, nil
	}

	// 3. Fallback: check existing GoTrue users for matching forumline_id before creating
	users, listErr := gotrueAdminListUsers(h.Config.GoTrueURL, h.Config.GoTrueServiceRoleKey)
	if listErr == nil {
		for _, u := range users {
			if meta, ok := u.UserMetadata["forumline_id"].(string); ok && meta == identity.ForumlineID {
				h.ensureProfileWithForumlineID(ctx, u.ID, identity)
				return u.ID, nil
			}
		}
	}

	newUserID, err := gotrueAdminCreateUser(h.Config.GoTrueURL, h.Config.GoTrueServiceRoleKey, map[string]interface{}{
		"email":         identity.Username + "@forumline.local",
		"password":      randomHex(16),
		"email_confirm": true,
		"user_metadata": map[string]string{
			"username":     identity.Username,
			"display_name": identity.DisplayName,
			"forumline_id": identity.ForumlineID,
		},
	})
	if err != nil {
		return "", fmt.Errorf("failed to create local user: %w", err)
	}

	h.ensureProfileWithForumlineID(ctx, newUserID, identity)
	return newUserID, nil
}

// ensureProfileWithForumlineID creates or updates a profile with the forumline_id set.
func (h *Handlers) ensureProfileWithForumlineID(ctx context.Context, userID string, identity *forumlineIdentity) {
	if _, err := h.Pool.Exec(ctx,
		`INSERT INTO profiles (id, username, display_name, forumline_id)
		 VALUES ($1, $2, $3, $4)
		 ON CONFLICT (id) DO UPDATE SET forumline_id = $4, display_name = $3`,
		userID, identity.Username, identity.DisplayName, identity.ForumlineID); err != nil {
		log.Printf("ensureProfileWithForumlineID error: %v", err)
	}
}

// setForumlineCookies sets the standard set of Forumline httpOnly cookies.
func (h *Handlers) setForumlineCookies(w http.ResponseWriter, identityToken, localUserID, forumlineAccessToken string) {
	http.SetCookie(w, &http.Cookie{
		Name: "forumline_identity", Value: identityToken,
		Path: "/", HttpOnly: true, SameSite: http.SameSiteNoneMode, Secure: true, MaxAge: 3600,
	})
	http.SetCookie(w, &http.Cookie{
		Name: "forumline_user_id", Value: localUserID,
		Path: "/", HttpOnly: true, SameSite: http.SameSiteNoneMode, Secure: true, MaxAge: 3600,
	})
	if forumlineAccessToken != "" {
		http.SetCookie(w, &http.Cookie{
			Name: "forumline_access_token", Value: forumlineAccessToken,
			Path: "/", HttpOnly: true, SameSite: http.SameSiteNoneMode, Secure: true, MaxAge: 3600,
		})
	}
}

func (h *Handlers) clearForumlineCookies(w http.ResponseWriter) {
	for _, name := range []string{"forumline_identity", "forumline_user_id"} {
		http.SetCookie(w, &http.Cookie{
			Name: name, Value: "",
			Path: "/", HttpOnly: true, SameSite: http.SameSiteNoneMode, Secure: true, MaxAge: -1,
		})
	}
}

func clearCookie(w http.ResponseWriter, name string) {
	http.SetCookie(w, &http.Cookie{
		Name: name, Value: "",
		Path: "/", HttpOnly: true, SameSite: http.SameSiteNoneMode, Secure: true, MaxAge: -1,
	})
}

func randomHex(n int) string {
	b := make([]byte, n)
	rand.Read(b)
	return hex.EncodeToString(b)
}

func nilIfEmpty(s string) interface{} {
	if s == "" {
		return nil
	}
	return s
}

// --- Hosted mode (no GoTrue) ---

// isHostedMode returns true when there's no local GoTrue instance.
// In hosted mode, users authenticate exclusively via Forumline identity.
func (h *Handlers) isHostedMode() bool {
	return h.Config.GoTrueURL == ""
}

// createOrLinkUserHosted creates or links a local profile without GoTrue.
// Profile UUIDs are generated directly; the forumline_id is the source of truth.
func (h *Handlers) createOrLinkUserHosted(r *http.Request, identity *forumlineIdentity) (string, error) {
	ctx := r.Context()

	// Check if a profile with this forumline_id already exists
	var existingID string
	err := h.Pool.QueryRow(ctx,
		"SELECT id FROM profiles WHERE forumline_id = $1", identity.ForumlineID).Scan(&existingID)
	if err == nil && existingID != "" {
		// Update display info
		if _, err := h.Pool.Exec(ctx,
			"UPDATE profiles SET display_name = $1, avatar_url = $2, updated_at = now() WHERE id = $3",
			identity.DisplayName, identity.AvatarURL, existingID); err != nil {
			log.Printf("update hosted profile error: %v", err)
		}
		return existingID, nil
	}

	// Create new profile — use forumline_id as the profile UUID to keep things simple.
	// This works because forumline_id is already a UUID.
	_, err = h.Pool.Exec(ctx,
		`INSERT INTO profiles (id, username, display_name, avatar_url, forumline_id)
		 VALUES ($1, $2, $3, $4, $5)
		 ON CONFLICT (id) DO UPDATE SET forumline_id = $5, display_name = $3`,
		identity.ForumlineID, identity.Username, identity.DisplayName, identity.AvatarURL, identity.ForumlineID)
	if err != nil {
		return "", fmt.Errorf("create profile: %w", err)
	}

	return identity.ForumlineID, nil
}

// signHostedSession creates a JWT for hosted-mode auth.
// The JWT uses the same format as GoTrue tokens so the existing auth middleware
// validates it transparently. Signed with JWT_SECRET (HMAC).
func (h *Handlers) signHostedSession(userID string) (string, error) {
	secret := h.Config.ForumlineJWTSecret
	if secret == "" {
		return "", fmt.Errorf("no JWT secret configured")
	}

	now := time.Now()
	claims := jwt.RegisteredClaims{
		Subject:   userID,
		IssuedAt:  jwt.NewNumericDate(now),
		ExpiresAt: jwt.NewNumericDate(now.Add(24 * time.Hour)),
		Issuer:    "forumline-hosted",
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(secret))
}
