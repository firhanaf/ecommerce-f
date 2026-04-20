package postgres

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"ecommerce-api/internal"
	"ecommerce-api/internal/repository"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// ─── Product ─────────────────────────────────────────────────────────────────

type productRepository struct {
	db *pgxpool.Pool
}

func NewProductRepository(db *pgxpool.Pool) repository.ProductRepository {
	return &productRepository{db: db}
}

func (r *productRepository) Create(ctx context.Context, p *domain.Product) error {
	query := `
		INSERT INTO products (id, name, slug, description, category_id, is_active, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
	`
	_, err := r.db.Exec(ctx, query,
		p.ID, p.Name, p.Slug, p.Description,
		p.CategoryID, p.IsActive, p.CreatedAt, p.UpdatedAt,
	)
	if err != nil {
		return fmt.Errorf("productRepository.Create: %w", err)
	}
	return nil
}

func (r *productRepository) FindByID(ctx context.Context, id uuid.UUID) (*domain.Product, error) {
	query := `
		SELECT id, name, slug, description, category_id, is_active, created_at, updated_at
		FROM products
		WHERE id = $1
	`
	row := r.db.QueryRow(ctx, query, id)
	return scanProduct(row)
}

func (r *productRepository) FindBySlug(ctx context.Context, slug string) (*domain.Product, error) {
	query := `
		SELECT id, name, slug, description, category_id, is_active, created_at, updated_at
		FROM products
		WHERE slug = $1 AND is_active = true
	`
	row := r.db.QueryRow(ctx, query, slug)
	return scanProduct(row)
}

// FindAll dengan dynamic filter — tidak pakai ORM tapi tetap bersih
func (r *productRepository) FindAll(ctx context.Context, filter repository.ProductFilter) ([]domain.Product, int, error) {
	// Build WHERE clause secara dinamis
	conditions := []string{"1=1"}
	args := []any{}
	argIdx := 1

	if filter.CategoryID != nil {
		conditions = append(conditions, fmt.Sprintf("category_id = $%d", argIdx))
		args = append(args, *filter.CategoryID)
		argIdx++
	}

	if filter.Search != "" {
		conditions = append(conditions, fmt.Sprintf("name ILIKE $%d", argIdx))
		args = append(args, "%"+filter.Search+"%")
		argIdx++
	}

	if filter.IsActive != nil {
		conditions = append(conditions, fmt.Sprintf("is_active = $%d", argIdx))
		args = append(args, *filter.IsActive)
		argIdx++
	}

	where := strings.Join(conditions, " AND ")

	// Count total untuk pagination
	countQuery := fmt.Sprintf("SELECT COUNT(*) FROM products WHERE %s", where)
	var total int
	if err := r.db.QueryRow(ctx, countQuery, args...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("productRepository.FindAll count: %w", err)
	}

	// Pagination
	if filter.Limit == 0 {
		filter.Limit = 20
	}
	offset := (filter.Page - 1) * filter.Limit

	dataQuery := fmt.Sprintf(`
		SELECT id, name, slug, description, category_id, is_active, created_at, updated_at
		FROM products
		WHERE %s
		ORDER BY created_at DESC
		LIMIT $%d OFFSET $%d
	`, where, argIdx, argIdx+1)

	args = append(args, filter.Limit, offset)

	rows, err := r.db.Query(ctx, dataQuery, args...)
	if err != nil {
		return nil, 0, fmt.Errorf("productRepository.FindAll query: %w", err)
	}
	defer rows.Close()

	var products []domain.Product
	for rows.Next() {
		var p domain.Product
		if err := rows.Scan(
			&p.ID, &p.Name, &p.Slug, &p.Description,
			&p.CategoryID, &p.IsActive, &p.CreatedAt, &p.UpdatedAt,
		); err != nil {
			return nil, 0, err
		}
		products = append(products, p)
	}

	return products, total, nil
}

func (r *productRepository) Update(ctx context.Context, p *domain.Product) error {
	query := `
		UPDATE products
		SET name = $1, description = $2, category_id = $3, is_active = $4, updated_at = $5
		WHERE id = $6
	`
	_, err := r.db.Exec(ctx, query,
		p.Name, p.Description, p.CategoryID, p.IsActive, time.Now(), p.ID,
	)
	return err
}

func (r *productRepository) Delete(ctx context.Context, id uuid.UUID) error {
	// soft delete — tinggal set is_active = false
	query := `UPDATE products SET is_active = false, updated_at = $1 WHERE id = $2`
	_, err := r.db.Exec(ctx, query, time.Now(), id)
	return err
}

func scanProduct(row pgx.Row) (*domain.Product, error) {
	var p domain.Product
	err := row.Scan(
		&p.ID, &p.Name, &p.Slug, &p.Description,
		&p.CategoryID, &p.IsActive, &p.CreatedAt, &p.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("scanProduct: %w", err)
	}
	return &p, nil
}

// ─── Product Variant ─────────────────────────────────────────────────────────

type productVariantRepository struct {
	db *pgxpool.Pool
}

func NewProductVariantRepository(db *pgxpool.Pool) repository.ProductVariantRepository {
	return &productVariantRepository{db: db}
}

func (r *productVariantRepository) Create(ctx context.Context, v *domain.ProductVariant) error {
	query := `
		INSERT INTO product_variants (id, product_id, name, sku, price, stock, is_active, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
	`
	_, err := r.db.Exec(ctx, query,
		v.ID, v.ProductID, v.Name, v.SKU,
		v.Price, v.Stock, v.IsActive, v.CreatedAt, v.UpdatedAt,
	)
	return err
}

func (r *productVariantRepository) FindByProductID(ctx context.Context, productID uuid.UUID) ([]domain.ProductVariant, error) {
	query := `
		SELECT id, product_id, name, sku, price, stock, is_active, created_at, updated_at
		FROM product_variants
		WHERE product_id = $1 AND is_active = true
		ORDER BY created_at ASC
	`
	rows, err := r.db.Query(ctx, query, productID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var variants []domain.ProductVariant
	for rows.Next() {
		var v domain.ProductVariant
		if err := rows.Scan(
			&v.ID, &v.ProductID, &v.Name, &v.SKU,
			&v.Price, &v.Stock, &v.IsActive, &v.CreatedAt, &v.UpdatedAt,
		); err != nil {
			return nil, err
		}
		variants = append(variants, v)
	}
	return variants, nil
}

func (r *productVariantRepository) FindByID(ctx context.Context, id uuid.UUID) (*domain.ProductVariant, error) {
	query := `
		SELECT id, product_id, name, sku, price, stock, is_active, created_at, updated_at
		FROM product_variants WHERE id = $1
	`
	var v domain.ProductVariant
	err := r.db.QueryRow(ctx, query, id).Scan(
		&v.ID, &v.ProductID, &v.Name, &v.SKU,
		&v.Price, &v.Stock, &v.IsActive, &v.CreatedAt, &v.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	return &v, nil
}

func (r *productVariantRepository) Update(ctx context.Context, v *domain.ProductVariant) error {
	query := `
		UPDATE product_variants
		SET name = $1, sku = $2, price = $3, stock = $4, is_active = $5, updated_at = $6
		WHERE id = $7
	`
	_, err := r.db.Exec(ctx, query,
		v.Name, v.SKU, v.Price, v.Stock, v.IsActive, time.Now(), v.ID,
	)
	return err
}

// DecrementStock menggunakan atomic UPDATE dengan CHECK agar tidak bisa negatif
// Ini pola paling aman untuk race condition saat checkout bersamaan
func (r *productVariantRepository) DecrementStock(ctx context.Context, variantID uuid.UUID, qty int) error {
	query := `
		UPDATE product_variants
		SET stock = stock - $1, updated_at = $2
		WHERE id = $3 AND stock >= $1
	`
	result, err := r.db.Exec(ctx, query, qty, time.Now(), variantID)
	if err != nil {
		return fmt.Errorf("DecrementStock: %w", err)
	}

	// Jika 0 rows affected berarti stok tidak cukup
	if result.RowsAffected() == 0 {
		return fmt.Errorf("insufficient stock for variant %s", variantID)
	}
	return nil
}

func (r *productVariantRepository) Delete(ctx context.Context, id uuid.UUID) error {
	query := `UPDATE product_variants SET is_active = false, updated_at = $1 WHERE id = $2`
	_, err := r.db.Exec(ctx, query, time.Now(), id)
	return err
}

// ─── Product Image ────────────────────────────────────────────────────────────

type productImageRepository struct {
	db *pgxpool.Pool
}

func NewProductImageRepository(db *pgxpool.Pool) repository.ProductImageRepository {
	return &productImageRepository{db: db}
}

func (r *productImageRepository) Create(ctx context.Context, img *domain.ProductImage) error {
	query := `
		INSERT INTO product_images (id, product_id, url, is_primary, sort_order, created_at)
		VALUES ($1, $2, $3, $4, $5, $6)
	`
	_, err := r.db.Exec(ctx, query,
		img.ID, img.ProductID, img.URL, img.IsPrimary, img.SortOrder, img.CreatedAt,
	)
	return err
}

func (r *productImageRepository) FindByID(ctx context.Context, id uuid.UUID) (*domain.ProductImage, error) {
	query := `
		SELECT id, product_id, url, is_primary, sort_order, created_at
		FROM product_images WHERE id = $1
	`
	var img domain.ProductImage
	err := r.db.QueryRow(ctx, query, id).Scan(
		&img.ID, &img.ProductID, &img.URL, &img.IsPrimary, &img.SortOrder, &img.CreatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	return &img, nil
}

func (r *productImageRepository) FindByProductID(ctx context.Context, productID uuid.UUID) ([]domain.ProductImage, error) {
	query := `
		SELECT id, product_id, url, is_primary, sort_order, created_at
		FROM product_images
		WHERE product_id = $1
		ORDER BY sort_order ASC, is_primary DESC
	`
	rows, err := r.db.Query(ctx, query, productID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var images []domain.ProductImage
	for rows.Next() {
		var img domain.ProductImage
		if err := rows.Scan(&img.ID, &img.ProductID, &img.URL, &img.IsPrimary, &img.SortOrder, &img.CreatedAt); err != nil {
			return nil, err
		}
		images = append(images, img)
	}
	return images, nil
}

func (r *productImageRepository) Delete(ctx context.Context, id uuid.UUID) error {
	_, err := r.db.Exec(ctx, "DELETE FROM product_images WHERE id = $1", id)
	return err
}
