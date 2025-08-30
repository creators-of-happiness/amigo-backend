package profile

import (
	"context"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/creators-of-happiness/amigo-backend/internal/middleware"
)

func Register(v1 *gin.RouterGroup, pool *pgxpool.Pool, authSecret string) {
	me := v1.Group("/me", middleware.Auth(authSecret))

	// 진행상태 조회(프론트가 어느 단계부터 시작할지 판단)
	me.GET("/onboarding-state", func(c *gin.Context) {
		uid := c.GetString("uid")
		ctx, cancel := context.WithTimeout(c.Request.Context(), 1*time.Second)
		defer cancel()

		var nickname *string
		var gender *string
		var birth *time.Time
		var regionID *int
		var profileImg *string

		_ = pool.QueryRow(ctx, `SELECT nickname FROM app_users WHERE id=$1`, uid).Scan(&nickname)
		_ = pool.QueryRow(ctx, `
			SELECT gender, birth_date, region_id, ma.url
			FROM user_profile up
			LEFT JOIN media_asset ma ON ma.id = up.profile_image_id
			WHERE up.user_id=$1`, uid).Scan(&gender, &birth, &regionID, &profileImg)

		var catCode *string
		var charID, bgID *string
		_ = pool.QueryRow(ctx, `
			SELECT category_code, character_id, bg_id FROM user_avatar WHERE user_id=$1`, uid).
			Scan(&catCode, &charID, &bgID)

		var jobCat *string
		_ = pool.QueryRow(ctx, `SELECT category FROM user_job WHERE user_id=$1`, uid).Scan(&jobCat)

		c.JSON(http.StatusOK, gin.H{
			"nickname":  nickname != nil,
			"gender":    gender != nil,
			"birthdate": birth != nil,
			"region":    regionID != nil,
			"job":       jobCat != nil,
			"avatar":    (charID != nil && bgID != nil),
			"photo":     profileImg != nil,
		})
	})

	// 1) 닉네임
	me.PATCH("/nickname", func(c *gin.Context) {
		uid := c.GetString("uid")
		var in struct {
			Nickname string `json:"nickname" binding:"required,min=1,max=30"`
		}
		if err := c.ShouldBindJSON(&in); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		ctx, cancel := context.WithTimeout(c.Request.Context(), 1*time.Second)
		defer cancel()
		_, err := pool.Exec(ctx, `UPDATE app_users SET nickname=$1, updated_at=now() WHERE id=$2`, in.Nickname, uid)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, gin.H{"ok": true})
	})

	// 2) 성별
	me.PATCH("/gender", func(c *gin.Context) {
		uid := c.GetString("uid")
		var in struct {
			Gender string `json:"gender" binding:"required"`
		}
		if err := c.ShouldBindJSON(&in); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		ctx, cancel := context.WithTimeout(c.Request.Context(), 1*time.Second)
		defer cancel()
		_, err := upsertProfile(ctx, pool, uid, "gender", in.Gender, nil, nil, nil)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, gin.H{"ok": true})
	})

	// 3) 생년월일(YYYY-MM-DD)
	me.PATCH("/birthdate", func(c *gin.Context) {
		uid := c.GetString("uid")
		var in struct {
			Birthdate string `json:"birthdate" binding:"required"`
		}
		if err := c.ShouldBindJSON(&in); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		ctx, cancel := context.WithTimeout(c.Request.Context(), 1*time.Second)
		defer cancel()
		_, err := pool.Exec(ctx, `
			INSERT INTO user_profile (user_id, birth_date, created_at, updated_at)
			VALUES ($1, $2::date, now(), now())
			ON CONFLICT (user_id) DO UPDATE SET birth_date=EXCLUDED.birth_date, updated_at=now()`, uid, in.Birthdate)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, gin.H{"ok": true})
	})

	// 4) 지역
	me.PATCH("/region", func(c *gin.Context) {
		uid := c.GetString("uid")
		var in struct {
			RegionID int `json:"region_id" binding:"required,min=1"`
		}
		if err := c.ShouldBindJSON(&in); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		ctx, cancel := context.WithTimeout(c.Request.Context(), 1*time.Second)
		defer cancel()
		_, err := pool.Exec(ctx, `
			INSERT INTO user_profile (user_id, region_id, created_at, updated_at)
			VALUES ($1, $2, now(), now())
			ON CONFLICT (user_id) DO UPDATE SET region_id=EXCLUDED.region_id, updated_at=now()`, uid, in.RegionID)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, gin.H{"ok": true})
	})

	// 5) 직업
	me.PUT("/job", func(c *gin.Context) {
		uid := c.GetString("uid")
		var in struct {
			Category string `json:"category" binding:"required"` // job_category.code
			Detail   string `json:"detail"`
		}
		if err := c.ShouldBindJSON(&in); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		ctx, cancel := context.WithTimeout(c.Request.Context(), 1*time.Second)
		defer cancel()
		_, err := pool.Exec(ctx, `
			INSERT INTO user_job (user_id, category, detail, created_at)
			VALUES ($1, $2, $3, now())
			ON CONFLICT (user_id) DO UPDATE SET category=EXCLUDED.category, detail=EXCLUDED.detail`, uid, in.Category, in.Detail)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, gin.H{"ok": true})
	})

	// 6) 프로필(아바타: 캐릭터+배경)
	me.PUT("/avatar", func(c *gin.Context) {
		uid := c.GetString("uid")
		var in struct {
			CategoryCode string `json:"category_code"` // optional
			CharacterID  string `json:"character_id" binding:"required"`
			BgID         string `json:"bg_id" binding:"required"`
		}
		if err := c.ShouldBindJSON(&in); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		ctx, cancel := context.WithTimeout(c.Request.Context(), 1*time.Second)
		defer cancel()
		_, err := pool.Exec(ctx, `
			INSERT INTO user_avatar (user_id, category_code, character_id, bg_id, selected_at)
			VALUES ($1, NULLIF($2,''), $3, $4, now())
			ON CONFLICT (user_id) DO UPDATE SET category_code=EXCLUDED.category_code, character_id=EXCLUDED.character_id, bg_id=EXCLUDED.bg_id, selected_at=now()
		`, uid, in.CategoryCode, in.CharacterID, in.BgID)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, gin.H{"ok": true})
	})

	// 7) 프로필 사진 업로드(간단화: URL 직접 입력 또는 multipart 파일 미구현 시 URL만)
	me.PATCH("/photo", func(c *gin.Context) {
		uid := c.GetString("uid")
		var in struct {
			URL string `json:"url" binding:"required"`
		}
		if err := c.ShouldBindJSON(&in); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		ctx, cancel := context.WithTimeout(c.Request.Context(), 2*time.Second)
		defer cancel()

		// media_asset 생성
		var assetID string
		if err := pool.QueryRow(ctx, `INSERT INTO media_asset (id, kind, url) VALUES (gen_random_uuid(),'image',$1) RETURNING id`, in.URL).Scan(&assetID); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		_, err := pool.Exec(ctx, `
			INSERT INTO user_profile (user_id, profile_image_id, created_at, updated_at)
			VALUES ($1, $2, now(), now())
			ON CONFLICT (user_id) DO UPDATE SET profile_image_id=EXCLUDED.profile_image_id, updated_at=now()`, uid, assetID)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		// 업로드 로그(선택)
		_, _ = pool.Exec(ctx, `
			INSERT INTO user_face_upload (id, user_id, url, status, created_at)
			VALUES (gen_random_uuid(), $1, $2, 'uploaded', now())`, uid, in.URL)

		c.JSON(http.StatusOK, gin.H{"ok": true, "asset_id": assetID})
	})
}

func upsertProfile(ctx context.Context, pool *pgxpool.Pool, uid, field, str string, i1, i2, i3 any) (int64, error) {
	// 간단화: gender만 이 헬퍼를 사용
	ct, err := pool.Exec(ctx, `
		INSERT INTO user_profile (user_id, `+field+`, created_at, updated_at)
		VALUES ($1, $2, now(), now())
		ON CONFLICT (user_id) DO UPDATE SET `+field+`=EXCLUDED.`+field+`, updated_at=now()`, uid, str)
	return ct.RowsAffected(), err
}
