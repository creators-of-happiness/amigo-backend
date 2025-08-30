package auth

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/creators-of-happiness/amigo-backend/internal/config"
	"github.com/creators-of-happiness/amigo-backend/internal/otp"
	"github.com/creators-of-happiness/amigo-backend/internal/repo"
	"github.com/creators-of-happiness/amigo-backend/internal/token"
	"github.com/creators-of-happiness/amigo-backend/internal/util"
)

func Register(v1 *gin.RouterGroup, pool *pgxpool.Pool, cfg config.Config) {
	g := v1.Group("/auth")

	g.POST("/request-code", func(c *gin.Context) {
		var in struct {
			Phone   string `json:"phone" binding:"required"`
			Purpose string `json:"purpose"`
		}
		if err := c.ShouldBindJSON(&in); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		if in.Purpose == "" {
			in.Purpose = "login"
		}
		if !util.LooksLikePhone(in.Phone) {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid phone format"})
			return
		}
		_ = otp.LogRequest(c.Request.Context(), pool, in.Phone, in.Purpose,
			cfg.OTPFixedCode, cfg.OTPExpiresMinutes, util.ClientIP(c.Request), c.Request.UserAgent())

		c.JSON(http.StatusOK, gin.H{
			"ok":           true,
			"message":      "verification code sent (dev: fixed code active)",
			"dev_hintCode": cfg.OTPFixedCode,
		})
	})

	g.POST("/verify", func(c *gin.Context) {
		var in struct {
			Phone    string `json:"phone" binding:"required"`
			Code     string `json:"code"  binding:"required"`
			Nickname string `json:"nickname"`
		}
		if err := c.ShouldBindJSON(&in); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		if !util.LooksLikePhone(in.Phone) {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid phone format"})
			return
		}
		if in.Code != cfg.OTPFixedCode {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid code"})
			return
		}
		u, err := repo.FindOrCreateUser(c.Request.Context(), pool, in.Phone, in.Nickname)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		accessTTL := time.Duration(cfg.AccessTokenTTLHrs) * time.Hour
		tok, exp, err := token.Sign(cfg.AuthSecret, u.ID, u.Phone, accessTTL)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to sign token"})
			return
		}
		c.JSON(http.StatusOK, gin.H{
			"token_type":   "Bearer",
			"access_token": tok,
			"expires_in":   int(time.Until(exp).Seconds()),
			"user": gin.H{
				"id":       u.ID,
				"phone":    u.Phone,
				"nickname": u.Nickname,
			},
		})
	})
}
