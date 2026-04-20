package postgres

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"ecommerce-api/internal"
	"ecommerce-api/internal/repository"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// ─── Cart ─────────────────────────────────────────────────────────────────────

type cartRepository struct {
	db *pgxpool.Pool
}

func NewCartRepository(db *pgxpool.Pool) repository.CartRepository {
	return &cartRepository{db: db}
}

func (r *cartRepository) FindOrCreateByUserID(ctx context.Context, userID uuid.UUID) (*domain.Cart, error) {
	// INSERT ... ON CONFLICT DO NOTHING lalu SELECT — atomic, aman untuk concurrent request
	insertQuery := `
		INSERT INTO carts (id, user_id, created_at, updated_at)
		VALUES ($1, $2, $3, $4)
		ON CONFLICT (user_id) DO NOTHING
	`
	now := time.Now()
	_, err := r.db.Exec(ctx, insertQuery, uuid.New(), userID, now, now)
	if err != nil {
		return nil, fmt.Errorf("FindOrCreateByUserID insert: %w", err)
	}

	return r.FindByUserID(ctx, userID)
}

func (r *cartRepository) FindByUserID(ctx context.Context, userID uuid.UUID) (*domain.Cart, error) {
	cartQuery := `
		SELECT id, user_id, created_at, updated_at
		FROM carts WHERE user_id = $1
	`
	var cart domain.Cart
	err := r.db.QueryRow(ctx, cartQuery, userID).Scan(
		&cart.ID, &cart.UserID, &cart.CreatedAt, &cart.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}

	// Ambil items + info produk/variant sekaligus
	itemsQuery := `
		SELECT
			ci.id, ci.cart_id, ci.variant_id, ci.quantity,
			ci.created_at, ci.updated_at,
			pv.name AS variant_name, pv.price, pv.stock,
			p.id AS product_id, p.name AS product_name
		FROM cart_items ci
		JOIN product_variants pv ON pv.id = ci.variant_id
		JOIN products p ON p.id = pv.product_id
		WHERE ci.cart_id = $1
	`
	rows, err := r.db.Query(ctx, itemsQuery, cart.ID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var item domain.CartItem
		var variant domain.ProductVariant
		var product domain.Product

		if err := rows.Scan(
			&item.ID, &item.CartID, &item.VariantID, &item.Quantity,
			&item.CreatedAt, &item.UpdatedAt,
			&variant.Name, &variant.Price, &variant.Stock,
			&product.ID, &product.Name,
		); err != nil {
			return nil, err
		}

		variant.ID = item.VariantID
		variant.ProductID = product.ID
		item.Variant = &variant
		item.Product = &product
		cart.Items = append(cart.Items, item)
	}

	return &cart, nil
}

func (r *cartRepository) AddItem(ctx context.Context, item *domain.CartItem) error {
	// Kalau variant sudah ada di cart, tambah qty — kalau belum, insert baru
	query := `
		INSERT INTO cart_items (id, cart_id, variant_id, quantity, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6)
		ON CONFLICT (cart_id, variant_id) DO UPDATE
		SET quantity = cart_items.quantity + EXCLUDED.quantity,
		    updated_at = NOW()
	`
	_, err := r.db.Exec(ctx, query,
		item.ID, item.CartID, item.VariantID,
		item.Quantity, item.CreatedAt, item.UpdatedAt,
	)
	return err
}

func (r *cartRepository) UpdateItemQuantity(ctx context.Context, cartItemID uuid.UUID, qty int) error {
	query := `UPDATE cart_items SET quantity = $1, updated_at = $2 WHERE id = $3`
	_, err := r.db.Exec(ctx, query, qty, time.Now(), cartItemID)
	return err
}

func (r *cartRepository) RemoveItem(ctx context.Context, cartItemID uuid.UUID) error {
	_, err := r.db.Exec(ctx, "DELETE FROM cart_items WHERE id = $1", cartItemID)
	return err
}

func (r *cartRepository) ClearCart(ctx context.Context, cartID uuid.UUID) error {
	_, err := r.db.Exec(ctx, "DELETE FROM cart_items WHERE cart_id = $1", cartID)
	return err
}

// ─── Payment ──────────────────────────────────────────────────────────────────

type paymentRepository struct {
	db *pgxpool.Pool
}

func NewPaymentRepository(db *pgxpool.Pool) repository.PaymentRepository {
	return &paymentRepository{db: db}
}

func (r *paymentRepository) Create(ctx context.Context, p *domain.Payment) error {
	rawJSON, _ := json.Marshal(p.RawResponse)
	query := `
		INSERT INTO payments (
			id, order_id, provider, payment_method, status,
			transaction_id, amount, raw_response, paid_at, expired_at,
			created_at, updated_at
		) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12)
	`
	_, err := r.db.Exec(ctx, query,
		p.ID, p.OrderID, p.Provider, p.PaymentMethod, p.Status,
		p.TransactionID, p.Amount, rawJSON, p.PaidAt, p.ExpiredAt,
		p.CreatedAt, p.UpdatedAt,
	)
	return err
}

func (r *paymentRepository) FindByOrderID(ctx context.Context, orderID uuid.UUID) (*domain.Payment, error) {
	return r.findOne(ctx, "order_id", orderID)
}

func (r *paymentRepository) FindByTransactionID(ctx context.Context, txID string) (*domain.Payment, error) {
	return r.findOne(ctx, "transaction_id", txID)
}

func (r *paymentRepository) findOne(ctx context.Context, field string, val any) (*domain.Payment, error) {
	query := fmt.Sprintf(`
		SELECT id, order_id, provider, payment_method, status,
		       transaction_id, amount, raw_response, paid_at, expired_at,
		       created_at, updated_at
		FROM payments WHERE %s = $1
	`, field)

	var p domain.Payment
	var rawJSON []byte

	err := r.db.QueryRow(ctx, query, val).Scan(
		&p.ID, &p.OrderID, &p.Provider, &p.PaymentMethod, &p.Status,
		&p.TransactionID, &p.Amount, &rawJSON, &p.PaidAt, &p.ExpiredAt,
		&p.CreatedAt, &p.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	json.Unmarshal(rawJSON, &p.RawResponse)
	return &p, nil
}

func (r *paymentRepository) UpdateStatus(ctx context.Context, id uuid.UUID, status domain.PaymentStatus, rawResponse map[string]any) error {
	rawJSON, _ := json.Marshal(rawResponse)
	var paidAt *time.Time
	if status == domain.PaymentStatusSettlement {
		now := time.Now()
		paidAt = &now
	}
	query := `
		UPDATE payments
		SET status = $1, raw_response = $2, paid_at = $3, updated_at = $4
		WHERE id = $5
	`
	_, err := r.db.Exec(ctx, query, status, rawJSON, paidAt, time.Now(), id)
	return err
}

// ─── Shipment ─────────────────────────────────────────────────────────────────

type shipmentRepository struct {
	db *pgxpool.Pool
}

func NewShipmentRepository(db *pgxpool.Pool) repository.ShipmentRepository {
	return &shipmentRepository{db: db}
}

func (r *shipmentRepository) Create(ctx context.Context, s *domain.Shipment) error {
	query := `
		INSERT INTO shipments (
			id, order_id, courier, courier_service, tracking_number,
			status, shipped_at, delivered_at, created_at, updated_at
		) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10)
	`
	_, err := r.db.Exec(ctx, query,
		s.ID, s.OrderID, s.Courier, s.CourierService, s.TrackingNumber,
		s.Status, s.ShippedAt, s.DeliveredAt, s.CreatedAt, s.UpdatedAt,
	)
	return err
}

func (r *shipmentRepository) FindByOrderID(ctx context.Context, orderID uuid.UUID) (*domain.Shipment, error) {
	query := `
		SELECT id, order_id, courier, courier_service, tracking_number,
		       status, shipped_at, delivered_at, created_at, updated_at
		FROM shipments WHERE order_id = $1
	`
	var s domain.Shipment
	err := r.db.QueryRow(ctx, query, orderID).Scan(
		&s.ID, &s.OrderID, &s.Courier, &s.CourierService, &s.TrackingNumber,
		&s.Status, &s.ShippedAt, &s.DeliveredAt, &s.CreatedAt, &s.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	return &s, nil
}

func (r *shipmentRepository) Update(ctx context.Context, s *domain.Shipment) error {
	query := `
		UPDATE shipments
		SET tracking_number = $1, status = $2, shipped_at = $3,
		    delivered_at = $4, updated_at = $5
		WHERE id = $6
	`
	_, err := r.db.Exec(ctx, query,
		s.TrackingNumber, s.Status, s.ShippedAt,
		s.DeliveredAt, time.Now(), s.ID,
	)
	return err
}

// ─── Audit Log ────────────────────────────────────────────────────────────────

type auditLogRepository struct {
	db *pgxpool.Pool
}

func NewAuditLogRepository(db *pgxpool.Pool) repository.AuditLogRepository {
	return &auditLogRepository{db: db}
}

func (r *auditLogRepository) Create(ctx context.Context, log *domain.AuditLog) error {
	oldJSON, _ := json.Marshal(log.OldData)
	newJSON, _ := json.Marshal(log.NewData)

	query := `
		INSERT INTO audit_logs (
			id, actor_id, actor_role, action, entity_type,
			entity_id, old_data, new_data, ip_address, user_agent,
			notes, created_at
		) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12)
	`
	_, err := r.db.Exec(ctx, query,
		log.ID, log.ActorID, log.ActorRole, log.Action, log.EntityType,
		log.EntityID, oldJSON, newJSON, log.IPAddress, log.UserAgent,
		log.Notes, log.CreatedAt,
	)
	// Audit log failure tidak boleh crash aplikasi — hanya log error
	if err != nil {
		fmt.Printf("WARNING: failed to write audit log: %v\n", err)
	}
	return nil
}

func (r *auditLogRepository) FindAll(ctx context.Context, filter repository.AuditFilter) ([]domain.AuditLog, int, error) {
	conditions := []string{"1=1"}
	args := []any{}
	argIdx := 1

	if filter.ActorID != nil {
		conditions = append(conditions, fmt.Sprintf("actor_id = $%d", argIdx))
		args = append(args, *filter.ActorID)
		argIdx++
	}
	if filter.EntityType != "" {
		conditions = append(conditions, fmt.Sprintf("entity_type = $%d", argIdx))
		args = append(args, filter.EntityType)
		argIdx++
	}
	if filter.EntityID != nil {
		conditions = append(conditions, fmt.Sprintf("entity_id = $%d", argIdx))
		args = append(args, *filter.EntityID)
		argIdx++
	}
	if filter.Action != nil {
		conditions = append(conditions, fmt.Sprintf("action = $%d", argIdx))
		args = append(args, *filter.Action)
		argIdx++
	}

	where := fmt.Sprintf("%s", joinConditions(conditions))

	var total int
	r.db.QueryRow(ctx, fmt.Sprintf("SELECT COUNT(*) FROM audit_logs WHERE %s", where), args...).Scan(&total)

	if filter.Limit == 0 {
		filter.Limit = 50
	}
	offset := (filter.Page - 1) * filter.Limit

	query := fmt.Sprintf(`
		SELECT id, actor_id, actor_role, action, entity_type,
		       entity_id, old_data, new_data, ip_address, user_agent,
		       notes, created_at
		FROM audit_logs
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

	var logs []domain.AuditLog
	for rows.Next() {
		var l domain.AuditLog
		var oldJSON, newJSON []byte
		if err := rows.Scan(
			&l.ID, &l.ActorID, &l.ActorRole, &l.Action, &l.EntityType,
			&l.EntityID, &oldJSON, &newJSON, &l.IPAddress, &l.UserAgent,
			&l.Notes, &l.CreatedAt,
		); err != nil {
			return nil, 0, err
		}
		json.Unmarshal(oldJSON, &l.OldData)
		json.Unmarshal(newJSON, &l.NewData)
		logs = append(logs, l)
	}

	return logs, total, nil
}

func joinConditions(conditions []string) string {
	result := ""
	for i, c := range conditions {
		if i > 0 {
			result += " AND "
		}
		result += c
	}
	return result
}
