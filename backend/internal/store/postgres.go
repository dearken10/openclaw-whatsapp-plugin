package store

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Postgres struct {
	pool *pgxpool.Pool
}

func NewPostgres(ctx context.Context, dsn string) (*Postgres, error) {
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		return nil, err
	}
	if err = pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, err
	}
	return &Postgres{pool: pool}, nil
}

func (p *Postgres) CreatePending(record *PairingRecord) error {
	_, err := p.pool.Exec(context.Background(), `
		INSERT INTO device_mappings
		(id, phone_number, instance_id, pairing_code, status, api_key, wab_number, created_at, updated_at, expires_at)
		VALUES ($1, '', $2, $3, $4, $5, $6, $7, $8, $9)
	`, record.ID, record.InstanceID, record.PairingCode, string(record.Status), record.APIKey, record.WabNumber, record.CreatedAt, record.UpdatedAt, record.ExpiresAt)
	return err
}

func (p *Postgres) FindByPhone(phone string) (*PairingRecord, bool, error) {
	row := p.pool.QueryRow(context.Background(), `
		SELECT id, instance_id, api_key, pairing_code, phone_number, wab_number, status, expires_at, created_at, updated_at
		FROM device_mappings
		WHERE phone_number = $1
		LIMIT 1
	`, phone)
	return scanPairingRecord(row)
}

func (p *Postgres) FindByAPIKey(apiKey string) (*PairingRecord, bool, error) {
	row := p.pool.QueryRow(context.Background(), `
		SELECT id, instance_id, api_key, pairing_code, phone_number, wab_number, status, expires_at, created_at, updated_at
		FROM device_mappings
		WHERE api_key = $1
		LIMIT 1
	`, apiKey)
	return scanPairingRecord(row)
}

func (p *Postgres) ActivatePairing(code string, phone string, now time.Time) (*PairingRecord, error) {
	tx, err := p.pool.Begin(context.Background())
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(context.Background())

	row := tx.QueryRow(context.Background(), `
		SELECT id, instance_id, api_key, pairing_code, phone_number, wab_number, status, expires_at, created_at, updated_at
		FROM device_mappings
		WHERE pairing_code = $1
		FOR UPDATE
	`, code)
	record, found, err := scanPairingRecord(row)
	if err != nil {
		return nil, err
	}
	if !found {
		return nil, errors.New("pairing code not found")
	}
	if record.ExpiresAt.Before(now) {
		return nil, errors.New("pairing code expired")
	}
	// Evict any existing active record for the same (wab_number, phone_number)
	// so the partial unique index on (wab_number, phone_number) doesn't conflict.
	_, err = tx.Exec(context.Background(), `
		UPDATE device_mappings
		SET status = $1, updated_at = $2
		WHERE wab_number = $3 AND phone_number = $4 AND status = $5 AND id != $6
	`, string(StatusDisconnected), now, record.WabNumber, phone, string(StatusActive), record.ID)
	if err != nil {
		return nil, err
	}

	_, err = tx.Exec(context.Background(), `
		UPDATE device_mappings
		SET phone_number = $1, status = $2, pairing_code = '', updated_at = $3
		WHERE id = $4
	`, phone, string(StatusActive), now, record.ID)
	if err != nil {
		return nil, err
	}
	if err = tx.Commit(context.Background()); err != nil {
		return nil, err
	}
	record.PhoneNumber = phone
	record.Status = StatusActive
	record.PairingCode = ""
	record.UpdatedAt = now
	return record, nil
}

func (p *Postgres) TrackPairRequest(clientIP string, now time.Time, limit int) (bool, error) {
	_, err := p.pool.Exec(context.Background(), `
		INSERT INTO pair_requests(instance_id, created_at)
		VALUES ($1, $2)
	`, clientIP, now)
	if err != nil {
		return false, err
	}
	var count int
	err = p.pool.QueryRow(context.Background(), `
		SELECT COUNT(*)
		FROM pair_requests
		WHERE instance_id = $1 AND created_at > $2
	`, clientIP, now.Add(-1*time.Hour)).Scan(&count)
	if err != nil {
		return false, err
	}
	return count <= limit, nil
}

func (p *Postgres) Close() error {
	p.pool.Close()
	return nil
}

type rowScanner interface {
	Scan(dest ...interface{}) error
}

func scanPairingRecord(row rowScanner) (*PairingRecord, bool, error) {
	record := &PairingRecord{}
	var status string
	err := row.Scan(
		&record.ID,
		&record.InstanceID,
		&record.APIKey,
		&record.PairingCode,
		&record.PhoneNumber,
		&record.WabNumber,
		&status,
		&record.ExpiresAt,
		&record.CreatedAt,
		&record.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, false, nil
		}
		return nil, false, err
	}
	record.Status = Status(status)
	return record, true, nil
}
