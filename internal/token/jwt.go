package token

import (
	"time"

	"github.com/golang-jwt/jwt/v5"
)

func Sign(secret, uid, phone string, ttl time.Duration) (string, time.Time, error) {
	now := time.Now()
	exp := now.Add(ttl)
	claims := jwt.MapClaims{
		"uid":   uid,
		"phone": phone,
		"iat":   now.Unix(),
		"nbf":   now.Unix(),
		"exp":   exp.Unix(),
	}
	t := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signed, err := t.SignedString([]byte(secret))
	return signed, exp, err
}
