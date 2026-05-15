# payment-orchestrator-go

Servicio de orquestación de pagos escrito en Go. Recibe órdenes de pago a través de una API REST, las procesa de forma asíncrona mediante un worker interno y simula el ciclo completo de despacho y confirmación con un sistema externo.

---

## Requisitos

- Go 1.22 o superior
- CGO habilitado (necesario para SQLite)
- `gcc` o compilador C equivalente

---

## Correr el proyecto

```bash
go mod tidy
make run
```

El servicio arranca en `http://localhost:8080` y persiste datos en `./data/orders.db`.

---

## Seed de datos

Con el servicio corriendo, en otra terminal:

```bash
make seed
```

Crea 5 órdenes y las autoriza. El worker las tomará automáticamente en el siguiente ciclo.

---

## Endpoints

| Método | Ruta | Descripción |
|--------|------|-------------|
| `POST` | `/orders` | Crear orden |
| `GET` | `/orders` | Listar órdenes (opcional: `?status=AUTHORIZED`) |
| `GET` | `/orders/:id` | Detalle de orden |
| `PUT` | `/orders/:id/authorize` | Autorizar orden |
| `GET` | `/worker` | Estado del worker y del broker |
| `PUT` | `/worker/stop` | Detener el worker |
| `PUT` | `/worker/start` | Iniciar el worker |
| `GET` | `/health` | Health check |

### Estados posibles de una orden

```
CREATED → AUTHORIZED → DISPATCHED → PROCESSED
                  ↑          ↓
                  └── RETRY ←┘
                             ↓
                           FAILED
```

---

## Ejemplos curl

```bash
# Crear una orden
curl -X POST http://localhost:8080/orders \
  -H "Content-Type: application/json" \
  -d '{
    "tracking_key": "TRK-TEST-001",
    "amount": 5000.00,
    "destination": "DEST-001122334455",
    "priority": 7
  }'

# Autorizar (reemplaza {id} con el id retornado)
curl -X PUT http://localhost:8080/orders/{id}/authorize

# Ver estado de la orden
curl http://localhost:8080/orders/{id}

# Listar todas las órdenes
curl http://localhost:8080/orders

# Filtrar por estado
curl "http://localhost:8080/orders?status=DISPATCHED"

# Ver estado del worker y colas
curl http://localhost:8080/worker

# Detener y volver a iniciar el worker
curl -X PUT http://localhost:8080/worker/stop
curl -X PUT http://localhost:8080/worker/start
```

---

## Tests

```bash
make test
```

---

## Estructura del proyecto

```
cmd/main.go              → entrypoint + handlers HTTP
internal/
  domain/order.go        → tipos y estados del dominio
  database/db.go         → inicialización y schema
  repository/repository.go → acceso a datos
  service/service.go     → lógica de negocio
  mq/broker.go           → cola de mensajes en memoria
  worker/worker.go       → procesamiento asíncrono
tests/
  service_test.go        → pruebas unitarias
scripts/
  seed.sh                → datos de ejemplo
```

---

## Tu tarea

Eres parte del equipo que mantiene este servicio. Lleva algunos meses en producción y ha crecido de forma orgánica.

**Lo que se espera de ti:**

1. **Corre el servicio** y observa su comportamiento. Usa el seed y los endpoints para generar actividad.
2. **Lee el código** en voz alta explicando qué entiendes de cada parte.
3. **Identifica riesgos técnicos** — áreas donde el sistema podría comportarse mal, perder datos o degradarse.
4. **Explica tu razonamiento** — qué encontraste, por qué es un problema y cómo lo resolverías.

No se espera que arregles todo. Se espera que pienses en voz alta.
