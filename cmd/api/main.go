package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/creators-of-happiness/amigo-backend/internal/config"
	"github.com/creators-of-happiness/amigo-backend/internal/db"
	"github.com/creators-of-happiness/amigo-backend/internal/handlers/auth"
	"github.com/creators-of-happiness/amigo-backend/internal/handlers/health"
	"github.com/creators-of-happiness/amigo-backend/internal/handlers/misc"
	"github.com/creators-of-happiness/amigo-backend/internal/httpserver"
)

func main() {
	cfg := config.FromEnv()

	gin.SetMode(cfg.GinMode)

	// DB 연결
	pool, err := db.NewPool(context.Background(), db.DSNFromEnv())
	if err != nil {
		log.Fatalf("postgres connect failed: %v", err)
	}
	defer pool.Close()

	// 라우터
	r := gin.New()
	r.Use(gin.Logger(), gin.Recovery())

	// 헬스
	health.Register(r, pool)

	// API v1
	v1 := r.Group("/api/v1")
	misc.Register(v1, pool, cfg.AuthSecret) // /ping, /dbtime, /me
	auth.Register(v1, pool, cfg)            // /auth/request-code, /auth/verify

	// HTTP 서버 + graceful shutdown
	srv := httpserver.New(":"+cfg.Port, r)

	sigCtx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	go func() {
		log.Printf("listening on :%s", cfg.Port)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("listen: %v", err)
		}
	}()

	<-sigCtx.Done()

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		log.Fatalf("server forced to shutdown: %v", err)
	}

	log.Println("server exiting")
}
