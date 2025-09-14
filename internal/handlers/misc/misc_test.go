package misc_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/creators-of-happiness/amigo-backend/internal/handlers/misc"
	"github.com/creators-of-happiness/amigo-backend/internal/repo"
	"github.com/creators-of-happiness/amigo-backend/internal/token"
)

// ---- helpers ----------------------------------------------------------------

func newTestPool(t *testing.T) *pgxpool.Pool {
	t.Helper()

	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		// 로컬 기본값(프로젝트의 docker-compose / .env 와 일치)
		dsn = "postgres://postgres:postgres@127.0.0.1:5432/appdb?sslmode=disable&connect_timeout=1"
	}

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Skipf("skipping misc tests: cannot create pgx pool: %v (run postgres or use docker compose)", err)
	}
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		t.Skipf("skipping misc tests: DB not reachable: %v (run migrations before tests)", err)
	}
	t.Cleanup(func() { pool.Close() })
	return pool
}

func ensureAppUsers(t *testing.T, pool *pgxpool.Pool) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	var tbl string
	if err := pool.QueryRow(ctx, `SELECT COALESCE(to_regclass('public.app_users')::text, '')`).Scan(&tbl); err != nil || tbl == "" {
		t.Skipf("skipping: table public.app_users not found (run migrations first)")
	}
}

func setupRouter(pool *pgxpool.Pool, secret string) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(gin.Recovery())
	v1 := r.Group("/api/v1")
	misc.Register(v1, pool, secret)
	return r
}

// ---- tests ------------------------------------------------------------------

// /api/v1/ping
func TestPing_OK(t *testing.T) {
	r := setupRouter(nil, "irrelevant")
	req := httptest.NewRequest(http.MethodGet, "/api/v1/ping", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	var out map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &out); err != nil {
		t.Fatalf("invalid json: %v", err)
	}
	if out["message"] != "pong" {
		t.Fatalf("expected message=pong, got %v", out["message"])
	}
}

// /api/v1/dbtime (DB 필요)
func TestDBTime_OK(t *testing.T) {
	pool := newTestPool(t)
	r := setupRouter(pool, "irrelevant")

	req := httptest.NewRequest(http.MethodGet, "/api/v1/dbtime", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d; body=%s", w.Code, w.Body.String())
	}
	var out map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &out); err != nil {
		t.Fatalf("invalid json: %v", err)
	}
	nowStr, ok := out["now"].(string)
	if !ok || nowStr == "" {
		t.Fatalf("expected now field (string), got %T %v", out["now"], out["now"])
	}
	// RFC3339 / RFC3339Nano 모두 허용
	if _, err := time.Parse(time.RFC3339, nowStr); err != nil {
		if _, err2 := time.Parse(time.RFC3339Nano, nowStr); err2 != nil {
			t.Fatalf("now not RFC3339( Nano ) parseable: %q (%v / %v)", nowStr, err, err2)
		}
	}
}

// /api/v1/me (인증 필요): 토큰 없음 → 401
func TestMe_Unauthorized_WithoutBearer(t *testing.T) {
	// 미들웨어가 선처리하므로 pool 없어도 됨
	r := setupRouter(nil, "secret")
	req := httptest.NewRequest(http.MethodGet, "/api/v1/me", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d; body=%s", w.Code, w.Body.String())
	}
}

// /api/v1/me (인증 필요): 정상 토큰 → 200 + 사용자 정보
func TestMe_OK_WithBearer(t *testing.T) {
	pool := newTestPool(t)
	ensureAppUsers(t, pool)

	// 1) 사전 유저 생성(또는 upsert)
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	phone := fmt.Sprintf("+82 10-%04d-%04d", time.Now().Unix()%10000, time.Now().UnixNano()%10000)
	nickname := fmt.Sprintf("misc-test-user-%d", time.Now().UnixNano())

	u, err := repo.FindOrCreateUser(ctx, pool, phone, nickname)
	if err != nil {
		t.Fatalf("create user: %v", err)
	}
	// 테스트 종료 시 정리
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), `DELETE FROM app_users WHERE phone=$1`, phone)
	})

	// 2) 토큰 발급
	secret := "test-secret"
	tok, _, err := token.Sign(secret, u.ID, u.Phone, time.Hour)
	if err != nil {
		t.Fatalf("sign token: %v", err)
	}

	// 3) 요청
	r := setupRouter(pool, secret)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/me", nil)
	req.Header.Set("Authorization", "Bearer "+tok)

	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d; body=%s", w.Code, w.Body.String())
	}

	var out struct {
		ID       string  `json:"id"`
		Phone    string  `json:"phone"`
		Nickname *string `json:"nickname"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &out); err != nil {
		t.Fatalf("invalid json: %v", err)
	}

	if out.ID == "" || out.Phone != phone {
		t.Fatalf("unexpected user: id=%q phone=%q", out.ID, out.Phone)
	}
	if out.Nickname == nil || *out.Nickname != nickname {
		t.Fatalf("nickname mismatch: want %q, got %v", nickname, out.Nickname)
	}
}
