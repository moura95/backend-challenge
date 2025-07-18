// Code generated by sqlc. DO NOT EDIT.
// versions:
//   sqlc v1.25.0
// source: email.sql

package sqlc

import (
	"context"
	"database/sql"

	"github.com/google/uuid"
)

const createEmail = `-- name: CreateEmail :one
INSERT INTO emails (to_email, subject, body, type, status, attempts, max_attempts)
VALUES ($1, $2, $3, $4, $5, $6, $7)
RETURNING uuid, to_email, subject, body, type, status, attempts, max_attempts, error_msg, sent_at, created_at, updated_at
`

type CreateEmailParams struct {
	ToEmail     string
	Subject     string
	Body        string
	Type        string
	Status      string
	Attempts    int32
	MaxAttempts int32
}

func (q *Queries) CreateEmail(ctx context.Context, arg CreateEmailParams) (Email, error) {
	row := q.db.QueryRowContext(ctx, createEmail,
		arg.ToEmail,
		arg.Subject,
		arg.Body,
		arg.Type,
		arg.Status,
		arg.Attempts,
		arg.MaxAttempts,
	)
	var i Email
	err := row.Scan(
		&i.Uuid,
		&i.ToEmail,
		&i.Subject,
		&i.Body,
		&i.Type,
		&i.Status,
		&i.Attempts,
		&i.MaxAttempts,
		&i.ErrorMsg,
		&i.SentAt,
		&i.CreatedAt,
		&i.UpdatedAt,
	)
	return i, err
}

const getEmailByID = `-- name: GetEmailByID :one
SELECT uuid, to_email, subject, body, type, status, attempts, max_attempts, error_msg, sent_at, created_at, updated_at
FROM emails
WHERE uuid = $1
`

func (q *Queries) GetEmailByID(ctx context.Context, argUuid uuid.UUID) (Email, error) {
	row := q.db.QueryRowContext(ctx, getEmailByID, argUuid)
	var i Email
	err := row.Scan(
		&i.Uuid,
		&i.ToEmail,
		&i.Subject,
		&i.Body,
		&i.Type,
		&i.Status,
		&i.Attempts,
		&i.MaxAttempts,
		&i.ErrorMsg,
		&i.SentAt,
		&i.CreatedAt,
		&i.UpdatedAt,
	)
	return i, err
}

const getPendingEmails = `-- name: GetPendingEmails :many
SELECT uuid, to_email, subject, body, type, status, attempts, max_attempts, error_msg, sent_at, created_at, updated_at
FROM emails
WHERE status = 'pending'
ORDER BY created_at ASC
LIMIT $1
`

func (q *Queries) GetPendingEmails(ctx context.Context, limit int32) ([]Email, error) {
	rows, err := q.db.QueryContext(ctx, getPendingEmails, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var items []Email
	for rows.Next() {
		var i Email
		if err := rows.Scan(
			&i.Uuid,
			&i.ToEmail,
			&i.Subject,
			&i.Body,
			&i.Type,
			&i.Status,
			&i.Attempts,
			&i.MaxAttempts,
			&i.ErrorMsg,
			&i.SentAt,
			&i.CreatedAt,
			&i.UpdatedAt,
		); err != nil {
			return nil, err
		}
		items = append(items, i)
	}
	if err := rows.Close(); err != nil {
		return nil, err
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return items, nil
}

const updateEmail = `-- name: UpdateEmail :exec
UPDATE emails
SET
    status = COALESCE($2, status),
    attempts = COALESCE($3, attempts),
    error_msg = COALESCE($4, error_msg),
    sent_at = COALESCE($5, sent_at),
    updated_at = NOW()
WHERE uuid = $1
`

type UpdateEmailParams struct {
	Uuid     uuid.UUID
	Status   sql.NullString
	Attempts sql.NullInt32
	ErrorMsg sql.NullString
	SentAt   sql.NullTime
}

func (q *Queries) UpdateEmail(ctx context.Context, arg UpdateEmailParams) error {
	_, err := q.db.ExecContext(ctx, updateEmail,
		arg.Uuid,
		arg.Status,
		arg.Attempts,
		arg.ErrorMsg,
		arg.SentAt,
	)
	return err
}
