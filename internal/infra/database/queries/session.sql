-- name: CreateSession :one
INSERT INTO user_sessions (uuid,
                           user_uuid,
                           refresh_token,
                           user_agent,
                           client_ip,
                           is_blocked,
                           expires_at)
VALUES ($1, $2, $3, $4, $5, $6, $7)
RETURNING *;

-- name: GetSessionByID :one
SELECT *
FROM user_sessions
WHERE uuid = $1;