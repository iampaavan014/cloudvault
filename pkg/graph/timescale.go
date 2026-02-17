package graph

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/cloudvault-io/cloudvault/pkg/types"
	_ "github.com/lib/pq"
)

// TimescaleDB handles historical metric persistence for the SIG
type TimescaleDB struct {
	db *sql.DB
}

// NewTimescaleDB creates a new TimescaleDB client
func NewTimescaleDB(connStr string) (*TimescaleDB, error) {
	db, err := sql.Open("postgres", connStr)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to timescale: %w", err)
	}

	if err := db.Ping(); err != nil {
		return nil, fmt.Errorf("failed to ping timescale: %w", err)
	}

	ts := &TimescaleDB{db: db}
	if err := ts.initializeSchema(); err != nil {
		return nil, err
	}

	return ts, nil
}

func (t *TimescaleDB) initializeSchema() error {
	// Create metrics table if not exists
	_, err := t.db.Exec(`
		CREATE TABLE IF NOT EXISTS pvc_metrics (
			time TIMESTAMPTZ NOT NULL,
			pvc_name TEXT NOT NULL,
			namespace TEXT NOT NULL,
			used_bytes BIGINT,
			egress_bytes BIGINT,
			iops DOUBLE PRECISION,
			monthly_cost DOUBLE PRECISION
		);
		SELECT create_hypertable('pvc_metrics', 'time', if_not_exists => TRUE);
	`)
	return err
}

// RecordMetrics saves a batch of PVC metrics to TimescaleDB
func (t *TimescaleDB) RecordMetrics(ctx context.Context, metrics []types.PVCMetric) error {
	tx, err := t.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()

	stmt, err := tx.PrepareContext(ctx, `
		INSERT INTO pvc_metrics (time, pvc_name, namespace, used_bytes, egress_bytes, iops, monthly_cost)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
	`)
	if err != nil {
		return err
	}
	defer func() { _ = stmt.Close() }()

	now := time.Now()
	for _, m := range metrics {
		_, err := stmt.ExecContext(ctx,
			now,
			m.Name,
			m.Namespace,
			m.UsedBytes,
			m.EgressBytes,
			m.ReadIOPS+m.WriteIOPS,
			m.MonthlyCost,
		)
		if err != nil {
			return err
		}
	}

	return tx.Commit()
}

// GetHistory retrieves historical metrics for a specific PVC to feed AI models
func (t *TimescaleDB) GetHistory(ctx context.Context, namespace, name string, duration time.Duration) ([]float64, error) {
	rows, err := t.db.QueryContext(ctx, `
		SELECT used_bytes FROM pvc_metrics 
		WHERE namespace = $1 AND pvc_name = $2 AND time > $3 
		ORDER BY time ASC
	`, namespace, name, time.Now().Add(-duration))
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	var history []float64
	for rows.Next() {
		var val int64
		if err := rows.Scan(&val); err == nil {
			history = append(history, float64(val))
		}
	}
	return history, nil
}

func (t *TimescaleDB) Close() error {
	return t.db.Close()
}
