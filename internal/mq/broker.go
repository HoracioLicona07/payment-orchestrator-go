package mq

import (
	"sync"
	"time"
)

type Message struct {
	ID          string
	OrderID     string
	TrackingKey string
	Amount      float64
	Destination string
	SentAt      time.Time
}

type Response struct {
	MessageID string
	OrderID   string
	OK        bool
	Detail    string
}

// Broker simula una cola de mensajes en memoria.
type Broker struct {
	mu       sync.Mutex
	outgoing chan *Message
	incoming chan *Response
}

var (
	singleton *Broker
	once      sync.Once
)

func GetBroker(size int) *Broker {
	once.Do(func() {
		singleton = &Broker{
			outgoing: make(chan *Message, size),
			incoming: make(chan *Response, size),
		}
	})
	return singleton
}

// Send publica un mensaje en la cola de salida.
// Si la cola está llena, el mensaje se descarta sin error.
// El caller no puede distinguir entre envío exitoso y descarte.
func (b *Broker) Send(msg *Message) error {
	b.mu.Lock()
	defer b.mu.Unlock()
	select {
	case b.outgoing <- msg:
	default:
		// cola llena — se descarta silenciosamente
	}
	return nil
}

// Respond publica una respuesta simulando al sistema externo.
func (b *Broker) Respond(r *Response) error {
	select {
	case b.incoming <- r:
	default:
	}
	return nil
}

func (b *Broker) Outgoing() <-chan *Message  { return b.outgoing }
func (b *Broker) Incoming() <-chan *Response { return b.incoming }

func (b *Broker) Len() map[string]int {
	return map[string]int{
		"outgoing": len(b.outgoing),
		"incoming": len(b.incoming),
	}
}
