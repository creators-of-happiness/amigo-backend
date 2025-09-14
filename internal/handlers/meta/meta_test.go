package meta_test

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

	"github.com/creators-of-happiness/amigo-backend/internal/handlers/meta"
	"github.com/creators-of-happiness/amigo-backend/internal/token"
)

// ---- helpers ----------------------------------------------------------------

func newTestPool(t *testing.T) *pgxpool.Pool {
	t.Helper()

	dsn := os.Getenv("DATABASE_URL")
	fmt.Println(dsn)
	if dsn == "" {
		// docker-compose 기본값과 유사 (로컬에서 직접 실행 시)
		dsn = "postgres://postgres:postgres@127.0.0.1:5432/appdb?sslmode=disable&connect_timeout=1"
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Skipf("skipping meta tests: cannot create pgx pool: %v", err)
	}
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		t.Skipf("skipping meta tests: DB not reachable: %v (run migrations before tests)", err)
	}
	t.Cleanup(func() { pool.Close() })
	return pool
}

// 스키마 존재 여부 확인. 없으면 skip (migrations 필요)
func ensureSchema(t *testing.T, pool *pgxpool.Pool) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	tables := []string{
		"public.region",
		"public.job_category",
		"public.media_asset",
		"public.character_category",
		"public.character_item",
		"public.bg_item",
	}
	for _, tbl := range tables {
		var exists string
		if err := pool.QueryRow(ctx, `SELECT COALESCE(to_regclass($1)::text, '')`, tbl).Scan(&exists); err != nil || exists == "" {
			t.Skipf("skipping: table %s not found (run migrations before tests)", tbl)
		}
	}
}

// 최소 시드 데이터 보강(이미 있으면 건너뜀)
func seedBasicMeta(t *testing.T, pool *pgxpool.Pool) (categoryCode string) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 4*time.Second)
	defer cancel()

	// region 하나 보장
	_, _ = pool.Exec(ctx, `INSERT INTO region (name) VALUES ('Testland') ON CONFLICT (name) DO NOTHING`)

	// job_category 하나 보장
	_, _ = pool.Exec(ctx, `INSERT INTO job_category (code, name) VALUES ('ut', 'Unit Testing') ON CONFLICT (code) DO NOTHING`)

	// character_category 하나 보장
	categoryCode = "unit-test"
	_, _ = pool.Exec(ctx, `INSERT INTO character_category (code, name) VALUES ($1, 'Unit Test') ON CONFLICT (code) DO NOTHING`, categoryCode)

	// media_asset 두 개 생성(항상 신규 URL 사용)
	var asset1, asset2 string
	url1 := fmt.Sprintf("https://example.com/ut-char-%d.png", time.Now().UnixNano())
	url2 := fmt.Sprintf("https://example.com/ut-bg-%d.png", time.Now().UnixNano())
	if err := pool.QueryRow(ctx, `INSERT INTO media_asset (kind, url) VALUES ('image', $1) RETURNING id`, url1).Scan(&asset1); err != nil {
		t.Fatalf("seed media_asset 1: %v", err)
	}
	if err := pool.QueryRow(ctx, `INSERT INTO media_asset (kind, url) VALUES ('image', $1) RETURNING id`, url2).Scan(&asset2); err != nil {
		t.Fatalf("seed media_asset 2: %v", err)
	}

	// character_item 하나 보장
	var _charID string
	if err := pool.QueryRow(ctx, `
		WITH ins AS (
			INSERT INTO character_item (category_code, name, preview_asset)
			VALUES ($1, 'UT Character 1', $2)
			ON CONFLICT (category_code, name) DO NOTHING
			RETURNING id
		)
		SELECT id FROM ins
		UNION ALL
		SELECT id FROM character_item WHERE category_code=$1 AND name='UT Character 1'
		LIMIT 1
	`, categoryCode, asset1).Scan(&_charID); err != nil {
		t.Fatalf("seed character_item: %v", err)
	}

	// bg_item 하나 보장
	var _bgID string
	if err := pool.QueryRow(ctx, `
		WITH ins AS (
			INSERT INTO bg_item (name, preview_asset)
			VALUES ('UT Background 1', $1)
			ON CONFLICT (name) DO NOTHING
			RETURNING id
		)
		SELECT id FROM ins
		UNION ALL
		SELECT id FROM bg_item WHERE name='UT Background 1'
		LIMIT 1
	`, asset2).Scan(&_bgID); err != nil {
		t.Fatalf("seed bg_item: %v", err)
	}

	t.Cleanup(func() {
		ctx2, cancel2 := context.WithTimeout(context.Background(), 4*time.Second)
		defer cancel2()
		_, _ = pool.Exec(ctx2, `DELETE FROM character_item WHERE name='UT Character 1'`)
		_, _ = pool.Exec(ctx2, `DELETE FROM bg_item WHERE name='UT Background 1'`)
		_, _ = pool.Exec(ctx2, `DELETE FROM character_category WHERE code=$1`, categoryCode)
		_, _ = pool.Exec(ctx2, `DELETE FROM job_category WHERE code='ut'`)
		_, _ = pool.Exec(ctx2, `DELETE FROM region WHERE name='Testland'`)
		_, _ = pool.Exec(ctx2, `DELETE FROM media_asset WHERE url IN ($1,$2)`, url1, url2)
	})
	return categoryCode
}

func setupRouter(pool *pgxpool.Pool, secret string) (*gin.Engine, string) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(gin.Recovery())
	v1 := r.Group("/api/v1")
	meta.Register(v1, pool, secret)

	// 임의 사용자 클레임으로 토큰 생성(미들웨어는 클레임만 확인)
	tok, _, _ := token.Sign(secret, "00000000-0000-0000-0000-000000000000", "+82 10-0000-0000", time.Hour)
	return r, tok
}

func doGET(t *testing.T, r http.Handler, path, bearer string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, path, nil)
	if bearer != "" {
		req.Header.Set("Authorization", "Bearer "+bearer)
	}
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

// ---- tests ------------------------------------------------------------------

// 인증 누락 → 401
func TestMeta_Unauthorized_WithoutBearer(t *testing.T) {
	pool := newTestPool(t)
	ensureSchema(t, pool)
	r, _ := setupRouter(pool, "test-secret")

	w := doGET(t, r, "/api/v1/meta/regions", "")
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d; body=%s", w.Code, w.Body.String())
	}
}

func TestMeta_Regions_OK(t *testing.T) {
	pool := newTestPool(t)
	ensureSchema(t, pool)
	_ = seedBasicMeta(t, pool)

	r, tok := setupRouter(pool, "test-secret")
	w := doGET(t, r, "/api/v1/meta/regions", tok)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d; body=%s", w.Code, w.Body.String())
	}

	var out struct {
		Items []struct {
			ID        int      `json:"id"`
			Name      string   `json:"name"`
			Latitude  *float64 `json:"latitude"`
			Longitude *float64 `json:"longitude"`
		} `json:"items"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &out); err != nil {
		t.Fatalf("invalid json: %v", err)
	}
	if len(out.Items) == 0 {
		t.Fatalf("expected at least 1 region, got 0")
	}
	if out.Items[0].ID == 0 || out.Items[0].Name == "" {
		t.Fatalf("region shape invalid: %+v", out.Items[0])
	}
}

func TestMeta_JobCategories_OK(t *testing.T) {
	pool := newTestPool(t)
	ensureSchema(t, pool)
	_ = seedBasicMeta(t, pool)

	r, tok := setupRouter(pool, "test-secret")
	w := doGET(t, r, "/api/v1/meta/job-categories", tok)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d; body=%s", w.Code, w.Body.String())
	}
	var out struct {
		Items []struct {
			Code string `json:"code"`
			Name string `json:"name"`
		} `json:"items"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &out); err != nil {
		t.Fatalf("invalid json: %v", err)
	}
	if len(out.Items) == 0 {
		t.Fatalf("expected at least 1 job category, got 0")
	}
	if out.Items[0].Code == "" || out.Items[0].Name == "" {
		t.Fatalf("job_category shape invalid: %+v", out.Items[0])
	}
}

func TestMeta_CharacterCategories_OK(t *testing.T) {
	pool := newTestPool(t)
	ensureSchema(t, pool)
	code := seedBasicMeta(t, pool)

	r, tok := setupRouter(pool, "test-secret")
	w := doGET(t, r, "/api/v1/meta/character-categories", tok)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d; body=%s", w.Code, w.Body.String())
	}
	var out struct {
		Items []struct {
			Code string `json:"code"`
			Name string `json:"name"`
		} `json:"items"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &out); err != nil {
		t.Fatalf("invalid json: %v", err)
	}
	if len(out.Items) == 0 {
		t.Fatalf("expected at least 1 character category, got 0")
	}
	// 시드한 카테고리가 포함되어 있는지 느슨히 확인
	found := false
	for _, it := range out.Items {
		if it.Code == code {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected category %q to exist, but not found", code)
	}
}

func TestMeta_Characters_OK(t *testing.T) {
	pool := newTestPool(t)
	ensureSchema(t, pool)
	_ = seedBasicMeta(t, pool)

	r, tok := setupRouter(pool, "test-secret")
	w := doGET(t, r, "/api/v1/meta/characters", tok)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d; body=%s", w.Code, w.Body.String())
	}
	var out struct {
		Items []struct {
			ID         string  `json:"id"`
			Name       string  `json:"name"`
			PreviewURL *string `json:"preview_url"`
		} `json:"items"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &out); err != nil {
		t.Fatalf("invalid json: %v", err)
	}
	if len(out.Items) == 0 {
		t.Fatalf("expected at least 1 character, got 0")
	}
	if out.Items[0].ID == "" || out.Items[0].Name == "" {
		t.Fatalf("character shape invalid: %+v", out.Items[0])
	}
}

func TestMeta_Characters_FilterByCategory_OK(t *testing.T) {
	pool := newTestPool(t)
	ensureSchema(t, pool)
	code := seedBasicMeta(t, pool)

	r, tok := setupRouter(pool, "test-secret")
	path := "/api/v1/meta/characters?category=" + code
	w := doGET(t, r, path, tok)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d; body=%s", w.Code, w.Body.String())
	}
	var out struct {
		Items []struct {
			ID         string  `json:"id"`
			Name       string  `json:"name"`
			PreviewURL *string `json:"preview_url"`
		} `json:"items"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &out); err != nil {
		t.Fatalf("invalid json: %v", err)
	}
	// 필터링 결과가 0일 수는 없도록 seedBasicMeta 보장
	if len(out.Items) == 0 {
		t.Fatalf("expected at least 1 character for category %q, got 0", code)
	}
}

func TestMeta_Backgrounds_OK(t *testing.T) {
	pool := newTestPool(t)
	ensureSchema(t, pool)
	_ = seedBasicMeta(t, pool)

	r, tok := setupRouter(pool, "test-secret")
	w := doGET(t, r, "/api/v1/meta/backgrounds", tok)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d; body=%s", w.Code, w.Body.String())
	}
	var out struct {
		Items []struct {
			ID         string  `json:"id"`
			Name       string  `json:"name"`
			PreviewURL *string `json:"preview_url"`
		} `json:"items"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &out); err != nil {
		t.Fatalf("invalid json: %v", err)
	}
	if len(out.Items) == 0 {
		t.Fatalf("expected at least 1 background, got 0")
	}
	if out.Items[0].ID == "" || out.Items[0].Name == "" {
		t.Fatalf("background shape invalid: %+v", out.Items[0])
	}
}
