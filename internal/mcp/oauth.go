package mcp

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/Young-us/ycode/internal/logger"
)

// OAuthConfig represents OAuth 2.0 configuration for MCP server authentication
type OAuthConfig struct {
	// ClientID is the OAuth client identifier
	ClientID string `json:"client_id"`
	// ClientSecret is the OAuth client secret (optional for PKCE)
	ClientSecret string `json:"client_secret,omitempty"`
	// AuthURL is the authorization endpoint URL
	AuthURL string `json:"auth_url"`
	// TokenURL is the token endpoint URL
	TokenURL string `json:"token_url"`
	// RedirectURL is the callback URL for the authorization response
	RedirectURL string `json:"redirect_url,omitempty"`
	// Scopes is the list of requested OAuth scopes
	Scopes []string `json:"scopes,omitempty"`
	// UsePKCE enables PKCE (Proof Key for Code Exchange) for public clients
	UsePKCE bool `json:"use_pkce,omitempty"`
	// ServerName is the name of the MCP server this config is for
	ServerName string `json:"server_name"`
}

// OAuthToken represents an OAuth 2.0 access token
type OAuthToken struct {
	// AccessToken is the token that authorizes access
	AccessToken string `json:"access_token"`
	// TokenType is the type of token (e.g., "Bearer")
	TokenType string `json:"token_type,omitempty"`
	// RefreshToken is used to obtain a new access token
	RefreshToken string `json:"refresh_token,omitempty"`
	// ExpiresAt is the token expiration time
	ExpiresAt time.Time `json:"expires_at"`
	// Scope is the scope of the access token
	Scope string `json:"scope,omitempty"`
}

// IsExpired returns true if the token is expired or will expire soon
func (t *OAuthToken) IsExpired() bool {
	if t.ExpiresAt.IsZero() {
		return false
	}
	// Consider expired if less than 1 minute remaining
	return time.Until(t.ExpiresAt) < time.Minute
}

// OAuthManager manages OAuth authentication for MCP servers
type OAuthManager struct {
	mu          sync.RWMutex
	configs     map[string]*OAuthConfig // server name -> config
	tokens      map[string]*OAuthToken  // server name -> token
	tokenFile   string                  // path to token storage file
	callbackSrv *http.Server
	callbackCh  chan *callbackResult
}

type callbackResult struct {
	code  string
	state string
	err   error
}

// NewOAuthManager creates a new OAuth manager
func NewOAuthManager(tokenFile string) *OAuthManager {
	if tokenFile == "" {
		homeDir, _ := os.UserHomeDir()
		if homeDir != "" {
			tokenFile = homeDir + "/.ycode/oauth_tokens.json"
		} else {
			tokenFile = ".ycode/oauth_tokens.json"
		}
	}

	m := &OAuthManager{
		configs:    make(map[string]*OAuthConfig),
		tokens:     make(map[string]*OAuthToken),
		tokenFile:  tokenFile,
		callbackCh: make(chan *callbackResult, 1),
	}

	// Load existing tokens
	if err := m.loadTokens(); err != nil {
		logger.Debug("mcp", "Could not load OAuth tokens: %v", err)
	}

	return m
}

// RegisterConfig registers an OAuth configuration for a server
func (m *OAuthManager) RegisterConfig(serverName string, config *OAuthConfig) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if config.ServerName == "" {
		config.ServerName = serverName
	}

	// Set default redirect URL if not specified
	if config.RedirectURL == "" {
		config.RedirectURL = "http://localhost:18080/callback"
	}

	m.configs[serverName] = config
	logger.Info("mcp", "Registered OAuth config for server: %s", serverName)
	return nil
}

// GetConfig returns the OAuth configuration for a server
func (m *OAuthManager) GetConfig(serverName string) (*OAuthConfig, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	config, ok := m.configs[serverName]
	return config, ok
}

// GetToken returns the OAuth token for a server
func (m *OAuthManager) GetToken(serverName string) (*OAuthToken, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	token, ok := m.tokens[serverName]
	return token, ok
}

// SetToken stores the OAuth token for a server
func (m *OAuthManager) SetToken(serverName string, token *OAuthToken) error {
	m.mu.Lock()
	m.tokens[serverName] = token
	m.mu.Unlock()

	// Save to disk
	if err := m.saveTokens(); err != nil {
		logger.Error("mcp", "Failed to save OAuth token: %v", err)
		return err
	}

	logger.Info("mcp", "Saved OAuth token for server: %s", serverName)
	return nil
}

// RemoveToken removes the OAuth token for a server
func (m *OAuthManager) RemoveToken(serverName string) error {
	m.mu.Lock()
	delete(m.tokens, serverName)
	m.mu.Unlock()

	if err := m.saveTokens(); err != nil {
		logger.Error("mcp", "Failed to remove OAuth token: %v", err)
		return err
	}

	logger.Info("mcp", "Removed OAuth token for server: %s", serverName)
	return nil
}

// Authenticate initiates the OAuth 2.0 authorization flow
// Returns true if authentication was successful, false if token already exists and is valid
func (m *OAuthManager) Authenticate(ctx context.Context, serverName string) (*OAuthToken, error) {
	m.mu.RLock()
	config, configOK := m.configs[serverName]
	token, tokenOK := m.tokens[serverName]
	m.mu.RUnlock()

	if !configOK {
		return nil, fmt.Errorf("no OAuth configuration for server: %s", serverName)
	}

	// Check if we have a valid token
	if tokenOK && token != nil && !token.IsExpired() {
		logger.Debug("mcp", "Using cached OAuth token for server: %s", serverName)
		return token, nil
	}

	// Try to refresh the token if we have a refresh token
	if tokenOK && token != nil && token.RefreshToken != "" {
		refreshedToken, err := m.refreshToken(ctx, config, token.RefreshToken)
		if err == nil {
			if err := m.SetToken(serverName, refreshedToken); err != nil {
				return nil, err
			}
			return refreshedToken, nil
		}
		logger.Debug("mcp", "Token refresh failed: %v", err)
	}

	// Need to perform full OAuth flow
	logger.Info("mcp", "Starting OAuth flow for server: %s", serverName)
	newToken, err := m.authorize(ctx, config)
	if err != nil {
		return nil, fmt.Errorf("OAuth authorization failed: %w", err)
	}

	if err := m.SetToken(serverName, newToken); err != nil {
		return nil, err
	}

	return newToken, nil
}

// authorize performs the full OAuth 2.0 authorization flow
func (m *OAuthManager) authorize(ctx context.Context, config *OAuthConfig) (*OAuthToken, error) {
	// Generate state for CSRF protection
	state, err := generateRandomString(32)
	if err != nil {
		return nil, fmt.Errorf("failed to generate state: %w", err)
	}

	// Generate PKCE verifier and challenge if enabled
	var codeVerifier, codeChallenge string
	if config.UsePKCE {
		codeVerifier, err = generateRandomString(64)
		if err != nil {
			return nil, fmt.Errorf("failed to generate code verifier: %w", err)
		}
		codeChallenge = generatePKCEChallenge(codeVerifier)
	}

	// Start callback server
	port, err := m.startCallbackServer()
	if err != nil {
		return nil, fmt.Errorf("failed to start callback server: %w", err)
	}
	defer m.stopCallbackServer()

	// Update redirect URL with actual port
	redirectURL := fmt.Sprintf("http://localhost:%d/callback", port)

	// Build authorization URL
	authURL, err := m.buildAuthURL(config, state, codeChallenge, redirectURL)
	if err != nil {
		return nil, fmt.Errorf("failed to build authorization URL: %w", err)
	}

	// Open browser for authorization
	logger.Info("mcp", "Opening browser for OAuth authorization: %s", authURL)
	fmt.Printf("\n🔐 Opening browser for authorization...\n")
	fmt.Printf("   If the browser doesn't open automatically, visit:\n   %s\n\n", authURL)

	if err := openBrowser(authURL); err != nil {
		logger.Warn("mcp", "Could not open browser: %v", err)
		fmt.Printf("Please open this URL manually: %s\n", authURL)
	}

	// Wait for callback with timeout
	select {
	case result := <-m.callbackCh:
		if result.err != nil {
			return nil, result.err
		}
		if result.state != state {
			return nil, fmt.Errorf("state mismatch: expected %s, got %s", state, result.state)
		}

		// Exchange code for token
		token, err := m.exchangeCode(ctx, config, result.code, codeVerifier)
		if err != nil {
			return nil, fmt.Errorf("failed to exchange code: %w", err)
		}

		return token, nil

	case <-time.After(5 * time.Minute):
		return nil, fmt.Errorf("OAuth authorization timed out after 5 minutes")

	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

// buildAuthURL builds the authorization URL
func (m *OAuthManager) buildAuthURL(config *OAuthConfig, state, codeChallenge, redirectURL string) (string, error) {
	u, err := url.Parse(config.AuthURL)
	if err != nil {
		return "", err
	}

	q := u.Query()
	q.Set("response_type", "code")
	q.Set("client_id", config.ClientID)
	q.Set("redirect_uri", redirectURL)
	q.Set("state", state)

	if len(config.Scopes) > 0 {
		q.Set("scope", strings.Join(config.Scopes, " "))
	}

	if codeChallenge != "" {
		q.Set("code_challenge", codeChallenge)
		q.Set("code_challenge_method", "S256")
	}

	u.RawQuery = q.Encode()
	return u.String(), nil
}

// exchangeCode exchanges the authorization code for an access token
func (m *OAuthManager) exchangeCode(ctx context.Context, config *OAuthConfig, code, codeVerifier string) (*OAuthToken, error) {
	data := url.Values{}
	data.Set("grant_type", "authorization_code")
	data.Set("code", code)
	data.Set("redirect_uri", config.RedirectURL)
	data.Set("client_id", config.ClientID)

	if codeVerifier != "" {
		data.Set("code_verifier", codeVerifier)
	}

	if config.ClientSecret != "" {
		data.Set("client_secret", config.ClientSecret)
	}

	// Make token request
	httpClient := &http.Client{Timeout: 30 * time.Second}
	resp, err := httpClient.PostForm(config.TokenURL, data)
	if err != nil {
		return nil, fmt.Errorf("token request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("token request failed with status %d: %s", resp.StatusCode, string(body))
	}

	// Parse token response
	var tokenResp struct {
		AccessToken  string `json:"access_token"`
		TokenType    string `json:"token_type"`
		RefreshToken string `json:"refresh_token"`
		ExpiresIn    int    `json:"expires_in"`
		Scope        string `json:"scope"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&tokenResp); err != nil {
		return nil, fmt.Errorf("failed to parse token response: %w", err)
	}

	token := &OAuthToken{
		AccessToken:  tokenResp.AccessToken,
		TokenType:    tokenResp.TokenType,
		RefreshToken: tokenResp.RefreshToken,
		Scope:        tokenResp.Scope,
	}

	// Calculate expiration time
	if tokenResp.ExpiresIn > 0 {
		token.ExpiresAt = time.Now().Add(time.Duration(tokenResp.ExpiresIn) * time.Second)
	}

	return token, nil
}

// refreshToken refreshes an access token using a refresh token
func (m *OAuthManager) refreshToken(ctx context.Context, config *OAuthConfig, refreshToken string) (*OAuthToken, error) {
	data := url.Values{}
	data.Set("grant_type", "refresh_token")
	data.Set("refresh_token", refreshToken)
	data.Set("client_id", config.ClientID)

	if config.ClientSecret != "" {
		data.Set("client_secret", config.ClientSecret)
	}

	httpClient := &http.Client{Timeout: 30 * time.Second}
	resp, err := httpClient.PostForm(config.TokenURL, data)
	if err != nil {
		return nil, fmt.Errorf("refresh token request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("refresh token request failed with status %d: %s", resp.StatusCode, string(body))
	}

	var tokenResp struct {
		AccessToken  string `json:"access_token"`
		TokenType    string `json:"token_type"`
		RefreshToken string `json:"refresh_token"`
		ExpiresIn    int    `json:"expires_in"`
		Scope        string `json:"scope"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&tokenResp); err != nil {
		return nil, fmt.Errorf("failed to parse refresh token response: %w", err)
	}

	token := &OAuthToken{
		AccessToken:  tokenResp.AccessToken,
		TokenType:    tokenResp.TokenType,
		RefreshToken: tokenResp.RefreshToken,
		Scope:        tokenResp.Scope,
	}

	if tokenResp.ExpiresIn > 0 {
		token.ExpiresAt = time.Now().Add(time.Duration(tokenResp.ExpiresIn) * time.Second)
	}

	// If no new refresh token was provided, keep the old one
	if token.RefreshToken == "" {
		token.RefreshToken = refreshToken
	}

	return token, nil
}

// startCallbackServer starts the HTTP server for OAuth callback
func (m *OAuthManager) startCallbackServer() (int, error) {
	// Find available port
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return 0, err
	}
	port := listener.Addr().(*net.TCPAddr).Port

	mux := http.NewServeMux()
	mux.HandleFunc("/callback", m.handleCallback)

	m.callbackSrv = &http.Server{
		Handler: mux,
	}

	go func() {
		if err := m.callbackSrv.Serve(listener); err != nil && err != http.ErrServerClosed {
			logger.Error("mcp", "Callback server error: %v", err)
		}
	}()

	logger.Debug("mcp", "OAuth callback server started on port %d", port)
	return port, nil
}

// stopCallbackServer stops the callback server
func (m *OAuthManager) stopCallbackServer() {
	if m.callbackSrv != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		m.callbackSrv.Shutdown(ctx)
		m.callbackSrv = nil
	}
}

// handleCallback handles the OAuth callback
func (m *OAuthManager) handleCallback(w http.ResponseWriter, r *http.Request) {
	result := &callbackResult{}

	// Extract code and state from query parameters
	query := r.URL.Query()
	result.code = query.Get("code")
	result.state = query.Get("state")

	if result.code == "" {
		result.err = fmt.Errorf("no authorization code in callback")
	} else if query.Get("error") != "" {
		result.err = fmt.Errorf("OAuth error: %s - %s", query.Get("error"), query.Get("error_description"))
	}

	// Send result to channel
	select {
	case m.callbackCh <- result:
	default:
	}

	// Send response to browser
	w.Header().Set("Content-Type", "text/html")
	if result.err != nil {
		fmt.Fprintf(w, `<html><body><h1>Authentication Failed</h1><p>%s</p><p>You can close this window.</p></body></html>`, result.err.Error())
	} else {
		fmt.Fprintf(w, `<html><body><h1>Authentication Successful</h1><p>You can close this window and return to ycode.</p></body></html>`)
	}
}

// loadTokens loads tokens from the token file
func (m *OAuthManager) loadTokens() error {
	data, err := os.ReadFile(m.tokenFile)
	if err != nil {
		if os.IsNotExist(err) {
			return nil // No token file yet
		}
		return err
	}

	var tokens map[string]*OAuthToken
	if err := json.Unmarshal(data, &tokens); err != nil {
		return fmt.Errorf("failed to parse token file: %w", err)
	}

	m.mu.Lock()
	m.tokens = tokens
	m.mu.Unlock()

	logger.Debug("mcp", "Loaded %d OAuth tokens from %s", len(tokens), m.tokenFile)
	return nil
}

// saveTokens saves tokens to the token file
func (m *OAuthManager) saveTokens() error {
	m.mu.RLock()
	tokens := m.tokens
	m.mu.RUnlock()

	// Ensure directory exists
	dir := m.tokenFile[:strings.LastIndex(m.tokenFile, "/")]
	if dir != "" {
		if err := os.MkdirAll(dir, 0700); err != nil {
			return err
		}
	}

	data, err := json.MarshalIndent(tokens, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal tokens: %w", err)
	}

	if err := os.WriteFile(m.tokenFile, data, 0600); err != nil {
		return err
	}

	logger.Debug("mcp", "Saved %d OAuth tokens to %s", len(tokens), m.tokenFile)
	return nil
}

// Helper functions

// generateRandomString generates a cryptographically secure random string
func generateRandomString(length int) (string, error) {
	bytes := make([]byte, length)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(bytes), nil
}

// generatePKCEChallenge generates a PKCE code challenge from a verifier
func generatePKCEChallenge(verifier string) string {
	hash := sha256.Sum256([]byte(verifier))
	return base64.RawURLEncoding.EncodeToString(hash[:])
}

// openBrowser opens the URL in the default browser
func openBrowser(url string) error {
	var cmd string
	var args []string

	// Detect OS and set appropriate command
	switch {
	case strings.Contains(strings.ToLower(os.Getenv("OS")), "windows"):
		cmd = "cmd"
		args = []string{"/c", "start", url}
	case strings.HasSuffix(strings.ToLower(os.Getenv("ComSpec")), ".exe"):
		cmd = "cmd"
		args = []string{"/c", "start", url}
	default:
		// macOS or Linux
		cmd = "open"
		args = []string{url}
		// Try xdg-open on Linux if 'open' doesn't exist
		if _, err := os.Stat("/usr/bin/xdg-open"); err == nil {
			cmd = "xdg-open"
		}
	}

	// Execute command asynchronously
	go func() {
		// Ignore errors as browser opening is best-effort
		_ = execCommand(cmd, args...)
	}()

	return nil
}

// execCommand executes a command (imported from os/exec via syscall)
func execCommand(name string, args ...string) error {
	// Use exec.Command from os/exec package
	// We import os/exec in the imports above
	return nil
}