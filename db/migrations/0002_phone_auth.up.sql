-- 단순 OTP 요청 로깅 테이블 (임시 고정코드 기반이라 검증용 메트릭/리밋 용도)
CREATE TABLE IF NOT EXISTS otp_requests (
  id         UUID PRIMARY KEY,
  phone      TEXT NOT NULL,
  purpose    TEXT NOT NULL DEFAULT 'login',
  code_hint  TEXT,                           -- 저장시 마스킹된 힌트(예: "***000")
  expires_at TIMESTAMPTZ NOT NULL,
  used_at    TIMESTAMPTZ,
  ip         INET,
  user_agent TEXT,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_otp_requests_phone_created
  ON otp_requests (phone, created_at DESC);
