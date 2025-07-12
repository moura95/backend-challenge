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

CREATE TABLE IF NOT EXISTS emails (
                                      uuid         UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
                                      to_email     VARCHAR(255) NOT NULL,
                                      subject      VARCHAR(255) NOT NULL,
                                      body         TEXT NOT NULL,
                                      type         VARCHAR(50) NOT NULL,
                                      status       VARCHAR(50) NOT NULL DEFAULT 'pending',
                                      attempts     INTEGER NOT NULL DEFAULT 0,
                                      max_attempts INTEGER NOT NULL DEFAULT 3,
                                      error_msg    TEXT,
                                      sent_at      TIMESTAMPTZ,
                                      created_at   TIMESTAMPTZ NOT NULL DEFAULT NOW(),
                                      updated_at   TIMESTAMPTZ NOT NULL DEFAULT NOW()
);



-- Índices essenciais
CREATE INDEX idx_users_email ON users(email);
CREATE INDEX idx_users_email_password ON users(email, password);
CREATE INDEX idx_user_sessions_user_uuid ON user_sessions(user_uuid);
CREATE INDEX idx_user_sessions_refresh_token ON user_sessions(refresh_token);
-- Índices para performance
CREATE INDEX idx_emails_status ON emails(status);
CREATE INDEX idx_emails_type ON emails(type);
CREATE INDEX idx_emails_to_email ON emails(to_email);
CREATE INDEX idx_emails_created_at ON emails(created_at);