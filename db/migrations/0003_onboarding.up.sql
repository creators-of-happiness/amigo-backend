-- 기본정보(온보딩)용 제약·인덱스·시드

-- 성별 제약(자유 텍스트를 유지하되 기본 3값 권장)
ALTER TABLE user_profile
  ADD CONSTRAINT chk_user_profile_gender
  CHECK (gender IS NULL OR gender IN ('male','female','other'));

-- 생년월일 합리성(1900-01-01 ~ 오늘)
ALTER TABLE user_profile
  ADD CONSTRAINT chk_user_profile_birth
  CHECK (birth_date IS NULL OR (birth_date >= DATE '1900-01-01' AND birth_date <= CURRENT_DATE));

-- 지역, 직업 카테고리 시드(예시)
INSERT INTO region (name, latitude, longitude) VALUES
  ('Seoul', 37.5665, 126.9780),
  ('Busan', 35.1796, 129.0756),
  ('Incheon', 37.4563, 126.7052)
ON CONFLICT DO NOTHING;

INSERT INTO job_category (code, name) VALUES
  ('dev','Developer'),
  ('design','Designer'),
  ('pm','Product Manager'),
  ('sales','Sales')
ON CONFLICT DO NOTHING;

-- 캐릭터/배경 샘플
INSERT INTO character_category (code, name) VALUES
  ('basic','Basic')
ON CONFLICT DO NOTHING;

-- 미디어 에셋 더미(프리뷰 URL 기준)
-- 주: 실제 저장 방식/스토리지는 서비스 정책에 맞춰 교체
INSERT INTO media_asset (id, kind, url) VALUES
  (gen_random_uuid(), 'image', 'https://example.com/char1.png'),
  (gen_random_uuid(), 'image', 'https://example.com/char2.png'),
  (gen_random_uuid(), 'image', 'https://example.com/bg1.png'),
  (gen_random_uuid(), 'image', 'https://example.com/bg2.png')
ON CONFLICT DO NOTHING;

-- 위 4개 중 앞 2개를 캐릭터, 뒤 2개를 배경으로 연결
WITH assets AS (
  SELECT id, url,
         ROW_NUMBER() OVER () AS rn
  FROM media_asset
  ORDER BY created_at NULLS FIRST, id
)
INSERT INTO character_item (id, category_code, name, preview_asset)
SELECT gen_random_uuid(), 'basic', CONCAT('Character ', rn), id
FROM assets WHERE rn <= 2
ON CONFLICT DO NOTHING;

WITH assets AS (
  SELECT id, url,
         ROW_NUMBER() OVER () AS rn
  FROM media_asset
  ORDER BY created_at NULLS FIRST, id
)
INSERT INTO bg_item (id, name, preview_asset)
SELECT gen_random_uuid(), CONCAT('Background ', rn-2), id
FROM assets WHERE rn > 2 AND rn <= 4
ON CONFLICT DO NOTHING;

-- 조회 성능 인덱스
CREATE INDEX IF NOT EXISTS idx_user_profile_region ON user_profile(region_id);
CREATE INDEX IF NOT EXISTS idx_user_job_category   ON user_job(category);

-- 사용자 프로필 사진 FK는 이미 profile_image_id -> media_asset(id)로 연결되어 있음
