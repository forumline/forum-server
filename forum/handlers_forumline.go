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
	"github.com/forumline/forum-server/forum/model"
)

// HandleForumlineAuth handles GET/POST /api/forumline/auth.
// Supports two flows:
// 1. forumline_token query param — server-side OAuth for iframe usage
// 2. No params — redirect to Forumline authorize page
func (h *Handlers) HandleForumlineAuth(w http.ResponseWriter, r *http.Request) {
	forumlineToken := r.URL.Query().Get("forumline_token")
	if forumlineToken != "" {
		h.handleServerSideAuth(w, r, forumlineToken)
		return
	}

	// Default: redirect to Forumline authorize page
	h.redirectToForumlineAuth(w, r)
}

// handleServerSideAuth does the entire OAuth exchange server-side (for iframe usage).
func (h *Handlers) handleServerSideAuth(w http.ResponseWriter, r *http.Request, forumlineToken string) {
	log.Println("[Forumline:Auth] Starting server-side auth with forumline_token")

	if h.Config.ForumlineClientID == "" {
		log.Println("[Forumline:Auth] No OAuth client_id configured for this forum")
		http.Redirect(w, r, h.Config.SiteURL+"/login?error=auth_failed", http.StatusFound)
		return
	}

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
	localUserID, err := h.createOrLinkUser(r, identity)
	if err != nil {
		log.Printf("[Forumline:Auth] createOrLinkUser failed: %v", err)
		http.Redirect(w, r, h.Config.SiteURL+"/login?error=auth_failed", http.StatusFound)
		return
	}

	// Step 4: Set cookies and generate session
	h.setForumlineCookies(w, identityToken, localUserID, forumlineAccessToken)

	accessToken, err := h.signSession(localUserID)
	if err != nil {
		log.Printf("[Forumline:Auth] signSession failed: %v", err)
		http.Redirect(w, r, h.Config.SiteURL+"/login?error=auth_failed", http.StatusFound)
		return
	}
	http.Redirect(w, r, fmt.Sprintf("%s/#access_token=%s&type=bearer", h.Config.SiteURL, accessToken), http.StatusFound)
}

// redirectToForumlineAuth redirects browser to the forumline OAuth authorize page.
func (h *Handlers) redirectToForumlineAuth(w http.ResponseWriter, r *http.Request) {
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

	localUserID, err := h.createOrLinkUser(r, identity)
	if err != nil {
		log.Printf("[Forumline:Callback] createOrLinkUser failed: %v", err)
		http.Redirect(w, r, h.Config.SiteURL+"/login?error=auth_failed", http.StatusFound)
		return
	}

	// Set cookies
	clearCookie(w, "forumline_state")
	h.setForumlineCookies(w, identityToken, localUserID, forumlineAccessToken)

	accessToken, err := h.signSession(localUserID)
	if err != nil {
		log.Printf("[Forumline:Callback] signSession failed: %v", err)
		http.Redirect(w, r, h.Config.SiteURL+"/login?error=auth_failed", http.StatusFound)
		return
	}
	http.Redirect(w, r, fmt.Sprintf("%s/#access_token=%s&type=bearer", h.Config.SiteURL, accessToken), http.StatusFound)
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
		forumlineID, err := h.Store.GetForumlineID(r.Context(), localUserID)
		if err != nil {
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

// handleDisconnect clears Forumline session cookies.
func (h *Handlers) handleDisconnect(w http.ResponseWriter, r *http.Request) {
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
func (h *Handlers) exchangeCodeForTokens(code, redirectURI string) (*model.ForumlineIdentity, string, string, error) {
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
		Identity       *model.ForumlineIdentity `json:"identity"`
		IdentityToken  string                   `json:"identity_token"`
		HubAccessToken string                   `json:"forumline_access_token"`
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
func (h *Handlers) createOrLinkUser(r *http.Request, identity *model.ForumlineIdentity) (string, error) {
	ctx := r.Context()

	existingID, err := h.Store.GetProfileIDByForumlineID(ctx, identity.ForumlineID)
	if err == nil && existingID != "" {
		_ = h.Store.UpdateDisplayNameAndAvatar(ctx, existingID, identity.DisplayName, identity.AvatarURL)
		return existingID, nil
	}

	if err := h.Store.CreateProfileHosted(ctx, identity); err != nil {
		return "", fmt.Errorf("create profile: %w", err)
	}
	return identity.ForumlineID, nil
}

// signSession creates a JWT session token for a local user.
func (h *Handlers) signSession(userID string) (string, error) {
	secret := h.Config.ForumlineJWTSecret
	if secret == "" {
		return "", fmt.Errorf("no JWT secret configured")
	}

	now := time.Now()
	claims := jwt.RegisteredClaims{
		Subject:   userID,
		IssuedAt:  jwt.NewNumericDate(now),
		ExpiresAt: jwt.NewNumericDate(now.Add(24 * time.Hour)),
		Issuer:    "forumline-forum",
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(secret))
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

