package tests

import (
	"database/sql"
	"testing"

	"github.com/payment-orchestrator-go/internal/database"
	"github.com/payment-orchestrator-go/internal/domain"
	"github.com/payment-orchestrator-go/internal/repository"
	"github.com/payment-orchestrator-go/internal/service"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupDB(t *testing.T) *sql.DB {
	t.Helper()
	db, err := database.Init(":memory:")
	require.NoError(t, err)
	return db
}

func newSvc(t *testing.T) *service.OrderService {
	return service.NewOrderService(repository.New(setupDB(t)))
}

func TestCreate(t *testing.T) {
	svc := newSvc(t)
	o, err := svc.Create(&domain.CreateRequest{
		TrackingKey: "TRK-001",
		Amount:      1500.0,
		Destination: "DEST-001",
	})
	require.NoError(t, err)
	assert.Equal(t, domain.StatusCreated, o.Status)
	assert.NotEmpty(t, o.ID)
}

func TestCreateDuplicateTrackingKey(t *testing.T) {
	svc := newSvc(t)
	req := &domain.CreateRequest{TrackingKey: "TRK-DUP", Amount: 100, Destination: "D"}
	_, err := svc.Create(req)
	require.NoError(t, err)
	_, err = svc.Create(req)
	assert.Error(t, err)
}

func TestAuthorize(t *testing.T) {
	svc := newSvc(t)
	o, _ := svc.Create(&domain.CreateRequest{TrackingKey: "TRK-A", Amount: 200, Destination: "D"})
	auth, err := svc.Authorize(o.ID)
	require.NoError(t, err)
	assert.Equal(t, domain.StatusAuthorized, auth.Status)
}

func TestAuthorizeInvalidTransition(t *testing.T) {
	svc := newSvc(t)
	o, _ := svc.Create(&domain.CreateRequest{TrackingKey: "TRK-B", Amount: 200, Destination: "D"})
	_, _ = svc.Authorize(o.ID)
	_, err := svc.Authorize(o.ID) // segunda vez
	assert.Error(t, err)
}

func TestListByStatus(t *testing.T) {
	svc := newSvc(t)
	o, _ := svc.Create(&domain.CreateRequest{TrackingKey: "TRK-C", Amount: 300, Destination: "D"})
	_, _ = svc.Authorize(o.ID)

	authorized, err := svc.List(domain.StatusAuthorized)
	require.NoError(t, err)
	assert.Len(t, authorized, 1)
}

func TestGetRetryableResetsStatus(t *testing.T) {
	svc := newSvc(t)
	o, _ := svc.Create(&domain.CreateRequest{TrackingKey: "TRK-R", Amount: 400, Destination: "D"})
	_ = svc.MarkRetry(o.ID, "timeout")

	retryable, err := svc.GetRetryable()
	require.NoError(t, err)
	assert.Len(t, retryable, 1)

	// después de llamar GetRetryable, la orden debe estar en AUTHORIZED
	updated, _ := svc.GetByID(o.ID)
	assert.Equal(t, domain.StatusAuthorized, updated.Status)
}

func TestMarkProcessed(t *testing.T) {
	svc := newSvc(t)
	o, _ := svc.Create(&domain.CreateRequest{TrackingKey: "TRK-P", Amount: 500, Destination: "D"})
	_ = svc.MarkProcessed(o.ID)
	updated, _ := svc.GetByID(o.ID)
	assert.Equal(t, domain.StatusProcessed, updated.Status)
}
