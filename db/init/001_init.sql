CREATE TABLE app_users (
  id          UUID PRIMARY KEY,
  phone       TEXT UNIQUE,
  nickname    TEXT UNIQUE,
  created_at  TIMESTAMPTZ DEFAULT now(),
  updated_at  TIMESTAMPTZ DEFAULT now()
);

CREATE TABLE user_profile (
  user_id   UUID PRIMARY KEY REFERENCES app_users(id) ON DELETE CASCADE,
  gender    TEXT,
  birth_date DATE,
  region_id  INTEGER,
  profile_image_id UUID,
  created_at TIMESTAMPTZ DEFAULT now(),
  updated_at TIMESTAMPTZ DEFAULT now()
);

CREATE TABLE region (
  id        SERIAL PRIMARY KEY,
  name      TEXT NOT NULL,
  latitude  NUMERIC(9,6),
  longitude NUMERIC(9,6)
);

CREATE TABLE job_category (
  code TEXT PRIMARY KEY,
  name TEXT NOT NULL
);

CREATE TABLE user_job (
  user_id   UUID PRIMARY KEY REFERENCES app_users(id) ON DELETE CASCADE,
  category  TEXT REFERENCES job_category(code),
  detail    TEXT,
  created_at TIMESTAMPTZ DEFAULT now()
);

CREATE TABLE media_asset (
  id   UUID PRIMARY KEY,
  kind TEXT NOT NULL,
  url  TEXT NOT NULL
);

CREATE TABLE character_category (
  code TEXT PRIMARY KEY,
  name TEXT NOT NULL
);

CREATE TABLE character_item (
  id UUID PRIMARY KEY,
  category_code TEXT REFERENCES character_category(code),
  name TEXT NOT NULL,
  preview_asset UUID REFERENCES media_asset(id)
);

CREATE TABLE bg_item (
  id UUID PRIMARY KEY,
  name TEXT NOT NULL,
  preview_asset UUID REFERENCES media_asset(id)
);

CREATE TABLE user_avatar (
  user_id UUID PRIMARY KEY REFERENCES app_users(id) ON DELETE CASCADE,
  category_code TEXT REFERENCES character_category(code),
  character_id UUID REFERENCES character_item(id),
  bg_id UUID REFERENCES bg_item(id),
  selected_at TIMESTAMPTZ DEFAULT now()
);

CREATE TABLE user_face_upload (
  id        UUID PRIMARY KEY,
  user_id   UUID REFERENCES app_users(id) ON DELETE CASCADE,
  url       TEXT NOT NULL,
  status    TEXT NOT NULL,
  created_at TIMESTAMPTZ DEFAULT now()
);

CREATE TABLE pref_type (
  code TEXT PRIMARY KEY,
  name TEXT NOT NULL
);

CREATE TABLE pref_item (
  id UUID PRIMARY KEY,
  type_code TEXT REFERENCES pref_type(code),
  name TEXT NOT NULL
);

CREATE TABLE user_pref (
  user_id UUID REFERENCES app_users(id) ON DELETE CASCADE,
  type_code TEXT REFERENCES pref_type(code),
  item_id UUID REFERENCES pref_item(id),
  PRIMARY KEY (user_id, type_code, item_id)
);

CREATE TABLE user_pref_custom (
  id UUID PRIMARY KEY,
  user_id UUID REFERENCES app_users(id) ON DELETE CASCADE,
  type_code TEXT REFERENCES pref_type(code),
  text TEXT NOT NULL
);
