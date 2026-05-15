package service

import (
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/payment-orchestrator-go/internal/domain"
	"github.com/payment-orchestrator-go/internal/repository"
)

type OrderService struct {
	repo *repository.OrderRepo
}

func NewOrderService(repo *repository.OrderRepo) *OrderService {
	return &OrderService{repo: repo}
}

// Create registra una nueva orden de pago.
//
// La unicidad del tracking_key se verifica antes del insert, pero ambas
// operaciones no son atómicas: bajo carga concurrente dos requests con el
// mismo tracking_key pueden pasar la validación simultáneamente e insertarse
// las dos. El esquema tampoco tiene UNIQUE constraint como segunda línea de defensa.
func (s *OrderService) Create(req *domain.CreateRequest) (*domain.Order, error) {
	n, err := s.repo.CountByTrackingKey(req.TrackingKey)
	if err != nil {
		return nil, err
	}
	if n > 0 {
		return nil, fmt.Errorf("tracking_key %q ya existe", req.TrackingKey)
	}

	p := req.Priority
	if p == 0 {
		p = 5
	}

	o := &domain.Order{
		ID:          uuid.NewString(),
		TrackingKey: req.TrackingKey,
		Amount:      req.Amount,
		Destination: req.Destination,
		Priority:    p,
		Status:      domain.StatusCreated,
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}
	if err := s.repo.Insert(o); err != nil {
		return nil, err
	}
	return o, nil
}

func (s *OrderService) Authorize(id string) (*domain.Order, error) {
	o, err := s.repo.FindByID(id)
	if err != nil {
		return nil, err
	}
	if o == nil {
		return nil, fmt.Errorf("orden %s no encontrada", id)
	}
	if o.Status != domain.StatusCreated {
		return nil, fmt.Errorf("no se puede autorizar una orden en estado %s", o.Status)
	}
	if err := s.repo.SetStatus(id, domain.StatusAuthorized); err != nil {
		return nil, err
	}
	o.Status = domain.StatusAuthorized
	return o, nil
}

func (s *OrderService) GetByID(id string) (*domain.Order, error) {
	o, err := s.repo.FindByID(id)
	if err != nil {
		return nil, err
	}
	if o == nil {
		return nil, fmt.Errorf("orden %s no encontrada", id)
	}
	return o, nil
}

func (s *OrderService) List(status domain.Status) ([]*domain.Order, error) {
	if status == "" {
		return s.repo.FindAll()
	}
	return s.repo.FindByStatus(status)
}

func (s *OrderService) Dispatch(o *domain.Order, msgID string) error {
	return s.repo.Dispatch(o.ID, msgID)
}

func (s *OrderService) MarkProcessed(id string) error {
	return s.repo.SetStatus(id, domain.StatusProcessed)
}

func (s *OrderService) MarkFailed(id, reason string) error {
	return s.repo.SetStatusAndError(id, domain.StatusFailed, reason)
}

func (s *OrderService) MarkRetry(id, reason string) error {
	_ = s.repo.SetStatusAndError(id, domain.StatusRetry, reason)
	_ = s.repo.IncrRetry(id)
	return nil
}

// GetRetryable devuelve órdenes en estado RETRY y las resetea a AUTHORIZED
// para que sean reprocesadas en el siguiente ciclo del worker.
//
// Problema 1: un método Get* con side-effect muta la base de datos.
// Problema 2: si dos goroutines lo llaman simultáneamente, las mismas órdenes
//             se reprocesarán dos veces.
// Problema 3: no hay límite de reintentos — un error permanente reintenta
//             indefinidamente.
func (s *OrderService) GetRetryable() ([]*domain.Order, error) {
	orders, err := s.repo.FindByStatus(domain.StatusRetry)
	if err != nil {
		return nil, err
	}
	for _, o := range orders {
		_ = s.repo.SetStatus(o.ID, domain.StatusAuthorized)
	}
	return orders, nil
}
