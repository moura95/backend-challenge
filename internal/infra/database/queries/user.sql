-- name: CreateUser :one
INSERT INTO users (email, password, name)
VALUES ($1, $2, $3)
RETURNING *;

-- name: GetUserByID :one
SELECT *
FROM users
WHERE users.uuid = $1;

-- name: GetUserByEmail :one
SELECT *
FROM users
WHERE email = $1;

-- name: GetUserPasswordByID :one
SELECT password
FROM users
WHERE uuid = $1;

-- name: RemoveUserByID :one
DELETE
FROM users
WHERE uuid = $1
RETURNING *;

-- name: UpdateUserByUUID :exec
UPDATE users
SET
    name   = COALESCE(sqlc.narg('name'), name),
    email = COALESCE(sqlc.narg('email'), email),
    updated_at = NOW()
WHERE uuid = $1;

-- name: EmailExists :one
SELECT EXISTS(SELECT 1 FROM users WHERE email = $1);

-- name: ListUsers :many
SELECT uuid, name, email, created_at, updated_at
FROM users
WHERE
    CASE
        WHEN sqlc.narg('search')::text IS NOT NULL THEN
            (name ILIKE '%' || sqlc.narg('search')::text || '%' OR
             email ILIKE '%' || sqlc.narg('search')::text || '%')
        ELSE TRUE
        END
ORDER BY created_at DESC
LIMIT sqlc.narg('limit')::int
    OFFSET sqlc.narg('offset')::int;