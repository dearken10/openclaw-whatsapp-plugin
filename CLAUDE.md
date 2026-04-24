# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## What This Repo Is

An MVP for connecting WhatsApp Cloud API (via imBee routing) to the OpenClaw AI agent platform. It consists of two independently deployable pieces:

- **`backend/`** — Go routing server that bridges Meta webhooks to connected OpenClaw plugin instances via WebSocket
- **`plugin/`** — TypeScript OpenClaw channel plugin that runs inside the OpenClaw gateway process

## Commands

### Backend (Go)

```bash
# Run all tests
cd backend && go test ./...

# Run a single package's tests
cd backend && go test ./internal/http/...

# Start full stack (Docker Compose: backend + Postgres)
make up        # or ./scripts/dev-up.sh

# Stop stack
make down

# Smoke test (healthz + pair + webhook round-trip)
make smoke

# Pairing workflow (dev CLI)
make pair                                      # request pairing code
make ws API_KEY=imbee_xxx                      # open WS listener
make send API_KEY=imbee_xxx TEXT="hello"       # send outbound stub
TEXT="hello" ./scripts/replay-webhook.sh      # simulate inbound webhook
```

### Plugin (TypeScript / Node)

The plugin has no build step — OpenClaw loads `.ts` files directly at runtime (Node >=22, peer dep `openclaw >=2026.4.15`). Install into a local OpenClaw environment:

```bash
openclaw plugins install -l ./plugin    # dev link (no copy)
openclaw gateway restart
```

## Architecture

### Backend message flow

```
Meta Cloud API webhook
  → POST /webhooks/whatsapp   (HMAC-SHA256 verified)
    → handlers.go:routeIncoming()
      → pairingCodeRegex match → store.ActivatePairing() → ws.Hub.Send("PAIRING_COMPLETE")
      → normal message       → store.FindByPhone()     → ws.Hub.Send("INBOUND_MESSAGE")

Plugin (WebSocket client, /ws endpoint)
  ← receives WsEnvelope{type, payload, timestamp, message_id}
```

- **`ws.Hub`** (`backend/internal/ws/hub.go`) — in-memory map of `instanceId → *websocket.Conn`; routes outbound envelopes to the right connected plugin instance
- **`store.Repository`** (`backend/internal/store/`) — interface with two drivers: `memory` (default dev) and `postgres`. Switch via `STORE_DRIVER=memory|postgres`
- **`pairing.Service`** (`backend/internal/pairing/service.go`) — creates `CLAW-XXXX-YYYY` codes, rate-limits requests, manages TTL
- **`/api/v1/send`** — currently a stub (`accepted`); does not call Meta Cloud API yet

### Plugin flow

```
OpenClaw gateway
  → startWhatsappOfficialGatewayAccount() (gateway.ts)
    → WebSocket to backend /ws (exponential backoff reconnect)
      → on INBOUND_MESSAGE → handleWhatsappOfficialInbound() (inbound.ts)
        → channelRuntime.channel.routing.resolveAgentRoute()
        → dispatchInboundReplyWithBase()    (openclaw SDK)
          → deliver() → sendOutboundText()  (transport.ts → POST /api/v1/send)
```

- **`plugin/src/transport.ts`** — config resolution (`resolveAccountFromCfg`) and HTTP calls (`requestPairingCode`, `sendOutboundText`)
- **`plugin/src/inbound.ts`** — assembles the OpenClaw agent envelope from raw WA message fields; delegates delivery back via `sendOutboundText`
- **`plugin/src/gateway.ts`** — long-lived WS loop with exponential backoff; the main account entry-point
- Plugin config lives under `channels.whatsapp-official` in the OpenClaw config file

### Environment variables (backend)

| Variable | Default | Purpose |
|---|---|---|
| `HTTP_ADDR` | `:8080` | Listen address |
| `STORE_DRIVER` | `memory` | `memory` or `postgres` |
| `POSTGRES_DSN` | local dev DSN | Postgres connection |
| `SHARED_WA_NUMBER` | `+18885550100` | Shown in `wa.me` pairing URL |
| `WEBHOOK_APP_SECRET` | `dev-secret` | HMAC key for Meta signature |
| `PAIRING_CODE_TTL_SECONDS` | `600` | Pairing code expiry |
| `PAIR_RATE_LIMIT_PER_HOUR` | `5` | Max pair requests per hour |

Docker Compose defaults expose backend on `localhost:28080` and Postgres on `localhost:28032`.

## Key Constraints

- `PLUGIN_ID = "whatsapp-official"` must stay in sync across `constants.ts`, `openclaw.plugin.json`, and any `defineChannelPluginEntry({ id })` call.
- Plugin version must be bumped in **both** `plugin/package.json` and `plugin/openclaw.plugin.json` together before publishing.
- `/api/v1/send` is an MVP stub — it does not call Meta. Production work requires implementing the actual Cloud API call in `handlers.go:handleSend`.
