package util

import (
	"net"
	"net/http"
	"strings"
)

func ClientIP(r *http.Request) string {
	// 1) X-Forwarded-For 사용 시, 첫 번째 IP를 추출하고 포트가 있으면 제거
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		first := strings.TrimSpace(strings.Split(xff, ",")[0])
		if host, _, err := net.SplitHostPort(first); err == nil {
			return host
		}
		return first
	}
	// 2) RemoteAddr은 보통 host:port 이므로 host만 반환
	if host, _, err := net.SplitHostPort(r.RemoteAddr); err == nil {
		return host
	}
	// 3) 예외적으로 포트 분리가 불가한 경우 원문 반환
	return r.RemoteAddr
}
