-- OTP request log (dev / metrics / rate-limit)
CREATE TABLE IF NOT EXISTS otp_requests (
  id         UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  phone      TEXT NOT NULL,
  purpose    TEXT NOT NULL DEFAULT 'login',
  code_hint  TEXT,
  expires_at TIMESTAMPTZ NOT NULL,
  used_at    TIMESTAMPTZ,
  ip         INET,
  user_agent TEXT,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_otp_requests_phone_created
  ON otp_requests (phone, created_at DESC);
