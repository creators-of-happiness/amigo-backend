package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgxpool"
)

func main() {
	port := getenv("PORT", "8080")
	mode := getenv("GIN_MODE", gin.ReleaseMode) // debug|release|test
	gin.SetMode(mode)

	// 1) Postgres 연결 (커넥션 풀)
	pool, err := newPostgresPool()
	if err != nil {
		log.Fatalf("postgres connect failed: %v", err)
	}
	// 시작시 간단 핑으로 조기검증(선택)
	{
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		if err := pool.Ping(ctx); err != nil {
			log.Fatalf("postgres ping failed: %v", err)
		}
	}

	r := gin.New()
	r.Use(gin.Logger(), gin.Recovery())

	r.GET("/liveness", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})
	r.GET("/readiness", func(c *gin.Context) {
		// 의존성(예: DB) 체크
		ctx, cancel := context.WithTimeout(c.Request.Context(), 500*time.Millisecond)
		defer cancel()
		if err := pool.Ping(ctx); err != nil {
			c.JSON(http.StatusServiceUnavailable, gin.H{"status": "degraded", "db": err.Error()})
			return
		}
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})
	r.NoRoute(func(c *gin.Context) {
		c.JSON(http.StatusNotFound, gin.H{"error": "not found"})
	})

	v1 := r.Group("/api/v1")
	{
		v1.GET("/ping", func(c *gin.Context) {
			c.JSON(http.StatusOK, gin.H{"message": "pong"})
		})

		// 예시: DB에서 현재 시각 조회
		v1.GET("/dbtime", func(c *gin.Context) {
			ctx, cancel := context.WithTimeout(c.Request.Context(), 1*time.Second)
			defer cancel()
			var now time.Time
			// 단순 예시 쿼리
			if err := pool.QueryRow(ctx, "SELECT NOW()").Scan(&now); err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
				return
			}
			c.JSON(http.StatusOK, gin.H{"now": now})
		})
	}

	// 서버 & 그레이스풀 셧다운
	srv := &http.Server{
		Addr:              ":" + port,
		Handler:           r,
		ReadHeaderTimeout: 2 * time.Second,
		ReadTimeout:       5 * time.Second,
		WriteTimeout:      10 * time.Second,
		IdleTimeout:       60 * time.Second,
	}

	// SIGINT/SIGTERM 모두 처리
	sigCtx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	go func() {
		log.Printf("listening on :%s", port)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("listen: %v", err)
		}
	}()

	<-sigCtx.Done() // 종료 신호 대기

	// HTTP 서버 종료 대기
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		log.Fatalf("server forced to shutdown: %v", err)
	}

	// 커넥션 풀 정리
	pool.Close()

	log.Println("server exiting")
}

func getenv(k, def string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return def
}

func newPostgresPool() (*pgxpool.Pool, error) {
	dsn := postgresDSNFromEnv()
	cfg, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		return nil, fmt.Errorf("parse dsn: %w", err)
	}

	cfg.MinConns = 0
	cfg.MaxConns = 10
	cfg.HealthCheckPeriod = 30 * time.Second
	cfg.MaxConnLifetime = 30 * time.Minute
	cfg.MaxConnIdleTime = 5 * time.Minute

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	return pgxpool.NewWithConfig(ctx, cfg)
}

func postgresDSNFromEnv() string {
	// 우선순위 1: DATABASE_URL (예: postgres://user:pass@host:5432/dbname?sslmode=disable)
	if v := os.Getenv("DATABASE_URL"); v != "" {
		return v
	}
	// 우선순위 2: PG* 환경변수로 DSN 구성
	host := getenv("PGHOST", "127.0.0.1")
	port := getenv("PGPORT", "5432")
	user := getenv("PGUSER", "postgres")
	pass := os.Getenv("PGPASSWORD") // 기본값 없음
	db := getenv("PGDATABASE", "postgres")
	sslmode := getenv("PGSSLMODE", "disable")

	u := &url.URL{
		Scheme: "postgres",
		User:   url.UserPassword(user, pass),
		Host:   fmt.Sprintf("%s:%s", host, port),
		Path:   "/" + db,
	}
	q := u.Query()
	q.Set("sslmode", sslmode)
	u.RawQuery = q.Encode()
	return u.String()
}
