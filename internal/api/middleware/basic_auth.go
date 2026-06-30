package middleware

import (
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"v/internal/logger"
	"v/internal/settings"
)

const basicAuthCookieName = "vpanel_basic_auth"
const basicAuthCookieMaxAge = 12 * time.Hour

// BasicAuthGate adds an optional HTTP Basic Auth gate in front of the panel.
func BasicAuthGate(settingsService *settings.Service, log logger.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		if shouldSkipBasicAuth(c.Request) {
			c.Next()
			return
		}
		if settingsService == nil {
			c.Next()
			return
		}

		systemSettings, err := settingsService.GetSystemSettings(c.Request.Context())
		if err != nil {
			if log != nil {
				log.Error("failed to load basic auth settings", logger.F("error", err))
			}
			c.JSON(http.StatusInternalServerError, gin.H{
				"success": false,
				"error": gin.H{
					"code":    "SETTINGS_UNAVAILABLE",
					"message": "failed to load authentication settings",
				},
			})
			c.Abort()
			return
		}

		basic := systemSettings.Auth.BasicAuth
		if !basic.Enabled {
			clearBasicAuthCookie(c)
			c.Next()
			return
		}

		username := strings.TrimSpace(basic.Username)
		password := strings.TrimSpace(basic.Password)
		if username == "" || password == "" {
			if log != nil {
				log.Error("basic authentication is enabled without complete credentials")
			}
			c.JSON(http.StatusServiceUnavailable, gin.H{
				"success": false,
				"error": gin.H{
					"code":    "BASIC_AUTH_MISCONFIGURED",
					"message": "basic authentication is not fully configured",
				},
			})
			c.Abort()
			return
		}

		if isValidBasicAuthCookie(c, basic) {
			c.Next()
			return
		}

		requestUsername, requestPassword, ok := c.Request.BasicAuth()
		if !ok || !constantTimeEqual(requestUsername, username) || !constantTimeEqual(requestPassword, password) {
			realm := strings.TrimSpace(basic.Realm)
			if realm == "" {
				realm = "V Panel"
			}
			c.Header("WWW-Authenticate", `Basic realm="`+escapeBasicAuthRealm(realm)+`"`)
			c.JSON(http.StatusUnauthorized, gin.H{
				"success": false,
				"error": gin.H{
					"code":    "BASIC_AUTH_REQUIRED",
					"message": "basic authentication required",
				},
			})
			c.Abort()
			return
		}

		setBasicAuthCookie(c, basic)
		c.Next()
	}
}

func shouldSkipBasicAuth(req *http.Request) bool {
	if req == nil {
		return true
	}
	if req.Method == http.MethodOptions {
		return true
	}

	path := req.URL.EscapedPath()
	if path == "" {
		path = req.URL.Path
	}
	path = strings.TrimRight(path, "/")
	if path == "" {
		path = "/"
	}

	if path == "/health" || path == "/ready" || strings.HasSuffix(path, "/health") || strings.HasSuffix(path, "/ready") {
		return true
	}

	for _, marker := range []string{
		"/api/node/",
		"/api/subscription/",
		"/api/payments/callback/",
		"/api/admin/nodes/agent/download",
	} {
		if strings.Contains(path, marker) {
			return true
		}
	}

	return hasPublicShortSubscriptionPath(path)
}

func hasPublicShortSubscriptionPath(path string) bool {
	parts := strings.Split(strings.Trim(path, "/"), "/")
	for i := 0; i < len(parts)-1; i++ {
		if parts[i] == "s" && parts[i+1] != "" {
			return true
		}
	}
	return false
}

func constantTimeEqual(left, right string) bool {
	leftHash := sha256.Sum256([]byte(left))
	rightHash := sha256.Sum256([]byte(right))
	return subtle.ConstantTimeCompare(leftHash[:], rightHash[:]) == 1
}

func isValidBasicAuthCookie(c *gin.Context, basic settings.BasicAuthSettings) bool {
	cookieValue, err := c.Cookie(basicAuthCookieName)
	if err != nil || cookieValue == "" {
		return false
	}
	return constantTimeEqual(cookieValue, basicAuthCookieValue(basic))
}

func setBasicAuthCookie(c *gin.Context, basic settings.BasicAuthSettings) {
	c.SetSameSite(http.SameSiteLaxMode)
	c.SetCookie(
		basicAuthCookieName,
		basicAuthCookieValue(basic),
		int(basicAuthCookieMaxAge.Seconds()),
		"/",
		"",
		isSecureRequest(c),
		true,
	)
}

func clearBasicAuthCookie(c *gin.Context) {
	if _, err := c.Cookie(basicAuthCookieName); err != nil {
		return
	}
	c.SetSameSite(http.SameSiteLaxMode)
	c.SetCookie(basicAuthCookieName, "", -1, "/", "", isSecureRequest(c), true)
}

func basicAuthCookieValue(basic settings.BasicAuthSettings) string {
	sum := sha256.Sum256([]byte(strings.TrimSpace(basic.Username) + "\x00" + strings.TrimSpace(basic.Password) + "\x00" + strings.TrimSpace(basic.Realm)))
	return hex.EncodeToString(sum[:])
}

func isSecureRequest(c *gin.Context) bool {
	if c == nil || c.Request == nil {
		return false
	}
	return c.Request.TLS != nil || strings.EqualFold(c.GetHeader("X-Forwarded-Proto"), "https")
}

func escapeBasicAuthRealm(realm string) string {
	replacer := strings.NewReplacer(`\`, `\\`, `"`, `\"`)
	return replacer.Replace(realm)
}
