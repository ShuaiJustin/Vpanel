# 错误处理使用指南

## 概述

本项目使用统一的错误处理机制，确保所有 API 返回一致的错误响应格式。

## 错误响应格式

所有错误响应遵循以下格式：

```json
{
  "code": "ERROR_CODE",
  "message": "用户友好的错误消息",
  "details": {},
  "request_id": "uuid",
  "timestamp": "2024-01-01T00:00:00Z"
}
```

## 在 Handler 中使用

### 1. 使用辅助函数（推荐）

```go
import "v/internal/api/middleware"

func (h *Handler) GetUser(c *gin.Context) {
    id, err := strconv.ParseInt(c.Param("id"), 10, 64)
    if err != nil {
        middleware.HandleBadRequest(c, "无效的用户ID")
        return
    }
    
    user, err := h.userRepo.GetByID(c.Request.Context(), id)
    if err != nil {
        middleware.HandleNotFound(c, "user", id)
        return
    }
    
    middleware.Success(c, user)
}
```

### 2. 使用错误包创建错误

```go
import (
    "v/internal/api/middleware"
    "v/pkg/errors"
)

func (h *Handler) CreateUser(c *gin.Context) {
    var req CreateUserRequest
    if err := c.ShouldBindJSON(&req); err != nil {
        middleware.HandleValidationError(c, "请求参数错误", extractValidationErrors(err))
        return
    }
    
    // 检查用户名是否已存在
    existing, _ := h.userRepo.GetByUsername(c.Request.Context(), req.Username)
    if existing != nil {
        middleware.HandleConflict(c, "user", "username", req.Username)
        return
    }
    
    user, err := h.userService.Create(c.Request.Context(), &req)
    if err != nil {
        // 使用友好的错误消息
        middleware.HandleInternalError(c, errors.MsgUserCreateFailed, err)
        return
    }
    
    middleware.Created(c, user)
}
```

### 3. 使用预定义的错误消息

```go
import (
    "v/internal/api/middleware"
    "v/pkg/errors"
)

func (h *Handler) Login(c *gin.Context) {
    var req LoginRequest
    if err := c.ShouldBindJSON(&req); err != nil {
        middleware.HandleBadRequest(c, errors.MsgInvalidRequest)
        return
    }
    
    user, err := h.authService.Authenticate(c.Request.Context(), req.Username, req.Password)
    if err != nil {
        // 使用预定义的友好消息
        middleware.HandleError(c, errors.NewInvalidCredentialsError())
        return
    }
    
    middleware.Success(c, user)
}
```

## 可用的辅助函数

### 错误处理函数

- `HandleError(c, err)` - 处理任意错误
- `HandleValidationError(c, message, fields)` - 处理验证错误
- `HandleNotFound(c, entity, id)` - 处理资源未找到
- `HandleUnauthorized(c, message)` - 处理未授权
- `HandleForbidden(c, message)` - 处理禁止访问
- `HandleBadRequest(c, message)` - 处理错误请求
- `HandleConflict(c, entity, field, value)` - 处理冲突
- `HandleInternalError(c, message, cause)` - 处理内部错误

### 成功响应函数

- `Success(c, data)` - 返回成功响应
- `SuccessWithMessage(c, message, data)` - 返回带消息的成功响应
- `Created(c, data)` - 返回创建成功响应（201）
- `NoContent(c)` - 返回无内容响应（204）

## 预定义的错误消息

在 `pkg/errors/messages.go` 中定义了大量用户友好的错误消息常量：

```go
// 用户相关
errors.MsgUserNotFound
errors.MsgUserAlreadyExists
errors.MsgInvalidCredentials
errors.MsgUserDisabled

// 代理相关
errors.MsgProxyNotFound
errors.MsgProxyPortInUse
errors.MsgProxyCreateFailed

// 节点相关
errors.MsgNodeNotFound
errors.MsgNodeConnectionFailed
errors.MsgNodeDeployFailed

// 证书相关
errors.MsgCertNotFound
errors.MsgCertApplyFailed
errors.MsgCertExpired

// 订单和支付
errors.MsgOrderNotFound
errors.MsgPaymentFailed
errors.MsgInsufficientBalance

// 认证相关
errors.MsgTokenExpired
errors.MsgTokenInvalid
errors.Msg2FARequired
```

## 创建自定义错误

```go
import "v/pkg/errors"

// 方式1：使用构造函数
err := errors.NewUserNotFoundError(userID)

// 方式2：使用 New 创建
err := errors.New(errors.ErrCodeNotFound, "自定义错误消息")

// 方式3：包装现有错误
err := errors.Wrap(originalErr, errors.ErrCodeDatabase, "数据库操作失败")

// 添加上下文信息
err = err.WithOperation("CreateUser").WithEntity("user", userID)
```

## 错误码

系统定义了以下错误码：

- `VALIDATION_ERROR` - 验证错误（400）
- `NOT_FOUND` - 资源未找到（404）
- `UNAUTHORIZED` - 未授权（401）
- `FORBIDDEN` - 禁止访问（403）
- `CONFLICT` - 资源冲突（409）
- `BAD_REQUEST` - 错误请求（400）
- `RATE_LIMIT_EXCEEDED` - 请求过于频繁（429）
- `INTERNAL_ERROR` - 内部错误（500）
- `DATABASE_ERROR` - 数据库错误（500）
- `XRAY_ERROR` - Xray 错误（500）

## 最佳实践

### ✅ 推荐做法

```go
// 1. 使用预定义的错误消息
middleware.HandleError(c, errors.NewUserNotFoundError(id))

// 2. 使用辅助函数
middleware.HandleNotFound(c, "user", id)

// 3. 提供清晰的验证错误
fields := map[string]string{
    "username": "用户名长度必须在3-20个字符之间",
    "email": "邮箱格式无效",
}
middleware.HandleValidationError(c, "请求参数错误", fields)

// 4. 记录详细的错误日志
h.logger.Error("failed to create user",
    logger.F("username", req.Username),
    logger.Err(err))
middleware.HandleInternalError(c, errors.MsgUserCreateFailed, err)
```

### ❌ 避免的做法

```go
// 1. 不要直接使用 gin.H
c.JSON(http.StatusBadRequest, gin.H{"error": "错误"})

// 2. 不要暴露技术细节给用户
c.JSON(http.StatusInternalServerError, gin.H{
    "error": "sql: no rows in result set",
})

// 3. 不要使用不一致的响应格式
c.JSON(http.StatusOK, gin.H{
    "code": 0,
    "msg": "success",
    "data": data,
})
```

## 迁移现有代码

将现有的错误处理代码迁移到新机制：

### 迁移前

```go
func (h *Handler) GetUser(c *gin.Context) {
    id, err := strconv.ParseInt(c.Param("id"), 10, 64)
    if err != nil {
        c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid ID"})
        return
    }
    
    user, err := h.userRepo.GetByID(c.Request.Context(), id)
    if err != nil {
        c.JSON(http.StatusNotFound, gin.H{"error": "User not found"})
        return
    }
    
    c.JSON(http.StatusOK, user)
}
```

### 迁移后

```go
func (h *Handler) GetUser(c *gin.Context) {
    id, err := strconv.ParseInt(c.Param("id"), 10, 64)
    if err != nil {
        middleware.HandleBadRequest(c, "无效的用户ID")
        return
    }
    
    user, err := h.userRepo.GetByID(c.Request.Context(), id)
    if err != nil {
        middleware.HandleNotFound(c, "user", id)
        return
    }
    
    middleware.Success(c, user)
}
```

## 测试

```go
func TestGetUser_NotFound(t *testing.T) {
    // ... setup ...
    
    w := httptest.NewRecorder()
    req, _ := http.NewRequest("GET", "/api/users/999", nil)
    router.ServeHTTP(w, req)
    
    assert.Equal(t, http.StatusNotFound, w.Code)
    
    var response errors.ErrorResponse
    json.Unmarshal(w.Body.Bytes(), &response)
    
    assert.Equal(t, "NOT_FOUND", response.Code)
    assert.Equal(t, "用户不存在", response.Message)
    assert.NotEmpty(t, response.RequestID)
    assert.NotEmpty(t, response.Timestamp)
}
```
