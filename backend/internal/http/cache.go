package http

import (
	"sync"

	"github.com/imbee/openclaw-whatsapp-official/backend/internal/store"
)

// recordCache is a write-through in-process cache for PairingRecords.
// It is keyed by both phone number and API key so all hot-path lookups
// (inbound webhook routing and outbound send auth) avoid round-trips to
// the store after the first hit.
type recordCache struct {
	mu       sync.RWMutex
	byPhone  map[string]*store.PairingRecord
	byAPIKey map[string]*store.PairingRecord
}

func newRecordCache() *recordCache {
	return &recordCache{
		byPhone:  make(map[string]*store.PairingRecord),
		byAPIKey: make(map[string]*store.PairingRecord),
	}
}

func (c *recordCache) getByPhone(phone string) (*store.PairingRecord, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	r, ok := c.byPhone[phone]
	return r, ok
}

func (c *recordCache) getByAPIKey(apiKey string) (*store.PairingRecord, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	r, ok := c.byAPIKey[apiKey]
	return r, ok
}

// set stores (or replaces) a record in both indexes.
// Callers must pass the pointer returned directly by the store so the
// cached value always reflects the latest persisted state.
func (c *recordCache) set(r *store.PairingRecord) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if r.PhoneNumber != "" {
		c.byPhone[r.PhoneNumber] = r
	}
	if r.APIKey != "" {
		c.byAPIKey[r.APIKey] = r
	}
}
