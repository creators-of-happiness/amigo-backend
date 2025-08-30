package meta

import (
	"context"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/creators-of-happiness/amigo-backend/internal/middleware"
)

func Register(v1 *gin.RouterGroup, pool *pgxpool.Pool, authSecret string) {
	g := v1.Group("/meta", middleware.Auth(authSecret))

	g.GET("/regions", func(c *gin.Context) {
		ctx, cancel := context.WithTimeout(c.Request.Context(), 1*time.Second)
		defer cancel()
		rows, err := pool.Query(ctx, `SELECT id, name, latitude, longitude FROM region ORDER BY id`)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		defer rows.Close()
		var out []gin.H
		for rows.Next() {
			var id int
			var name string
			var lat, lng *float64
			if err := rows.Scan(&id, &name, &lat, &lng); err != nil {
				continue
			}
			out = append(out, gin.H{"id": id, "name": name, "latitude": lat, "longitude": lng})
		}
		c.JSON(http.StatusOK, gin.H{"items": out})
	})

	g.GET("/job-categories", func(c *gin.Context) {
		ctx, cancel := context.WithTimeout(c.Request.Context(), 1*time.Second)
		defer cancel()
		rows, err := pool.Query(ctx, `SELECT code, name FROM job_category ORDER BY name`)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		defer rows.Close()
		var out []gin.H
		for rows.Next() {
			var code, name string
			if err := rows.Scan(&code, &name); err != nil {
				continue
			}
			out = append(out, gin.H{"code": code, "name": name})
		}
		c.JSON(http.StatusOK, gin.H{"items": out})
	})

	g.GET("/character-categories", func(c *gin.Context) {
		ctx, cancel := context.WithTimeout(c.Request.Context(), 1*time.Second)
		defer cancel()
		rows, err := pool.Query(ctx, `SELECT code, name FROM character_category ORDER BY name`)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		defer rows.Close()
		var out []gin.H
		for rows.Next() {
			var code, name string
			if err := rows.Scan(&code, &name); err != nil {
				continue
			}
			out = append(out, gin.H{"code": code, "name": name})
		}
		c.JSON(http.StatusOK, gin.H{"items": out})
	})

	g.GET("/characters", func(c *gin.Context) {
		category := c.Query("category")
		ctx, cancel := context.WithTimeout(c.Request.Context(), 1*time.Second)
		defer cancel()
		var rows pgx.Rows
		var err error
		if category != "" {
			rows, err = pool.Query(ctx, `
				SELECT i.id, i.name, m.url
				FROM character_item i
				LEFT JOIN media_asset m ON m.id = i.preview_asset
				WHERE i.category_code=$1
				ORDER BY i.name`, category)
		} else {
			rows, err = pool.Query(ctx, `
				SELECT i.id, i.name, m.url
				FROM character_item i
				LEFT JOIN media_asset m ON m.id = i.preview_asset
				ORDER BY i.name`)
		}
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		defer rows.Close()
		var out []gin.H
		for rows.Next() {
			var id, name string
			var url *string
			if err := rows.Scan(&id, &name, &url); err != nil {
				continue
			}
			out = append(out, gin.H{"id": id, "name": name, "preview_url": url})
		}
		c.JSON(http.StatusOK, gin.H{"items": out})
	})

	g.GET("/backgrounds", func(c *gin.Context) {
		ctx, cancel := context.WithTimeout(c.Request.Context(), 1*time.Second)
		defer cancel()
		rows, err := pool.Query(ctx, `
			SELECT b.id, b.name, m.url
			FROM bg_item b
			LEFT JOIN media_asset m ON m.id = b.preview_asset
			ORDER BY b.name`)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		defer rows.Close()
		var out []gin.H
		for rows.Next() {
			var id, name string
			var url *string
			if err := rows.Scan(&id, &name, &url); err != nil {
				continue
			}
			out = append(out, gin.H{"id": id, "name": name, "preview_url": url})
		}
		c.JSON(http.StatusOK, gin.H{"items": out})
	})
}
