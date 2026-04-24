CREATE TABLE IF NOT EXISTS device_mappings (
  id TEXT PRIMARY KEY,
  phone_number VARCHAR(20) NOT NULL DEFAULT '',
  instance_id VARCHAR(64) NOT NULL,
  pairing_code VARCHAR(16) NOT NULL DEFAULT '',
  status VARCHAR(16) NOT NULL,
  api_key TEXT NOT NULL UNIQUE,
  created_at TIMESTAMP NOT NULL,
  updated_at TIMESTAMP NOT NULL,
  expires_at TIMESTAMP NOT NULL
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_device_mappings_pairing_code
  ON device_mappings(pairing_code)
  WHERE pairing_code <> '';

CREATE INDEX IF NOT EXISTS idx_device_mappings_phone_number
  ON device_mappings(phone_number);

CREATE TABLE IF NOT EXISTS pair_requests (
  instance_id VARCHAR(64) NOT NULL,
  created_at TIMESTAMP NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_pair_requests_instance_created
  ON pair_requests(instance_id, created_at);
