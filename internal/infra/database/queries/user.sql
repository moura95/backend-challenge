-- name: CreateUser :one
INSERT INTO users (email, password, name)
VALUES ($1, $2,$3)
RETURNING *;

-- name: GetUserByID :one
SELECT *
FROM users
WHERE users.uuid = $1;


-- name: GetUserPasswordByID :one
SELECT password
FROM users
WHERE uuid = $1;

-- name: GetUserByEmail :one
SELECT *
FROM users
WHERE email = $1;

-- name: RemoveUserByID :one
DELETE
FROM users
WHERE uuid = $1
RETURNING *;


-- name: UpdateUserByUUID :exec
UPDATE users
SET cpf        = COALESCE(sqlc.narg('cpf'), cpf),
    name   = COALESCE(sqlc.narg('name'), name),
    email = COALESCE(sqlc.narg('email'), email),
    updated_at = NOW()
WHERE uuid = $1;
