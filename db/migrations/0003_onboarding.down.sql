-- 온보딩 관련 시드만 롤백(운영에선 실제 서비스 데이터 삭제 주의)
ALTER TABLE user_profile DROP CONSTRAINT IF EXISTS chk_user_profile_gender;
ALTER TABLE user_profile DROP CONSTRAINT IF EXISTS chk_user_profile_birth;

DROP INDEX IF EXISTS idx_user_profile_region;
DROP INDEX IF EXISTS idx_user_job_category;

-- 시드 제거(데모 목적)
DELETE FROM bg_item WHERE name LIKE 'Background %';
DELETE FROM character_item WHERE name LIKE 'Character %' AND category_code='basic';
DELETE FROM character_category WHERE code='basic';
-- media_asset 은 공용일 수 있어 보존
DELETE FROM job_category WHERE code IN ('dev','design','pm','sales');
DELETE FROM region WHERE name IN ('Seoul','Busan','Incheon');
