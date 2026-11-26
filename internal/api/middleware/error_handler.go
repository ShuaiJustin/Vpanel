// Package middleware provides HTTP middleware for the API.
package middleware

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"v/internal/logger"
	"v/pkg/errors"
)

// ErrorHandler 统一错误处理中间件
// 捕获 panic 并转换为标准错误响应
func ErrorHandler(log logger.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		defer func() {
			if err := recover(); err != nil {
				log.Error("panic recovered",
					logger.F("error", err),
					logger.F("path", c.Request.URL.Path),
					logger.F("method", c.Request.Method))
				
				requestID := c.GetString("request_id")
				response := errors.ToErrorResponse(
					errors.NewInternalError("服务器内部错误", nil),
					requestID,
				)
				
				c.JSON(http.StatusInternalServerError, response)
				c.Abort()
			}
		}()
		
		c.Next()
		
		// 检查是否有错误被设置
		if len(c.Errors) > 0 {
			// 获取最后一个错误
			err := c.Errors.Last().Err
			
			// 转换为 AppError
			var appErr *errors.AppError
			if e, ok := err.(*errors.AppError); ok {
				appErr = e
			} else {
				// 非 AppError，包装为内部错误
				appErr = errors.NewInternalError("操作失败", err)
			}
			
			requestID := c.GetString("request_id")
			response := appErr.ToResponse(requestID)
			
			// 记录错误日志
			if appErr.HTTPStatus() >= 500 {
				log.Error("internal error",
					logger.F("code", appErr.Code),
					logger.F("message", appErr.Message),
					logger.F("path", c.Request.URL.Path),
					logger.F("method", c.Request.Method),
					logger.Err(appErr))
			} else {
				log.Warn("client error",
					logger.F("code", appErr.Code),
					logger.F("message", appErr.Message),
					logger.F("path", c.Request.URL.Path),
					logger.F("method", c.Request.Method))
			}
			
			c.JSON(appErr.HTTPStatus(), response)
			c.Abort()
		}
	}
}

// HandleError 辅助函数：处理错误并返回响应
// 使用方式：middleware.HandleError(c, err)
func HandleError(c *gin.Context, err error) {
	if err == nil {
		return
	}
	
	// 转换为 AppError
	var appErr *errors.AppError
	if e, ok := err.(*errors.AppError); ok {
		appErr = e
	} else {
		// 非 AppError，包装为内部错误
		appErr = errors.NewInternalError("操作失败", err)
	}
	
	requestID := c.GetString("request_id")
	response := appErr.ToResponse(requestID)
	
	c.JSON(appErr.HTTPStatus(), response)
}

// AbortWithError 辅助函数：中止请求并返回错误
func AbortWithError(c *gin.Context, err error) {
	HandleError(c, err)
	c.Abort()
}

// HandleValidationError 辅助函数：处理验证错误
func HandleValidationError(c *gin.Context, message string, fields map[string]string) {
	err := errors.NewValidationErrorWithFields(message, fields)
	HandleError(c, err)
}

// HandleNotFound 辅助函数：处理资源未找到错误
func HandleNotFound(c *gin.Context, entity string, id any) {
	err := errors.NewNotFoundError(entity, id)
	HandleError(c, err)
}

// HandleUnauthorized 辅助函数：处理未授权错误
func HandleUnauthorized(c *gin.Context, message string) {
	if message == "" {
		message = "未授权访问"
	}
	err := errors.NewUnauthorizedError(message)
	HandleError(c, err)
}

// HandleForbidden 辅助函数：处理禁止访问错误
func HandleForbidden(c *gin.Context, message string) {
	if message == "" {
		message = "没有权限执行此操作"
	}
	err := errors.NewForbiddenError(message)
	HandleError(c, err)
}

// HandleBadRequest 辅助函数：处理错误请求
func HandleBadRequest(c *gin.Context, message string) {
	if message == "" {
		message = "请求参数错误"
	}
	err := errors.NewBadRequestError(message)
	HandleError(c, err)
}

// HandleConflict 辅助函数：处理冲突错误
func HandleConflict(c *gin.Context, entity, field string, value any) {
	err := errors.NewConflictError(entity, field, value)
	HandleError(c, err)
}

// HandleInternalError 辅助函数：处理内部错误
func HandleInternalError(c *gin.Context, message string, cause error) {
	if message == "" {
		message = "服务器内部错误"
	}
	err := errors.NewInternalError(message, cause)
	HandleError(c, err)
}

// Success 辅助函数：返回成功响应
func Success(c *gin.Context, data any) {
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    data,
	})
}

// SuccessWithMessage 辅助函数：返回带消息的成功响应
func SuccessWithMessage(c *gin.Context, message string, data any) {
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": message,
		"data":    data,
	})
}

// Created 辅助函数：返回创建成功响应
func Created(c *gin.Context, data any) {
	c.JSON(http.StatusCreated, gin.H{
		"success": true,
		"data":    data,
	})
}

// NoContent 辅助函数：返回无内容响应
func NoContent(c *gin.Context) {
	c.Status(http.StatusNoContent)
}
