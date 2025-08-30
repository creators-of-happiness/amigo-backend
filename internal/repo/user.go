package repo

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

type User struct {
	ID       string
	Phone    string
	Nickname *string
}

func FindOrCreateUser(ctx context.Context, pool *pgxpool.Pool, phone, nickname string) (*User, error) {
	var u User
	err := pool.QueryRow(ctx, `SELECT id, phone, nickname FROM app_users WHERE phone=$1`, phone).
		Scan(&u.ID, &u.Phone, &u.Nickname)
	if err == nil {
		return &u, nil
	}

	id := uuid.New().String()
	var nick *string
	if nickname != "" {
		nick = &nickname
	}
	_, err = pool.Exec(ctx, `
		INSERT INTO app_users (id, phone, nickname, created_at, updated_at)
		VALUES ($1, $2, $3, now(), now())
	`, id, phone, nick)
	if err != nil {
		err2 := pool.QueryRow(ctx, `SELECT id, phone, nickname FROM app_users WHERE phone=$1`, phone).
			Scan(&u.ID, &u.Phone, &u.Nickname)
		if err2 == nil {
			return &u, nil
		}
		return nil, fmt.Errorf("create user: %w", err)
	}
	return &User{ID: id, Phone: phone, Nickname: nick}, nil
}
