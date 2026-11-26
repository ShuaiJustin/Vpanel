package handlers

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"

	"v/internal/database/repository"
	"v/internal/settings"
)

func TestPortalOAuthWeComProviderReady(t *testing.T) {
	provider := settings.OAuthProviderSettings{
		Enabled:      true,
		ClientSecret: "secret",
		AuthorizeURL: "https://login.work.weixin.qq.com/wwlogin/sso/login",
		TokenURL:     "https://qyapi.weixin.qq.com/cgi-bin/gettoken",
		CorpID:       "corp-id",
		AgentID:      "1000022",
	}

	if !isPortalOAuthProviderLoginReady("wecom", provider) {
		t.Fatal("expected configured wecom provider to be login-ready")
	}
	if label := portalOAuthProviderLabel("wecom"); label != "企业微信" {
		t.Fatalf("expected wecom label, got %q", label)
	}
}

func TestBuildPortalOAuthWeComAuthorizeURL(t *testing.T) {
	gin.SetMode(gin.TestMode)
	req := httptest.NewRequest(http.MethodGet, "https://panel.example.com/api/portal/auth/oauth/wecom/start", nil)
	w := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(w)
	ctx.Request = req

	provider := settings.OAuthProviderSettings{
		AuthorizeURL: "https://login.work.weixin.qq.com/wwlogin/sso/login",
		CorpID:       "corp-id",
		AgentID:      "1000022",
		Scopes:       []string{"snsapi_privateinfo"},
	}
	got, err := buildPortalOAuthAuthorizeURL(ctx, "wecom", provider, "state-token")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	parsed, err := url.Parse(got)
	if err != nil {
		t.Fatalf("invalid url: %v", err)
	}
	query := parsed.Query()
	if query.Get("login_type") != "CorpApp" {
		t.Fatalf("expected CorpApp login type, got %q", query.Get("login_type"))
	}
	if query.Get("appid") != "corp-id" {
		t.Fatalf("expected corp id appid, got %q", query.Get("appid"))
	}
	if query.Get("agentid") != "1000022" {
		t.Fatalf("expected agent id, got %q", query.Get("agentid"))
	}
	if query.Get("redirect_uri") != "https://panel.example.com/api/portal/auth/oauth/wecom/callback" {
		t.Fatalf("unexpected redirect uri: %q", query.Get("redirect_uri"))
	}
	if query.Get("scope") != "snsapi_privateinfo" {
		t.Fatalf("expected privateinfo scope, got %q", query.Get("scope"))
	}
}

func TestFetchWeComOAuthUserInfoUsesStableUserIDFallback(t *testing.T) {
	info, err := fetchWeComOAuthUserInfo(context.Background(), "zhangsan", "", "", "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if info.ExternalID != "zhangsan" {
		t.Fatalf("expected stable user id, got %q", info.ExternalID)
	}
	if info.Email != "zhangsan@wecom.local" {
		t.Fatalf("expected synthetic wecom email, got %q", info.Email)
	}
	if !info.EmailVerified {
		t.Fatal("expected wecom fallback identity to be treated as verified")
	}
}

func TestFetchWeComOAuthUserInfoRejectsMissingUserID(t *testing.T) {
	if _, err := fetchWeComOAuthUserInfo(context.Background(), "", "ticket", "token", ""); err == nil {
		t.Fatal("expected missing user id to be rejected")
	}
}

func TestBuildPortalOAuthWeComEmbedConfigExcludesSecret(t *testing.T) {
	gin.SetMode(gin.TestMode)
	req := httptest.NewRequest(http.MethodGet, "https://panel.example.com/api/portal/auth/oauth/wecom/embed", nil)
	w := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(w)
	ctx.Request = req
	provider := settings.OAuthProviderSettings{
		CorpID:       "corp-id",
		AgentID:      "1000022",
		ClientSecret: "must-not-leak",
	}

	config := buildPortalOAuthEmbedConfig(ctx, "wecom", provider, "state-token", "https://login.example.com")
	if config["appid"] != "corp-id" || config["agentid"] != "1000022" {
		t.Fatalf("unexpected public config: %#v", config)
	}
	if config["redirect_uri"] != "https://panel.example.com/api/portal/auth/oauth/wecom/callback" {
		t.Fatalf("unexpected redirect uri: %q", config["redirect_uri"])
	}
	if _, exists := config["client_secret"]; exists {
		t.Fatal("embed config must not expose client secret")
	}
}

func TestPortalOAuthSecurityHardening(t *testing.T) {
	if got := safePortalOAuthRedirect("/\\evil.com"); got != "/user/dashboard" {
		t.Fatalf("expected backslash open-redirect blocked, got %q", got)
	}
	if got := safePortalOAuthRedirect("//evil.com"); got != "/user/dashboard" {
		t.Fatalf("expected protocol-relative redirect blocked, got %q", got)
	}

	h := &PortalAuthHandler{}
	signed, err := h.encodeSignedOAuthState(portalOAuthState{
		State:    "nonce-1",
		Provider: "github",
		Redirect: "/user/dashboard",
		Expires:  9999999999,
	})
	if err != nil {
		t.Fatalf("encodeSignedOAuthState failed: %v", err)
	}
	if _, err := h.decodeSignedOAuthState(signed); err != nil {
		t.Fatalf("decodeSignedOAuthState failed for valid token: %v", err)
	}
	if _, err := h.decodeSignedOAuthState(signed + "tamper"); err == nil {
		t.Fatal("expected tampered signed oauth state to be rejected")
	}

	repo := newPortalMockUserRepo()
	_ = repo.Create(context.Background(), &repository.User{
		Username: "rootadmin",
		Email:    "admin@example.com",
		Role:     "admin",
		Enabled:  true,
	})
	h.userRepo = repo
	_, linkErr := h.resolvePortalOAuthUser(context.Background(), settings.AuthSettings{
		OAuth: settings.OAuthSettings{
			AllowAccountLinking:  true,
			RequireVerifiedEmail: true,
		},
	}, "github", portalOAuthUserInfo{
		ExternalID:    "gh-1",
		Email:         "admin@example.com",
		EmailVerified: true,
	})
	if linkErr == nil {
		t.Fatal("expected OAuth auto-linking to admin account to be rejected")
	}
}

func TestFetchPortalOAuthGitHubPublicEmailVerifiedViaEmailsAPI(t *testing.T) {
	var seenUA string
	client := &http.Client{
		Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			seenUA = req.Header.Get("User-Agent")
			switch req.URL.String() {
			case "https://api.github.com/user":
				return &http.Response{
					StatusCode: http.StatusOK,
					Header:     make(http.Header),
					Body:       io.NopCloser(strings.NewReader(`{"id":42,"login":"octocat","email":"octocat@example.com"}`)),
				}, nil
			case "https://api.github.com/user/emails":
				return &http.Response{
					StatusCode: http.StatusOK,
					Header:     make(http.Header),
					Body:       io.NopCloser(strings.NewReader(`[{"email":"octocat@example.com","primary":true,"verified":true}]`)),
				}, nil
			default:
				return &http.Response{
					StatusCode: http.StatusNotFound,
					Header:     make(http.Header),
					Body:       io.NopCloser(strings.NewReader(`{}`)),
				}, nil
			}
		}),
	}

	ctx := context.WithValue(context.Background(), portalOAuthHTTPClientKey{}, client)
	h := &PortalAuthHandler{}
	info, err := h.fetchPortalOAuthUserInfo(ctx, "github", settings.OAuthProviderSettings{
		UserInfoURL: "https://api.github.com/user",
	}, portalOAuthTokenResponse{AccessToken: "test-access-token"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if info.Email != "octocat@example.com" || !info.EmailVerified {
		t.Fatalf("expected verified github public email, got email=%q verified=%v", info.Email, info.EmailVerified)
	}
	if seenUA == "" {
		t.Fatal("expected User-Agent header on GitHub API request")
	}

	gin.SetMode(gin.TestMode)
	req := httptest.NewRequest(http.MethodGet, "https://panel.example.com/vpanel/api/portal/auth/oauth/github/start", nil)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = req
	if got := portalOAuthRedirectURI(c, "github", settings.OAuthProviderSettings{}); got != "https://panel.example.com/vpanel/api/portal/auth/oauth/github/callback" {
		t.Fatalf("expected sub-path redirect uri, got %q", got)
	}
}
