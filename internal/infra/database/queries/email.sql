-- name: CreateEmail :one
INSERT INTO emails (to_email, subject, body, type, status, attempts, max_attempts)
VALUES ($1, $2, $3, $4, $5, $6, $7)
RETURNING *;

-- name: GetEmailByID :one
SELECT *
FROM emails
WHERE uuid = $1;

-- name: UpdateEmail :exec
UPDATE emails
SET
    status = COALESCE(sqlc.narg('status'), status),
    attempts = COALESCE(sqlc.narg('attempts'), attempts),
    error_msg = COALESCE(sqlc.narg('error_msg'), error_msg),
    sent_at = COALESCE(sqlc.narg('sent_at'), sent_at),
    updated_at = NOW()
WHERE uuid = $1;

-- name: GetPendingEmails :many
SELECT *
FROM emails
WHERE status = 'pending'
ORDER BY created_at ASC
LIMIT $1;