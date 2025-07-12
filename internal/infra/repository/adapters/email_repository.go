package adapters

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/google/uuid"
	"github.com/moura95/backend-challenge/internal/domain/email"
	"github.com/moura95/backend-challenge/internal/infra/repository/sqlc"
)

type emailRepository struct {
	db *sqlc.Queries
}

func NewEmailRepository(db *sqlc.Queries) email.Repository {
	return &emailRepository{
		db: db,
	}
}

func (r *emailRepository) Create(ctx context.Context, domainEmail *email.Email) error {
	params := sqlc.CreateEmailParams{
		ToEmail:     domainEmail.To,
		Subject:     domainEmail.Subject,
		Body:        domainEmail.Body,
		Type:        string(domainEmail.Type),
		Status:      string(domainEmail.Status),
		Attempts:    int32(domainEmail.Attempts),
		MaxAttempts: int32(domainEmail.MaxAttempts),
	}

	sqlcEmail, err := r.db.CreateEmail(ctx, params)
	if err != nil {
		return fmt.Errorf("repository: create email failed: %w", err)
	}

	domainEmail.ID = sqlcEmail.Uuid
	domainEmail.CreatedAt = sqlcEmail.CreatedAt

	return nil
}

func (r *emailRepository) GetByID(ctx context.Context, id uuid.UUID) (*email.Email, error) {
	sqlcEmail, err := r.db.GetEmailByID(ctx, id)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("repository: get email by id failed: email not found")
		}
		return nil, fmt.Errorf("repository: get email by id failed: %w", err)
	}

	return sqlcEmailToDomain(sqlcEmail), nil
}

func (r *emailRepository) Update(ctx context.Context, domainEmail *email.Email) error {
	params := sqlc.UpdateEmailParams{
		Uuid: domainEmail.ID,
		Status: sql.NullString{
			String: string(domainEmail.Status),
			Valid:  true,
		},
		Attempts: sql.NullInt32{
			Int32: int32(domainEmail.Attempts),
			Valid: true,
		},
	}

	if domainEmail.ErrorMsg != "" {
		params.ErrorMsg = sql.NullString{
			String: domainEmail.ErrorMsg,
			Valid:  true,
		}
	}

	if domainEmail.SentAt != nil {
		params.SentAt = sql.NullTime{
			Time:  *domainEmail.SentAt,
			Valid: true,
		}
	}

	err := r.db.UpdateEmail(ctx, params)
	if err != nil {
		if err == sql.ErrNoRows {
			return fmt.Errorf("repository: update email failed: email not found")
		}
		return fmt.Errorf("repository: update email failed: %w", err)
	}

	return nil
}

func (r *emailRepository) GetPendingEmails(ctx context.Context, limit int) ([]*email.Email, error) {
	if limit <= 0 {
		limit = 10
	}

	sqlcEmails, err := r.db.GetPendingEmails(ctx, int32(limit))
	if err != nil {
		return nil, fmt.Errorf("repository: get pending emails failed: %w", err)
	}

	emails := make([]*email.Email, len(sqlcEmails))
	for i, sqlcEmail := range sqlcEmails {
		emails[i] = sqlcEmailToDomain(sqlcEmail)
	}

	return emails, nil
}

func sqlcEmailToDomain(sqlcEmail sqlc.Email) *email.Email {
	domainEmail := &email.Email{
		ID:          sqlcEmail.Uuid,
		To:          sqlcEmail.ToEmail,
		Subject:     sqlcEmail.Subject,
		Body:        sqlcEmail.Body,
		Type:        email.EmailType(sqlcEmail.Type),
		Status:      email.Status(sqlcEmail.Status),
		Attempts:    int(sqlcEmail.Attempts),
		MaxAttempts: int(sqlcEmail.MaxAttempts),
		CreatedAt:   sqlcEmail.CreatedAt,
	}

	if sqlcEmail.ErrorMsg.Valid {
		domainEmail.ErrorMsg = sqlcEmail.ErrorMsg.String
	}

	if sqlcEmail.SentAt.Valid {
		domainEmail.SentAt = &sqlcEmail.SentAt.Time
	}

	return domainEmail
}
