package api

import (
	"context"
	"errors"
	"lan-im-go/models"
	"lan-im-go/pkg"
	"lan-im-go/repository"
	authsecurity "lan-im-go/services/auth/security"
	"lan-im-go/shared/observability/metrics"

	"net/http"
	"os"
	"strconv"
	"strings"
	"time"
	//"strings"

	"github.com/gin-gonic/gin"
	"golang.org/x/crypto/bcrypt"
)

var loginProtector = authsecurity.NewLoginProtector(loginProtectionConfig())

func loginProtectionConfig() authsecurity.Config {
	cfg := authsecurity.DefaultConfig()
	cfg.IPLimit = positiveEnvInt("LOGIN_IP_LIMIT_PER_MINUTE", cfg.IPLimit)
	cfg.PairLimit = positiveEnvInt("LOGIN_IP_USERNAME_LIMIT_PER_MINUTE", cfg.PairLimit)
	cfg.BcryptConcurrent = positiveEnvInt("LOGIN_BCRYPT_CONCURRENCY", cfg.BcryptConcurrent)
	return cfg
}

func positiveEnvInt(name string, fallback int) int {
	value, err := strconv.Atoi(strings.TrimSpace(os.Getenv(name)))
	if err != nil || value <= 0 {
		return fallback
	}
	return value
}

func loginTimeout() time.Duration {
	seconds := positiveEnvInt("LOGIN_TIMEOUT_SECONDS", 5)
	return time.Duration(seconds) * time.Second
}

// LoginRequest 登录请求参数
type LoginRequest struct {
	Username string `json:"username" binding:"required"`
	Password string `json:"password" binding:"required"`
}

// LoginHandler 用户登录接口
func LoginHandler(c *gin.Context) {
	startedAt := time.Now()
	result := "internal_error"
	defer func() { metrics.ObserveLogin(startedAt, result) }()

	var req LoginRequest
	// 参数绑定与校验

	pkg.Infof("登录界面进入成功\n")

	if err := c.ShouldBindJSON(&req); err != nil {
		result = "invalid_request"
		c.JSON(http.StatusBadRequest, gin.H{"error": "请求参数不合法"})
		return
	}
	if err := loginProtector.Allow(c.ClientIP(), req.Username); err != nil {
		result = "rate_limited"
		c.Header("Retry-After", "60")
		c.JSON(http.StatusTooManyRequests, gin.H{"error": "登录尝试过于频繁，请稍后再试"})
		return
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), loginTimeout())
	defer cancel()

	// 根据用户名查询用户信息
	user, err := repository.User.GetByUsernameContext(ctx, req.Username)
	if err != nil {
		if errors.Is(err, context.DeadlineExceeded) || errors.Is(err, context.Canceled) {
			result = "timeout"
			c.JSON(http.StatusGatewayTimeout, gin.H{"error": "登录处理超时，请稍后重试"})
			return
		}
		result = "invalid_credentials"
		// 统一错误提示，防止账号枚举攻击
		c.JSON(http.StatusUnauthorized, gin.H{"error": "账号或密码错误"})
		return
	}

	err = loginProtector.Compare(ctx, func() error {
		metrics.BcryptStarted()
		defer metrics.BcryptFinished()
		return bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(req.Password))
	})
	if err != nil {
		if errors.Is(err, context.DeadlineExceeded) || errors.Is(err, context.Canceled) {
			result = "timeout"
			c.JSON(http.StatusGatewayTimeout, gin.H{"error": "登录处理超时，请稍后重试"})
			return
		}
		if errors.Is(err, authsecurity.ErrBcryptBusy) {
			result = "busy"
			c.Header("Retry-After", "1")
			c.JSON(http.StatusServiceUnavailable, gin.H{"error": "登录请求繁忙，请稍后重试"})
			return
		}
		result = "invalid_credentials"
		c.JSON(http.StatusUnauthorized, gin.H{"error": "账号或密码错误"})
		return
	}
	// 密码校验：使用bcrypt对比加密密码，禁止明文验证
	if user.Role != models.RoleUser {
		result = "admin_login_required"
		c.JSON(http.StatusForbidden, gin.H{"error": "管理员账号请使用管理后台登录"})
		return
	}

	// 生成JWT身份令牌
	token, err := pkg.GenerateToken(user.ID, user.Role)
	if err != nil {
		result = "token_error"
		pkg.Infof("token generate error: %v\n", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "令牌生成失败"})
		return
	}

	pkg.Infof("%v 登录成功\n", user.Username)
	result = "success"
	// 返回登录成功响应
	c.JSON(http.StatusOK, gin.H{
		"msg":   "登录成功",
		"token": token,
		"user": gin.H{
			"id":       user.ID,
			"username": user.Username,
			"role":     user.Role,
			"avatar":   user.Avatar,
		},
	})
}

// RegisterRequest 注册请求参数
type RegisterRequest struct {
	// 参数长度约束，保障数据合法性
	Username string `json:"username" binding:"required,min=3,max=32"`
	Password string `json:"password" binding:"required,min=6,max=32"`
}

// RegisterHandler 用户注册接口
func RegisterHandler(c *gin.Context) {
	var req RegisterRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "参数格式不合法：账号或密码长度不符合规范"})
		return
	}

	// 校验用户名是否已存在，减轻数据库压力
	existingUser, _ := repository.User.GetByUsername(req.Username)
	if existingUser != nil {
		c.JSON(http.StatusConflict, gin.H{"error": "该用户名已被注册，请更换"})
		return
	}

	//使用bcrypt加密密码，不使用明文存储
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "密码加密失败，请稍后再试"})
		return
	}

	// 构建用户数据
	user := &models.User{
		Username: req.Username,
		Password: string(hashedPassword),
		Role:     0,  // 0=普通用户，默认权限
		Avatar:   "", // 默认头像
	}

	// 用户数据入库，数据库唯一索引保障并发注册安全
	if err := repository.User.CreateUser(user); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "注册失败，并发冲突请重试"})
		return
	}

	// 注册成功，强制跳转登录，统一身份验证流程
	c.JSON(http.StatusOK, gin.H{
		"msg": "注册成功，请前往登录",
	})
}
