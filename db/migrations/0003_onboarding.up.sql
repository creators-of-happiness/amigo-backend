-- 온보딩: 제약, 인덱스, 시드

-- 제약(자유 텍스트 유지 + 권장값)
ALTER TABLE user_profile
  ADD CONSTRAINT chk_user_profile_gender
  CHECK (gender IS NULL OR gender IN ('male','female','other'));

-- 생년월일 합리성(1900-01-01 ~ 오늘)
ALTER TABLE user_profile
  ADD CONSTRAINT chk_user_profile_birth
  CHECK (birth_date IS NULL OR (birth_date >= DATE '1900-01-01' AND birth_date <= CURRENT_DATE));

-- 시드: 지역/직업
INSERT INTO region (name, latitude, longitude) VALUES
  ('Seoul', 37.5665, 126.9780),
  ('Busan', 35.1796, 129.0756),
  ('Incheon', 37.4563, 126.7052)
ON CONFLICT (name) DO NOTHING;

INSERT INTO job_category (code, name) VALUES
  ('dev','Developer'),
  ('design','Designer'),
  ('pm','Product Manager'),
  ('sales','Sales')
ON CONFLICT (code) DO NOTHING;

-- 캐릭터 카테고리
INSERT INTO character_category (code, name) VALUES
  ('basic','Basic')
ON CONFLICT (code) DO NOTHING;

-- 미디어 에셋(프리뷰)
INSERT INTO media_asset (kind, url) VALUES
  ('image', 'https://example.com/char1.png'),
  ('image', 'https://example.com/char2.png'),
  ('image', 'https://example.com/bg1.png'),
  ('image', 'https://example.com/bg2.png');

-- 위 4개를 캐릭터/배경으로 연결
WITH assets AS (
  SELECT id, url,
         ROW_NUMBER() OVER (ORDER BY created_at, id) AS rn
  FROM media_asset
  WHERE url IN (
    'https://example.com/char1.png',
    'https://example.com/char2.png',
    'https://example.com/bg1.png',
    'https://example.com/bg2.png'
  )
)
INSERT INTO character_item (category_code, name, preview_asset)
SELECT 'basic', CONCAT('Character ', rn), id
FROM assets WHERE rn <= 2
ON CONFLICT (category_code, name) DO NOTHING;

WITH assets AS (
  SELECT id, url,
         ROW_NUMBER() OVER (ORDER BY created_at, id) AS rn
  FROM media_asset
  WHERE url IN (
    'https://example.com/char1.png',
    'https://example.com/char2.png',
    'https://example.com/bg1.png',
    'https://example.com/bg2.png'
  )
)
INSERT INTO bg_item (name, preview_asset)
SELECT CONCAT('Background ', rn - 2), id
FROM assets WHERE rn > 2 AND rn <= 4
ON CONFLICT (name) DO NOTHING;

-- 조회 성능 인덱스
CREATE INDEX IF NOT EXISTS idx_user_profile_region ON user_profile(region_id);
CREATE INDEX IF NOT EXISTS idx_user_job_category   ON user_job(category);
