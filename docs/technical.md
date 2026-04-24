# Technical Implementation Plan: Official WhatsApp Plugin for OpenClaw

## 1. Scope recap

You are building **two deliverables**:

1. **Local plugin** (`openclaw-channel-whatsapp-official` on npm / ClawHub): OpenClaw-native channel + setup wizard, WSS to imBee, REST for outbound.
2. **Central routing service** (imBee): Meta credentials, webhooks, pairing, mapping DB, WebSocket fan-out, optional 24h queue, rate limits.

Meta **Cloud API** and **WhatsApp conversation pricing** are the same whether traffic hits AWS or Azure; only **imBee's hosting** differs.

---

## 2. High-level architecture (implementation view)

```mermaid
flowchart LR
  subgraph user_machine [User machine]
    OC[OpenClaw + plugin]
  end
  subgraph imbee [imBee cloud]
    API[HTTPS API]
    WS[WSS /ws]
    WH[/webhooks/whatsapp]
    SVC[Routing service]
    DB[(PostgreSQL)]
    Q[(Ephemeral queue / cache)]
  end
  subgraph meta [Meta]
    WA[WhatsApp Cloud API]
  end
  OC -->|pair/request, send, Bearer| API
  OC <-->|events, PAIRING_COMPLETE| WS
  WA -->|webhooks| WH
  WH --> SVC
  SVC --> DB
  SVC --> Q
  SVC --> WA
```

---

## 3. Technical plan by component

### 3.1 OpenClaw plugin (local)

| Area | Plan |
|------|------|
| **Package** | npm package + `openclaw.plugin.json` per PRD §7.1; `configSchema` for base URL, optional env overrides. |
| **Lifecycle** | On load: `registerChannel`, `ChannelMessageActionAdapter`, `registerSetupWizardStep` per §6.1. |
| **Setup wizard** | Step flow: `POST /api/v1/pair/request` → render QR + `wa.me` URL (§5.2–5.3) → open WSS with returned instance key → wait for `PAIRING_COMPLETE` (optional `GET /api/v1/pair/status` for UX/polling fallback). |
| **Secrets** | Store per-instance API key via OpenClaw `SecretRef`; send `Authorization: Bearer` on REST and WSS subprotocol or header (§8.2). |
| **WebSocket client** | JSON envelope §7.5; handle `INBOUND_MESSAGE`, `PAIRING_COMPLETE`, `HEARTBEAT`, `ERROR`; exponential backoff + jitter, cap 60s (§6.1). |
| **Outbound** | Agent reply → `POST /api/v1/send` with same Bearer; map OpenClaw message model ↔ WhatsApp payload fields expected by your server contract. |
| **TLS** | TLS 1.2+, pin to public CAs in production; no self-signed (§8.1). |
| **Tests** | Unit: code format, `wa.me` URL (AC-03); integration: mock server for pairing + message round-trip (AC-04, AC-06). |

### 3.2 Routing server (imBee backend)

| Area | Plan |
|------|------|
| **HTTP surface** | Implement §6.2 table: `pair/request`, `pair/status`, `send`, `webhooks/whatsapp`. |
| **Webhook handler** | Strict order: verify `X-Hub-Signature-256` → **200 quickly** → async parse/route (§7.4, §9 Meta retries). |
| **Pairing** | CSPRNG codes `CLAW-[A-Z0-9]{4}-[A-Z0-9]{4}`, 10m TTL, single-use; rate limit pair requests (e.g. 5/instance/hour) §8.5. |
| **Routing** | Regex branch for pairing vs normal message; multi-row per `instance_id` for multiple phones §9. |
| **WebSocket** | Authenticate at connect; maintain `ws_connection_id` ↔ `instance_id`; push envelopes §7.5. |
| **Queue** | If no active WS: enqueue encrypted payload, TTL ≤ 24h, notify user on discard §8.4, §9. |
| **Persistence** | `device_mappings` as in §7.3; **do not** persist message bodies in SQL after forward (AC-09); queue store is ephemeral/encrypted only. |
| **Observability** | Structured logs, metrics (webhook latency, WS connected count, queue depth), tracing on critical paths. |
| **Ops** | Secrets for Meta app secret, WABA tokens, DB creds in cloud secret manager; rotate keys. |

### 3.3 Meta integration

- Register app webhook URL → imBee `POST /webhooks/whatsapp`.
- Outbound: Cloud API sends using imBee's phone number ID / tokens (server-only).
- Document **conversation window** and Meta billing in runbooks (not cloud-specific).

---

## 4. Phased delivery

| Phase | Outcome |
|-------|---------|
| **P0 – Contracts** | OpenAPI (or shared TS types) for REST + WSS message shapes; error codes for wizard. |
| **P1 – Routing MVP** | Webhook verify + parse + pairing path + DB + minimal `send` + WSS notify pairing complete. |
| **P2 – Plugin MVP** | Wizard + WSS + channel adapter calling mock then real server. |
| **P3 – Runtime** | Full inbound forward, outbound `send`, reconnect policy, dedupe. |
| **P4 – Resilience** | 24h queue, DISCONNECTED state, user-visible "missed messages" path §9. |
| **P5 – Hardening** | Rate limits, security review (AC-07, AC-09), load test AC-05. |

---

## 5. Suggested implementation stacks (both clouds)

**Application runtime:** Node.js or Go (strong WS + JSON ecosystem); single service binary or small modular services if you split webhook vs WS later.

**Database:** PostgreSQL (RDS / Azure Database for PostgreSQL Flexible Server) — fits UUID, enums, uniqueness on `phone_number` / `pairing_code`.

**Queue / ephemeral store:**
- AWS: SQS (visibility + DLQ) or Redis (ElastiCache) for short TTL semantics you control.
- Azure: Service Bus (sessions/TTL) or Azure Cache for Redis.

**Comparable reference architectures**

- **AWS (hybrid):** API Gateway **WebSocket** for `/ws` + **HTTP API** or ALB for REST + webhook; **Lambda** or **ECS Fargate** for handlers; **RDS PostgreSQL**; **SQS**; **Secrets Manager**.
- **AWS (uniform containers):** **ECS Fargate** + **ALB** (HTTP + WebSocket upgrade on same app) simplifies one codebase, one scaling model.
- **Azure:** **Container Apps** (HTTP + WebSocket on same revision) + **Azure Database for PostgreSQL** + **Service Bus** or Redis + **Key Vault**.

---

## 6. Azure vs AWS cost comparison

**Important:** Dollar figures are **order-of-magnitude estimates** for planning. Use [AWS Pricing Calculator](https://calculator.aws/) and [Azure Pricing Calculator](https://azure.microsoft.com/pricing/calculator/) with your region and sustained connection counts.

### 6.1 Shared assumptions (example "early production")

| Assumption | Value |
|------------|--------|
| Concurrent WebSocket clients (paired instances) | **1,000** |
| Hours connected per client per day | **12** |
| Inbound + outbound WS **application** messages per client per day (heartbeats excluded where possible) | **600** total (similar scale to AWS's published chat example) |
| Meta webhooks + REST (`pair`, `send`) per month | **5M** requests (order of ~few per message + overhead) |
| PostgreSQL | **db.t4g.micro**-class / **B1ms**-class, single AZ, small storage |
| Ephemeral queue | modest footprint (mostly small JSON, 24h cap) |

### 6.2 AWS — WebSocket-centric split (API Gateway WebSocket + small compute)

From [Amazon API Gateway pricing](https://aws.amazon.com/api-gateway/pricing/) (WebSocket example pattern):

- **Connection minutes:** 1,000 × 12 × 60 × 30 ≈ **21.6M min/month** → about **21.6 × $0.25/million ≈ $5.40**/month for connectivity (same formula as AWS's 1000-user × 12h example).
- **Messages:** 600 × 1,000 × 30 ≈ **18M messages/month** → about **18 × $1.00/million ≈ $18**/month (first billion tier).

So **API Gateway WebSocket alone** is often **~$20–30/month** at this scale **before** Lambda/ECS, RDS, NAT, and data transfer.

Add rough monthly **indicative** extras (vary by region and tuning):

| Line item | Rough range |
|-----------|-------------|
| RDS PostgreSQL (small) | ~$15–40 |
| Lambda or light Fargate for webhook/API glue | ~$10–80 |
| SQS + Secrets Manager + CloudWatch | ~$5–25 |
| Data transfer / NAT (if Lambda in VPC) | **$0–50+** (NAT can dominate if not careful) |

**AWS total (indicative):** roughly **$50–150/month** at this toy scale; **NAT gateways and high API Gateway REST volume** can push it higher.

### 6.3 Azure — Container-centric (Container Apps + PostgreSQL)

Azure typically **does not bill WebSocket the same way as API Gateway** when it is **inbound to your container**; you pay mainly for **vCPU-GB-seconds**, requests, and **ingress** (see [Azure Container Apps pricing](https://azure.microsoft.com/pricing/details/container-apps/)).

For a **single small always-on revision** (e.g. 0.25 vCPU, 0.5 GiB, 1 min replica) handling 1k WS + webhooks:

| Line item | Rough range |
|-----------|-------------|
| Container Apps (always-on small replica) | ~$30–90 (highly region/plan dependent) |
| Azure Database for PostgreSQL Flexible Server (Burstable small) | ~$15–50 |
| Service Bus / Redis (small) | ~$10–40 |
| Key Vault + monitoring | ~$5–15 |

**Azure total (indicative):** often **~$60–180/month** for a similar early footprint, with **less discrete "per WebSocket message"** line item than AWS API Gateway WebSocket.

### 6.4 Head-to-head summary

| Dimension | AWS | Azure |
|-----------|-----|-------|
| **WebSocket pricing model** | Very explicit: **$0.25/million connection-minutes** + **$1/million messages** (WebSocket API) | Usually folded into **container consumption** + ingress; fewer discrete WS meters |
| **Likely winner at "many long-lived WS + moderate messages"** | Can be **cheap and predictable** if you use **API Gateway WebSocket** at moderate scale (per AWS examples) | Can be **competitive** if one **Container App** holds connections efficiently |
| **Risk of bill spikes** | High **REST** volume on API Gateway REST pricing; **NAT** in VPC | Mis-sized **always-on** replicas; **Premium** messaging tiers |
| **Operational fit** | Strong if you already use Lambda/API GW ecosystem | Strong if team is Microsoft-centric and wants **Container Apps** + **Flexible Server** |

**Practical recommendation for cost engineering**

- **AWS:** Prefer **one ECS Fargate service + ALB** (WebSocket + HTTP together) *or* **API Gateway WebSocket + Lambda** — model both in the calculator; the second splits cost clearly but adds integration complexity.
- **Azure:** Model **Container Apps** with your expected **min replicas** and **concurrent connections**; WebSocket load often maps to **compute**, not a separate message meter.

---

## 7. What the PRD leaves for product/engineering (tie-in to §12)

- Throughput targets → instance size and rate limits.
- Unpair UX and retention/GDPR → schema and deletion jobs.
- Multi-agent routing → out of v1 per PRD non-goals.

---

## 8. References from the PRD

- OpenClaw plugin architecture: [docs.openclaw.ai/plugins/architecture](https://docs.openclaw.ai/plugins/architecture)
- Meta webhooks: [developers.facebook.com/.../webhooks/overview](https://developers.facebook.com/documentation/business-messaging/whatsapp/webhooks/overview/)

---

## 9. Node.js vs Go Cost Sensitivity

Language choice usually impacts **compute efficiency**, not managed service pricing. For this architecture, the biggest variable is how many long-lived WebSocket connections each runtime can handle per vCPU and per GiB RAM.

### 9.1 What language does and does not change

- **Usually unchanged:** API Gateway/WebSocket metered pricing, DB tier list prices, queue pricing, storage pricing, Meta WhatsApp conversation charges.
- **Potentially improved with Go:** lower memory footprint, steadier CPU under high concurrency, smaller container/VM sizing for the same throughput.
- **Result:** language may lower the **compute component** materially, but total system savings are moderated by fixed costs (DB, secrets, observability, base networking).

### 9.2 Scenario table (order-of-magnitude planning)

| Scenario | Typical runtime pressure | Expected total monthly savings with Go vs Node.js | Notes |
|----------|---------------------------|---------------------------------------------------|-------|
| **Low concurrency MVP** (tens of concurrent users) | light CPU, low memory | **~0-10%** | Fixed costs dominate; difference may be single-digit dollars. |
| **Medium concurrency** (hundreds of concurrent users) | moderate WS fan-out + webhook handling | **~5-20%** | Compute share grows; Go often allows a smaller instance class. |
| **High concurrency** (thousands+ long-lived WS) | memory and scheduler pressure | **~15-35%** | Language/runtime efficiency matters more; validate with load tests. |

### 9.3 Practical estimate for this document's MVP ranges

- From the earlier planning ranges:
  - **AWS total:** ~$50-150/month
  - **Azure total:** ~$60-180/month
- A realistic Go-over-Node savings band is often **~$5-30/month** at this scale.
- The most reliable savings still come from:
  - right-sizing PostgreSQL tier,
  - minimizing always-on idle replicas,
  - avoiding unnecessary NAT/data-transfer paths,
  - and reducing nonessential managed services early.

### 9.4 Recommendation

If the team is equally productive in both languages, **Go is a cost-efficient default** for a WebSocket-heavy routing service. If Node.js accelerates delivery significantly, shipping with Node.js first and optimizing infra topology can still outperform a slower Go timeline economically.

---

If needed, this can be split next into:
- `implementation-roadmap.md` (engineering work breakdown)
- `cost-model.xlsx` equivalent assumptions table for CFO/ops review
