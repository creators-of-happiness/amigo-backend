package otp

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

func LogRequest(ctx context.Context, pool *pgxpool.Pool, phone, purpose, fixed string, expMin int, ip, ua string) error {
	id := uuid.New().String()
	exp := time.Now().Add(time.Duration(expMin) * time.Minute)

	hint := ""
	if len(fixed) >= 3 {
		hint = "***" + fixed[len(fixed)-3:]
	}
	_, err := pool.Exec(ctx, `
		INSERT INTO otp_requests (id, phone, purpose, code_hint, expires_at, ip, user_agent, created_at)
		VALUES ($1, $2, $3, $4, $5, $6::inet, $7, now())
	`, id, phone, purpose, hint, exp, ip, ua)
	return err
}
