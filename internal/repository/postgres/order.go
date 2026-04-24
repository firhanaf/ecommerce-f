package postgres

import (
	"context"
	"encoding/json"
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

type orderRepository struct {
	db *pgxpool.Pool
}

func NewOrderRepository(db *pgxpool.Pool) repository.OrderRepository {
	return &orderRepository{db: db}
}

// Create menyimpan order + order_items + decrement stok dalam satu transaksi DB.
// Jika salah satu gagal, semua di-rollback — tidak ada stok berkurang tanpa order.
func (r *orderRepository) Create(ctx context.Context, order *domain.Order, decrements []repository.StockDecrement) error {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return fmt.Errorf("orderRepository.Create begin tx: %w", err)
	}
	defer tx.Rollback(ctx)

	snapshotJSON, err := json.Marshal(order.SnapshotAddress)
	if err != nil {
		return fmt.Errorf("marshal snapshot address: %w", err)
	}

	_, err = tx.Exec(ctx, `
		INSERT INTO orders (
			id, user_id, address_id, snapshot_address, status,
			subtotal, shipping_cost, total, courier, courier_service,
			notes, created_at, updated_at
		) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13)`,
		order.ID, order.UserID, order.AddressID, snapshotJSON,
		order.Status, order.Subtotal, order.ShippingCost, order.Total,
		order.Courier, order.CourierService, order.Notes,
		order.CreatedAt, order.UpdatedAt,
	)
	if err != nil {
		return fmt.Errorf("insert order: %w", err)
	}

	if len(order.Items) > 0 {
		valueStrings := make([]string, 0, len(order.Items))
		valueArgs := make([]any, 0, len(order.Items)*10)
		argIdx := 1

		for _, item := range order.Items {
			valueStrings = append(valueStrings, fmt.Sprintf(
				"($%d,$%d,$%d,$%d,$%d,$%d,$%d,$%d,$%d,$%d)",
				argIdx, argIdx+1, argIdx+2, argIdx+3, argIdx+4,
				argIdx+5, argIdx+6, argIdx+7, argIdx+8, argIdx+9,
			))
			valueArgs = append(valueArgs,
				item.ID, order.ID, item.VariantID, item.ProductName,
				item.VariantName, item.ProductImage, item.Quantity,
				item.UnitPrice, item.Subtotal, item.CreatedAt,
			)
			argIdx += 10
		}

		_, err = tx.Exec(ctx,
			`INSERT INTO order_items (
				id, order_id, variant_id, product_name, variant_name,
				product_image, quantity, unit_price, subtotal, created_at
			) VALUES `+strings.Join(valueStrings, ","),
			valueArgs...,
		)
		if err != nil {
			return fmt.Errorf("insert order_items: %w", err)
		}
	}

	// Decrement stok dalam transaksi yang sama — atomic, rollback kalau stok tidak cukup
	for _, d := range decrements {
		result, err := tx.Exec(ctx,
			`UPDATE product_variants SET stock = stock - $1, updated_at = NOW()
			 WHERE id = $2 AND stock >= $1`,
			d.Quantity, d.VariantID,
		)
		if err != nil {
			return fmt.Errorf("decrement stock variant %s: %w", d.VariantID, err)
		}
		if result.RowsAffected() == 0 {
			return fmt.Errorf("insufficient stock for variant %s", d.VariantID)
		}
	}

	return tx.Commit(ctx)
}

func (r *orderRepository) FindByID(ctx context.Context, id uuid.UUID) (*domain.Order, error) {
	// Ambil order + items sekaligus dengan LEFT JOIN
	orderQuery := `
		SELECT id, user_id, address_id, snapshot_address, status,
		       subtotal, shipping_cost, total, courier, courier_service,
		       notes, created_at, updated_at
		FROM orders WHERE id = $1
	`
	var order domain.Order
	var snapshotJSON []byte

	err := r.db.QueryRow(ctx, orderQuery, id).Scan(
		&order.ID, &order.UserID, &order.AddressID, &snapshotJSON,
		&order.Status, &order.Subtotal, &order.ShippingCost, &order.Total,
		&order.Courier, &order.CourierService, &order.Notes,
		&order.CreatedAt, &order.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("FindByID order: %w", err)
	}

	if err := json.Unmarshal(snapshotJSON, &order.SnapshotAddress); err != nil {
		return nil, fmt.Errorf("unmarshal snapshot: %w", err)
	}

	// Ambil order items
	itemsQuery := `
		SELECT id, order_id, variant_id, product_name, variant_name,
		       product_image, quantity, unit_price, subtotal, created_at
		FROM order_items WHERE order_id = $1
	`
	rows, err := r.db.Query(ctx, itemsQuery, id)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var item domain.OrderItem
		if err := rows.Scan(
			&item.ID, &item.OrderID, &item.VariantID,
			&item.ProductName, &item.VariantName, &item.ProductImage,
			&item.Quantity, &item.UnitPrice, &item.Subtotal, &item.CreatedAt,
		); err != nil {
			return nil, err
		}
		order.Items = append(order.Items, item)
	}

	return &order, nil
}

func (r *orderRepository) FindAll(ctx context.Context, filter repository.OrderFilter) ([]domain.Order, int, error) {
	conditions := []string{"1=1"}
	args := []any{}
	argIdx := 1

	if filter.UserID != nil {
		conditions = append(conditions, fmt.Sprintf("user_id = $%d", argIdx))
		args = append(args, *filter.UserID)
		argIdx++
	}

	if filter.Status != nil {
		conditions = append(conditions, fmt.Sprintf("status = $%d", argIdx))
		args = append(args, *filter.Status)
		argIdx++
	}

	where := strings.Join(conditions, " AND ")

	var total int
	if err := r.db.QueryRow(ctx, fmt.Sprintf("SELECT COUNT(*) FROM orders WHERE %s", where), args...).Scan(&total); err != nil {
		return nil, 0, err
	}

	if filter.Limit == 0 {
		filter.Limit = 20
	}
	offset := (filter.Page - 1) * filter.Limit

	query := fmt.Sprintf(`
		SELECT id, user_id, address_id, snapshot_address, status,
		       subtotal, shipping_cost, total, courier, courier_service,
		       notes, created_at, updated_at
		FROM orders
		WHERE %s
		ORDER BY created_at DESC
		LIMIT $%d OFFSET $%d
	`, where, argIdx, argIdx+1)

	args = append(args, filter.Limit, offset)

	rows, err := r.db.Query(ctx, query, args...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var orders []domain.Order
	for rows.Next() {
		var o domain.Order
		var snapshotJSON []byte
		if err := rows.Scan(
			&o.ID, &o.UserID, &o.AddressID, &snapshotJSON,
			&o.Status, &o.Subtotal, &o.ShippingCost, &o.Total,
			&o.Courier, &o.CourierService, &o.Notes,
			&o.CreatedAt, &o.UpdatedAt,
		); err != nil {
			return nil, 0, err
		}
		json.Unmarshal(snapshotJSON, &o.SnapshotAddress)
		orders = append(orders, o)
	}

	return orders, total, nil
}

func (r *orderRepository) UpdateStatus(ctx context.Context, id uuid.UUID, status domain.OrderStatus) error {
	query := `UPDATE orders SET status = $1, updated_at = $2 WHERE id = $3`
	_, err := r.db.Exec(ctx, query, status, time.Now(), id)
	return err
}

// Cancel update status ke cancelled dan restore stok semua item dalam satu transaksi.
func (r *orderRepository) Cancel(ctx context.Context, id uuid.UUID) error {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return fmt.Errorf("orderRepository.Cancel begin tx: %w", err)
	}
	defer tx.Rollback(ctx)

	_, err = tx.Exec(ctx,
		`UPDATE orders SET status = $1, updated_at = $2 WHERE id = $3`,
		domain.OrderStatusCancelled, time.Now(), id,
	)
	if err != nil {
		return fmt.Errorf("cancel order: %w", err)
	}

	// Restore stok untuk setiap item — hanya jika variant_id tidak null (variant masih ada)
	rows, err := tx.Query(ctx,
		`SELECT variant_id, quantity FROM order_items WHERE order_id = $1 AND variant_id IS NOT NULL`,
		id,
	)
	if err != nil {
		return fmt.Errorf("fetch order items: %w", err)
	}
	defer rows.Close()

	type stockItem struct {
		variantID uuid.UUID
		quantity  int
	}
	var items []stockItem
	for rows.Next() {
		var s stockItem
		if err := rows.Scan(&s.variantID, &s.quantity); err != nil {
			return err
		}
		items = append(items, s)
	}
	rows.Close()

	for _, s := range items {
		_, err = tx.Exec(ctx,
			`UPDATE product_variants SET stock = stock + $1, updated_at = NOW() WHERE id = $2`,
			s.quantity, s.variantID,
		)
		if err != nil {
			return fmt.Errorf("restore stock variant %s: %w", s.variantID, err)
		}
	}

	return tx.Commit(ctx)
}
