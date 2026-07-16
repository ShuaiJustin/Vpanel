package handlers

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"v/internal/database/repository"
	"v/internal/logger"
	"v/internal/settings"
	pkgerrors "v/pkg/errors"
)

const portalOAuthStateCookie = "vpanel_portal_oauth_state"

var portalOAuthProviderLabels = map[string]string{
	"custom":  "自定义 OAuth",
	"github":  "GitHub",
	"discord": "Discord",
	"oidc":    "OIDC",
	"linuxdo": "LinuxDO",
	"wechat":  "微信",
	"wecom":   "企业微信",
}

var portalOAuthGenericProviders = map[string]struct{}{
	"custom":  {},
	"github":  {},
	"discord": {},
	"oidc":    {},
	"linuxdo": {},
}

type portalOAuthState struct {
	State    string `json:"state"`
	Provider string `json:"provider"`
	Redirect string `json:"redirect"`
	Expires  int64  `json:"expires"`
}

type portalOAuthUserInfo struct {
	ExternalID    string
	Email         string
	EmailVerified bool
	Username      string
	DisplayName   string
	AvatarURL     string
}

type portalOAuthTokenResponse struct {
	AccessToken string `json:"access_token"`
	TokenType   string `json:"token_type"`
	IDToken     string `json:"id_token"`
	Error       string `json:"error"`
	Description string `json:"error_description"`
}

type portalOAuthHTTPClientKey struct{}

// GetOAuthProviders returns public OAuth login providers for the portal login page.
func (h *PortalAuthHandler) GetOAuthProviders(c *gin.Context) {
	authSettings, ok := h.loadPortalAuthSettings(c)
	if !ok {
		return
	}

	providers := make([]gin.H, 0)
	if authSettings.OAuth.Enabled {
		for _, key := range authSettings.OAuth.ProviderOrder {
			provider, exists := authSettings.OAuth.Providers[key]
			if !exists || !isPortalOAuthProviderLoginReady(key, provider) {
				continue
			}
			providers = append(providers, gin.H{
				"key":   key,
				"label": portalOAuthProviderLabel(key),
			})
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"enabled":   authSettings.OAuth.Enabled,
		"providers": providers,
	})
}

// StartOAuth redirects the browser to the selected provider authorization URL.
func (h *PortalAuthHandler) StartOAuth(c *gin.Context) {
	providerKey := strings.ToLower(strings.TrimSpace(c.Param("provider")))
	_, _, authorizeURL, ok := h.initializePortalOAuth(c, providerKey)
	if !ok {
		return
	}
	c.Redirect(http.StatusFound, authorizeURL)
}

// GetOAuthEmbedConfig returns public data required by the official embedded login widget.
func (h *PortalAuthHandler) GetOAuthEmbedConfig(c *gin.Context) {
	providerKey := strings.ToLower(strings.TrimSpace(c.Param("provider")))
	if providerKey != "wecom" {
		c.JSON(http.StatusNotFound, gin.H{"error": "该登录方式不支持嵌入式登录"})
		return
	}

	provider, stateValue, authorizeURL, ok := h.initializePortalOAuth(c, providerKey)
	if !ok {
		return
	}
	c.Header("Cache-Control", "no-store")
	c.JSON(http.StatusOK, buildPortalOAuthEmbedConfig(c, providerKey, provider, stateValue, authorizeURL))
}

func buildPortalOAuthEmbedConfig(c *gin.Context, providerKey string, provider settings.OAuthProviderSettings, stateValue, authorizeURL string) gin.H {
	return gin.H{
		"appid":         strings.TrimSpace(provider.CorpID),
		"agentid":       strings.TrimSpace(provider.AgentID),
		"redirect_uri":  portalOAuthRedirectURI(c, providerKey, provider),
		"state":         stateValue,
		"authorize_url": authorizeURL,
	}
}

func (h *PortalAuthHandler) initializePortalOAuth(c *gin.Context, providerKey string) (settings.OAuthProviderSettings, string, string, bool) {
	_, provider, ok := h.resolvePortalOAuthProvider(c, providerKey)
	if !ok {
		return settings.OAuthProviderSettings{}, "", "", false
	}

	stateValue, err := randomPortalOAuthState()
	if err != nil {
		h.logger.Error("failed to generate oauth state", logger.F("error", err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "无法初始化第三方登录"})
		return settings.OAuthProviderSettings{}, "", "", false
	}
	state := portalOAuthState{
		State:    stateValue,
		Provider: providerKey,
		Redirect: safePortalOAuthRedirect(c.Query("redirect")),
		Expires:  time.Now().Add(10 * time.Minute).Unix(),
	}
	encodedState, err := encodePortalOAuthState(state)
	if err != nil {
		h.logger.Error("failed to encode oauth state", logger.F("error", err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "无法初始化第三方登录"})
		return settings.OAuthProviderSettings{}, "", "", false
	}
	c.SetSameSite(http.SameSiteLaxMode)
	c.SetCookie(portalOAuthStateCookie, encodedState, 600, "/", "", isPortalSecureRequest(c), true)

	authorizeURL, err := buildPortalOAuthAuthorizeURL(c, providerKey, provider, stateValue)
	if err != nil {
		h.logger.Warn("invalid oauth authorize url", logger.F("provider", providerKey), logger.F("error", err))
		c.JSON(http.StatusBadRequest, gin.H{"error": "第三方登录配置无效"})
		return settings.OAuthProviderSettings{}, "", "", false
	}
	return provider, stateValue, authorizeURL, true
}

// OAuthCallback handles an OAuth authorization code callback.
func (h *PortalAuthHandler) OAuthCallback(c *gin.Context) {
	providerKey := strings.ToLower(strings.TrimSpace(c.Param("provider")))
	if providerError := strings.TrimSpace(c.Query("error")); providerError != "" {
		c.Redirect(http.StatusFound, portalOAuthLoginErrorURL("第三方登录已取消或失败"))
		return
	}

	state, ok := h.validatePortalOAuthState(c, providerKey)
	if !ok {
		return
	}
	c.SetCookie(portalOAuthStateCookie, "", -1, "/", "", isPortalSecureRequest(c), true)

	authSettings, provider, ok := h.resolvePortalOAuthProvider(c, providerKey)
	if !ok {
		return
	}
	code := strings.TrimSpace(c.Query("code"))
	if code == "" {
		c.Redirect(http.StatusFound, portalOAuthLoginErrorURL("第三方登录回调缺少授权码"))
		return
	}

	tokenResponse, err := h.exchangePortalOAuthCode(c.Request.Context(), c, providerKey, provider, code)
	if err != nil {
		h.logger.Warn("oauth token exchange failed", logger.F("provider", providerKey), logger.Err(err))
		c.Redirect(http.StatusFound, portalOAuthLoginErrorURL("第三方登录授权失败"))
		return
	}

	userInfo, err := h.fetchPortalOAuthUserInfo(c.Request.Context(), providerKey, provider, tokenResponse)
	if err != nil {
		h.logger.Warn("oauth userinfo failed", logger.F("provider", providerKey), logger.Err(err))
		c.Redirect(http.StatusFound, portalOAuthLoginErrorURL("无法获取第三方账号信息"))
		return
	}

	user, err := h.resolvePortalOAuthUser(c.Request.Context(), authSettings, providerKey, userInfo)
	if err != nil {
		h.logger.Warn("oauth user resolve failed", logger.F("provider", providerKey), logger.Err(err))
		c.Redirect(http.StatusFound, portalOAuthLoginErrorURL(err.Error()))
		return
	}

	if !user.Enabled {
		c.Redirect(http.StatusFound, portalOAuthLoginErrorURL("账号已被禁用，请联系管理员"))
		return
	}
	if user.IsExpired() {
		c.Redirect(http.StatusFound, portalOAuthLoginErrorURL("账号已过期，请续费"))
		return
	}
	if h.entitlement != nil {
		if _, _, entitlementErr := h.entitlement.EnsureRuntimeProxies(c.Request.Context(), user.ID); entitlementErr != nil && !pkgerrors.IsForbidden(entitlementErr) {
			h.logger.Warn("failed to initialize portal oauth entitlement",
				logger.F("user_id", user.ID),
				logger.F("error", entitlementErr),
			)
		}
	}

	token, err := h.authService.GenerateToken(user.ID, user.Username, user.Role)
	if err != nil {
		h.logger.Error("oauth token generation failed", logger.F("user_id", user.ID), logger.F("error", err))
		c.Redirect(http.StatusFound, portalOAuthLoginErrorURL("登录令牌生成失败"))
		return
	}
	h.updateLastLogin(c, user.ID)

	payload := gin.H{
		"id":                    user.ID,
		"username":              user.Username,
		"email":                 user.Email,
		"role":                  user.Role,
		"permissions":           h.getRolePermissions(c, user.Role),
		"force_password_change": user.ForcePasswordChange,
	}
	userPayload, _ := json.Marshal(payload)
	fragment := url.Values{}
	fragment.Set("token", token)
	fragment.Set("user", base64.RawURLEncoding.EncodeToString(userPayload))
	fragment.Set("redirect", safePortalOAuthRedirect(state.Redirect))

	c.Redirect(http.StatusFound, "/user/oauth/callback#"+fragment.Encode())
}

func (h *PortalAuthHandler) loadPortalAuthSettings(c *gin.Context) (settings.AuthSettings, bool) {
	if h.settingsService == nil {
		return settings.DefaultAuthSettings(), true
	}
	systemSettings, err := h.settingsService.GetSystemSettings(c.Request.Context())
	if err != nil {
		h.logger.Error("failed to load portal auth settings", logger.F("error", err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "无法读取登录设置"})
		return settings.AuthSettings{}, false
	}
	return systemSettings.Auth, true
}

func (h *PortalAuthHandler) resolvePortalOAuthProvider(c *gin.Context, providerKey string) (settings.AuthSettings, settings.OAuthProviderSettings, bool) {
	authSettings, ok := h.loadPortalAuthSettings(c)
	if !ok {
		return settings.AuthSettings{}, settings.OAuthProviderSettings{}, false
	}
	if !authSettings.OAuth.Enabled {
		c.JSON(http.StatusNotFound, gin.H{"error": "第三方登录未启用"})
		return authSettings, settings.OAuthProviderSettings{}, false
	}
	provider, exists := authSettings.OAuth.Providers[providerKey]
	if !exists || !isPortalOAuthProviderLoginReady(providerKey, provider) {
		c.JSON(http.StatusNotFound, gin.H{"error": "第三方登录方式不可用"})
		return authSettings, settings.OAuthProviderSettings{}, false
	}
	return authSettings, provider, true
}

func isPortalOAuthProviderLoginReady(key string, provider settings.OAuthProviderSettings) bool {
	if !provider.Enabled {
		return false
	}
	if key == "wecom" {
		return strings.TrimSpace(provider.CorpID) != "" &&
			strings.TrimSpace(provider.AgentID) != "" &&
			strings.TrimSpace(provider.ClientSecret) != "" &&
			strings.TrimSpace(provider.AuthorizeURL) != "" &&
			strings.TrimSpace(provider.TokenURL) != ""
	}
	if _, ok := portalOAuthGenericProviders[key]; !ok {
		return false
	}
	return strings.TrimSpace(provider.ClientID) != "" &&
		strings.TrimSpace(provider.ClientSecret) != "" &&
		strings.TrimSpace(provider.AuthorizeURL) != "" &&
		strings.TrimSpace(provider.TokenURL) != "" &&
		strings.TrimSpace(provider.UserInfoURL) != ""
}

func portalOAuthProviderLabel(key string) string {
	if label, ok := portalOAuthProviderLabels[key]; ok {
		return label
	}
	return key
}

func randomPortalOAuthState() (string, error) {
	buf := make([]byte, 24)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(buf), nil
}

func encodePortalOAuthState(state portalOAuthState) (string, error) {
	payload, err := json.Marshal(state)
	if err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(payload), nil
}

func decodePortalOAuthState(value string) (portalOAuthState, error) {
	payload, err := base64.RawURLEncoding.DecodeString(value)
	if err != nil {
		return portalOAuthState{}, err
	}
	var state portalOAuthState
	if err := json.Unmarshal(payload, &state); err != nil {
		return portalOAuthState{}, err
	}
	return state, nil
}

func (h *PortalAuthHandler) validatePortalOAuthState(c *gin.Context, providerKey string) (portalOAuthState, bool) {
	cookieValue, err := c.Cookie(portalOAuthStateCookie)
	if err != nil || cookieValue == "" {
		c.Redirect(http.StatusFound, portalOAuthLoginErrorURL("第三方登录状态已过期，请重试"))
		return portalOAuthState{}, false
	}
	state, err := decodePortalOAuthState(cookieValue)
	if err != nil {
		c.Redirect(http.StatusFound, portalOAuthLoginErrorURL("第三方登录状态无效，请重试"))
		return portalOAuthState{}, false
	}
	if time.Now().Unix() > state.Expires {
		c.Redirect(http.StatusFound, portalOAuthLoginErrorURL("第三方登录状态已过期，请重试"))
		return portalOAuthState{}, false
	}
	if state.Provider != providerKey || subtle.ConstantTimeCompare([]byte(state.State), []byte(c.Query("state"))) != 1 {
		c.Redirect(http.StatusFound, portalOAuthLoginErrorURL("第三方登录状态不匹配，请重试"))
		return portalOAuthState{}, false
	}
	return state, true
}

func buildPortalOAuthAuthorizeURL(c *gin.Context, providerKey string, provider settings.OAuthProviderSettings, state string) (string, error) {
	parsed, err := url.Parse(strings.TrimSpace(provider.AuthorizeURL))
	if err != nil {
		return "", err
	}
	query := parsed.Query()
	if providerKey == "wecom" {
		query.Set("login_type", "CorpApp")
		query.Set("appid", strings.TrimSpace(provider.CorpID))
		query.Set("agentid", strings.TrimSpace(provider.AgentID))
		query.Set("redirect_uri", portalOAuthRedirectURI(c, providerKey, provider))
		query.Set("state", state)
		if len(provider.Scopes) > 0 {
			query.Set("scope", strings.Join(provider.Scopes, " "))
		}
		parsed.RawQuery = query.Encode()
		return parsed.String(), nil
	}
	query.Set("response_type", "code")
	query.Set("client_id", strings.TrimSpace(provider.ClientID))
	query.Set("redirect_uri", portalOAuthRedirectURI(c, providerKey, provider))
	query.Set("state", state)
	if len(provider.Scopes) > 0 {
		query.Set("scope", strings.Join(provider.Scopes, " "))
	}
	parsed.RawQuery = query.Encode()
	return parsed.String(), nil
}

func portalOAuthRedirectURI(c *gin.Context, providerKey string, provider settings.OAuthProviderSettings) string {
	if redirectURI := strings.TrimSpace(provider.RedirectURI); redirectURI != "" {
		return redirectURI
	}
	scheme := "http"
	if isPortalSecureRequest(c) {
		scheme = "https"
	}
	host := c.Request.Host
	return fmt.Sprintf("%s://%s/api/portal/auth/oauth/%s/callback", scheme, host, url.PathEscape(providerKey))
}

func (h *PortalAuthHandler) exchangePortalOAuthCode(ctx context.Context, c *gin.Context, providerKey string, provider settings.OAuthProviderSettings, code string) (portalOAuthTokenResponse, error) {
	if providerKey == "wecom" {
		return h.exchangeWeComOAuthCode(ctx, provider, code)
	}
	form := url.Values{}
	form.Set("grant_type", "authorization_code")
	form.Set("code", code)
	form.Set("client_id", strings.TrimSpace(provider.ClientID))
	form.Set("client_secret", strings.TrimSpace(provider.ClientSecret))
	form.Set("redirect_uri", portalOAuthRedirectURI(c, providerKey, provider))

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, strings.TrimSpace(provider.TokenURL), strings.NewReader(form.Encode()))
	if err != nil {
		return portalOAuthTokenResponse{}, err
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := (&http.Client{Timeout: 15 * time.Second}).Do(req)
	if err != nil {
		return portalOAuthTokenResponse{}, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return portalOAuthTokenResponse{}, err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return portalOAuthTokenResponse{}, fmt.Errorf("token endpoint returned %d: %s", resp.StatusCode, string(body))
	}

	var tokenResponse portalOAuthTokenResponse
	contentType := strings.ToLower(resp.Header.Get("Content-Type"))
	if strings.Contains(contentType, "application/json") || bytes.HasPrefix(bytes.TrimSpace(body), []byte("{")) {
		if err := json.Unmarshal(body, &tokenResponse); err != nil {
			return portalOAuthTokenResponse{}, err
		}
	} else {
		values, err := url.ParseQuery(string(body))
		if err != nil {
			return portalOAuthTokenResponse{}, err
		}
		tokenResponse.AccessToken = values.Get("access_token")
		tokenResponse.TokenType = values.Get("token_type")
		tokenResponse.Error = values.Get("error")
		tokenResponse.Description = values.Get("error_description")
	}
	if tokenResponse.Error != "" {
		return portalOAuthTokenResponse{}, fmt.Errorf("%s: %s", tokenResponse.Error, tokenResponse.Description)
	}
	if strings.TrimSpace(tokenResponse.AccessToken) == "" {
		return portalOAuthTokenResponse{}, fmt.Errorf("missing access token")
	}
	return tokenResponse, nil
}

func (h *PortalAuthHandler) fetchPortalOAuthUserInfo(ctx context.Context, providerKey string, provider settings.OAuthProviderSettings, tokenResponse portalOAuthTokenResponse) (portalOAuthUserInfo, error) {
	accessToken := strings.TrimSpace(tokenResponse.AccessToken)
	if providerKey == "wecom" {
		return fetchWeComOAuthUserInfo(ctx, accessToken, tokenResponse.TokenType, tokenResponse.IDToken, provider.UserInfoURL)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, strings.TrimSpace(provider.UserInfoURL), nil)
	if err != nil {
		return portalOAuthUserInfo{}, err
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Authorization", "Bearer "+accessToken)

	resp, err := (&http.Client{Timeout: 15 * time.Second}).Do(req)
	if err != nil {
		return portalOAuthUserInfo{}, err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return portalOAuthUserInfo{}, err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return portalOAuthUserInfo{}, fmt.Errorf("userinfo endpoint returned %d: %s", resp.StatusCode, string(body))
	}

	var raw map[string]any
	if err := json.Unmarshal(body, &raw); err != nil {
		return portalOAuthUserInfo{}, err
	}
	info := parsePortalOAuthUserInfo(raw)
	if providerKey == "github" && info.Email == "" {
		if email, verified := fetchGitHubPrimaryEmail(ctx, accessToken); email != "" {
			info.Email = email
			info.EmailVerified = verified
		}
	}
	if info.ExternalID == "" {
		return portalOAuthUserInfo{}, fmt.Errorf("provider did not return a stable user id")
	}
	if info.Email == "" {
		return portalOAuthUserInfo{}, fmt.Errorf("provider did not return an email")
	}
	return info, nil
}

func (h *PortalAuthHandler) exchangeWeComOAuthCode(ctx context.Context, provider settings.OAuthProviderSettings, code string) (portalOAuthTokenResponse, error) {
	tokenURL, err := url.Parse(strings.TrimSpace(provider.TokenURL))
	if err != nil {
		return portalOAuthTokenResponse{}, err
	}
	query := tokenURL.Query()
	query.Set("corpid", strings.TrimSpace(provider.CorpID))
	query.Set("corpsecret", strings.TrimSpace(provider.ClientSecret))
	tokenURL.RawQuery = query.Encode()

	body, err := readPortalOAuthJSON(ctx, tokenURL.String())
	if err != nil {
		return portalOAuthTokenResponse{}, err
	}
	if err := weComAPIError(body); err != nil {
		return portalOAuthTokenResponse{}, fmt.Errorf("wecom gettoken failed: %w", err)
	}
	accessToken := firstString(body, "access_token")
	if accessToken == "" {
		return portalOAuthTokenResponse{}, fmt.Errorf("missing access token")
	}

	userInfoURL := "https://qyapi.weixin.qq.com/cgi-bin/auth/getuserinfo"
	userInfoQuery := url.Values{}
	userInfoQuery.Set("access_token", accessToken)
	userInfoQuery.Set("code", strings.TrimSpace(code))
	userInfo, err := readPortalOAuthJSON(ctx, userInfoURL+"?"+userInfoQuery.Encode())
	if err != nil {
		return portalOAuthTokenResponse{}, err
	}
	if err := weComAPIError(userInfo); err != nil {
		return portalOAuthTokenResponse{}, fmt.Errorf("wecom getuserinfo failed: %w", err)
	}
	userID := firstString(userInfo, "UserId", "userid", "OpenId", "openid")
	if userID == "" {
		return portalOAuthTokenResponse{}, fmt.Errorf("missing wecom user identity")
	}
	tokenResponse := portalOAuthTokenResponse{
		AccessToken: userID,
		TokenType:   firstString(userInfo, "user_ticket"),
		IDToken:     accessToken,
	}
	return tokenResponse, nil
}

func fetchWeComOAuthUserInfo(ctx context.Context, userID, userTicket, corpAccessToken, userInfoURL string) (portalOAuthUserInfo, error) {
	userID = strings.TrimSpace(userID)
	if userID == "" {
		return portalOAuthUserInfo{}, fmt.Errorf("missing wecom user identity")
	}
	if strings.TrimSpace(userInfoURL) != "" && strings.TrimSpace(userTicket) != "" && strings.TrimSpace(corpAccessToken) != "" {
		detailURL, err := url.Parse(strings.TrimSpace(userInfoURL))
		if err != nil {
			return portalOAuthUserInfo{}, err
		}
		query := detailURL.Query()
		if query.Get("access_token") == "" {
			query.Set("access_token", strings.TrimSpace(corpAccessToken))
		}
		detailURL.RawQuery = query.Encode()
		body, err := postPortalOAuthJSON(ctx, detailURL.String(), strings.NewReader(`{"user_ticket":`+strconv.Quote(strings.TrimSpace(userTicket))+`}`))
		if err == nil && weComAPIError(body) == nil {
			info := parsePortalOAuthUserInfo(body)
			info.EmailVerified = true
			if info.ExternalID != "" && info.Email != "" {
				return info, nil
			}
		}
	}
	emailUser := strings.NewReplacer("@", "_", "/", "_").Replace(userID)
	return portalOAuthUserInfo{
		ExternalID:    userID,
		Email:         normalizePortalEmail(emailUser + "@wecom.local"),
		EmailVerified: true,
		Username:      emailUser,
		DisplayName:   emailUser,
	}, nil
}

func readPortalOAuthJSON(ctx context.Context, requestURL string) (map[string]any, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, requestURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/json")
	resp, err := portalOAuthHTTPClient(ctx).Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("endpoint returned %d", resp.StatusCode)
	}
	var raw map[string]any
	if err := json.NewDecoder(io.LimitReader(resp.Body, 1<<20)).Decode(&raw); err != nil {
		return nil, err
	}
	return raw, nil
}

func postPortalOAuthJSON(ctx context.Context, requestURL string, body io.Reader) (map[string]any, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, requestURL, body)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Content-Type", "application/json")
	resp, err := portalOAuthHTTPClient(ctx).Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("endpoint returned %d", resp.StatusCode)
	}
	var raw map[string]any
	if err := json.NewDecoder(io.LimitReader(resp.Body, 1<<20)).Decode(&raw); err != nil {
		return nil, err
	}
	return raw, nil
}

func portalOAuthHTTPClient(ctx context.Context) *http.Client {
	if client, ok := ctx.Value(portalOAuthHTTPClientKey{}).(*http.Client); ok && client != nil {
		return client
	}
	return &http.Client{Timeout: 15 * time.Second}
}

func weComAPIError(raw map[string]any) error {
	code, ok := raw["errcode"]
	if !ok || fmt.Sprint(code) == "0" {
		return nil
	}
	return fmt.Errorf("wecom api error %v: %s", code, firstString(raw, "errmsg"))
}

func parsePortalOAuthUserInfo(raw map[string]any) portalOAuthUserInfo {
	info := portalOAuthUserInfo{
		ExternalID:  firstString(raw, "sub", "id", "userid", "UserId", "openid", "OpenId", "unionid"),
		Email:       normalizePortalEmail(firstString(raw, "email", "mail")),
		Username:    firstString(raw, "preferred_username", "login", "username", "userid", "UserId", "nickname", "name"),
		DisplayName: firstString(raw, "name", "display_name", "nickname", "login", "username"),
		AvatarURL:   firstString(raw, "picture", "avatar_url", "avatar"),
	}
	if verified, ok := firstBool(raw, "email_verified", "verified"); ok {
		info.EmailVerified = verified
	}
	return info
}

func fetchGitHubPrimaryEmail(ctx context.Context, accessToken string) (string, bool) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "https://api.github.com/user/emails", nil)
	if err != nil {
		return "", false
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("Authorization", "Bearer "+accessToken)
	resp, err := (&http.Client{Timeout: 15 * time.Second}).Do(req)
	if err != nil {
		return "", false
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", false
	}
	var emails []map[string]any
	if err := json.NewDecoder(io.LimitReader(resp.Body, 1<<20)).Decode(&emails); err != nil {
		return "", false
	}
	for _, item := range emails {
		if primary, _ := firstBool(item, "primary"); primary {
			return normalizePortalEmail(firstString(item, "email")), boolValue(item["verified"])
		}
	}
	return "", false
}

func (h *PortalAuthHandler) resolvePortalOAuthUser(ctx context.Context, authSettings settings.AuthSettings, providerKey string, info portalOAuthUserInfo) (*repository.User, error) {
	if authSettings.OAuth.RequireVerifiedEmail && !info.EmailVerified {
		return nil, fmt.Errorf("第三方账号邮箱未验证")
	}

	user, err := h.userRepo.GetByEmail(ctx, info.Email)
	if err == nil {
		if !authSettings.OAuth.AllowAccountLinking {
			return nil, fmt.Errorf("该邮箱已存在账号，当前未允许账号绑定")
		}
		if info.EmailVerified && !user.EmailVerified {
			now := time.Now().UTC()
			user.EmailVerified = true
			user.EmailVerifiedAt = &now
			_ = h.userRepo.Update(ctx, user)
		}
		return user, nil
	}
	if !pkgerrors.IsNotFound(err) {
		return nil, err
	}
	if !authSettings.OAuth.AllowRegistration {
		return nil, fmt.Errorf("当前未允许第三方账号自动注册")
	}

	password, err := randomPortalOAuthState()
	if err != nil {
		return nil, err
	}
	hash, err := h.authService.HashPassword(password)
	if err != nil {
		return nil, err
	}
	now := time.Now().UTC()
	role := strings.TrimSpace(authSettings.OAuth.DefaultRole)
	if role == "" {
		role = "user"
	}
	user = &repository.User{
		Username:        h.uniquePortalOAuthUsername(ctx, info),
		PasswordHash:    hash,
		Email:           info.Email,
		Role:            role,
		Enabled:         true,
		EmailVerified:   info.EmailVerified,
		EmailVerifiedAt: nil,
		DisplayName:     info.DisplayName,
		AvatarURL:       info.AvatarURL,
	}
	if info.EmailVerified {
		user.EmailVerifiedAt = &now
	}
	if err := h.userRepo.Create(ctx, user); err != nil {
		return nil, err
	}
	return user, nil
}

func (h *PortalAuthHandler) uniquePortalOAuthUsername(ctx context.Context, info portalOAuthUserInfo) string {
	base := sanitizePortalOAuthUsername(info.Username)
	if base == "" && info.Email != "" {
		base = sanitizePortalOAuthUsername(strings.Split(info.Email, "@")[0])
	}
	if base == "" {
		base = "oauth_user"
	}
	if len(base) > 42 {
		base = base[:42]
	}
	for i := 0; i < 100; i++ {
		candidate := base
		if i > 0 {
			candidate = fmt.Sprintf("%s_%d", base, i)
		}
		if _, err := h.userRepo.GetByUsername(ctx, candidate); pkgerrors.IsNotFound(err) {
			return candidate
		}
	}
	return fmt.Sprintf("%s_%d", base, time.Now().Unix())
}

func sanitizePortalOAuthUsername(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	re := regexp.MustCompile(`[^a-z0-9_.-]+`)
	value = strings.Trim(re.ReplaceAllString(value, "_"), "_.-")
	if len(value) > 50 {
		value = value[:50]
	}
	if len(value) < 3 {
		return ""
	}
	return value
}

func firstString(raw map[string]any, keys ...string) string {
	for _, key := range keys {
		switch value := raw[key].(type) {
		case string:
			if strings.TrimSpace(value) != "" {
				return strings.TrimSpace(value)
			}
		case float64:
			return fmt.Sprintf("%.0f", value)
		case json.Number:
			return value.String()
		}
	}
	return ""
}

func firstBool(raw map[string]any, keys ...string) (bool, bool) {
	for _, key := range keys {
		if value, ok := raw[key]; ok {
			return boolValue(value), true
		}
	}
	return false, false
}

func boolValue(value any) bool {
	switch typed := value.(type) {
	case bool:
		return typed
	case string:
		return strings.EqualFold(typed, "true") || typed == "1"
	case float64:
		return typed != 0
	default:
		return false
	}
}

func safePortalOAuthRedirect(value string) string {
	value = strings.TrimSpace(value)
	if value == "" || !strings.HasPrefix(value, "/") || strings.HasPrefix(value, "//") {
		return "/user/dashboard"
	}
	return value
}

func portalOAuthLoginErrorURL(message string) string {
	values := url.Values{}
	values.Set("oauth_error", message)
	return "/user/login?" + values.Encode()
}

func isPortalSecureRequest(c *gin.Context) bool {
	return c.Request.TLS != nil || strings.EqualFold(c.GetHeader("X-Forwarded-Proto"), "https")
}
