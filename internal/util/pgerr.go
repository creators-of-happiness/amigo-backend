package util

import "github.com/jackc/pgconn"

// 스키마 제약 위반/캐스팅 오류 등을 4xx로 분류
func IsClientInputError(err error) bool {
	if err == nil {
		return false
	}
	if pg, ok := err.(*pgconn.PgError); ok {
		switch pg.Code {
		case "23503", // foreign_key_violation
			"23514", // check_violation
			"23505", // unique_violation (옵션)
			"22007", // invalid_datetime_format
			"22P02": // invalid_text_representation (예: uuid/date 캐스트)
			return true
		}
	}
	return false
}

// Optionally map pg errors to HTTP + app code
func PGToHTTP(err error) (status int, code string) {
	if pg, ok := err.(*pgconn.PgError); ok {
		switch pg.Code {
		case "23505":
			return 409, "CONFLICT"
		case "23503", "23514", "22007", "22P02":
			return 400, "VALIDATION"
		}
	}
	return 500, "INTERNAL"
}
