# Product Requirements Document
## Official WhatsApp Business API Channel Plugin for OpenClaw

**Product:** `whatsapp-official` (plugin id); npm package `openclaw-channel-whatsapp-official`; channel id `whatsapp-official`
**Author:** imBee Limited
**Status:** Draft v1.2
**Date:** April 23, 2026

---

## 1. Executive Summary

This document defines the requirements for an **Official WhatsApp Business API channel plugin** for the OpenClaw AI agent platform. The product enables any OpenClaw user to connect their locally-running AI agent to WhatsApp through a **shared, centrally-managed Official WhatsApp Business number**, without requiring individual users to complete Meta's Business verification process or manage their own API credentials.

The solution is delivered as a native OpenClaw channel plugin that integrates into the platform's existing setup wizard. During onboarding, the local machine requests a unique pairing code from a centralized routing server, renders it as a QR code and a `wa.me` deep link, and waits for the user to send that code from their personal WhatsApp account. Once the code is received, the server creates a permanent mapping between the user's phone number and their local OpenClaw instance, enabling all subsequent WhatsApp messages — including text and media — to be forwarded in real time over a persistent connection.

---

## 2. Problem Statement

OpenClaw supports WhatsApp as a channel today via the Baileys library, which relies on WhatsApp Web's unofficial reverse-engineered protocol. This approach carries significant risks: it violates WhatsApp's Terms of Service, is subject to sudden breakage when WhatsApp updates its clients, and can result in the user's personal number being banned. There is currently no supported path for OpenClaw users to connect to WhatsApp using the **Official WhatsApp Business API (Cloud API)** without undergoing a complex, multi-day Meta Business verification process.

The gap this product fills is a **managed, shared-number model** where a single verified WhatsApp Business Account (WABA) is operated by imBee, and individual OpenClaw users are onboarded to it in seconds via a pairing code flow.

---

## 3. Goals and Non-Goals

### 3.1 Goals

- Deliver a native OpenClaw channel plugin that appears as a first-class option in the setup wizard alongside existing channels.
- Complete the full pairing flow in under 2 minutes for a new user.
- Route incoming WhatsApp messages (text and media) from the shared number to the correct local OpenClaw instance with sub-500ms forwarding latency.
- Support image, video, audio, sticker, and document message types; embed images as base64 data URIs so vision-capable models can read them directly.
- Ensure all communication between the local machine and the routing server is encrypted and authenticated.
- Support both Meta Cloud API and 360dialog as WhatsApp provider backends, selectable via server configuration.
- Adhere to OpenClaw's plugin capability model and SDK contracts so the plugin is compatible with future OpenClaw releases.

### 3.2 Non-Goals

- This product does **not** provide each user with their own dedicated WhatsApp Business number. The shared-number model is intentional for the initial release.
- This product does **not** replace or modify OpenClaw's existing Baileys-based WhatsApp channel. The two will coexist as separate channel options.
- This product does **not** include a graphical user interface beyond the terminal-rendered QR code and the OpenClaw web UI's standard channel management screens.

---

## 4. User Personas

| Persona | Description | Primary Need |
| :--- | :--- | :--- |
| **OpenClaw Power User** | A developer or technical professional running OpenClaw locally on macOS or Linux. | Connect their agent to WhatsApp via an official, reliable API without managing Meta credentials. |
| **Small Business Owner** | A non-technical user running OpenClaw to automate customer communications. | A simple, guided setup that works without understanding API tokens or webhooks. |
| **imBee Platform Admin** | The team at imBee managing the shared WABA number and routing infrastructure. | Monitor connection health, manage the shared number's message throughput, and ensure routing reliability. |

---

## 5. User Flow

### 5.1 Setup Wizard — Pairing Flow

The following describes the complete end-to-end pairing experience from the user's perspective.

**Step 1 — Channel Selection.** The user runs `openclaw channels add` or proceeds through `openclaw onboard`. In the channel selection menu, they choose **"Official WhatsApp API (via imBee)"** from the list of available channels.

**Step 2 — Code Request.** The plugin immediately makes an authenticated HTTPS request to the imBee routing server to request a unique pairing code. The server generates a short-lived, cryptographically random code (e.g., `CLAW-A3F9-Z7`) and returns it along with an expiration timestamp and the instance's API key.

**Step 3 — QR Code Display.** The terminal renders a QR code and displays the following information:

```
┌─────────────────────────────────────────────────────────┐
│  Connect your WhatsApp to OpenClaw                      │
│                                                         │
│  1. Scan the QR code below with your phone, OR          │
│  2. Open this link on your phone:                       │
│     https://wa.me/+18885550100?text=CLAW-A3F9-Z7        │
│                                                         │
│  [QR CODE RENDERED HERE]                                │
│                                                         │
│  This code expires in 10 minutes.                       │
└─────────────────────────────────────────────────────────┘
```

The URL format is `https://wa.me/<shared_number>?text=<code>`, where `<shared_number>` is imBee's registered WhatsApp Business number and `<code>` is the unique pairing code.

**Step 4 — User Action.** The user scans the QR code or taps the link on their mobile device. WhatsApp opens with a pre-filled message containing the pairing code. The user taps **Send**.

**Step 5 — Server-Side Mapping.** The routing server receives the incoming webhook. It parses the message body, validates the code, and creates a persistent record mapping the sender's phone number to the local OpenClaw instance. The server sends a confirmation message back to the user via WhatsApp: *"✅ Pairing complete! Your OpenClaw AI agent is now connected to this WhatsApp number."*

**Step 6 — Wizard Completion.** The routing server notifies the local plugin over the persistent WebSocket connection that pairing is complete. The setup wizard displays a success message and exits. The channel is now active.

### 5.2 Runtime — Message Forwarding Flow

Once pairing is complete, all subsequent interactions follow this pattern:

1. The user sends a WhatsApp message (text or media) to the shared number.
2. Meta delivers the event to the routing server via webhook.
3. The server looks up the sender's phone number in the mapping database and identifies the target local instance.
4. The server forwards the full event payload (including media ID and direct download URL) to the local plugin over the persistent WebSocket connection. For text messages it also marks the message as read and sends a typing indicator to the sender.
5. The OpenClaw plugin passes the message to the agent runtime for processing. For media messages, the plugin downloads the content from the routing server and either embeds it as a base64 data URI (images) or describes it in text (other types).
6. The agent generates a response, which the plugin sends back to the routing server via a REST API call.
7. The routing server calls the WhatsApp Cloud API (via Meta or 360dialog) to deliver the response to the user's phone.

---

## 6. System Architecture

The architecture diagram below illustrates the complete three-phase interaction between all system components.

![System Architecture Diagram](https://private-us-east-1.manuscdn.com/sessionFile/c2ADEXGAPCtGjEcsiADpwq/sandbox/z0jxuS7WKDDhosuFlBLxP2-images_1776352263749_na1fn_L2hvbWUvdWJ1bnR1L2FyY2hpdGVjdHVyZV9kaWFncmFt.png?Policy=eyJTdGF0ZW1lbnQiOlt7IlJlc291cmNlIjoiaHR0cHM6Ly9wcml2YXRlLXVzLWVhc3QtMS5tYW51c2Nkbi5jb20vc2Vzc2lvbkZpbGUvYzJBREVYR0FQQ3RHakVjc2lBRHB3cS9zYW5kYm94L3owanh1UzdXS0REaG9zdUZsQkx4UDItaW1hZ2VzXzE3NzYzNTIyNjM3NDlfbmExZm5fTDJodmJXVXZkV0oxYm5SMUwyRnlZMmhwZEdWamRIVnlaVjlrYVdGbmNtRnQucG5nIiwiQ29uZGl0aW9uIjp7IkRhdGVMZXNzVGhhbiI6eyJBV1M6RXBvY2hUaW1lIjoxNzk4NzYxNjAwfX19XX0_&Key-Pair-Id=K2HSFNDJXOU9YS&Signature=chF6iWiDr3HNZaNvfX37WA~l0-4rSesCPG0mccCUKuUhwdZ4WeoU7~euDELCeefTsd0~XJIVreFhjshV1mQvKYRmvQAVI6tG~lXl1GDnMGA2s-K9pM7gXL0YgRo2fAGRNROQaTSsJJaQjizg8zgm3UrJYKH2qojniBpWBmLZ-7jbMTHYqAguMHNit4g517FqH8MB5CCJNvFCJrsntzX2ucwqfvPbKbc6~LVUwwGKjkm3NrEQ-waxzXHNfs8U-8a-IRBzjVPLjAszEm~EgCEJ~8Tec3OvwmL34IU3Zf~dSv3Tf~BsyspQpe~HWymHjdosEm8FgH4zrLKBSFbeM4ondQ__)

The system is composed of three logical tiers.

### 6.1 Local OpenClaw Plugin (id: `whatsapp-official`, npm: `openclaw-channel-whatsapp-official`)

This is the component that runs on the user's machine as part of their OpenClaw installation. It is a **native channel plugin** built against the OpenClaw SDK, implementing the `ChannelMessageActionAdapter` interface.

The plugin registers the following capabilities with the OpenClaw core registry upon load:

| Capability | Registration Method | Description |
| :--- | :--- | :--- |
| Channel (Inbound) | `api.registerChannel(...)` | Receives forwarded WhatsApp events from the routing server |
| Channel (Outbound) | `ChannelMessageActionAdapter` | Sends agent responses back through the routing server |
| Setup Hook | `api.registerSetupWizardStep(...)` | Injects the pairing flow into the `openclaw channels add` wizard |

The plugin maintains a **persistent WebSocket connection** (WSS) to the routing server. This connection is used exclusively for receiving forwarded message events. Outbound messages are sent via standard HTTPS REST calls. The plugin implements exponential backoff with jitter for reconnection, with a maximum retry interval of 60 seconds. A 30-second keepalive ping is sent on the WebSocket to prevent idle connection drops from proxies and tunnels (e.g., ngrok).

### 6.2 Centralized Routing Server (imBee Backend)

This is the server-side component operated by imBee. It is the only component that holds the WhatsApp Business API credentials and communicates directly with Meta (or 360dialog).

The routing server exposes the following API surface:

| Endpoint | Method | Auth | Purpose |
| :--- | :--- | :--- | :--- |
| `/healthz` | `GET` | None | Health check |
| `/api/v1/pair/request` | `POST` | None | Generates and returns a new pairing code and API key |
| `/api/v1/pair/status` | `GET` | Bearer | Polls the current pairing status for a given API key |
| `/api/v1/send` | `POST` | Bearer | Sends outbound text or media via the WhatsApp provider |
| `/api/v1/typing` | `POST` | Bearer | Marks a message as read and shows a typing indicator |
| `/api/v1/media/{mediaId}` | `GET` | Bearer | Downloads inbound media and proxies the binary to the plugin |
| `/webhooks/whatsapp` | `GET` | None | Responds to Meta's one-time webhook registration challenge |
| `/webhooks/whatsapp` | `POST` | HMAC/Header | Receives incoming webhook events from Meta or 360dialog |
| `/ws` | `WSS` | Bearer | Persistent WebSocket for forwarding inbound events to local instances |

### 6.3 WhatsApp Provider Backends

The routing server supports two interchangeable WhatsApp Cloud API provider backends, selected via the `WA_PROVIDER` environment variable:

| Provider | `WA_PROVIDER` value | Authentication | Notes |
| :--- | :--- | :--- | :--- |
| **Meta Cloud API** | `meta` | `Authorization: Bearer {WABA_TOKEN}` | Direct integration with `graph.facebook.com/v19.0` |
| **360dialog** | `360dialog` | `D360-API-KEY: {key}` | Routes through `waba-v2.360dialog.io`; webhook validated via `Ocp-Apim-Subscription-Key` header |
| **Stub / Dev** | `` (empty) | None | No network calls; returns synthetic wamid values |

Both provider backends implement the same `Provider` interface (`SendText`, `SendMedia`, `DownloadMedia`, `SendTypingIndicator`, `ValidateWebhook`), making them interchangeable without changes to routing or plugin logic.

### 6.4 Meta WhatsApp Cloud API

imBee operates a verified WhatsApp Business Account (WABA) with a single shared phone number. The routing server is registered as the webhook receiver for all events on this number. Meta delivers all inbound messages, delivery receipts, and status updates to the routing server's `/webhooks/whatsapp` endpoint.

---

## 7. Technical Specifications

### 7.1 Plugin Manifest (`openclaw.plugin.json`)

The plugin manifest `id`, the top-level export `id` in `index.ts`, and the `ChannelPlugin.id` in `channel.ts` must all be `"whatsapp-official"`. The npm package name (`openclaw-channel-whatsapp-official`) is separate and does not need to match.

```json
{
  "id": "whatsapp-official",
  "name": "Official WhatsApp API (imBee)",
  "version": "0.1.11",
  "description": "Connect your OpenClaw agent to WhatsApp via the Official WhatsApp Business API, powered by imBee.",
  "author": "imBee Limited",
  "capabilities": ["channel"],
  "channelId": "whatsapp-official",
  "setupWizard": true,
  "entryPoint": "./index.ts",
  "configSchema": "./schema/config.json"
}
```

### 7.2 Pairing Code Specification

| Property | Value |
| :--- | :--- |
| Format | `CLAW-[A-Z0-9]{4}-[A-Z0-9]{4}` (e.g., `CLAW-A3F9-Z7KL`) |
| Entropy | 36^8 ≈ 2.8 trillion combinations |
| Expiration | 10 minutes from issuance |
| Single-use | Yes — invalidated immediately upon successful mapping |
| Storage | Server-side only; never persisted on the local machine |

### 7.3 Routing Server Data Model

**Table: `device_mappings`**

| Column | Type | Constraints | Description |
| :--- | :--- | :--- | :--- |
| `id` | `TEXT` | `PRIMARY KEY` | Internal record identifier (UUID) |
| `phone_number` | `VARCHAR(20)` | `NOT NULL DEFAULT ''` | User's WhatsApp number in E.164 format |
| `wab_number` | `TEXT` | `NOT NULL DEFAULT ''` | The shared WABA number that received the pairing (from `SHARED_WA_NUMBER`) |
| `instance_id` | `VARCHAR(64)` | `NOT NULL` | Unique identifier of the local OpenClaw instance |
| `api_key` | `TEXT` | `UNIQUE, NOT NULL` | Per-instance Bearer token for API and WebSocket auth |
| `pairing_code` | `VARCHAR(16)` | `NOT NULL DEFAULT ''` | The code used during pairing; cleared after activation |
| `status` | `VARCHAR(16)` | `NOT NULL` | One of: `PENDING`, `ACTIVE`, `DISCONNECTED` |
| `expires_at` | `TIMESTAMP` | `NOT NULL` | Pairing code expiry (irrelevant once ACTIVE) |
| `created_at` | `TIMESTAMP` | `NOT NULL` | Record creation timestamp |
| `updated_at` | `TIMESTAMP` | `NOT NULL` | Last status change timestamp |

**Indexes:**

| Index | Type | Condition | Purpose |
| :--- | :--- | :--- | :--- |
| `device_mappings_pkey` | `UNIQUE` | — | Primary key on `id` |
| `device_mappings_api_key_key` | `UNIQUE` | — | Fast lookup by API key |
| `idx_device_mappings_pairing_code` | `UNIQUE` | `WHERE pairing_code <> ''` | Prevents duplicate codes; partial to allow multiple `PENDING` rows with cleared codes |
| `idx_device_mappings_phone_number` | Index | — | Fast lookup by inbound sender phone |
| `uq_device_wab_phone` | `UNIQUE` | `WHERE phone_number <> ''` | Enforces one active mapping per `(wab_number, phone_number)` pair; prevents duplicate active sessions for the same user on the same WABA number |

When a new pairing is activated for a `(wab_number, phone_number)` pair that already has an `ACTIVE` record, the existing record is set to `DISCONNECTED` before the new one is activated.

### 7.4 Webhook Registration

Before Meta delivers any events, the routing server must be registered as the webhook receiver in the Meta App Dashboard. Meta sends a one-time `GET /webhooks/whatsapp` request with the following query parameters:

| Parameter | Description |
| :--- | :--- |
| `hub.mode` | Always `subscribe` |
| `hub.verify_token` | The token set in the Meta dashboard; must match `WEBHOOK_VERIFY_TOKEN` on the server |
| `hub.challenge` | A random string that the server must echo back as the response body with `200 OK` |

The server rejects the request with `403` if `hub.mode` is not `subscribe` or if the verify token does not match.

For 360dialog, webhook authenticity is validated via the `Ocp-Apim-Subscription-Key` request header, which 360dialog sets to the client's API key on every inbound webhook.

### 7.5 Webhook Event Routing Logic

Upon receiving a `POST /webhooks/whatsapp` event, the routing server executes the following decision tree:

1. **Validate authenticity.** For Meta: verify `X-Hub-Signature-256` HMAC-SHA256 against the app secret. For 360dialog: verify `Ocp-Apim-Subscription-Key` matches `D360_API_KEY`. Reject with `403` if invalid.
2. **Acknowledge immediately.** Return `200 OK` to Meta/360dialog within 5 seconds.
3. **Parse payload.** Extract message fields from `entry[].changes[].value.messages[]`. Supported message types:

| `type` | Fields extracted | Forwarded in WS payload |
| :--- | :--- | :--- |
| `text` | `text.body` | `text` |
| `image` | `image.id`, `image.url`, `image.mime_type`, `image.caption` | `mediaId`, `mediaUrl`, `mediaType`, `mimeType`, `caption` |
| `video` | `video.id`, `video.url`, `video.mime_type`, `video.caption` | `mediaId`, `mediaUrl`, `mediaType`, `mimeType`, `caption` |
| `audio` / `voice` | `audio.id`, `audio.url`, `audio.mime_type` | `mediaId`, `mediaUrl`, `mediaType`, `mimeType` |
| `sticker` | `sticker.id`, `sticker.url`, `sticker.mime_type` | `mediaId`, `mediaUrl`, `mediaType`, `mimeType` |
| `document` | `document.id`, `document.url`, `document.mime_type`, `document.caption`, `document.filename` | `mediaId`, `mediaUrl`, `mediaType`, `mimeType`, `caption`, `fileName` |

4. **Check for pairing code.** If the message body matches `^CLAW-[A-Z0-9]{4}-[A-Z0-9]{4}$`:
   - Look up the code in `device_mappings`.
   - If found and not expired, set `phone_number = from`, `wab_number = SHARED_WA_NUMBER`, `status = ACTIVE`, clear `pairing_code`. Evict any existing `ACTIVE` record for the same `(wab_number, phone_number)` pair.
   - Populate the in-process record cache.
   - Notify the local instance via WebSocket with `PAIRING_COMPLETE`.
5. **Route regular message.** Look up `phone_number = from` in cache (fall back to DB on miss). If `ACTIVE`, forward via WebSocket as `INBOUND_MESSAGE`.

### 7.6 Typing Indicator

When an inbound text or media message is received and successfully routed to the local plugin, the plugin fires a best-effort `POST /api/v1/typing` request to the routing server with the `messageId`. The routing server calls the WhatsApp provider to:

1. Mark the message as **read** (blue ticks).
2. Show a **typing indicator** to the sender.

This is fire-and-forget; errors are silently swallowed and do not affect message delivery.

For 360dialog, the typing indicator is sent as a single `/messages` call:
```json
{
  "messaging_product": "whatsapp",
  "status": "read",
  "message_id": "<wamid>",
  "typing_indicator": { "type": "text" }
}
```

### 7.7 Media Download Flow

Inbound media messages are downloaded **internally** by the plugin (not surfaced as clickable URLs) so that vision-capable AI models can read images directly.

**Flow:**
1. Webhook payload includes `mediaId` (provider ID) and `mediaUrl` (Facebook CDN pre-signed URL). Both are forwarded to the plugin via the WebSocket envelope.
2. Plugin calls `GET /api/v1/media/{mediaId}?url=<encoded_mediaUrl>` with its Bearer token.
3. Backend passes the `directURL` to the provider's `DownloadMedia(mediaID, directURL)` method.
4. If `directURL` is non-empty, the backend skips the step-1 metadata lookup and goes directly to the binary download. If empty, it performs step-1 first.
5. **For 360dialog:** The Facebook CDN host (`lookaside.fbsbx.com`) in the direct URL is rewritten to `waba-v2.360dialog.io` before download, and `D360-API-KEY` is sent. This is required because 360dialog proxies all media through their own infrastructure.
6. **For Meta:** The direct URL is used as-is with `Authorization: Bearer {WABA_TOKEN}`.
7. The raw binary and MIME type are returned to the plugin via the HTTP response body and `Content-Type` header.

**Plugin-side media handling:**

| MIME type | Agent sees |
| :--- | :--- |
| `image/*` | Inline base64 data URI (`data:image/jpeg;base64,...`) — readable by vision models |
| All other types | `[type] · filename · mime_type · N KB` — descriptive text |
| Download failure | `[type received: caption]` — fallback notification |

### 7.8 Outbound Message Delivery

When the routing server receives `POST /api/v1/send`, it forwards the message to the configured WhatsApp provider.

**Text message request body:**
```json
{
  "toPhoneNumber": "+85296663768",
  "text": "Hello from OpenClaw"
}
```

**Media message request body:**
```json
{
  "toPhoneNumber": "+85296663768",
  "mediaUrl": "https://...",
  "mediaType": "image",
  "caption": "optional caption",
  "fileName": "optional filename"
}
```

`mediaType` must be one of: `image`, `video`, `audio`, `document`. If `mediaType` is omitted when `mediaUrl` is present, it defaults to `document`.

The server returns:
```json
{ "status": "accepted", "messageId": "<wamid>" }
```

### 7.9 Environment Variables (Routing Server)

| Variable | Default | Purpose |
| :--- | :--- | :--- |
| `HTTP_ADDR` | `:8080` | Server listen address |
| `STORE_DRIVER` | `memory` | `memory` or `postgres` |
| `POSTGRES_DSN` | local dev DSN | Postgres connection string |
| `SHARED_WA_NUMBER` | `+18885550100` | The WABA number shown in `wa.me` pairing URLs; stored as `wab_number` on each record |
| `WEBHOOK_APP_SECRET` | `dev-secret` | HMAC key for Meta `X-Hub-Signature-256` validation |
| `WEBHOOK_VERIFY_TOKEN` | `` | Meta webhook registration challenge token |
| `PAIRING_CODE_TTL_SECONDS` | `600` | Pairing code expiry (10 minutes) |
| `PAIR_RATE_LIMIT_PER_HOUR` | `5` | Max pairing requests per instance per hour |
| `WA_PROVIDER` | `` (stub) | `meta` or `360dialog` |
| `WABA_TOKEN` | — | Meta System User token (required when `WA_PROVIDER=meta`) |
| `WABA_PHONE_NUMBER_ID` | — | Meta phone number ID (required when `WA_PROVIDER=meta`) |
| `D360_API_KEY` | — | 360dialog client API key (required when `WA_PROVIDER=360dialog`) |
| `D360_BASE_URL` | `https://waba-v2.360dialog.io` | 360dialog API base URL; use `https://waba-sandbox.360dialog.io` for sandbox |

### 7.10 Production Deployment

The routing server is deployed on AWS EC2 (t4g.nano, ARM64, Amazon Linux 2023) behind **Caddy** for automatic TLS certificate provisioning via Let's Encrypt. The production endpoint is `https://openclaw-plugin.dev.ent.imbee.io`.

The server binary is built with `GOARCH=arm64 GOOS=linux` and deployed via `scripts/deploy-aws.sh`. Caddy is configured as a reverse proxy:

```
openclaw-plugin.dev.ent.imbee.io {
    reverse_proxy localhost:8080
}
```

The systemd service (`wa-server.service`) is managed with `systemctl`. The store driver is set to `memory` for the initial deployment; the store file (`/home/ec2-user/data/store.json`) persists pairings across restarts but is not replicated across instances.

**Deployment checklist:**
- Set `WEBHOOK_APP_SECRET` to a securely-generated random value (≥32 bytes, hex-encoded).
- Set `WA_PROVIDER=360dialog` and configure `D360_API_KEY`.
- Register the webhook URL with the provider, including the `Ocp-Apim-Subscription-Key: $D360_API_KEY` header in the registration payload (required for 360dialog to authenticate inbound webhook deliveries).

### 7.11 In-Process Record Cache

The routing server maintains an in-process write-through cache of `device_mappings` records, keyed by both `phone_number` and `api_key`. This eliminates database round-trips on the hot paths (inbound webhook routing and outbound send authentication) after the first access.

| Operation | Cache behaviour |
| :--- | :--- |
| Inbound message (`FindByPhone`) | Read cache first; on miss, query DB and populate cache |
| Send / WS / Media / Typing auth (`FindByAPIKey`) | Read cache first; on miss, query DB and populate cache |
| Pairing activation (`ActivatePairing`) | Always writes to DB; immediately populates both cache indexes on success |

The cache has no TTL. Because paired records are effectively permanent in v1, expiry is not required. If unpairing is added in a future release, the cache must expose a `delete` operation called at the point of record removal.

The cache is in-process only. Deployments running multiple backend replicas must use an external shared cache (e.g. Redis) instead.

### 7.12 WebSocket Message Protocol

All messages over the WebSocket connection use JSON with the following envelope:

```json
{
  "type": "INBOUND_MESSAGE | PAIRING_COMPLETE | HEARTBEAT | ERROR",
  "payload": { ... },
  "timestamp": "2026-04-15T10:00:00.000Z",
  "message_id": "wamid.XXXX"
}
```

For `INBOUND_MESSAGE`, the `payload` object contains:

| Field | Type | Present when |
| :--- | :--- | :--- |
| `from` | string | Always |
| `messageId` | string | Always |
| `text` | string | Text messages |
| `mediaId` | string | Media messages |
| `mediaUrl` | string | Media messages (Facebook CDN pre-signed URL from webhook) |
| `mediaType` | string | Media messages (`image`, `video`, `audio`, `sticker`, `document`) |
| `mimeType` | string | Media messages |
| `caption` | string | Image, video, document (when provided) |
| `fileName` | string | Document messages |

The WebSocket connection uses 30-second client-side pings to maintain liveness through proxies and load balancers. The server responds to `PING` frames with `PONG`.

---

## 8. Security Requirements

### 8.1 Transport Security

All communication between the local plugin and the routing server must use TLS 1.2 or higher. The WebSocket connection must use the `wss://` scheme. The plugin must validate the server's TLS certificate and reject self-signed certificates in production builds.

### 8.2 Instance Authentication

Local instances must authenticate with the routing server using a **per-instance API key** generated during the initial pairing request. This key is stored securely in OpenClaw's credential store (via the `SecretRef` surface) and included as a `Bearer` token in all subsequent API calls (`/api/v1/send`, `/api/v1/typing`, `/api/v1/media/{mediaId}`) and as a WebSocket handshake header.

### 8.3 Webhook Integrity

- **Meta:** The routing server verifies every incoming webhook using the `X-Hub-Signature-256` HMAC-SHA256 header.
- **360dialog:** The routing server verifies the `Ocp-Apim-Subscription-Key` header against the configured `D360_API_KEY`.

Any request failing these checks is rejected with `403 Forbidden`.

### 8.4 Data Minimization

The routing server must not persistently store the content of forwarded messages. Message payloads are processed in memory and forwarded immediately. Only routing metadata (`phone_number`, `wab_number`, `instance_id`, `status`) is persisted.

### 8.5 Pairing Code Security

Pairing codes must be generated using a cryptographically secure random number generator (CSPRNG). Codes expire after 10 minutes and are invalidated immediately upon use. The server enforces rate limiting on `/api/v1/pair/request` (default: 5 requests per instance per hour).

---

## 9. Error Handling and Edge Cases

| Scenario | Expected Behavior |
| :--- | :--- |
| Pairing code expires before use | The wizard displays an error and offers to generate a new code. |
| User sends wrong code | The server responds with "Invalid code. Please try again." No mapping is created. |
| Local instance disconnects mid-session | The server marks the instance as `DISCONNECTED` and queues incoming messages for up to 24 hours. |
| Queued messages exceed 24 hours | Messages are discarded. The user receives a notification that their agent was offline and messages were missed. |
| Meta webhook delivery fails | The routing server returns `200 OK` immediately (per Meta requirements). Internal processing errors are logged and retried asynchronously. |
| Duplicate webhook delivery from Meta | The server deduplicates using the `message_id` field. Duplicate events are acknowledged but not forwarded. |
| User re-pairs the same phone number | The existing `ACTIVE` record for `(wab_number, phone_number)` is set to `DISCONNECTED` and a new record is created, allowing clean re-pairing without orphaned records. |
| User attempts to pair a second phone number | The server creates a second `device_mappings` record for the same `instance_id`, allowing one agent to serve multiple users. |
| WebSocket reconnect race condition | Hub connection removal is identity-aware: a goroutine's cleanup only removes its own connection, never a connection registered by a concurrent reconnect. |
| Media download fails | The plugin notifies the agent with `[type received: caption]` so the agent is aware a media message arrived even if the binary could not be fetched. |
| Unsupported WhatsApp message type | The routing server silently drops the event. Only `text`, `image`, `video`, `audio`, `voice`, `sticker`, and `document` types are forwarded. |

---

## 10. Plugin Installation and Distribution

The plugin will be distributed via the **ClawHub** registry and **npm** (OpenClaw checks ClawHub first, then npm). Install with either:

```bash
openclaw plugins install openclaw-channel-whatsapp-official
openclaw plugins install clawhub:openclaw-channel-whatsapp-official
```

Upon installation, the plugin is automatically discovered by OpenClaw's plugin loader via its `openclaw.plugin.json` manifest. No manual configuration is required before running the setup wizard.

---

## 11. Acceptance Criteria

| # | Criterion | Verification Method |
| :--- | :--- | :--- |
| AC-01 | The plugin appears as a channel option in `openclaw channels add` after installation. | Manual test |
| AC-02 | A pairing code is displayed in the terminal within 3 seconds of selecting the channel. | Automated test |
| AC-03 | The `wa.me` URL correctly encodes the shared number and pairing code. | Unit test |
| AC-04 | Sending the pairing code from WhatsApp results in a `PAIRING_COMPLETE` notification to the local instance within 5 seconds. | Integration test |
| AC-05 | A WhatsApp text message from a paired user is forwarded to the local OpenClaw instance within 500ms of webhook receipt. | Load test |
| AC-06 | An expired pairing code is rejected by the server with an appropriate error message. | Unit test |
| AC-07 | A webhook with an invalid signature/auth header is rejected with `403`. | Security test |
| AC-08 | The plugin reconnects to the routing server within 60 seconds of a connection drop. | Resilience test |
| AC-09 | Message content is not present in any server-side database table after forwarding. | Security audit |
| AC-10 | The plugin passes `openclaw doctor` with no errors after successful pairing. | Manual test |
| AC-11 | Sending an image from WhatsApp results in the agent receiving a base64 data URI. | Integration test |
| AC-12 | Sending a document from WhatsApp results in the agent receiving a descriptive text summary (`[document] · filename · mime · N KB`). | Integration test |
| AC-13 | The agent sees a typing indicator in WhatsApp within 1 second of sending a message. | Manual test |
| AC-14 | Re-pairing the same phone number deactivates the previous record without leaving orphaned `ACTIVE` entries. | Integration test |
| AC-15 | Switching `WA_PROVIDER` between `meta` and `360dialog` requires no plugin changes. | Configuration test |

---

## 12. Open Questions

1. **Rate Limiting:** What is the expected message throughput per paired user? This will determine the routing server's infrastructure sizing and whether per-user rate limits need to be enforced at the server level.
2. **Multi-Agent Support:** Should a single WhatsApp phone number be mappable to different OpenClaw agents depending on the time of day or message content? This would require routing logic beyond simple phone-to-instance mapping.
3. **Opt-Out / Unpairing:** What is the user experience for unpairing a phone number? Should the user be able to send a specific command (e.g., `UNPAIR`) from WhatsApp to trigger this?
4. **Compliance:** What data retention policies apply to the `device_mappings` table, particularly for users in GDPR-regulated jurisdictions?
5. **Media Upload (Outbound):** The current `/api/v1/send` accepts a `mediaUrl` for outbound media. A future enhancement could accept raw binary uploads to support sending locally-generated files.

---

## 13. Future Enhancements

The following features are explicitly out of scope for v1.1 but are planned for subsequent releases.

**Dedicated Number Option (v2.0).** Provide an upgrade path for users who require their own dedicated WhatsApp Business number, integrating with imBee's existing BSP (Business Solution Provider) infrastructure to provision and manage per-user WABAs.

**Admin Dashboard (v1.2).** A web-based dashboard for imBee platform administrators to monitor active connections, view routing error rates, inspect message queues, and manage the shared number's Meta Business Manager settings.

**Multi-User per Instance (v1.2).** Allow a single local OpenClaw instance to serve multiple paired WhatsApp users, enabling small teams to share a single agent deployment.

**Message Queueing for Offline Instances (v1.2).** Persist inbound messages when the plugin is offline and deliver them on reconnection (currently messages to disconnected instances are dropped).

---

## References

[1] OpenClaw Plugin Architecture Documentation. https://docs.openclaw.ai/plugins/architecture

[2] OpenClaw Channels CLI Documentation. https://docs.openclaw.ai/cli/channels

[3] OpenClaw GitHub Repository. https://github.com/openclaw/openclaw

[4] Meta WhatsApp Cloud API Webhooks Overview. https://developers.facebook.com/documentation/business-messaging/whatsapp/webhooks/overview/

[5] Meta WhatsApp Business Phone Numbers. https://developers.facebook.com/documentation/business-messaging/whatsapp/business-phone-numbers/phone-numbers

[6] 360dialog WhatsApp Cloud API Documentation. https://docs.360dialog.com/docs/messaging/media/upload-retrieve-or-delete-media

[7] Guide to WhatsApp Webhooks: Features and Best Practices. https://hookdeck.com/webhooks/platforms/guide-to-whatsapp-webhooks-features-and-best-practices
