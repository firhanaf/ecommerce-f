package product

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/url"
	"strings"
	"time"

	domain "ecommerce-api/internal"
	"ecommerce-api/internal/repository"
	"ecommerce-api/pkg/storage"
	"github.com/google/uuid"
)

// ─── Errors ───────────────────────────────────────────────────────────────────

var (
	ErrProductNotFound   = errors.New("product not found")
	ErrSlugAlreadyExists = errors.New("product slug already exists")
	ErrVariantNotFound   = errors.New("variant not found")
	ErrImageNotFound     = errors.New("image not found")
)

// ─── Input DTOs ───────────────────────────────────────────────────────────────

type CreateProductInput struct {
	Name        string               `json:"name"`
	Description string               `json:"description"`
	CategoryID  *uuid.UUID           `json:"category_id"` // pointer karena opsional
	Variants    []CreateVariantInput `json:"variants"`
}

func (i CreateProductInput) Validate() error {
	if strings.TrimSpace(i.Name) == "" {
		return errors.New("product name is required")
	}
	if len(i.Variants) == 0 {
		return errors.New("at least one variant is required")
	}
	for idx, v := range i.Variants {
		if err := v.Validate(); err != nil {
			return fmt.Errorf("variant[%d]: %w", idx, err)
		}
	}
	return nil
}

type CreateVariantInput struct {
	Name  string `json:"name"`
	Price int64  `json:"price"`
	Stock uint   `json:"stock"`
	SKU   string `json:"sku"`
}

func (i CreateVariantInput) Validate() error {
	if strings.TrimSpace(i.Name) == "" {
		return errors.New("variant name is required")
	}
	if i.Price <= 0 {
		return errors.New("price must be greater than 0")
	}
	// stock boleh 0 (pre-order atau habis), tidak perlu validasi min
	return nil
}

type UpdateProductInput struct {
	Name        string     `json:"name"`
	Description string     `json:"description"`
	CategoryID  *uuid.UUID `json:"category_id"`
	IsActive    bool       `json:"is_active"`
}

func (i UpdateProductInput) Validate() error {
	if strings.TrimSpace(i.Name) == "" {
		return errors.New("product name is required")
	}
	return nil
}

type UpdateVariantInput struct {
	Name     string `json:"name"`
	Price    int64  `json:"price"`
	Stock    uint   `json:"stock"`
	SKU      string `json:"sku"`
	IsActive bool   `json:"is_active"`
}

func (i UpdateVariantInput) Validate() error {
	if strings.TrimSpace(i.Name) == "" {
		return errors.New("variant name is required")
	}
	if i.Price <= 0 {
		return errors.New("price must be greater than 0")
	}
	return nil
}

type ListProductFilter struct {
	CategoryID *uuid.UUID
	Search     string
	IsActive   *bool
	Page       int
	Limit      int
}

type UploadImageInput struct {
	ProductID uuid.UUID
	Filename  string
	Data      []byte
	Size      int64
	IsPrimary bool
}

// ─── Interface ────────────────────────────────────────────────────────────────

type UseCase interface {
	// Product CRUD
	CreateProduct(ctx context.Context, input CreateProductInput) (domain.Product, error)
	GetByID(ctx context.Context, id uuid.UUID) (domain.Product, error)
	GetBySlug(ctx context.Context, slug string) (domain.Product, error)
	ListAll(ctx context.Context, filter ListProductFilter) ([]domain.Product, int, error)
	UpdateByID(ctx context.Context, id uuid.UUID, input UpdateProductInput) (domain.Product, error)
	DeleteByID(ctx context.Context, id uuid.UUID) error

	// Variant
	CreateVariant(ctx context.Context, productID uuid.UUID, input CreateVariantInput) (domain.ProductVariant, error)
	UpdateVariant(ctx context.Context, productID, variantID uuid.UUID, input UpdateVariantInput) (domain.ProductVariant, error)
	DeleteVariant(ctx context.Context, productID, variantID uuid.UUID) error
	AdjustStock(ctx context.Context, productID, variantID uuid.UUID, stock int) (domain.ProductVariant, error)

	// Image
	UploadImage(ctx context.Context, input UploadImageInput) (domain.ProductImage, error)
	DeleteImage(ctx context.Context, imageID uuid.UUID) error
}

// ─── Implementation ───────────────────────────────────────────────────────────

type useCase struct {
	productRepo repository.ProductRepository
	variantRepo repository.ProductVariantRepository
	imageRepo   repository.ProductImageRepository
	storage     storage.Storage
	auditRepo   repository.AuditLogRepository
}

func NewUseCase(
	productRepo repository.ProductRepository,
	variantRepo repository.ProductVariantRepository,
	imageRepo repository.ProductImageRepository,
	storage storage.Storage,
	auditRepo repository.AuditLogRepository,
) UseCase {
	return &useCase{
		productRepo: productRepo,
		variantRepo: variantRepo,
		imageRepo:   imageRepo,
		storage:     storage,
		auditRepo:   auditRepo,
	}
}

// ─── Product ──────────────────────────────────────────────────────────────────

func (uc *useCase) CreateProduct(ctx context.Context, input CreateProductInput) (domain.Product, error) {
	if err := input.Validate(); err != nil {
		return domain.Product{}, err
	}

	// generate slug dari name; tambah suffix pendek jika sudah dipakai produk lain
	slug := generateSlug(input.Name)
	if existing, _ := uc.productRepo.FindBySlug(ctx, slug); existing != nil {
		slug = slug + "-" + uuid.New().String()[:8]
	}

	now := time.Now()
	product := &domain.Product{
		ID:          uuid.New(),
		Name:        input.Name,
		Slug:        slug,
		Description: input.Description,
		CategoryID:  input.CategoryID,
		IsActive:    true,
		CreatedAt:   now,
		UpdatedAt:   now,
	}

	if err := uc.productRepo.Create(ctx, product); err != nil {
		return domain.Product{}, fmt.Errorf("create product: %w", err)
	}

	// buat semua variant sekaligus
	for _, v := range input.Variants {
		variant := &domain.ProductVariant{
			ID:        uuid.New(),
			ProductID: product.ID,
			Name:      v.Name,
			SKU:       v.SKU,
			Price:     v.Price,
			Stock:     int(v.Stock),
			IsActive:  true,
			CreatedAt: now,
			UpdatedAt: now,
		}
		if err := uc.variantRepo.Create(ctx, variant); err != nil {
			return domain.Product{}, fmt.Errorf("create variant %q: %w", v.Name, err)
		}
		product.Variants = append(product.Variants, *variant)
	}

	uc.auditRepo.Create(ctx, &domain.AuditLog{
		ID:         uuid.New(),
		Action:     domain.AuditCreate,
		EntityType: "products",
		EntityID:   &product.ID,
		NewData:    map[string]any{"name": product.Name, "slug": product.Slug},
		CreatedAt:  now,
	})

	return *product, nil
}

func (uc *useCase) GetByID(ctx context.Context, id uuid.UUID) (domain.Product, error) {
	product, err := uc.productRepo.FindByID(ctx, id)
	if err != nil || product == nil {
		return domain.Product{}, ErrProductNotFound
	}

	// populate variants dan images
	product.Variants, _ = uc.variantRepo.FindByProductID(ctx, id)
	product.Images, _ = uc.imageRepo.FindByProductID(ctx, id)

	return *product, nil
}

func (uc *useCase) GetBySlug(ctx context.Context, slug string) (domain.Product, error) {
	product, err := uc.productRepo.FindBySlug(ctx, slug)
	if err != nil || product == nil {
		return domain.Product{}, ErrProductNotFound
	}

	product.Variants, _ = uc.variantRepo.FindByProductID(ctx, product.ID)
	product.Images, _ = uc.imageRepo.FindByProductID(ctx, product.ID)

	return *product, nil
}

func (uc *useCase) ListAll(ctx context.Context, filter ListProductFilter) ([]domain.Product, int, error) {
	if filter.Limit == 0 {
		filter.Limit = 20
	}
	if filter.Page == 0 {
		filter.Page = 1
	}

	repoFilter := repository.ProductFilter{
		CategoryID: filter.CategoryID,
		Search:     filter.Search,
		IsActive:   filter.IsActive,
		Page:       filter.Page,
		Limit:      filter.Limit,
	}

	products, total, err := uc.productRepo.FindAll(ctx, repoFilter)
	if err != nil {
		return nil, 0, err
	}

	for i := range products {
		// Load variants untuk hitung stok dan harga di card
		products[i].Variants, _ = uc.variantRepo.FindByProductID(ctx, products[i].ID)

		// Untuk listing cukup primary image saja
		images, _ := uc.imageRepo.FindByProductID(ctx, products[i].ID)
		for _, img := range images {
			if img.IsPrimary {
				products[i].Images = []domain.ProductImage{img}
				break
			}
		}
	}

	return products, total, nil
}

func (uc *useCase) UpdateByID(ctx context.Context, id uuid.UUID, input UpdateProductInput) (domain.Product, error) {
	if err := input.Validate(); err != nil {
		return domain.Product{}, err
	}

	product, err := uc.productRepo.FindByID(ctx, id)
	if err != nil || product == nil {
		return domain.Product{}, ErrProductNotFound
	}

	// simpan old data untuk audit
	oldData := map[string]any{
		"name":      product.Name,
		"is_active": product.IsActive,
	}

	product.Name = input.Name
	product.Description = input.Description
	product.CategoryID = input.CategoryID
	product.IsActive = input.IsActive
	product.UpdatedAt = time.Now()

	if err := uc.productRepo.Update(ctx, product); err != nil {
		return domain.Product{}, fmt.Errorf("update product: %w", err)
	}

	uc.auditRepo.Create(ctx, &domain.AuditLog{
		ID:         uuid.New(),
		Action:     domain.AuditUpdate,
		EntityType: "products",
		EntityID:   &id,
		OldData:    oldData,
		NewData:    map[string]any{"name": product.Name, "is_active": product.IsActive},
		CreatedAt:  time.Now(),
	})

	return *product, nil
}

func (uc *useCase) DeleteByID(ctx context.Context, id uuid.UUID) error {
	product, err := uc.productRepo.FindByID(ctx, id)
	if err != nil || product == nil {
		return ErrProductNotFound
	}

	if err := uc.productRepo.Delete(ctx, id); err != nil {
		return fmt.Errorf("delete product: %w", err)
	}

	uc.auditRepo.Create(ctx, &domain.AuditLog{
		ID:         uuid.New(),
		Action:     domain.AuditDelete,
		EntityType: "products",
		EntityID:   &id,
		OldData:    map[string]any{"name": product.Name},
		CreatedAt:  time.Now(),
	})

	return nil
}

// ─── Variant ──────────────────────────────────────────────────────────────────

func (uc *useCase) CreateVariant(ctx context.Context, productID uuid.UUID, input CreateVariantInput) (domain.ProductVariant, error) {
	if err := input.Validate(); err != nil {
		return domain.ProductVariant{}, err
	}

	// pastikan produk ada dulu
	product, err := uc.productRepo.FindByID(ctx, productID)
	if err != nil || product == nil {
		return domain.ProductVariant{}, ErrProductNotFound
	}

	now := time.Now()
	variant := &domain.ProductVariant{
		ID:        uuid.New(),
		ProductID: productID,
		Name:      input.Name,
		SKU:       input.SKU,
		Price:     input.Price,
		Stock:     int(input.Stock),
		IsActive:  true,
		CreatedAt: now,
		UpdatedAt: now,
	}

	if err := uc.variantRepo.Create(ctx, variant); err != nil {
		return domain.ProductVariant{}, fmt.Errorf("create variant: %w", err)
	}

	return *variant, nil
}

func (uc *useCase) UpdateVariant(ctx context.Context, productID, variantID uuid.UUID, input UpdateVariantInput) (domain.ProductVariant, error) {
	if err := input.Validate(); err != nil {
		return domain.ProductVariant{}, err
	}

	variant, err := uc.variantRepo.FindByID(ctx, variantID)
	if err != nil || variant == nil || variant.ProductID != productID {
		return domain.ProductVariant{}, ErrVariantNotFound
	}

	variant.Name = input.Name
	variant.SKU = input.SKU
	variant.Price = input.Price
	variant.Stock = int(input.Stock)
	variant.IsActive = input.IsActive
	variant.UpdatedAt = time.Now()

	if err := uc.variantRepo.Update(ctx, variant); err != nil {
		return domain.ProductVariant{}, fmt.Errorf("update variant: %w", err)
	}

	return *variant, nil
}

func (uc *useCase) DeleteVariant(ctx context.Context, productID, variantID uuid.UUID) error {
	variant, err := uc.variantRepo.FindByID(ctx, variantID)
	if err != nil || variant == nil || variant.ProductID != productID {
		return ErrVariantNotFound
	}
	return uc.variantRepo.Delete(ctx, variantID)
}

func (uc *useCase) AdjustStock(ctx context.Context, productID, variantID uuid.UUID, stock int) (domain.ProductVariant, error) {
	if stock < 0 {
		return domain.ProductVariant{}, errors.New("stock cannot be negative")
	}

	variant, err := uc.variantRepo.FindByID(ctx, variantID)
	if err != nil || variant == nil || variant.ProductID != productID {
		return domain.ProductVariant{}, ErrVariantNotFound
	}

	variant.Stock = stock
	variant.UpdatedAt = time.Now()

	if err := uc.variantRepo.Update(ctx, variant); err != nil {
		return domain.ProductVariant{}, fmt.Errorf("adjust stock: %w", err)
	}

	uc.auditRepo.Create(ctx, &domain.AuditLog{
		ID:         uuid.New(),
		Action:     domain.AuditUpdate,
		EntityType: "product_variants",
		EntityID:   &variantID,
		OldData:    map[string]any{"stock": variant.Stock},
		NewData:    map[string]any{"stock": stock},
		CreatedAt:  time.Now(),
	})

	return *variant, nil
}

// ─── Image ────────────────────────────────────────────────────────────────────

func (uc *useCase) UploadImage(ctx context.Context, input UploadImageInput) (domain.ProductImage, error) {
	// pastikan produk ada
	product, err := uc.productRepo.FindByID(ctx, input.ProductID)
	if err != nil || product == nil {
		return domain.ProductImage{}, ErrProductNotFound
	}

	// generate S3 key: products/{productID}/{uuid}.jpg
	key := storage.GenerateKey(
		fmt.Sprintf("products/%s", input.ProductID),
		input.Filename,
	)

	contentType := storage.ContentTypeFromFilename(input.Filename)

	// upload ke S3 (atau storage apapun yang dipakai)
	url, err := uc.storage.Upload(
		ctx,
		key,
		newBytesReader(input.Data),
		input.Size,
		contentType,
	)
	if err != nil {
		return domain.ProductImage{}, fmt.Errorf("upload image: %w", err)
	}

	// hitung sort_order dari jumlah image yang sudah ada
	existingImages, _ := uc.imageRepo.FindByProductID(ctx, input.ProductID)
	sortOrder := len(existingImages)

	// kalau ini image pertama, otomatis jadi primary
	isPrimary := input.IsPrimary || sortOrder == 0

	image := &domain.ProductImage{
		ID:        uuid.New(),
		ProductID: input.ProductID,
		URL:       url,
		IsPrimary: isPrimary,
		SortOrder: sortOrder,
		CreatedAt: time.Now(),
	}

	if err := uc.imageRepo.Create(ctx, image); err != nil {
		// kalau gagal simpan ke DB, hapus file dari storage
		uc.storage.Delete(ctx, key)
		return domain.ProductImage{}, fmt.Errorf("save image record: %w", err)
	}

	return *image, nil
}

func (uc *useCase) DeleteImage(ctx context.Context, imageID uuid.UUID) error {
	image, err := uc.imageRepo.FindByID(ctx, imageID)
	if err != nil {
		return fmt.Errorf("delete image: %w", err)
	}
	if image == nil {
		return ErrImageNotFound
	}

	if err := uc.imageRepo.Delete(ctx, imageID); err != nil {
		return fmt.Errorf("delete image record: %w", err)
	}

	// hapus file dari S3 — ekstrak key dari URL (best-effort, tidak gagalkan request)
	if parsed, err := url.Parse(image.URL); err == nil {
		key := strings.TrimPrefix(parsed.Path, "/")
		uc.storage.Delete(ctx, key)
	}

	return nil
}

// ─── Helpers ──────────────────────────────────────────────────────────────────

// generateSlug konversi nama produk jadi URL-friendly slug.
// "Kaos Polos (Merah)" → "kaos-polos-merah"
func generateSlug(name string) string {
	slug := strings.ToLower(strings.TrimSpace(name))
	var b strings.Builder
	for _, r := range slug {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9':
			b.WriteRune(r)
		case r == ' ', r == '-', r == '_':
			b.WriteByte('-')
		}
	}
	result := strings.Trim(b.String(), "-")
	for strings.Contains(result, "--") {
		result = strings.ReplaceAll(result, "--", "-")
	}
	return result
}

// newBytesReader wrap []byte menjadi io.Reader untuk upload
func newBytesReader(data []byte) *bytesReader {
	return &bytesReader{data: data, pos: 0}
}

type bytesReader struct {
	data []byte
	pos  int
}

func (b *bytesReader) Read(p []byte) (n int, err error) {
	if b.pos >= len(b.data) {
		return 0, io.EOF
	}
	n = copy(p, b.data[b.pos:])
	b.pos += n
	return n, nil
}
