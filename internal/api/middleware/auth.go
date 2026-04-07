// Package middleware provides HTTP middleware for the V Panel API.
package middleware

import (
	"context"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"

	"v/internal/auth"
	"v/internal/database/repository"
	"v/internal/logger"
	"v/pkg/errors"
)

// ContextKey is a type for context keys.
type ContextKey string

const (
	// UserClaimsKey is the context key for user claims.
	UserClaimsKey ContextKey = "user_claims"
)

// AuthMiddlewareHandler provides authentication middleware methods.
type AuthMiddlewareHandler struct {
	authService *auth.Service
	userRepo    repository.UserRepository
	roleRepo    repository.RoleRepository
	logger      logger.Logger
}

// NewAuthMiddleware creates a new authentication middleware handler.
func NewAuthMiddleware(authService *auth.Service, log logger.Logger) *AuthMiddlewareHandler {
	return &AuthMiddlewareHandler{
		authService: authService,
		logger:      log,
	}
}

// WithUserRepository enables runtime user state verification for authenticated requests.
func (h *AuthMiddlewareHandler) WithUserRepository(userRepo repository.UserRepository) *AuthMiddlewareHandler {
	h.userRepo = userRepo
	return h
}

// WithRoleRepository enables permission checks based on the latest role definition.
func (h *AuthMiddlewareHandler) WithRoleRepository(roleRepo repository.RoleRepository) *AuthMiddlewareHandler {
	h.roleRepo = roleRepo
	return h
}

func (h *AuthMiddlewareHandler) enrichClaimsWithCurrentUser(c *gin.Context, claims *auth.Claims) (*auth.Claims, bool) {
	if h == nil || h.userRepo == nil || claims == nil {
		return claims, true
	}

	user, err := h.userRepo.GetByID(c.Request.Context(), claims.UserID)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{
			"success": false,
			"error": gin.H{
				"code":    errors.ErrCodeUnauthorized,
				"message": "user no longer exists",
			},
		})
		c.Abort()
		return nil, false
	}

	if !user.Enabled {
		c.JSON(http.StatusForbidden, gin.H{
			"success": false,
			"error": gin.H{
				"code":    errors.ErrCodeForbidden,
				"message": "user account is disabled",
			},
		})
		c.Abort()
		return nil, false
	}

	claims.Username = user.Username
	claims.Role = user.Role
	return claims, true
}

// Authenticate returns a middleware that validates JWT tokens.
func (h *AuthMiddlewareHandler) Authenticate() gin.HandlerFunc {
	return func(c *gin.Context) {
		// Get token from Authorization header
		authHeader := c.GetHeader("Authorization")
		if authHeader == "" {
			c.JSON(http.StatusUnauthorized, gin.H{
				"success": false,
				"error": gin.H{
					"code":    errors.ErrCodeUnauthorized,
					"message": "missing authorization header",
				},
			})
			c.Abort()
			return
		}

		// Check Bearer prefix
		parts := strings.SplitN(authHeader, " ", 2)
		if len(parts) != 2 || strings.ToLower(parts[0]) != "bearer" {
			c.JSON(http.StatusUnauthorized, gin.H{
				"success": false,
				"error": gin.H{
					"code":    errors.ErrCodeUnauthorized,
					"message": "invalid authorization header format",
				},
			})
			c.Abort()
			return
		}

		token := parts[1]

		// Validate token
		claims, err := h.authService.ValidateToken(token)
		if err != nil {
			c.JSON(http.StatusUnauthorized, gin.H{
				"success": false,
				"error": gin.H{
					"code":    errors.ErrCodeUnauthorized,
					"message": "invalid or expired token",
				},
			})
			c.Abort()
			return
		}

		claims, ok := h.enrichClaimsWithCurrentUser(c, claims)
		if !ok {
			return
		}

		// Store claims in context
		c.Set(string(UserClaimsKey), claims)
		// Also store user_id for backward compatibility with handlers
		c.Set("user_id", claims.UserID)
		c.Set("username", claims.Username)
		c.Set("role", claims.Role)
		c.Next()
	}
}

// RequireRole returns a middleware that requires a specific role.
func (h *AuthMiddlewareHandler) RequireRole(role string) gin.HandlerFunc {
	return func(c *gin.Context) {
		claims, exists := c.Get(string(UserClaimsKey))
		if !exists {
			c.JSON(http.StatusUnauthorized, gin.H{
				"success": false,
				"error": gin.H{
					"code":    errors.ErrCodeUnauthorized,
					"message": "authentication required",
				},
			})
			c.Abort()
			return
		}

		userClaims, ok := claims.(*auth.Claims)
		if !ok {
			c.JSON(http.StatusInternalServerError, gin.H{
				"success": false,
				"error": gin.H{
					"code":    errors.ErrCodeInternal,
					"message": "invalid claims type",
				},
			})
			c.Abort()
			return
		}

		if userClaims.Role != role {
			c.JSON(http.StatusForbidden, gin.H{
				"success": false,
				"error": gin.H{
					"code":    errors.ErrCodeForbidden,
					"message": role + " access required",
				},
			})
			c.Abort()
			return
		}

		c.Next()
	}
}

func (h *AuthMiddlewareHandler) roleHasPermission(ctx context.Context, roleName, permission string) (bool, error) {
	if permission == "" {
		return true, nil
	}
	if roleName == "admin" {
		return true, nil
	}
	if h == nil || h.roleRepo == nil {
		return false, nil
	}

	role, err := h.roleRepo.GetByName(ctx, roleName)
	if err != nil {
		return false, err
	}
	if role == nil {
		return false, nil
	}

	perms, err := role.GetPermissionsList()
	if err != nil {
		return false, err
	}
	for _, perm := range perms {
		if perm == "*" || perm == permission {
			return true, nil
		}
	}
	return false, nil
}

// RequirePermission returns a middleware that requires a specific permission.
func (h *AuthMiddlewareHandler) RequirePermission(permission string) gin.HandlerFunc {
	return func(c *gin.Context) {
		claims, exists := c.Get(string(UserClaimsKey))
		if !exists {
			c.JSON(http.StatusUnauthorized, gin.H{
				"success": false,
				"error": gin.H{
					"code":    errors.ErrCodeUnauthorized,
					"message": "authentication required",
				},
			})
			c.Abort()
			return
		}

		userClaims, ok := claims.(*auth.Claims)
		if !ok {
			c.JSON(http.StatusInternalServerError, gin.H{
				"success": false,
				"error": gin.H{
					"code":    errors.ErrCodeInternal,
					"message": "invalid claims type",
				},
			})
			c.Abort()
			return
		}

		allowed, err := h.roleHasPermission(c.Request.Context(), userClaims.Role, permission)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"success": false,
				"error": gin.H{
					"code":    errors.ErrCodeInternal,
					"message": "failed to evaluate permissions",
				},
			})
			c.Abort()
			return
		}
		if !allowed {
			c.JSON(http.StatusForbidden, gin.H{
				"success": false,
				"error": gin.H{
					"code":    errors.ErrCodeForbidden,
					"message": permission + " permission required",
				},
			})
			c.Abort()
			return
		}

		c.Next()
	}
}

// RequireAllPermissions returns a middleware that requires all listed permissions.
func (h *AuthMiddlewareHandler) RequireAllPermissions(permissions ...string) gin.HandlerFunc {
	return func(c *gin.Context) {
		for _, permission := range permissions {
			if permission == "" {
				continue
			}
			allowed, err := h.roleHasPermission(c.Request.Context(), c.GetString("role"), permission)
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{
					"success": false,
					"error": gin.H{
						"code":    errors.ErrCodeInternal,
						"message": "failed to evaluate permissions",
					},
				})
				c.Abort()
				return
			}
			if !allowed {
				c.JSON(http.StatusForbidden, gin.H{
					"success": false,
					"error": gin.H{
						"code":    errors.ErrCodeForbidden,
						"message": permission + " permission required",
					},
				})
				c.Abort()
				return
			}
		}
		c.Next()
	}
}

// AuthMiddleware creates an authentication middleware.
func AuthMiddleware(authService auth.Service) gin.HandlerFunc {
	return func(c *gin.Context) {
		// Get token from Authorization header
		authHeader := c.GetHeader("Authorization")
		if authHeader == "" {
			c.JSON(http.StatusUnauthorized, gin.H{
				"success": false,
				"error": gin.H{
					"code":    errors.ErrCodeUnauthorized,
					"message": "missing authorization header",
				},
			})
			c.Abort()
			return
		}

		// Check Bearer prefix
		parts := strings.SplitN(authHeader, " ", 2)
		if len(parts) != 2 || strings.ToLower(parts[0]) != "bearer" {
			c.JSON(http.StatusUnauthorized, gin.H{
				"success": false,
				"error": gin.H{
					"code":    errors.ErrCodeUnauthorized,
					"message": "invalid authorization header format",
				},
			})
			c.Abort()
			return
		}

		token := parts[1]

		// Validate token
		claims, err := authService.ValidateToken(token)
		if err != nil {
			c.JSON(http.StatusUnauthorized, gin.H{
				"success": false,
				"error": gin.H{
					"code":    errors.ErrCodeUnauthorized,
					"message": "invalid or expired token",
				},
			})
			c.Abort()
			return
		}

		// Store claims in context
		c.Set(string(UserClaimsKey), claims)
		// Also store user_id for backward compatibility with handlers
		c.Set("user_id", claims.UserID)
		c.Set("username", claims.Username)
		c.Set("role", claims.Role)
		c.Next()
	}
}

// AdminMiddleware creates a middleware that requires admin role.
func AdminMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		claims, exists := c.Get(string(UserClaimsKey))
		if !exists {
			c.JSON(http.StatusUnauthorized, gin.H{
				"success": false,
				"error": gin.H{
					"code":    errors.ErrCodeUnauthorized,
					"message": "authentication required",
				},
			})
			c.Abort()
			return
		}

		userClaims, ok := claims.(*auth.Claims)
		if !ok {
			c.JSON(http.StatusInternalServerError, gin.H{
				"success": false,
				"error": gin.H{
					"code":    errors.ErrCodeInternal,
					"message": "invalid claims type",
				},
			})
			c.Abort()
			return
		}

		if userClaims.Role != "admin" {
			c.JSON(http.StatusForbidden, gin.H{
				"success": false,
				"error": gin.H{
					"code":    errors.ErrCodeForbidden,
					"message": "admin access required",
				},
			})
			c.Abort()
			return
		}

		c.Next()
	}
}

// GetUserClaims retrieves user claims from the context.
func GetUserClaims(c *gin.Context) (*auth.Claims, bool) {
	claims, exists := c.Get(string(UserClaimsKey))
	if !exists {
		return nil, false
	}

	userClaims, ok := claims.(*auth.Claims)
	return userClaims, ok
}

// OptionalAuthMiddleware creates an optional authentication middleware.
// It validates the token if present but doesn't require it.
func OptionalAuthMiddleware(authService auth.Service) gin.HandlerFunc {
	return func(c *gin.Context) {
		authHeader := c.GetHeader("Authorization")
		if authHeader == "" {
			c.Next()
			return
		}

		parts := strings.SplitN(authHeader, " ", 2)
		if len(parts) != 2 || strings.ToLower(parts[0]) != "bearer" {
			c.Next()
			return
		}

		token := parts[1]
		claims, err := authService.ValidateToken(token)
		if err == nil {
			c.Set(string(UserClaimsKey), claims)
			c.Set("user_id", claims.UserID)
			c.Set("userID", claims.UserID)
			c.Set("username", claims.Username)
			c.Set("role", claims.Role)
		}

		c.Next()
	}
}
