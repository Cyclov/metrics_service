package repository

import (
	"context"
	"errors"
	"time"

	models "github.com/Cyclov/metrics_service/internal/model"
	"github.com/jackc/pgerrcode"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

type PostgresStorage struct{ db *pgxpool.Pool }

var _ Storage = (*PostgresStorage)(nil)

func NewPostgresStorage(db *pgxpool.Pool) *PostgresStorage { return &PostgresStorage{db: db} }

func (s *PostgresStorage) SetGauge(ctx context.Context, name string, value float64) error {
	return retryPostgres(ctx, func() error {
		_, err := s.db.Exec(ctx, `INSERT INTO gauges (name, value) VALUES ($1, $2)
			ON CONFLICT (name) DO UPDATE SET value = EXCLUDED.value`, name, value)
		return err
	})
}

func (s *PostgresStorage) AddCounter(ctx context.Context, name string, delta int64) (int64, error) {
	var value int64
	err := retryPostgres(ctx, func() error {
		return s.db.QueryRow(ctx, `INSERT INTO counters (name, value) VALUES ($1, $2)
			ON CONFLICT (name) DO UPDATE SET value = counters.value + EXCLUDED.value
			RETURNING value`, name, delta).Scan(&value)
	})
	return value, err
}

func (s *PostgresStorage) UpdateBatch(ctx context.Context, metrics []models.Metrics) error {
	if err := validateBatch(metrics); err != nil {
		return err
	}
	return retryPostgres(ctx, func() error { return s.updateBatchOnce(ctx, metrics) })
}

func (s *PostgresStorage) updateBatchOnce(ctx context.Context, metrics []models.Metrics) error {
	tx, err := s.db.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	for _, metric := range metrics {
		switch metric.MType {
		case models.Gauge:
			_, err = tx.Exec(ctx, `INSERT INTO gauges (name, value) VALUES ($1, $2)
				ON CONFLICT (name) DO UPDATE SET value = EXCLUDED.value`, metric.ID, *metric.Value)
		case models.Counter:
			_, err = tx.Exec(ctx, `INSERT INTO counters (name, value) VALUES ($1, $2)
				ON CONFLICT (name) DO UPDATE SET value = counters.value + EXCLUDED.value`, metric.ID, *metric.Delta)
		}
		if err != nil {
			return err
		}
	}

	return tx.Commit(ctx)
}

func (s *PostgresStorage) Gauge(ctx context.Context, name string) (float64, bool, error) {
	var value float64
	err := retryPostgres(ctx, func() error {
		return s.db.QueryRow(ctx, `SELECT value FROM gauges WHERE name = $1`, name).Scan(&value)
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return 0, false, nil
	}
	return value, err == nil, err
}

func (s *PostgresStorage) Counter(ctx context.Context, name string) (int64, bool, error) {
	var value int64
	err := retryPostgres(ctx, func() error {
		return s.db.QueryRow(ctx, `SELECT value FROM counters WHERE name = $1`, name).Scan(&value)
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return 0, false, nil
	}
	return value, err == nil, err
}

func (s *PostgresStorage) AllMetrics(ctx context.Context) (map[string]float64, map[string]int64, error) {
	var gauges map[string]float64
	var counters map[string]int64
	err := retryPostgres(ctx, func() error {
		var err error
		gauges, counters, err = s.allMetricsOnce(ctx)
		return err
	})
	return gauges, counters, err
}

func (s *PostgresStorage) allMetricsOnce(ctx context.Context) (map[string]float64, map[string]int64, error) {
	gauges := make(map[string]float64)
	rows, err := s.db.Query(ctx, `SELECT name, value FROM gauges ORDER BY name`)
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
	rows, err = s.db.Query(ctx, `SELECT name, value FROM counters ORDER BY name`)
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

var postgresRetryDelays = []time.Duration{time.Second, 3 * time.Second, 5 * time.Second}

func retryPostgres(ctx context.Context, operation func() error) error {
	for attempt := 0; ; attempt++ {
		err := operation()
		if err == nil || !isRetriablePostgresError(err) || attempt == len(postgresRetryDelays) {
			return err
		}

		timer := time.NewTimer(postgresRetryDelays[attempt])
		select {
		case <-ctx.Done():
			timer.Stop()
			return ctx.Err()
		case <-timer.C:
		}
	}
}

func isRetriablePostgresError(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && (pgerrcode.IsConnectionException(pgErr.Code) ||
		pgerrcode.IsTransactionRollback(pgErr.Code))
}
