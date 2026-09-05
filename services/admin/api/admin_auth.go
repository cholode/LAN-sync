package api

import (
	"context"
	"errors"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"golang.org/x/crypto/bcrypt"

	"lan-im-go/models"
	"lan-im-go/pkg"
	"lan-im-go/repository"
	authsecurity "lan-im-go/services/auth/security"
	"lan-im-go/shared/observability/metrics"
)

var adminLoginProtector = authsecurity.NewLoginProtector(authsecurity.DefaultConfig())

type adminLoginRequest struct {
	Username string `json:"username" binding:"required"`
	Password string `json:"password" binding:"required"`
}

// AdminLogin 管理端专属登录入口，只向具备后台权限的账号签发令牌。
func AdminLogin(c *gin.Context) {
	startedAt := time.Now()
	result := "internal_error"
	defer func() { metrics.ObserveLogin(startedAt, "admin_"+result) }()

	var request adminLoginRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		result = "invalid_request"
		c.JSON(http.StatusBadRequest, gin.H{"error": "请求参数不合法"})
		return
	}
	if err := adminLoginProtector.Allow(c.ClientIP(), request.Username); err != nil {
		result = "rate_limited"
		c.Header("Retry-After", "60")
		c.JSON(http.StatusTooManyRequests, gin.H{"error": "登录尝试过于频繁，请稍后再试"})
		return
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), 5*time.Second)
	defer cancel()
	user, err := repository.User.GetByUsernameContext(ctx, request.Username)
	if err != nil {
		if errors.Is(err, context.DeadlineExceeded) || errors.Is(err, context.Canceled) {
			result = "timeout"
			c.JSON(http.StatusGatewayTimeout, gin.H{"error": "登录处理超时，请稍后重试"})
			return
		}
		result = "invalid_credentials"
		c.JSON(http.StatusUnauthorized, gin.H{"error": "账号或密码错误"})
		return
	}

	err = adminLoginProtector.Compare(ctx, func() error {
		metrics.BcryptStarted()
		defer metrics.BcryptFinished()
		return bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(request.Password))
	})
	if err != nil {
		switch {
		case errors.Is(err, context.DeadlineExceeded), errors.Is(err, context.Canceled):
			result = "timeout"
			c.JSON(http.StatusGatewayTimeout, gin.H{"error": "登录处理超时，请稍后重试"})
		case errors.Is(err, authsecurity.ErrBcryptBusy):
			result = "busy"
			c.Header("Retry-After", "1")
			c.JSON(http.StatusServiceUnavailable, gin.H{"error": "登录请求繁忙，请稍后重试"})
		default:
			result = "invalid_credentials"
			c.JSON(http.StatusUnauthorized, gin.H{"error": "账号或密码错误"})
		}
		return
	}

	if !models.IsAdminRole(user.Role) {
		result = "forbidden"
		c.JSON(http.StatusForbidden, gin.H{"error": "该账号无权登录管理后台"})
		return
	}
	token, err := pkg.GenerateToken(user.ID, user.Role)
	if err != nil {
		result = "token_error"
		c.JSON(http.StatusInternalServerError, gin.H{"error": "令牌生成失败"})
		return
	}

	result = "success"
	c.JSON(http.StatusOK, gin.H{
		"msg":   "管理员登录成功",
		"token": token,
		"user": gin.H{
			"id": user.ID, "username": user.Username, "role": user.Role, "avatar": user.Avatar,
		},
	})
}
