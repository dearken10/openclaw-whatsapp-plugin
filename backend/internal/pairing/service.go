package pairing

import (
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"math/big"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/imbee/openclaw-whatsapp-official/backend/internal/config"
	"github.com/imbee/openclaw-whatsapp-official/backend/internal/store"
)

const alphabet = "ABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"

type Service struct {
	cfg   config.Config
	store store.Repository
}

func NewService(cfg config.Config, store store.Repository) *Service {
	return &Service{cfg: cfg, store: store}
}

func (s *Service) CreatePairing(clientIP string) (*store.PairingRecord, error) {
	instanceID := uuid.NewString()
	now := time.Now().UTC()
	allowed, err := s.store.TrackPairRequest(clientIP, now, s.cfg.PairRequestPerHour)
	if err != nil {
		return nil, err
	}
	if !allowed {
		return nil, fmt.Errorf("rate limit exceeded")
	}
	code, err := generateCode()
	if err != nil {
		return nil, err
	}
	apiKey, err := generateAPIKey()
	if err != nil {
		return nil, err
	}
	record := &store.PairingRecord{
		ID:          uuid.NewString(),
		InstanceID:  instanceID,
		APIKey:      apiKey,
		PairingCode: code,
		WabNumber:   s.cfg.SharedNumber,
		Status:      store.StatusPending,
		CreatedAt:   now,
		UpdatedAt:   now,
		ExpiresAt:   now.Add(s.cfg.PairingCodeTTL),
	}
	if err = s.store.CreatePending(record); err != nil {
		return nil, err
	}
	return record, nil
}

func WaMeURL(sharedNumber string, pairingCode string) string {
	return fmt.Sprintf("https://wa.me/%s?text=%s", strings.TrimPrefix(sharedNumber, "+"), pairingCode)
}

func generateCode() (string, error) {
	partA, err := randomToken(4)
	if err != nil {
		return "", err
	}
	partB, err := randomToken(4)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("CLAW-%s-%s", partA, partB), nil
}

func randomToken(length int) (string, error) {
	var b strings.Builder
	for i := 0; i < length; i++ {
		n, err := rand.Int(rand.Reader, big.NewInt(int64(len(alphabet))))
		if err != nil {
			return "", err
		}
		b.WriteByte(alphabet[n.Int64()])
	}
	return b.String(), nil
}

func generateAPIKey() (string, error) {
	raw := make([]byte, 24)
	if _, err := rand.Read(raw); err != nil {
		return "", err
	}
	return "imbee_" + base64.RawURLEncoding.EncodeToString(raw), nil
}
