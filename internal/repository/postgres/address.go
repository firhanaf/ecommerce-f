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

type addressRepository struct {
	db *pgxpool.Pool
}

func NewAddressRepository(db *pgxpool.Pool) repository.AddressRepository {
	return &addressRepository{db: db}
}

func (r *addressRepository) Create(ctx context.Context, addr *domain.Address) error {
	query := `
		INSERT INTO addresses (
			id, user_id, recipient_name, phone, street,
			city, province, postal_code, is_default, created_at, updated_at
		) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11)
	`
	_, err := r.db.Exec(ctx, query,
		addr.ID, addr.UserID, addr.RecipientName, addr.Phone,
		addr.Street, addr.City, addr.Province, addr.PostalCode,
		addr.IsDefault, addr.CreatedAt, addr.UpdatedAt,
	)
	if err != nil {
		return fmt.Errorf("addressRepository.Create: %w", err)
	}
	return nil
}

func (r *addressRepository) FindByID(ctx context.Context, id uuid.UUID) (*domain.Address, error) {
	query := `
		SELECT id, user_id, recipient_name, phone, street,
		       city, province, postal_code, is_default, created_at, updated_at
		FROM addresses WHERE id = $1
	`
	row := r.db.QueryRow(ctx, query, id)
	return scanAddress(row)
}

func (r *addressRepository) FindByUserID(ctx context.Context, userID uuid.UUID) ([]domain.Address, error) {
	query := `
		SELECT id, user_id, recipient_name, phone, street,
		       city, province, postal_code, is_default, created_at, updated_at
		FROM addresses
		WHERE user_id = $1
		ORDER BY is_default DESC, created_at DESC
	`
	rows, err := r.db.Query(ctx, query, userID)
	if err != nil {
		return nil, fmt.Errorf("addressRepository.FindByUserID: %w", err)
	}
	defer rows.Close()

	var addrs []domain.Address
	for rows.Next() {
		var a domain.Address
		if err := rows.Scan(
			&a.ID, &a.UserID, &a.RecipientName, &a.Phone,
			&a.Street, &a.City, &a.Province, &a.PostalCode,
			&a.IsDefault, &a.CreatedAt, &a.UpdatedAt,
		); err != nil {
			return nil, err
		}
		addrs = append(addrs, a)
	}
	return addrs, nil
}

func (r *addressRepository) Update(ctx context.Context, addr *domain.Address) error {
	query := `
		UPDATE addresses
		SET recipient_name = $1, phone = $2, street = $3, city = $4,
		    province = $5, postal_code = $6, updated_at = $7
		WHERE id = $8 AND user_id = $9
	`
	_, err := r.db.Exec(ctx, query,
		addr.RecipientName, addr.Phone, addr.Street, addr.City,
		addr.Province, addr.PostalCode, time.Now(),
		addr.ID, addr.UserID,
	)
	return err
}

func (r *addressRepository) Delete(ctx context.Context, id uuid.UUID) error {
	_, err := r.db.Exec(ctx, "DELETE FROM addresses WHERE id = $1", id)
	return err
}

func (r *addressRepository) SetDefault(ctx context.Context, userID, addressID uuid.UUID) error {
	// Gunakan transaction agar atomic: unset semua dulu, baru set satu
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return fmt.Errorf("addressRepository.SetDefault begin: %w", err)
	}
	defer tx.Rollback(ctx)

	_, err = tx.Exec(ctx,
		`UPDATE addresses SET is_default = false, updated_at = $1 WHERE user_id = $2`,
		time.Now(), userID,
	)
	if err != nil {
		return err
	}

	_, err = tx.Exec(ctx,
		`UPDATE addresses SET is_default = true, updated_at = $1 WHERE id = $2 AND user_id = $3`,
		time.Now(), addressID, userID,
	)
	if err != nil {
		return err
	}

	return tx.Commit(ctx)
}

func scanAddress(row pgx.Row) (*domain.Address, error) {
	var a domain.Address
	err := row.Scan(
		&a.ID, &a.UserID, &a.RecipientName, &a.Phone,
		&a.Street, &a.City, &a.Province, &a.PostalCode,
		&a.IsDefault, &a.CreatedAt, &a.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("scanAddress: %w", err)
	}
	return &a, nil
}
