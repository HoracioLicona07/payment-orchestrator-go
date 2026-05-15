package repository

import (
	"database/sql"
	"fmt"
	"time"

	"github.com/payment-orchestrator-go/internal/domain"
)

type OrderRepo struct {
	db *sql.DB
}

func New(db *sql.DB) *OrderRepo {
	return &OrderRepo{db: db}
}

func (r *OrderRepo) Insert(o *domain.Order) error {
	_, err := r.db.Exec(
		`INSERT INTO orders
		 (id,tracking_key,amount,destination,priority,status,retry_count,error_msg,message_id,created_at,updated_at)
		 VALUES (?,?,?,?,?,?,?,?,?,?,?)`,
		o.ID, o.TrackingKey, o.Amount, o.Destination, o.Priority,
		o.Status, o.RetryCount, o.ErrorMsg, o.MessageID,
		o.CreatedAt, o.UpdatedAt,
	)
	return err
}

func (r *OrderRepo) FindByID(id string) (*domain.Order, error) {
	row := r.db.QueryRow(qSelect+" WHERE id=?", id)
	return scanOne(row)
}

func (r *OrderRepo) FindAll() ([]*domain.Order, error) {
	rows, err := r.db.Query(qSelect + " ORDER BY created_at DESC")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanMany(rows)
}

func (r *OrderRepo) FindByStatus(s domain.Status) ([]*domain.Order, error) {
	rows, err := r.db.Query(
		qSelect+" WHERE status=? ORDER BY priority DESC, created_at ASC",
		string(s),
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanMany(rows)
}

func (r *OrderRepo) CountByTrackingKey(key string) (int, error) {
	var n int
	err := r.db.QueryRow(`SELECT COUNT(*) FROM orders WHERE tracking_key=?`, key).Scan(&n)
	return n, err
}

func (r *OrderRepo) SetStatus(id string, s domain.Status) error {
	_, err := r.db.Exec(
		`UPDATE orders SET status=?, updated_at=? WHERE id=?`,
		string(s), time.Now(), id,
	)
	return err
}

// Dispatch marca la orden como DISPATCHED y guarda el message_id.
// Se hacen dos UPDATE independientes — si el proceso cae entre ambos,
// la orden queda en DISPATCHED sin message_id y no puede correlacionarse
// con su respuesta. No hay forma de recuperar ese estado automáticamente.
func (r *OrderRepo) Dispatch(id, messageID string) error {
	_, err := r.db.Exec(
		`UPDATE orders SET status=?, updated_at=? WHERE id=?`,
		string(domain.StatusDispatched), time.Now(), id,
	)
	if err != nil {
		return fmt.Errorf("actualizando status: %w", err)
	}
	_, err = r.db.Exec(
		`UPDATE orders SET message_id=? WHERE id=?`,
		messageID, id,
	)
	if err != nil {
		return fmt.Errorf("guardando message_id: %w", err)
	}
	return nil
}

func (r *OrderRepo) SetStatusAndError(id string, s domain.Status, msg string) error {
	_, err := r.db.Exec(
		`UPDATE orders SET status=?, error_msg=?, updated_at=? WHERE id=?`,
		string(s), msg, time.Now(), id,
	)
	return err
}

func (r *OrderRepo) IncrRetry(id string) error {
	_, err := r.db.Exec(
		`UPDATE orders SET retry_count=retry_count+1, updated_at=? WHERE id=?`,
		time.Now(), id,
	)
	return err
}

// ── helpers ──────────────────────────────────────────────────────────────────

const qSelect = `SELECT id,tracking_key,amount,destination,priority,status,retry_count,error_msg,message_id,created_at,updated_at FROM orders`

func scanOne(row *sql.Row) (*domain.Order, error) {
	o := &domain.Order{}
	err := row.Scan(
		&o.ID, &o.TrackingKey, &o.Amount, &o.Destination, &o.Priority,
		&o.Status, &o.RetryCount, &o.ErrorMsg, &o.MessageID,
		&o.CreatedAt, &o.UpdatedAt,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	return o, err
}

func scanMany(rows *sql.Rows) ([]*domain.Order, error) {
	var out []*domain.Order
	for rows.Next() {
		o := &domain.Order{}
		if err := rows.Scan(
			&o.ID, &o.TrackingKey, &o.Amount, &o.Destination, &o.Priority,
			&o.Status, &o.RetryCount, &o.ErrorMsg, &o.MessageID,
			&o.CreatedAt, &o.UpdatedAt,
		); err != nil {
			return nil, err
		}
		out = append(out, o)
	}
	return out, nil
}
