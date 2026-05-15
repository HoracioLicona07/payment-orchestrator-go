package database

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"

	_ "github.com/mattn/go-sqlite3"
)

func Init(path string) (*sql.DB, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return nil, fmt.Errorf("creando directorio: %w", err)
	}
	db, err := sql.Open("sqlite3", path)
	if err != nil {
		return nil, err
	}
	if err := db.Ping(); err != nil {
		return nil, err
	}
	db.SetMaxIdleConns(2)
	if _, err := db.Exec(schema); err != nil {
		return nil, fmt.Errorf("aplicando schema: %w", err)
	}
	return db, nil
}

const schema = `
CREATE TABLE IF NOT EXISTS orders (
    id           TEXT PRIMARY KEY,
    tracking_key TEXT NOT NULL,
    amount       REAL NOT NULL,
    destination  TEXT NOT NULL,
    priority     INTEGER DEFAULT 5,
    status       TEXT NOT NULL DEFAULT 'CREATED',
    retry_count  INTEGER DEFAULT 0,
    error_msg    TEXT DEFAULT '',
    message_id   TEXT DEFAULT '',
    created_at   TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at   TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);
CREATE INDEX IF NOT EXISTS idx_status       ON orders(status);
CREATE INDEX IF NOT EXISTS idx_tracking_key ON orders(tracking_key);
`

// Nota: tracking_key tiene índice pero no restricción UNIQUE.
// La unicidad se valida solo en la capa de servicio.
