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

type userRepository struct {
	db *pgxpool.Pool
}

func NewUserRepository(db *pgxpool.Pool) repository.UserRepository {
	return &userRepository{db: db}
}

func (r *userRepository) Create(ctx context.Context, user *domain.User) error {
	query := `
		INSERT INTO users (id, name, email, password_hash, role, phone, avatar_url, is_active, phone_verified, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
	`
	_, err := r.db.Exec(ctx, query,
		user.ID, user.Name, user.Email, user.PasswordHash,
		user.Role, user.Phone, user.AvatarURL, user.IsActive,
		user.PhoneVerified, user.CreatedAt, user.UpdatedAt,
	)
	if err != nil {
		return fmt.Errorf("userRepository.Create: %w", err)
	}
	return nil
}

func (r *userRepository) FindByID(ctx context.Context, id uuid.UUID) (*domain.User, error) {
	query := `
		SELECT id, name, email, password_hash, role, phone, avatar_url, is_active, phone_verified, created_at, updated_at
		FROM users
		WHERE id = $1
	`
	row := r.db.QueryRow(ctx, query, id)
	return scanUser(row)
}

func (r *userRepository) FindByEmail(ctx context.Context, email string) (*domain.User, error) {
	query := `
		SELECT id, name, email, password_hash, role, phone, avatar_url, is_active, phone_verified, created_at, updated_at
		FROM users
		WHERE email = $1
	`
	row := r.db.QueryRow(ctx, query, email)
	return scanUser(row)
}

func (r *userRepository) FindByPhone(ctx context.Context, phone string) (*domain.User, error) {
	query := `
		SELECT id, name, email, password_hash, role, phone, avatar_url, is_active, phone_verified, created_at, updated_at
		FROM users
		WHERE phone = $1
	`
	row := r.db.QueryRow(ctx, query, phone)
	return scanUser(row)
}

func (r *userRepository) Update(ctx context.Context, user *domain.User) error {
	query := `
		UPDATE users
		SET name = $1, phone = $2, avatar_url = $3, updated_at = $4
		WHERE id = $5
	`
	_, err := r.db.Exec(ctx, query,
		user.Name, user.Phone, user.AvatarURL, time.Now(), user.ID,
	)
	return err
}

func (r *userRepository) FindAll(ctx context.Context, page, limit int) ([]domain.User, int, error) {
	if limit == 0 {
		limit = 20
	}
	offset := (page - 1) * limit

	var total int
	if err := r.db.QueryRow(ctx, "SELECT COUNT(*) FROM users").Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("userRepository.FindAll count: %w", err)
	}

	query := `
		SELECT id, name, email, password_hash, role, phone, avatar_url, is_active, phone_verified, created_at, updated_at
		FROM users
		ORDER BY created_at DESC
		LIMIT $1 OFFSET $2
	`
	rows, err := r.db.Query(ctx, query, limit, offset)
	if err != nil {
		return nil, 0, fmt.Errorf("userRepository.FindAll: %w", err)
	}
	defer rows.Close()

	var users []domain.User
	for rows.Next() {
		var u domain.User
		if err := rows.Scan(
			&u.ID, &u.Name, &u.Email, &u.PasswordHash,
			&u.Role, &u.Phone, &u.AvatarURL, &u.IsActive,
			&u.PhoneVerified, &u.CreatedAt, &u.UpdatedAt,
		); err != nil {
			return nil, 0, err
		}
		users = append(users, u)
	}
	return users, total, nil
}

func (r *userRepository) UpdatePhoneVerified(ctx context.Context, id uuid.UUID) error {
	query := `UPDATE users SET phone_verified = true, updated_at = $1 WHERE id = $2`
	_, err := r.db.Exec(ctx, query, time.Now(), id)
	return err
}

func (r *userRepository) UpdateStatus(ctx context.Context, id uuid.UUID, isActive bool) error {
	query := `UPDATE users SET is_active = $1, updated_at = $2 WHERE id = $3`
	_, err := r.db.Exec(ctx, query, isActive, time.Now(), id)
	return err
}

func (r *userRepository) SoftDelete(ctx context.Context, id uuid.UUID) error {
	query := `UPDATE users SET is_active = false, updated_at = $1 WHERE id = $2`
	_, err := r.db.Exec(ctx, query, time.Now(), id)
	return err
}

// scanUser reusable scanner — satu tempat kalau kolom berubah
func scanUser(row pgx.Row) (*domain.User, error) {
	var u domain.User
	err := row.Scan(
		&u.ID, &u.Name, &u.Email, &u.PasswordHash,
		&u.Role, &u.Phone, &u.AvatarURL, &u.IsActive,
		&u.PhoneVerified, &u.CreatedAt, &u.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("scanUser: %w", err)
	}
	return &u, nil
}
