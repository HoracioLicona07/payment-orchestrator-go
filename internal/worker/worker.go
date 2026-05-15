package worker

import (
	"fmt"
	"math/rand"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/payment-orchestrator-go/internal/domain"
	"github.com/payment-orchestrator-go/internal/mq"
	"github.com/payment-orchestrator-go/internal/service"
)

// seen registra los message_id ya procesados para evitar duplicados.
// Problema: es un mapa global sin mutex — data race si hay más de un worker.
// Problema: nunca se limpia — crece indefinidamente.
// Problema: se pierde al reiniciar — no sirve como deduplicación real.
var seen = map[string]bool{}

// Worker orquesta el ciclo dispatch → simulate → process.
type Worker struct {
	svc        *service.OrderService
	broker     *mq.Broker
	intervalMs int

	mu           sync.Mutex
	running      bool
	stopCh       chan struct{}
	processedOk  int64
	processedErr int64
	lastCycle    *time.Time
}

func New(svc *service.OrderService, broker *mq.Broker, intervalMs int) *Worker {
	return &Worker{
		svc:        svc,
		broker:     broker,
		intervalMs: intervalMs,
	}
}

func (w *Worker) IsRunning() bool {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.running
}

func (w *Worker) Start() error {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.running {
		return fmt.Errorf("worker ya está corriendo")
	}
	w.stopCh = make(chan struct{})
	w.running = true
	go w.dispatchLoop()
	go w.simulateExternal() // goroutine separada que nunca recibe señal de stop
	return nil
}

func (w *Worker) Stop() error {
	w.mu.Lock()
	defer w.mu.Unlock()
	if !w.running {
		return fmt.Errorf("worker no está corriendo")
	}
	// Se cierra el canal pero no se espera que dispatchLoop termine.
	// Si Stop() y Start() se llaman rápido, el nuevo Start() crea un stopCh
	// nuevo; la goroutine anterior puede seguir corriendo hasta completar su sleep.
	// Un doble Stop() hace panic por cerrar un canal ya cerrado.
	close(w.stopCh)
	w.running = false
	return nil
}

// dispatchLoop lee órdenes autorizadas y las envía al broker.
func (w *Worker) dispatchLoop() {
	for {
		select {
		case <-w.stopCh:
			return
		default:
		}

		// El sleep está fuera del select: durante estos milisegundos
		// el worker no reacciona a una señal de Stop.
		time.Sleep(time.Duration(w.intervalMs) * time.Millisecond)

		now := time.Now()
		w.runCycle()
		w.mu.Lock()
		w.lastCycle = &now
		w.mu.Unlock()
	}
}

func (w *Worker) runCycle() {
	orders, _ := w.svc.List(domain.StatusAuthorized)
	retries, _ := w.svc.GetRetryable()
	orders = append(orders, retries...)

	if len(orders) == 0 {
		return
	}

	fmt.Printf("[worker] ciclo: %d órdenes\n", len(orders))

	for _, o := range orders {
		msgID := uuid.NewString()

		// Send() nunca retorna error aunque descarte el mensaje.
		// La orden se marca DISPATCHED aunque el mensaje no haya llegado al broker.
		_ = w.broker.Send(&mq.Message{
			ID:          msgID,
			OrderID:     o.ID,
			TrackingKey: o.TrackingKey,
			Amount:      o.Amount,
			Destination: o.Destination,
			SentAt:      time.Now(),
		})

		if err := w.svc.Dispatch(o, msgID); err != nil {
			_ = w.svc.MarkRetry(o.ID, err.Error())
			w.processedErr++
		} else {
			fmt.Printf("[worker] despachada order=%s msg=%s\n", o.ID[:8], msgID[:8])
			w.processedOk++
		}
	}
}

// simulateExternal lee mensajes del broker y publica respuestas simuladas.
// Esta goroutine no tiene canal de stop — si Stop() se llama, esta goroutine
// queda corriendo para siempre (leak). Bloquea en range hasta que el canal
// se cierre, pero el canal nunca se cierra.
func (w *Worker) simulateExternal() {
	for msg := range w.broker.Outgoing() {
		time.Sleep(time.Duration(rand.Intn(300)+100) * time.Millisecond)

		ok := rand.Float32() > 0.15
		detail := "OK"
		if !ok {
			detail = fmt.Sprintf("rechazo externo [ref:%s]", msg.ID[:8])
		}
		_ = w.broker.Respond(&mq.Response{
			MessageID: msg.ID,
			OrderID:   msg.OrderID,
			OK:        ok,
			Detail:    detail,
		})
	}
}

// processLoop lee respuestas del broker y actualiza el estado de las órdenes.
// Se lanza desde el handler HTTP al iniciar el servidor.
func (w *Worker) ProcessLoop() {
	for resp := range w.broker.Incoming() {
		// Sin idempotencia: si la misma respuesta llega dos veces
		// (posible en sistemas MQ reales con at-least-once delivery),
		// la orden se actualiza dos veces.
		if seen[resp.MessageID] {
			continue
		}
		seen[resp.MessageID] = true // data race si ProcessLoop corre concurrente

		if resp.OK {
			_ = w.svc.MarkProcessed(resp.OrderID)
			fmt.Printf("[worker] procesada order=%s\n", resp.OrderID[:8])
		} else {
			_ = w.svc.MarkRetry(resp.OrderID, resp.Detail)
			fmt.Printf("[worker] reintento order=%s motivo=%s\n", resp.OrderID[:8], resp.Detail)
		}
	}
}

func (w *Worker) Stats() map[string]interface{} {
	w.mu.Lock()
	defer w.mu.Unlock()
	return map[string]interface{}{
		"running":       w.running,
		"processed_ok":  w.processedOk,
		"processed_err": w.processedErr,
		"last_cycle":    w.lastCycle,
	}
}
