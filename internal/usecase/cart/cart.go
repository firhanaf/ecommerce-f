package cart

import (
	"context"
	"errors"
	"time"

	domain "ecommerce-api/internal"
	"ecommerce-api/internal/repository"
	"github.com/google/uuid"
)

// ─── Errors ───────────────────────────────────────────────────────────────────

var (
	ErrVariantNotFound   = errors.New("variant not found")
	ErrVariantOutOfStock = errors.New("variant is out of stock")
	ErrCartItemNotFound  = errors.New("cart item not found")
	ErrUnauthorized      = errors.New("unauthorized to modify this cart item")
)

// ─── Interface ────────────────────────────────────────────────────────────────

type UseCase interface {
	GetCart(ctx context.Context, userID uuid.UUID) (*domain.Cart, error)
	AddItem(ctx context.Context, userID, variantID uuid.UUID, qty int) error
	UpdateItem(ctx context.Context, userID, itemID uuid.UUID, qty int) error
	RemoveItem(ctx context.Context, userID, itemID uuid.UUID) error
}

// ─── Implementation ───────────────────────────────────────────────────────────

type useCase struct {
	cartRepo    repository.CartRepository
	variantRepo repository.ProductVariantRepository
}

func NewUseCase(
	cartRepo repository.CartRepository,
	variantRepo repository.ProductVariantRepository,
) UseCase {
	return &useCase{
		cartRepo:    cartRepo,
		variantRepo: variantRepo,
	}
}

func (uc *useCase) GetCart(ctx context.Context, userID uuid.UUID) (*domain.Cart, error) {
	cart, err := uc.cartRepo.FindOrCreateByUserID(ctx, userID)
	if err != nil {
		return nil, err
	}
	return cart, nil
}

func (uc *useCase) AddItem(ctx context.Context, userID, variantID uuid.UUID, qty int) error {
	// Pastikan variant ada dan punya stok
	variant, err := uc.variantRepo.FindByID(ctx, variantID)
	if err != nil || variant == nil {
		return ErrVariantNotFound
	}
	if variant.Stock <= 0 {
		return ErrVariantOutOfStock
	}

	// Ambil atau buat cart
	cart, err := uc.cartRepo.FindOrCreateByUserID(ctx, userID)
	if err != nil {
		return err
	}

	now := time.Now()
	item := &domain.CartItem{
		ID:        uuid.New(),
		CartID:    cart.ID,
		VariantID: variantID,
		Quantity:  qty,
		CreatedAt: now,
		UpdatedAt: now,
	}

	return uc.cartRepo.AddItem(ctx, item)
}

func (uc *useCase) UpdateItem(ctx context.Context, userID, itemID uuid.UUID, qty int) error {
	// Cari cart user
	cart, err := uc.cartRepo.FindByUserID(ctx, userID)
	if err != nil || cart == nil {
		return ErrCartItemNotFound
	}

	// Pastikan item milik cart user ini
	found := false
	for _, item := range cart.Items {
		if item.ID == itemID {
			found = true
			break
		}
	}
	if !found {
		return ErrUnauthorized
	}

	if qty <= 0 {
		// qty 0 atau negatif = hapus item
		return uc.cartRepo.RemoveItem(ctx, itemID)
	}

	return uc.cartRepo.UpdateItemQuantity(ctx, itemID, qty)
}

func (uc *useCase) RemoveItem(ctx context.Context, userID, itemID uuid.UUID) error {
	cart, err := uc.cartRepo.FindByUserID(ctx, userID)
	if err != nil || cart == nil {
		return ErrCartItemNotFound
	}

	// Verifikasi ownership
	for _, item := range cart.Items {
		if item.ID == itemID {
			return uc.cartRepo.RemoveItem(ctx, itemID)
		}
	}

	return ErrUnauthorized
}
