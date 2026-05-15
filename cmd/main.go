package main

import (
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/payment-orchestrator-go/internal/database"
	"github.com/payment-orchestrator-go/internal/domain"
	"github.com/payment-orchestrator-go/internal/mq"
	"github.com/payment-orchestrator-go/internal/repository"
	"github.com/payment-orchestrator-go/internal/service"
	"github.com/payment-orchestrator-go/internal/worker"
)

func main() {
	port := getEnv("PORT", "8080")
	dbPath := getEnv("DB_PATH", "./data/orders.db")
	intervalMs := 3000

	db, err := database.Init(dbPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error inicializando db: %v\n", err)
		os.Exit(1)
	}
	defer db.Close()

	broker := mq.GetBroker(50)
	repo := repository.New(db)
	svc := service.NewOrderService(repo)
	w := worker.New(svc, broker, intervalMs)

	// procesa respuestas del broker en background
	go w.ProcessLoop()

	if err := w.Start(); err != nil {
		fmt.Fprintf(os.Stderr, "error iniciando worker: %v\n", err)
		os.Exit(1)
	}

	gin.SetMode(gin.ReleaseMode)
	r := gin.Default()

	// ── órdenes ──────────────────────────────────────────────────────────────

	r.POST("/orders", func(c *gin.Context) {
		var req domain.CreateRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		o, err := svc.Create(&req)
		if err != nil {
			c.JSON(http.StatusConflict, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusCreated, o)
	})

	r.GET("/orders", func(c *gin.Context) {
		status := domain.Status(c.Query("status"))
		orders, err := svc.List(status)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		if orders == nil {
			orders = []*domain.Order{}
		}
		c.JSON(http.StatusOK, gin.H{"total": len(orders), "orders": orders})
	})

	r.GET("/orders/:id", func(c *gin.Context) {
		o, err := svc.GetByID(c.Param("id"))
		if err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, o)
	})

	r.PUT("/orders/:id/authorize", func(c *gin.Context) {
		o, err := svc.Authorize(c.Param("id"))
		if err != nil {
			c.JSON(http.StatusUnprocessableEntity, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, o)
	})

	// ── worker ───────────────────────────────────────────────────────────────

	r.GET("/worker", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"worker": w.Stats(),
			"broker": broker.Len(),
		})
	})

	r.PUT("/worker/stop", func(c *gin.Context) {
		if err := w.Stop(); err != nil {
			c.JSON(http.StatusConflict, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, gin.H{"status": "stopped"})
	})

	r.PUT("/worker/start", func(c *gin.Context) {
		if err := w.Start(); err != nil {
			c.JSON(http.StatusConflict, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, gin.H{"status": "started"})
	})

	// ── sistema ──────────────────────────────────────────────────────────────

	r.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok", "time": time.Now()})
	})

	// ── arranque ─────────────────────────────────────────────────────────────

	srv := &http.Server{Addr: ":" + port, Handler: r}
	go func() {
		fmt.Printf("payment-orchestrator corriendo en :%s\n", port)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			fmt.Fprintf(os.Stderr, "server error: %v\n", err)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	fmt.Println("apagando...")
	_ = w.Stop()
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
