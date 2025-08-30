package misc

import (
	"context"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/creators-of-happiness/amigo-backend/internal/middleware"
)

func Register(v1 *gin.RouterGroup, pool *pgxpool.Pool, authSecret string) {
	v1.GET("/ping", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"message": "pong"})
	})

	v1.GET("/dbtime", func(c *gin.Context) {
		ctx, cancel := context.WithTimeout(c.Request.Context(), 1*time.Second)
		defer cancel()
		var now time.Time
		if err := pool.QueryRow(ctx, "SELECT NOW()").Scan(&now); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, gin.H{"now": now})
	})

	v1.GET("/me", middleware.Auth(authSecret), func(c *gin.Context) {
		uid := c.GetString("uid")
		phone := c.GetString("phone")
		var nickname *string

		ctx, cancel := context.WithTimeout(c.Request.Context(), 1*time.Second)
		defer cancel()
		err := pool.QueryRow(ctx, `SELECT nickname FROM app_users WHERE id=$1`, uid).Scan(&nickname)
		if err != nil && err.Error() != "no rows in result set" {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, gin.H{"id": uid, "phone": phone, "nickname": nickname})
	})
}
