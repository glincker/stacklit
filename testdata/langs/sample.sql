CREATE TABLE users (
  id SERIAL PRIMARY KEY,
  name TEXT NOT NULL,
  email VARCHAR(255)
);

CREATE TABLE IF NOT EXISTS public.orders (
  id INT,
  user_id INT REFERENCES users(id)
);

CREATE VIEW active_users AS SELECT * FROM users WHERE id > 0;

CREATE OR REPLACE FUNCTION add_one(i integer) RETURNS integer AS $$ BEGIN RETURN i + 1; END; $$ LANGUAGE plpgsql;

CREATE INDEX idx_users_email ON users (email);
