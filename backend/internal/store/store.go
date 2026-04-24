package store

import "time"

type Status string

const (
	StatusPending      Status = "PENDING"
	StatusActive       Status = "ACTIVE"
	StatusDisconnected Status = "DISCONNECTED"
)

type PairingRecord struct {
	ID          string
	InstanceID  string
	APIKey      string
	PairingCode string
	PhoneNumber string
	WabNumber   string
	Status      Status
	ExpiresAt   time.Time
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

type Repository interface {
	CreatePending(record *PairingRecord) error
	FindByPhone(phone string) (*PairingRecord, bool, error)
	FindByAPIKey(apiKey string) (*PairingRecord, bool, error)
	ActivatePairing(code string, phone string, now time.Time) (*PairingRecord, error)
	TrackPairRequest(clientIP string, now time.Time, limit int) (bool, error)
	Close() error
}
