package postgres

import (
	"context"
	"errors"
	"fmt"

	"ecommerce-api/internal"
	"ecommerce-api/internal/repository"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type categoryRepository struct {
	db *pgxpool.Pool
}

func NewCategoryRepository(db *pgxpool.Pool) repository.CategoryRepository {
	return &categoryRepository{db: db}
}

func (r *categoryRepository) Create(ctx context.Context, cat *domain.Category) error {
	_, err := r.db.Exec(ctx, `
		INSERT INTO categories (id, name, slug, parent_id, is_active, created_at)
		VALUES ($1, $2, $3, $4, $5, $6)`,
		cat.ID, cat.Name, cat.Slug, cat.ParentID, cat.IsActive, cat.CreatedAt,
	)
	if err != nil {
		return fmt.Errorf("categoryRepository.Create: %w", err)
	}
	return nil
}

func (r *categoryRepository) FindByID(ctx context.Context, id uuid.UUID) (*domain.Category, error) {
	var cat domain.Category
	err := r.db.QueryRow(ctx,
		`SELECT id, name, slug, parent_id, is_active, created_at FROM categories WHERE id = $1`, id,
	).Scan(&cat.ID, &cat.Name, &cat.Slug, &cat.ParentID, &cat.IsActive, &cat.CreatedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("categoryRepository.FindByID: %w", err)
	}
	return &cat, nil
}

func (r *categoryRepository) FindAll(ctx context.Context) ([]domain.Category, error) {
	rows, err := r.db.Query(ctx,
		`SELECT id, name, slug, parent_id, is_active, created_at FROM categories ORDER BY name ASC`,
	)
	if err != nil {
		return nil, fmt.Errorf("categoryRepository.FindAll: %w", err)
	}
	defer rows.Close()

	var cats []domain.Category
	for rows.Next() {
		var cat domain.Category
		if err := rows.Scan(&cat.ID, &cat.Name, &cat.Slug, &cat.ParentID, &cat.IsActive, &cat.CreatedAt); err != nil {
			return nil, err
		}
		cats = append(cats, cat)
	}
	return cats, nil
}

func (r *categoryRepository) Update(ctx context.Context, cat *domain.Category) error {
	_, err := r.db.Exec(ctx,
		`UPDATE categories SET name = $1, slug = $2, parent_id = $3, is_active = $4 WHERE id = $5`,
		cat.Name, cat.Slug, cat.ParentID, cat.IsActive, cat.ID,
	)
	return err
}

func (r *categoryRepository) Delete(ctx context.Context, id uuid.UUID) error {
	_, err := r.db.Exec(ctx, `DELETE FROM categories WHERE id = $1`, id)
	return err
}
