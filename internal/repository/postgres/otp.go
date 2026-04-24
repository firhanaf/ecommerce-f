package postgres

import (
	"context"
	"errors"
	"fmt"
	"time"

	"ecommerce-api/internal"
	"ecommerce-api/internal/repository"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type otpRepository struct {
	db *pgxpool.Pool
}

func NewOTPRepository(db *pgxpool.Pool) repository.OTPRepository {
	return &otpRepository{db: db}
}

func (r *otpRepository) Create(ctx context.Context, otp *domain.OTPToken) error {
	query := `
		INSERT INTO otp_tokens (id, user_id, code, type, expires_at, attempts, created_at)
		VALUES ($1, $2, $3, $4, $5, 0, $6)
	`
	_, err := r.db.Exec(ctx, query,
		otp.ID, otp.UserID, otp.Code, otp.Type,
		otp.ExpiresAt, otp.CreatedAt,
	)
	if err != nil {
		return fmt.Errorf("otpRepository.Create: %w", err)
	}
	return nil
}

// FindLatest mengambil OTP terbaru yang belum dipakai untuk user + type tertentu
func (r *otpRepository) FindLatest(ctx context.Context, userID uuid.UUID, otpType string) (*domain.OTPToken, error) {
	query := `
		SELECT id, user_id, code, type, expires_at, used_at, attempts, created_at
		FROM otp_tokens
		WHERE user_id = $1 AND type = $2 AND used_at IS NULL
		ORDER BY created_at DESC
		LIMIT 1
	`
	var otp domain.OTPToken
	err := r.db.QueryRow(ctx, query, userID, otpType).Scan(
		&otp.ID, &otp.UserID, &otp.Code, &otp.Type,
		&otp.ExpiresAt, &otp.UsedAt, &otp.Attempts, &otp.CreatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("otpRepository.FindLatest: %w", err)
	}
	return &otp, nil
}

func (r *otpRepository) IncrementAttempts(ctx context.Context, id uuid.UUID) error {
	query := `UPDATE otp_tokens SET attempts = attempts + 1 WHERE id = $1`
	_, err := r.db.Exec(ctx, query, id)
	return err
}

func (r *otpRepository) MarkUsed(ctx context.Context, id uuid.UUID) error {
	query := `UPDATE otp_tokens SET used_at = $1 WHERE id = $2`
	_, err := r.db.Exec(ctx, query, time.Now(), id)
	return err
}

// CountRecent menghitung berapa kali OTP dikirim ke user dalam window waktu tertentu
// Dipakai untuk rate limiting resend OTP
func (r *otpRepository) CountRecent(ctx context.Context, userID uuid.UUID, otpType string, since time.Time) (int, error) {
	query := `
		SELECT COUNT(*) FROM otp_tokens
		WHERE user_id = $1 AND type = $2 AND created_at >= $3
	`
	var count int
	err := r.db.QueryRow(ctx, query, userID, otpType, since).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("otpRepository.CountRecent: %w", err)
	}
	return count, nil
}
