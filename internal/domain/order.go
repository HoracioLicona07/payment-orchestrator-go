package domain

import "time"

type Status string

const (
	StatusCreated    Status = "CREATED"
	StatusAuthorized Status = "AUTHORIZED"
	StatusDispatched Status = "DISPATCHED"
	StatusProcessed  Status = "PROCESSED"
	StatusFailed     Status = "FAILED"
	StatusRetry      Status = "RETRY"
)

type Order struct {
	ID          string    `json:"id"`
	TrackingKey string    `json:"tracking_key"`
	Amount      float64   `json:"amount"`
	Destination string    `json:"destination"`
	Priority    int       `json:"priority"`
	Status      Status    `json:"status"`
	RetryCount  int       `json:"retry_count"`
	ErrorMsg    string    `json:"error_msg,omitempty"`
	MessageID   string    `json:"message_id,omitempty"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

type CreateRequest struct {
	TrackingKey string  `json:"tracking_key" binding:"required"`
	Amount      float64 `json:"amount"       binding:"required,gt=0"`
	Destination string  `json:"destination"  binding:"required"`
	Priority    int     `json:"priority"`
}
