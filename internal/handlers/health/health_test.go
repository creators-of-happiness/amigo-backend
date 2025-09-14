package health_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/creators-of-happiness/amigo-backend/internal/handlers/health"
)

// newTestPool tries to create a live pgx pool for readiness tests.
// If a DB isn't reachable, the test is skipped instead of failing.
func newTestPool(t *testing.T) *pgxpool.Pool {
	t.Helper()

	// Prefer explicit env for tests; fall back to typical local dev DSN.
	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		// Matches docker-compose defaults (host=localhost for local runs)
		dsn = "postgres://postgres:postgres@127.0.0.1:5432/appdb?sslmode=disable&connect_timeout=1"
	}

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Skipf("skipping readiness tests: cannot create pgx pool: %v (set AMIGO_TEST_DATABASE_URL to a reachable DB to enable)", err)
	}

	// Quick ping to ensure it's actually reachable.
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		t.Skipf("skipping readiness tests: DB not reachable: %v (set AMIGO_TEST_DATABASE_URL to a reachable DB to enable)", err)
	}
	t.Cleanup(func() { pool.Close() })
	return pool
}

func setupRouter(pool *pgxpool.Pool) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	health.Register(r, pool)
	return r
}

func TestLivenessOK(t *testing.T) {
	// /liveness doesn't use the DB; nil pool is fine here.
	r := setupRouter(nil)

	req := httptest.NewRequest(http.MethodGet, "/liveness", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", w.Code)
	}
	var body map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("invalid json response: %v", err)
	}
	if body["status"] != "ok" {
		t.Fatalf("expected status=ok, got %v", body["status"])
	}
}

func TestReadinessOK(t *testing.T) {
	pool := newTestPool(t) // may Skip if DB not available
	r := setupRouter(pool)

	req := httptest.NewRequest(http.MethodGet, "/readiness", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d; body=%s", w.Code, w.Body.String())
	}
	var body map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("invalid json response: %v", err)
	}
	if body["status"] != "ok" {
		t.Fatalf("expected status=ok, got %v", body["status"])
	}
}

func TestReadinessDegraded_WhenPoolClosed(t *testing.T) {
	pool := newTestPool(t) // may Skip if DB not available
	// Intentionally close to simulate DB errors (Ping should fail with "pool closed").
	pool.Close()

	r := setupRouter(pool)
	req := httptest.NewRequest(http.MethodGet, "/readiness", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected status 503, got %d; body=%s", w.Code, w.Body.String())
	}
	var body map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("invalid json response: %v", err)
	}
	if body["status"] != "degraded" {
		t.Fatalf("expected status=degraded, got %v", body["status"])
	}
	if _, ok := body["db"]; !ok {
		t.Fatalf("expected db error field to be present")
	}
}
