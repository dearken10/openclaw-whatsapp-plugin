package store

import (
	"database/sql"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	_ "modernc.org/sqlite"
)

// sqliteStore is an SQLite-backed persistent store.
// It requires no external server — the database lives in a single .db file.
// WAL mode is enabled so concurrent reads never block writes.
type sqliteStore struct {
	db *sql.DB
}

const sqliteSchema = `
CREATE TABLE IF NOT EXISTS pairing_records (
	id           TEXT PRIMARY KEY,
	instance_id  TEXT NOT NULL,
	api_key      TEXT NOT NULL UNIQUE,
	pairing_code TEXT,
	phone_number TEXT,
	wab_number   TEXT NOT NULL DEFAULT '',
	status       TEXT NOT NULL DEFAULT 'PENDING',
	expires_at   INTEGER NOT NULL,
	created_at   INTEGER NOT NULL,
	updated_at   INTEGER NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_pairing_records_phone  ON pairing_records(phone_number);
CREATE INDEX IF NOT EXISTS idx_pairing_records_code   ON pairing_records(pairing_code);

CREATE TABLE IF NOT EXISTS pair_requests (
	client_ip    TEXT NOT NULL,
	requested_at INTEGER NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_pair_requests_ip ON pair_requests(client_ip, requested_at);
`

// NewSQLite opens (or creates) an SQLite database at path and runs the schema.
func NewSQLite(path string) (*sqliteStore, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, fmt.Errorf("sqlite store: mkdir: %w", err)
	}
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, fmt.Errorf("sqlite store: open: %w", err)
	}
	// WAL mode: concurrent reads don't block writes; single writer at a time.
	if _, err = db.Exec(`PRAGMA journal_mode=WAL; PRAGMA foreign_keys=ON;`); err != nil {
		return nil, fmt.Errorf("sqlite store: pragma: %w", err)
	}
	if _, err = db.Exec(sqliteSchema); err != nil {
		return nil, fmt.Errorf("sqlite store: schema: %w", err)
	}
	return &sqliteStore{db: db}, nil
}

func (s *sqliteStore) CreatePending(r *PairingRecord) error {
	_, err := s.db.Exec(`
		INSERT INTO pairing_records
			(id, instance_id, api_key, pairing_code, phone_number, wab_number, status, expires_at, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		r.ID, r.InstanceID, r.APIKey, r.PairingCode, r.PhoneNumber, r.WabNumber,
		string(r.Status), r.ExpiresAt.Unix(), r.CreatedAt.Unix(), r.UpdatedAt.Unix(),
	)
	if err != nil {
		return fmt.Errorf("sqlite store: CreatePending: %w", err)
	}
	return nil
}

func (s *sqliteStore) FindByPhone(phone string) (*PairingRecord, bool, error) {
	row := s.db.QueryRow(`
		SELECT id, instance_id, api_key, pairing_code, phone_number, wab_number,
		       status, expires_at, created_at, updated_at
		FROM   pairing_records
		WHERE  phone_number = ? AND status = 'ACTIVE'
		ORDER  BY updated_at DESC
		LIMIT  1`, phone)
	r, err := scanRecord(row)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, fmt.Errorf("sqlite store: FindByPhone: %w", err)
	}
	return r, true, nil
}

func (s *sqliteStore) FindByAPIKey(apiKey string) (*PairingRecord, bool, error) {
	row := s.db.QueryRow(`
		SELECT id, instance_id, api_key, pairing_code, phone_number, wab_number,
		       status, expires_at, created_at, updated_at
		FROM   pairing_records
		WHERE  api_key = ?
		LIMIT  1`, apiKey)
	r, err := scanRecord(row)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, fmt.Errorf("sqlite store: FindByAPIKey: %w", err)
	}
	return r, true, nil
}

func (s *sqliteStore) ActivatePairing(code string, phone string, now time.Time) (*PairingRecord, error) {
	tx, err := s.db.Begin()
	if err != nil {
		return nil, fmt.Errorf("sqlite store: ActivatePairing: begin: %w", err)
	}
	defer tx.Rollback() //nolint:errcheck

	// Fetch the pending record.
	row := tx.QueryRow(`
		SELECT id, instance_id, api_key, pairing_code, phone_number, wab_number,
		       status, expires_at, created_at, updated_at
		FROM   pairing_records
		WHERE  pairing_code = ?`, code)
	r, err := scanRecord(row)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, errors.New("pairing code not found")
	}
	if err != nil {
		return nil, fmt.Errorf("sqlite store: ActivatePairing: lookup: %w", err)
	}
	if r.ExpiresAt.Before(now) {
		return nil, errors.New("pairing code expired")
	}

	// Evict any existing active record for the same (wab_number, phone_number).
	_, err = tx.Exec(`
		UPDATE pairing_records
		SET    status = 'DISCONNECTED', updated_at = ?
		WHERE  wab_number = ? AND phone_number = ? AND status = 'ACTIVE' AND id != ?`,
		now.Unix(), r.WabNumber, phone, r.ID)
	if err != nil {
		return nil, fmt.Errorf("sqlite store: ActivatePairing: evict: %w", err)
	}

	// Activate.
	_, err = tx.Exec(`
		UPDATE pairing_records
		SET    phone_number = ?, status = 'ACTIVE', pairing_code = NULL, updated_at = ?
		WHERE  id = ?`,
		phone, now.Unix(), r.ID)
	if err != nil {
		return nil, fmt.Errorf("sqlite store: ActivatePairing: activate: %w", err)
	}

	if err = tx.Commit(); err != nil {
		return nil, fmt.Errorf("sqlite store: ActivatePairing: commit: %w", err)
	}

	r.PhoneNumber = phone
	r.Status = StatusActive
	r.PairingCode = ""
	r.UpdatedAt = now
	return r, nil
}

func (s *sqliteStore) TrackPairRequest(clientIP string, now time.Time, limit int) (bool, error) {
	tx, err := s.db.Begin()
	if err != nil {
		return false, fmt.Errorf("sqlite store: TrackPairRequest: begin: %w", err)
	}
	defer tx.Rollback() //nolint:errcheck

	oneHourAgo := now.Add(-time.Hour).Unix()

	// Purge stale entries.
	if _, err = tx.Exec(`DELETE FROM pair_requests WHERE client_ip = ? AND requested_at < ?`,
		clientIP, oneHourAgo); err != nil {
		return false, fmt.Errorf("sqlite store: TrackPairRequest: purge: %w", err)
	}

	// Count recent requests.
	var count int
	if err = tx.QueryRow(`SELECT COUNT(*) FROM pair_requests WHERE client_ip = ?`, clientIP).
		Scan(&count); err != nil {
		return false, fmt.Errorf("sqlite store: TrackPairRequest: count: %w", err)
	}
	if count >= limit {
		_ = tx.Commit()
		return false, nil
	}

	if _, err = tx.Exec(`INSERT INTO pair_requests (client_ip, requested_at) VALUES (?, ?)`,
		clientIP, now.Unix()); err != nil {
		return false, fmt.Errorf("sqlite store: TrackPairRequest: insert: %w", err)
	}
	if err = tx.Commit(); err != nil {
		return false, fmt.Errorf("sqlite store: TrackPairRequest: commit: %w", err)
	}
	return true, nil
}

func (s *sqliteStore) Close() error {
	return s.db.Close()
}

// scanRecord reads one row from a QueryRow result.
func scanRecord(row *sql.Row) (*PairingRecord, error) {
	var r PairingRecord
	var status string
	var expiresAt, createdAt, updatedAt int64
	var pairingCode, phoneNumber sql.NullString
	err := row.Scan(
		&r.ID, &r.InstanceID, &r.APIKey, &pairingCode, &phoneNumber, &r.WabNumber,
		&status, &expiresAt, &createdAt, &updatedAt,
	)
	if err != nil {
		return nil, err
	}
	r.PairingCode = pairingCode.String
	r.PhoneNumber = phoneNumber.String
	r.Status = Status(status)
	r.ExpiresAt = time.Unix(expiresAt, 0).UTC()
	r.CreatedAt = time.Unix(createdAt, 0).UTC()
	r.UpdatedAt = time.Unix(updatedAt, 0).UTC()
	return &r, nil
}
