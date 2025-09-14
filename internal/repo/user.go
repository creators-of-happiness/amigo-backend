package repo

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
)

type User struct {
	ID       string
	Phone    string
	Nickname *string
}

func FindOrCreateUser(ctx context.Context, pool *pgxpool.Pool, phone, nickname string) (*User, error) {
	// DB가 UUID 기본값을 생성하며, phone UNIQUE에 대해 UPSERT 수행
	var u User
	err := pool.QueryRow(ctx, `
		INSERT INTO app_users (phone, nickname, created_at, updated_at)
		VALUES ($1, NULLIF($2,''), now(), now())
		ON CONFLICT (phone) DO UPDATE
		  SET nickname = COALESCE(EXCLUDED.nickname, app_users.nickname),
		      updated_at = now()
		RETURNING id, phone, nickname
	`, phone, nickname).Scan(&u.ID, &u.Phone, &u.Nickname)
	if err != nil {
		return nil, fmt.Errorf("create/find user: %w", err)
	}
	return &u, nil
}
