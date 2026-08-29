package repository

import (
	"context"
	"database/sql"

	models "github.com/Cyclov/metrics_service/internal/model"
)

type PostgresStorage struct{ db *sql.DB }

var _ Storage = (*PostgresStorage)(nil)

func NewPostgresStorage(db *sql.DB) *PostgresStorage { return &PostgresStorage{db: db} }

func (s *PostgresStorage) SetGauge(ctx context.Context, name string, value float64) error {
	_, err := s.db.ExecContext(ctx, `INSERT INTO gauges (name, value) VALUES ($1, $2)
		ON CONFLICT (name) DO UPDATE SET value = EXCLUDED.value`, name, value)
	return err
}

func (s *PostgresStorage) AddCounter(ctx context.Context, name string, delta int64) (int64, error) {
	var value int64
	err := s.db.QueryRowContext(ctx, `INSERT INTO counters (name, value) VALUES ($1, $2)
		ON CONFLICT (name) DO UPDATE SET value = counters.value + EXCLUDED.value
		RETURNING value`, name, delta).Scan(&value)
	return value, err
}

func (s *PostgresStorage) UpdateBatch(ctx context.Context, metrics []models.Metrics) error {
	if err := validateBatch(metrics); err != nil {
		return err
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	for _, metric := range metrics {
		switch metric.MType {
		case models.Gauge:
			_, err = tx.ExecContext(ctx, `INSERT INTO gauges (name, value) VALUES ($1, $2)
				ON CONFLICT (name) DO UPDATE SET value = EXCLUDED.value`, metric.ID, *metric.Value)
		case models.Counter:
			_, err = tx.ExecContext(ctx, `INSERT INTO counters (name, value) VALUES ($1, $2)
				ON CONFLICT (name) DO UPDATE SET value = counters.value + EXCLUDED.value`, metric.ID, *metric.Delta)
		}
		if err != nil {
			return err
		}
	}

	return tx.Commit()
}

func (s *PostgresStorage) Gauge(ctx context.Context, name string) (float64, bool, error) {
	var value float64
	err := s.db.QueryRowContext(ctx, `SELECT value FROM gauges WHERE name = $1`, name).Scan(&value)
	if err == sql.ErrNoRows {
		return 0, false, nil
	}
	return value, err == nil, err
}

func (s *PostgresStorage) Counter(ctx context.Context, name string) (int64, bool, error) {
	var value int64
	err := s.db.QueryRowContext(ctx, `SELECT value FROM counters WHERE name = $1`, name).Scan(&value)
	if err == sql.ErrNoRows {
		return 0, false, nil
	}
	return value, err == nil, err
}

func (s *PostgresStorage) AllMetrics(ctx context.Context) (map[string]float64, map[string]int64, error) {
	gauges := make(map[string]float64)
	rows, err := s.db.QueryContext(ctx, `SELECT name, value FROM gauges ORDER BY name`)
	if err != nil {
		return nil, nil, err
	}
	for rows.Next() {
		var name string
		var value float64
		if err := rows.Scan(&name, &value); err != nil {
			rows.Close()
			return nil, nil, err
		}
		gauges[name] = value
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return nil, nil, err
	}
	rows.Close()

	counters := make(map[string]int64)
	rows, err = s.db.QueryContext(ctx, `SELECT name, value FROM counters ORDER BY name`)
	if err != nil {
		return nil, nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var name string
		var value int64
		if err := rows.Scan(&name, &value); err != nil {
			return nil, nil, err
		}
		counters[name] = value
	}
	if err := rows.Err(); err != nil {
		return nil, nil, err
	}
	return gauges, counters, nil
}
