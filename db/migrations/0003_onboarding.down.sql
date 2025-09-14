-- 온보딩 제약/인덱스 제거 + 시드 일부 롤백(주의: 운영 데이터 삭제 위험)
ALTER TABLE user_profile DROP CONSTRAINT IF EXISTS chk_user_profile_gender;
ALTER TABLE user_profile DROP CONSTRAINT IF EXISTS chk_user_profile_birth;

DROP INDEX IF EXISTS idx_user_profile_region;
DROP INDEX IF EXISTS idx_user_job_category;

-- 캐릭터/배경/카테고리 시드 제거
DELETE FROM bg_item WHERE name LIKE 'Background %';
DELETE FROM character_item WHERE category_code='basic' AND name LIKE 'Character %';
DELETE FROM character_category WHERE code='basic';

-- media_asset 은 공용으로 남김
DELETE FROM job_category WHERE code IN ('dev','design','pm','sales');
DELETE FROM region WHERE name IN ('Seoul','Busan','Incheon');
