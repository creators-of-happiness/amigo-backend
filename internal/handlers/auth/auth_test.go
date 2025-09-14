package auth_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"regexp"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/creators-of-happiness/amigo-backend/internal/config"
	"github.com/creators-of-happiness/amigo-backend/internal/handlers/auth"
)

// newTestPool tries to create a live pgx pool for tests that need Postgres.
// If a DB isn't reachable, the tests are skipped (so unit tests remain green on machines without DB).
func newTestPool(t *testing.T) *pgxpool.Pool {
	t.Helper()
	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		// local dev default (matches docker-compose defaults)
		dsn = "postgres://postgres:postgres@127.0.0.1:5432/appdb?sslmode=disable&connect_timeout=1"
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Skipf("skipping auth tests: cannot create pgx pool: %v", err)
	}
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		t.Skipf("skipping auth tests: DB not reachable: %v", err)
	}
	t.Cleanup(func() { pool.Close() })
	return pool
}

// ensureSchema verifies required tables exist; if not, the test is skipped
// with a clear hint to run migrations first.
func ensureSchema(t *testing.T, pool *pgxpool.Pool) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	var appUsers, otpReq string
	if err := pool.QueryRow(ctx, `SELECT COALESCE(to_regclass('public.app_users')::text, '')`).Scan(&appUsers); err != nil || appUsers == "" {
		t.Skipf("skipping: table public.app_users not found (run migrations before tests)")
	}
	if err := pool.QueryRow(ctx, `SELECT COALESCE(to_regclass('public.otp_requests')::text, '')`).Scan(&otpReq); err != nil || otpReq == "" {
		t.Skipf("skipping: table public.otp_requests not found (run migrations before tests)")
	}
}

func setupRouter(pool *pgxpool.Pool, cfg config.Config) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(gin.Recovery())
	v1 := r.Group("/api/v1")
	auth.Register(v1, pool, cfg)
	return r
}

func doJSON(t *testing.T, r http.Handler, method, path string, body any) *httptest.ResponseRecorder {
	t.Helper()
	var buf bytes.Buffer
	if body != nil {
		enc := json.NewEncoder(&buf)
		if err := enc.Encode(body); err != nil {
			t.Fatalf("encode json: %v", err)
		}
	}
	req := httptest.NewRequest(method, path, &buf)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

func TestAuth_RequestCode_OK(t *testing.T) {
	pool := newTestPool(t)
	ensureSchema(t, pool)

	cfg := config.Config{
		AuthSecret:        "test-secret",
		OTPFixedCode:      "000000",
		OTPExpiresMinutes: 5,
		AccessTokenTTLHrs: 1,
	}
	r := setupRouter(pool, cfg)

	phone := "+82 10-5555-4444"
	w := doJSON(t, r, http.MethodPost, "/api/v1/auth/request-code", map[string]any{
		"phone":   phone,
		"purpose": "login",
	})

	// 로그 테이블 정리(선택)
	t.Cleanup(func() { _, _ = pool.Exec(context.Background(), `DELETE FROM otp_requests WHERE phone=$1`, phone) })

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d, body=%s", w.Code, w.Body.String())
	}
	var out map[string]any
	_ = json.Unmarshal(w.Body.Bytes(), &out)
	if ok, _ := out["ok"].(bool); !ok {
		t.Fatalf("expected ok=true, got %v", out["ok"])
	}
	// dev_hintCode echoes fixed code in current handler
	if out["dev_hintCode"] != cfg.OTPFixedCode {
		t.Fatalf("expected dev_hintCode=%s, got %v", cfg.OTPFixedCode, out["dev_hintCode"])
	}
}

func TestAuth_RequestCode_InvalidPhone(t *testing.T) {
	pool := newTestPool(t)
	ensureSchema(t, pool)

	cfg := config.Config{
		AuthSecret:        "test-secret",
		OTPFixedCode:      "000000",
		OTPExpiresMinutes: 5,
		AccessTokenTTLHrs: 1,
	}
	r := setupRouter(pool, cfg)

	w := doJSON(t, r, http.MethodPost, "/api/v1/auth/request-code", map[string]any{
		"phone":   "abc!@#",
		"purpose": "login",
	})
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d, body=%s", w.Code, w.Body.String())
	}
}

func TestAuth_Verify_WrongCode(t *testing.T) {
	pool := newTestPool(t)
	ensureSchema(t, pool)

	cfg := config.Config{
		AuthSecret:        "test-secret",
		OTPFixedCode:      "000000",
		OTPExpiresMinutes: 5,
		AccessTokenTTLHrs: 1,
	}
	r := setupRouter(pool, cfg)

	phone := "+82 10-1111-2222"
	w := doJSON(t, r, http.MethodPost, "/api/v1/auth/verify", map[string]any{
		"phone": phone,
		"code":  "999999",
	})
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d, body=%s", w.Code, w.Body.String())
	}
}

func TestAuth_Verify_OK_NewUser(t *testing.T) {
	pool := newTestPool(t)
	ensureSchema(t, pool)

	cfg := config.Config{
		AuthSecret:        "test-secret",
		OTPFixedCode:      "000000",
		OTPExpiresMinutes: 5,
		AccessTokenTTLHrs: 1,
	}
	r := setupRouter(pool, cfg)

	phone := fmt.Sprintf("+82 10-%04d-%04d", time.Now().Unix()%10000, time.Now().UnixNano()%10000)
	nickname := fmt.Sprintf("hjyoon-%d", time.Now().UnixNano())

	// request-code step is optional for verify in this implementation; verify trusts fixed code
	w := doJSON(t, r, http.MethodPost, "/api/v1/auth/verify", map[string]any{
		"phone":    phone,
		"code":     cfg.OTPFixedCode,
		"nickname": nickname,
	})
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d, body=%s", w.Code, w.Body.String())
	}
	var out struct {
		TokenType   string `json:"token_type"`
		AccessToken string `json:"access_token"`
		ExpiresIn   int    `json:"expires_in"`
		User        struct {
			ID       string  `json:"id"`
			Phone    string  `json:"phone"`
			Nickname *string `json:"nickname"`
		} `json:"user"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &out); err != nil {
		t.Fatalf("invalid json: %v", err)
	}
	if out.TokenType != "Bearer" || out.AccessToken == "" || out.ExpiresIn <= 0 {
		t.Fatalf("unexpected token response: %+v", out)
	}
	if out.User.Phone != phone {
		t.Fatalf("user.phone mismatch: %s", out.User.Phone)
	}
	if out.User.Nickname == nil || *out.User.Nickname != nickname {
		t.Fatalf("expected nickname=%s, got %+v", nickname, out.User.Nickname)
	}
	// quick uuid shape check
	uuidRe := regexp.MustCompile(`^[0-9a-fA-F-]{36}$`)
	if !uuidRe.MatchString(out.User.ID) {
		t.Fatalf("user.id not a uuid: %s", out.User.ID)
	}
	t.Cleanup(func() { _, _ = pool.Exec(context.Background(), `DELETE FROM app_users WHERE phone=$1`, phone) })
}

func TestAuth_Verify_OK_ExistingUser_PreserveNicknameOnEmptyInput(t *testing.T) {
	pool := newTestPool(t)
	ensureSchema(t, pool)

	cfg := config.Config{
		AuthSecret:        "test-secret",
		OTPFixedCode:      "000000",
		OTPExpiresMinutes: 5,
		AccessTokenTTLHrs: 1,
	}
	r := setupRouter(pool, cfg)

	phone := "+82 10-9876-5432"
	first := fmt.Sprintf("first-nick-%d", time.Now().UnixNano())
	// 1) first login with nickname set
	w1 := doJSON(t, r, http.MethodPost, "/api/v1/auth/verify", map[string]any{
		"phone":    phone,
		"code":     cfg.OTPFixedCode,
		"nickname": first,
	})
	if w1.Code != http.StatusOK {
		t.Fatalf("first verify expected 200, got %d, body=%s", w1.Code, w1.Body.String())
	}
	// 2) second login with empty nickname => should preserve previous nickname
	w2 := doJSON(t, r, http.MethodPost, "/api/v1/auth/verify", map[string]any{
		"phone": phone,
		"code":  cfg.OTPFixedCode,
		// nickname omitted -> zero value "" in struct binding; repo uses NULLIF and COALESCE to preserve
	})
	if w2.Code != http.StatusOK {
		t.Fatalf("second verify expected 200, got %d, body=%s", w2.Code, w2.Body.String())
	}
	var out struct {
		User struct {
			Nickname *string `json:"nickname"`
		} `json:"user"`
	}
	if err := json.Unmarshal(w2.Body.Bytes(), &out); err != nil {
		t.Fatalf("invalid json: %v", err)
	}
	if out.User.Nickname == nil || *out.User.Nickname != first {
		t.Fatalf("expected nickname to be preserved = %q, got %v", first, out.User.Nickname)
	}
	t.Cleanup(func() { _, _ = pool.Exec(context.Background(), `DELETE FROM app_users WHERE phone=$1`, phone) })
}
