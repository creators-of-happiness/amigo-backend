package profile_test

import (
	"bytes"
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

	"github.com/creators-of-happiness/amigo-backend/internal/handlers/profile"
	"github.com/creators-of-happiness/amigo-backend/internal/repo"
	"github.com/creators-of-happiness/amigo-backend/internal/token"
)

// ----------------------------- helpers ---------------------------------------

func newTestPool(t *testing.T) *pgxpool.Pool {
	t.Helper()
	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		dsn = "postgres://postgres:postgres@127.0.0.1:5432/appdb?sslmode=disable&connect_timeout=1"
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Skipf("skipping profile tests: cannot create pgx pool: %v", err)
	}
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		t.Skipf("skipping profile tests: DB not reachable: %v (run migrations before tests)", err)
	}
	t.Cleanup(func() { pool.Close() })
	return pool
}

// 필요한 테이블이 없으면 skip (마이그레이션 전제)
func ensureSchema(t *testing.T, pool *pgxpool.Pool) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	req := []string{
		"public.app_users",
		"public.user_profile",
		"public.user_job",
		"public.user_avatar",
		"public.region",
		"public.job_category",
		"public.media_asset",
		"public.character_category",
		"public.character_item",
		"public.bg_item",
	}
	for _, tbl := range req {
		var exists string
		if err := pool.QueryRow(ctx, `SELECT COALESCE(to_regclass($1)::text, '')`, tbl).Scan(&exists); err != nil || exists == "" {
			t.Skipf("skipping: table %s not found (run migrations first)", tbl)
		}
	}
}

// 프로필 관련 최소 메타 시드 생성 (여러 번 호출해도 충돌 없도록 유니크 값 사용)
type metaSeed struct {
	RegionID    int
	JobCode     string
	CharCatCode string
	CharacterID string
	BgID        string
}

func seedMeta(t *testing.T, pool *pgxpool.Pool) metaSeed {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 6*time.Second)
	defer cancel()

	// region
	var regionID int
	_ = pool.QueryRow(ctx, `
		WITH ins AS (
			INSERT INTO region (name, latitude, longitude)
			VALUES ('Testland', 0, 0)
			ON CONFLICT (name) DO NOTHING
			RETURNING id
		)
		SELECT id FROM ins
		UNION ALL
		SELECT id FROM region WHERE name='Testland'
		LIMIT 1`).Scan(&regionID)

	// job_category
	jobCode := "ut-" + fmt.Sprint(time.Now().Unix()%100000)
	_, _ = pool.Exec(ctx, `INSERT INTO job_category (code, name) VALUES ($1,'Unit Test') ON CONFLICT (code) DO NOTHING`, jobCode)

	// character_category
	charCat := "ut-cat-" + fmt.Sprint(time.Now().UnixNano())
	_, _ = pool.Exec(ctx, `INSERT INTO character_category (code, name) VALUES ($1,'UT Cat') ON CONFLICT (code) DO NOTHING`, charCat)

	// media assets (unique urls)
	url1 := fmt.Sprintf("https://example.com/ut-char-%d.png", time.Now().UnixNano())
	url2 := fmt.Sprintf("https://example.com/ut-bg-%d.png", time.Now().UnixNano())
	var asset1, asset2 string
	if err := pool.QueryRow(ctx, `INSERT INTO media_asset (kind, url) VALUES ('image',$1) RETURNING id`, url1).Scan(&asset1); err != nil {
		t.Fatalf("seed media_asset 1: %v", err)
	}
	if err := pool.QueryRow(ctx, `INSERT INTO media_asset (kind, url) VALUES ('image',$1) RETURNING id`, url2).Scan(&asset2); err != nil {
		t.Fatalf("seed media_asset 2: %v", err)
	}

	// character_item
	var charID string
	if err := pool.QueryRow(ctx, `
		INSERT INTO character_item (category_code, name, preview_asset)
		VALUES ($1,'UT Character 1',$2)
		ON CONFLICT (category_code, name) DO NOTHING
		RETURNING id
	`, charCat, asset1).Scan(&charID); err != nil {
		// 이미 있으면 조회
		if err := pool.QueryRow(ctx, `SELECT id FROM character_item WHERE category_code=$1 AND name='UT Character 1'`, charCat).Scan(&charID); err != nil {
			t.Fatalf("seed character_item: %v", err)
		}
	}

	// bg_item
	var bgID string
	if err := pool.QueryRow(ctx, `
		INSERT INTO bg_item (name, preview_asset)
		VALUES ('UT Background 1',$1)
		ON CONFLICT (name) DO NOTHING
		RETURNING id
	`, asset2).Scan(&bgID); err != nil {
		if err := pool.QueryRow(ctx, `SELECT id FROM bg_item WHERE name='UT Background 1'`).Scan(&bgID); err != nil {
			t.Fatalf("seed bg_item: %v", err)
		}
	}

	// 정리(테스트 종료 후 시드 삭제)
	t.Cleanup(func() {
		ctx2, cancel2 := context.WithTimeout(context.Background(), 6*time.Second)
		defer cancel2()
		_, _ = pool.Exec(ctx2, `DELETE FROM character_item WHERE name='UT Character 1'`)
		_, _ = pool.Exec(ctx2, `DELETE FROM bg_item WHERE name='UT Background 1'`)
		_, _ = pool.Exec(ctx2, `DELETE FROM character_category WHERE code=$1`, charCat)
		_, _ = pool.Exec(ctx2, `DELETE FROM job_category WHERE code=$1`, jobCode)
		_, _ = pool.Exec(ctx2, `DELETE FROM region WHERE name='Testland'`)
		_, _ = pool.Exec(ctx2, `DELETE FROM media_asset WHERE url IN ($1,$2)`, url1, url2)
	})

	return metaSeed{
		RegionID:    regionID,
		JobCode:     jobCode,
		CharCatCode: charCat,
		CharacterID: charID,
		BgID:        bgID,
	}
}

func setupRouter(pool *pgxpool.Pool, secret string) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(gin.Recovery())
	v1 := r.Group("/api/v1")
	profile.Register(v1, pool, secret)
	return r
}

func newUserAndToken(t *testing.T, pool *pgxpool.Pool, secret string, withNickname bool) (uid, phone, tokenStr string) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	phone = fmt.Sprintf("+82 10-%04d-%04d", time.Now().Unix()%10000, time.Now().UnixNano()%10000)
	nick := ""
	if withNickname {
		nick = fmt.Sprintf("nick-%d", time.Now().UnixNano())
	}
	u, err := repo.FindOrCreateUser(ctx, pool, phone, nick)
	if err != nil {
		t.Fatalf("create user: %v", err)
	}
	t.Cleanup(func() { _, _ = pool.Exec(context.Background(), `DELETE FROM app_users WHERE phone=$1`, phone) })

	tok, _, err := token.Sign(secret, u.ID, u.Phone, time.Hour)
	if err != nil {
		t.Fatalf("sign token: %v", err)
	}
	return u.ID, u.Phone, tok
}

func doJSON(t *testing.T, r http.Handler, method, path, bearer string, body any) *httptest.ResponseRecorder {
	t.Helper()
	var buf bytes.Buffer
	if body != nil {
		if err := json.NewEncoder(&buf).Encode(body); err != nil {
			t.Fatalf("encode json: %v", err)
		}
	}
	req := httptest.NewRequest(method, path, &buf)
	req.Header.Set("Content-Type", "application/json")
	if bearer != "" {
		req.Header.Set("Authorization", "Bearer "+bearer)
	}
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

// ------------------------------ tests ----------------------------------------

// 기본 온보딩 상태 플래그 확인 (모두 false 기대)
func TestOnboardingState_DefaultsFalse(t *testing.T) {
	pool := newTestPool(t)
	ensureSchema(t, pool)
	_ = seedMeta(t, pool)

	secret := "test-secret"
	uid, _, tok := newUserAndToken(t, pool, secret, false)
	_ = uid

	r := setupRouter(pool, secret)

	w := doJSON(t, r, http.MethodGet, "/api/v1/me/onboarding-state", tok, nil)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d; body=%s", w.Code, w.Body.String())
	}
	var out map[string]bool
	if err := json.Unmarshal(w.Body.Bytes(), &out); err != nil {
		t.Fatalf("invalid json: %v", err)
	}
	keys := []string{"nickname", "gender", "birthdate", "region", "job", "avatar", "photo"}
	for _, k := range keys {
		if _, ok := out[k]; !ok {
			t.Fatalf("missing key %q in response", k)
		}
	}
	// 초기엔 모두 false일 확률이 높다(새 유저)
	for _, k := range keys {
		if out[k] {
			t.Fatalf("expected %s=false at start, got true", k)
		}
	}
}

// 닉네임 설정 OK
func TestNickname_Update_OK(t *testing.T) {
	pool := newTestPool(t)
	ensureSchema(t, pool)
	_ = seedMeta(t, pool)

	secret := "test-secret"
	uid, _, tok := newUserAndToken(t, pool, secret, false)
	_ = uid

	r := setupRouter(pool, secret)

	want := fmt.Sprintf("hjyoon-%d", time.Now().UnixNano())
	w := doJSON(t, r, http.MethodPatch, "/api/v1/me/nickname", tok, map[string]any{"nickname": want})
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d; body=%s", w.Code, w.Body.String())
	}

	// DB 확인
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	var got *string
	if err := pool.QueryRow(ctx, `SELECT nickname FROM app_users WHERE id=$1`, uid).Scan(&got); err != nil {
		t.Fatalf("query nickname: %v", err)
	}
	if got == nil || *got != want {
		t.Fatalf("nickname mismatch: want=%q got=%v", want, got)
	}
}

// 성별 설정 OK
func TestGender_Set_OK(t *testing.T) {
	pool := newTestPool(t)
	ensureSchema(t, pool)
	_ = seedMeta(t, pool)

	secret := "test-secret"
	uid, _, tok := newUserAndToken(t, pool, secret, true)

	r := setupRouter(pool, secret)
	w := doJSON(t, r, http.MethodPatch, "/api/v1/me/gender", tok, map[string]any{"gender": "male"})
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d; body=%s", w.Code, w.Body.String())
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	var g *string
	if err := pool.QueryRow(ctx, `SELECT gender FROM user_profile WHERE user_id=$1`, uid).Scan(&g); err != nil {
		t.Fatalf("query gender: %v", err)
	}
	if g == nil || *g != "male" {
		t.Fatalf("gender mismatch: want=male got=%v", g)
	}
}

// 생년월일 설정 OK (유효 범위)
func TestBirthdate_Set_OK(t *testing.T) {
	pool := newTestPool(t)
	ensureSchema(t, pool)
	_ = seedMeta(t, pool)

	secret := "test-secret"
	uid, _, tok := newUserAndToken(t, pool, secret, true)

	r := setupRouter(pool, secret)
	bd := "1992-04-01"
	w := doJSON(t, r, http.MethodPatch, "/api/v1/me/birthdate", tok, map[string]any{"birthdate": bd})
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d; body=%s", w.Code, w.Body.String())
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	var got time.Time
	if err := pool.QueryRow(ctx, `SELECT birth_date FROM user_profile WHERE user_id=$1`, uid).Scan(&got); err != nil {
		t.Fatalf("query birth_date: %v", err)
	}
	if got.Format("2006-01-02") != bd {
		t.Fatalf("birthdate mismatch: want=%s got=%s", bd, got.Format("2006-01-02"))
	}
}

// 지역 설정 OK (외래키)
func TestRegion_Set_OK(t *testing.T) {
	pool := newTestPool(t)
	ensureSchema(t, pool)
	seed := seedMeta(t, pool)

	secret := "test-secret"
	uid, _, tok := newUserAndToken(t, pool, secret, true)

	r := setupRouter(pool, secret)
	w := doJSON(t, r, http.MethodPatch, "/api/v1/me/region", tok, map[string]any{"region_id": seed.RegionID})
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d; body=%s", w.Code, w.Body.String())
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	var rid *int
	if err := pool.QueryRow(ctx, `SELECT region_id FROM user_profile WHERE user_id=$1`, uid).Scan(&rid); err != nil {
		t.Fatalf("query region_id: %v", err)
	}
	if rid == nil || *rid != seed.RegionID {
		t.Fatalf("region_id mismatch: want=%d got=%v", seed.RegionID, rid)
	}
}

// 직업 upsert OK
func TestJob_Upsert_OK(t *testing.T) {
	pool := newTestPool(t)
	ensureSchema(t, pool)
	seed := seedMeta(t, pool)

	secret := "test-secret"
	uid, _, tok := newUserAndToken(t, pool, secret, true)

	r := setupRouter(pool, secret)
	body := map[string]any{"category": seed.JobCode, "detail": "Backend/Platform"}
	w := doJSON(t, r, http.MethodPut, "/api/v1/me/job", tok, body)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d; body=%s", w.Code, w.Body.String())
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	var gotCat, gotDetail *string
	if err := pool.QueryRow(ctx, `SELECT category, detail FROM user_job WHERE user_id=$1`, uid).Scan(&gotCat, &gotDetail); err != nil {
		t.Fatalf("query user_job: %v", err)
	}
	if gotCat == nil || *gotCat != seed.JobCode {
		t.Fatalf("job.category mismatch: want=%s got=%v", seed.JobCode, gotCat)
	}
	if gotDetail == nil || *gotDetail != "Backend/Platform" {
		t.Fatalf("job.detail mismatch: want=%s got=%v", "Backend/Platform", gotDetail)
	}
}

// 아바타 upsert OK (character_id + bg_id)
func TestAvatar_Upsert_OK(t *testing.T) {
	pool := newTestPool(t)
	ensureSchema(t, pool)
	seed := seedMeta(t, pool)

	secret := "test-secret"
	uid, _, tok := newUserAndToken(t, pool, secret, true)

	r := setupRouter(pool, secret)
	body := map[string]any{
		"category_code": seed.CharCatCode,
		"character_id":  seed.CharacterID,
		"bg_id":         seed.BgID,
	}
	w := doJSON(t, r, http.MethodPut, "/api/v1/me/avatar", tok, body)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d; body=%s", w.Code, w.Body.String())
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	var cc, ch, bg *string
	if err := pool.QueryRow(ctx, `SELECT category_code, character_id, bg_id FROM user_avatar WHERE user_id=$1`, uid).Scan(&cc, &ch, &bg); err != nil {
		t.Fatalf("query user_avatar: %v", err)
	}
	if cc == nil || *cc != seed.CharCatCode {
		t.Fatalf("avatar.category_code mismatch: want=%s got=%v", seed.CharCatCode, cc)
	}
	if ch == nil || *ch != seed.CharacterID {
		t.Fatalf("avatar.character_id mismatch: want=%s got=%v", seed.CharacterID, ch)
	}
	if bg == nil || *bg != seed.BgID {
		t.Fatalf("avatar.bg_id mismatch: want=%s got=%v", seed.BgID, bg)
	}
}

// 프로필 사진 URL 설정 OK (media_asset 생성 + profile_image_id 세팅)
func TestPhoto_Set_OK(t *testing.T) {
	pool := newTestPool(t)
	ensureSchema(t, pool)
	_ = seedMeta(t, pool)

	secret := "test-secret"
	uid, _, tok := newUserAndToken(t, pool, secret, true)

	r := setupRouter(pool, secret)
	url := fmt.Sprintf("https://example.com/me-%d.jpg", time.Now().UnixNano())
	w := doJSON(t, r, http.MethodPatch, "/api/v1/me/photo", tok, map[string]any{"url": url})
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d; body=%s", w.Code, w.Body.String())
	}

	var out struct {
		OK      bool   `json:"ok"`
		AssetID string `json:"asset_id"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &out); err != nil {
		t.Fatalf("invalid json: %v", err)
	}
	if !out.OK || out.AssetID == "" {
		t.Fatalf("unexpected response: %+v", out)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	var pid *string
	if err := pool.QueryRow(ctx, `SELECT profile_image_id::text FROM user_profile WHERE user_id=$1`, uid).Scan(&pid); err != nil {
		t.Fatalf("query profile_image_id: %v", err)
	}
	if pid == nil || *pid != out.AssetID {
		t.Fatalf("profile_image_id mismatch: want=%s got=%v", out.AssetID, pid)
	}
}
