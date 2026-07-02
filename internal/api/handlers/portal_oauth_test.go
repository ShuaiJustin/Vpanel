package handlers

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/gin-gonic/gin"

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
