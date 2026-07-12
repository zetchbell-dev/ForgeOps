package postgres

import (
	"context"
	"errors"

	"github.com/enterprise-cicd-platform/auth-service/internal/domain"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

// pgUniqueViolation is Postgres's SQLSTATE code for a unique constraint
// violation (23505). Checked by code, not by string-matching the driver's
// error message — the message text is locale-dependent and not a stable
// contract to branch on.
const pgUniqueViolation = "23505"

// CredentialRepository implements domain.CredentialRepository against the
// `auth.credentials` table (M2 §5, amended §11.1 to add login_identifier).
type CredentialRepository struct {
	pool *pgxpool.Pool
}

func NewCredentialRepository(pool *pgxpool.Pool) *CredentialRepository {
	return &CredentialRepository{pool: pool}
}

func (r *CredentialRepository) Create(ctx context.Context, cred domain.Credential) error {
	const query = `
		INSERT INTO auth.credentials (user_id, login_identifier, password_hash, status, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6)
	`
	_, err := r.pool.Exec(ctx, query,
		cred.UserID, cred.LoginIdentifier, cred.PasswordHash, string(cred.Status), cred.CreatedAt, cred.UpdatedAt,
	)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == pgUniqueViolation {
			// The `login_identifier` UNIQUE constraint (M2 §11.1) is what
			// makes this translation meaningful — a duplicate `user_id`
			// can't happen since it's generated fresh by Register
			// (usecase/register.go calls uuid.New()) before this insert.
			return domain.ErrEmailAlreadyExists
		}
		return err
	}
	return nil
}

func (r *CredentialRepository) GetByLoginIdentifier(ctx context.Context, identifier string) (domain.Credential, error) {
	const query = `
		SELECT user_id, login_identifier, password_hash, status, created_at, updated_at
		FROM auth.credentials
		WHERE login_identifier = $1
	`
	return r.scanOne(ctx, query, identifier)
}

func (r *CredentialRepository) GetByUserID(ctx context.Context, userID uuid.UUID) (domain.Credential, error) {
	const query = `
		SELECT user_id, login_identifier, password_hash, status, created_at, updated_at
		FROM auth.credentials
		WHERE user_id = $1
	`
	return r.scanOne(ctx, query, userID)
}

func (r *CredentialRepository) scanOne(ctx context.Context, query string, arg any) (domain.Credential, error) {
	var (
		cred   domain.Credential
		status string
	)

	err := r.pool.QueryRow(ctx, query, arg).Scan(
		&cred.UserID, &cred.LoginIdentifier, &cred.PasswordHash, &status, &cred.CreatedAt, &cred.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			// M2 §6: "user not found" and "password mismatch" return the
			// SAME error, on the SAME code path, deliberately — this
			// repository has no separate not-found sentinel to
			// translate to. ErrInvalidCredentials IS the not-found
			// signal here; usecase/login.go relies on exactly this to
			// keep the two cases indistinguishable to a caller.
			return domain.Credential{}, domain.ErrInvalidCredentials
		}
		return domain.Credential{}, err
	}

	cred.Status = domain.CredentialStatus(status)
	return cred, nil
}
