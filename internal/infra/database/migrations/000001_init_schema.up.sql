SET TIME ZONE 'America/Sao_Paulo';
CREATE EXTENSION IF NOT EXISTS "uuid-ossp";

CREATE TABLE IF NOT EXISTS users (
                                     uuid         UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
                                     name         VARCHAR(255) NOT NULL,
                                     email        VARCHAR(100) NOT NULL UNIQUE,
                                     password     TEXT NOT NULL,
                                     created_at   TIMESTAMP NOT NULL DEFAULT NOW(),
                                     updated_at   TIMESTAMP NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS user_sessions (
                                             uuid          UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
                                             user_uuid     UUID NOT NULL,
                                             refresh_token VARCHAR NOT NULL,
                                             user_agent    VARCHAR NOT NULL,
                                             client_ip     VARCHAR NOT NULL,
                                             is_blocked    BOOLEAN NOT NULL DEFAULT false,
                                             expires_at    TIMESTAMPTZ NOT NULL,
                                             created_at    TIMESTAMPTZ NOT NULL DEFAULT NOW(),
                                             FOREIGN KEY (user_uuid) REFERENCES users(uuid) ON DELETE CASCADE
);

-- Índices essenciais
CREATE INDEX idx_users_email ON users(email);
CREATE INDEX idx_users_email_password ON users(email, password);
CREATE INDEX idx_user_sessions_user_uuid ON user_sessions(user_uuid);
CREATE INDEX idx_user_sessions_refresh_token ON user_sessions(refresh_token);