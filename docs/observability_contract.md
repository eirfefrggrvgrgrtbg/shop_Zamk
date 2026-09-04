# ZAMK Observability Architecture Contract

## 1. Categories and Storage Contract
The telemetry and data architecture is strictly divided into four distinct categories:

1. **Diagnostic Technical Log**
   - **Purpose:** Low-level runtime debugging, infrastructure monitoring, and diagnostic error triage.
   - **Destination:** **Loki** (via structured `slog` / OTLP) & **Tempo** (spans).
   - **Consistency:** Immediate, best-effort.

2. **Operational Business Event**
   - **Purpose:** Operational visibility, incident triage, operator activity audits, and distributed business flow tracking across services.
   - **Destination:** Structured `slog` to **Loki** + Semantic Span Events in **Tempo**.
   - **Consistency:** **MUST be emitted only after successful commit** of the corresponding business state transaction. Never emit on aborted or rolled-back transactions. Suppress duplicate mutation events on idempotent retries.

3. **Durable Domain / Audit Truth**
   - **Purpose:** Financial ledger, legal compliance, customer-facing history, and canonical transactional state.
   - **Destination:** **PostgreSQL** tables (e.g. `inventory_unit_history`, `audit_logs`, `order_events`, `stock_movements`, `inventory_reconciliation_resolutions`, `payment_records`).
   - **Consistency:** Transactionally atomic; committed inside the exact same DB transaction as the business state transition.

4. **Product Analytics**
   - **Purpose:** Funnel analysis, conversion metrics, user behavioral analytics, and long-term business intelligence.
   - **Destination:** **ClickHouse** / event streaming pipeline.
   - **Consistency:** Asynchronous, non-blocking, non-transactional.

**Grafana** is a visualization, query, and alerting surface only. Grafana/Loki/Tempo/Prometheus must never become the business source of truth.

### Mandatory Correlation Fields
- `service_name`
- `service_version` / `build_sha`
- `environment`
- `component`
- `operation`
- `request_id`
- `trace_id`
- `span_id`
- `actor_id`
- `actor_role`

**Entity Context Metadata (NOT indexed labels):**
`order_id`, `order_number`, `seller_id`, `product_id`, `product_variant_id`, `supply_id`, `fulfillment_id`, `shipment_id`, `return_id`, `refund_id`, `payment_id`, `inventory_unit_id`, `zmu`, `reconciliation_session_id`.

## 2. Cardinality Contract
**Loki Indexed Labels (Strictly Bounded Canonical Semantic Labels):**
- `service_name`
- `environment`
- `component`
- `level`

*Implementation-generated bounded transport labels:*
- `job` (bounded transport label assigned from service name)
- `detected_level` (bounded label optionally inferred by Loki)
*Transport labels dropped in pipeline:*
- `exporter` (explicitly dropped via `loki.process.filter_labels` stage in Alloy)

**NEVER use high-cardinality identifiers as Loki indexed labels or Prometheus metric labels:**
- `request_id`, `trace_id`, `span_id`
- `actor_id`, `user_id`, `session_id`
- `order_id`, `order_number`, `seller_id`, `product_id`, `product_variant_id`, `inventory_unit_id`, `zmu`, `payment_id`, `return_id`
Keep these as structured fields / structured metadata for logs, and avoid them in metric tags entirely.

**HTTP Metrics:**
HTTP metrics must use route templates.
- **GOOD:** `http.route="/api/admin/orders/{id}"`
- **BAD:** `http.route="/api/admin/orders/123e4567..."`

**Canonical Warehouse Operational Metrics:**
| Metric Name | Type | Allowed Low-Cardinality Labels | Valid Label Values |
|---|---|---|---|
| `warehouse_reconciliation_resolutions_total` | Counter | `action`, `result` | `action`: `close_stale_allocation`, `confirm_missing`, `replace_live_allocation`<br>`result`: `success`, `rejected`, `error` |
| `warehouse_picking_scans_total` | Counter | `result` | `ok`, `already_picked`, `not_found`, `wrong_variant`, `allocated_to_other_order`, `not_allocated`, `cannot_pick_serialized_with_barcode` |
| `inventory_writeoffs_total` | Counter | `reason_category` | `reconciliation_missing`, `damaged`, `lost`, `other` |

## 3. Security / Redaction Contract
**Never log the following:**
- `Authorization`, `Cookie`, `Set-Cookie`
- `JWT`, `access_token`, `refresh_token`
- `password`, `current_password`, `new_password`
- S3 / MinIO credentials, payment provider credentials
- Full payment/card data
- Full request/response bodies
- Customer email, customer phone, customer delivery address

Do not dump SQL arguments automatically.
Errors may safely contain: safe error message, error_code, db SQLSTATE, upstream name, upstream status, retryable, attempt.

> [!CAUTION]
> **LOCAL DEV ONLY: Anonymous Admin Authentication**
> Grafana is configured with anonymous Admin access strictly for frictionless local developer environment ergonomics.
> **NEVER use anonymous Admin in production.** Production deployments must enforce SSO/OIDC or strict role-based authentication.

## 4. Local Infrastructure Layout & Security Policy
All observability host-published ports are strictly bound to loopback (`127.0.0.1`) to prevent external interface exposure on developer workstations. ZAMK Shop binds `localhost:3000`, while Grafana binds `127.0.0.1:3003`.

| Component | Host Port (Loopback) | Container Port | Protocol / Purpose |
|---|---|---|---|
| Grafana | `127.0.0.1:3003` | `3000` | Web UI & Provisioned Dashboards |
| Loki | `127.0.0.1:3100` | `3100` | Structured Log Ingestion / Query |
| Tempo | `127.0.0.1:3200` | `3200` | Distributed Tracing API |
| Prometheus | `127.0.0.1:9090` | `9090` | Operational Metrics TSDB |
| Alloy UI / Health | `127.0.0.1:12345` | `12345` | Alloy HTTP Management / Health |
| Alloy OTLP gRPC | `127.0.0.1:4317` | `4317` | OpenTelemetry gRPC Ingestion Gateway |
| Alloy OTLP HTTP | `127.0.0.1:4318` | `4318` | OpenTelemetry HTTP Ingestion Gateway |
| MinIO S3 API | `0.0.0.0:9000` | `9000` | Shared MinIO Object Storage |

### Official Supported Version Matrix
All images use pinned releases from current upstream supported lines (zero `latest` tags):
- **Grafana:** `13.2.1` (Current supported `13.2.x` line; latest patch release)
- **Loki:** `3.7.7` (Current supported `3.7.x` line; monolithic single-binary, TSDB v13, MinIO S3 object store)
- **Tempo:** `3.0.3` (Recommended `3.0.x` line; migrated v3 configuration, MinIO S3 storage, span-metrics & service-graphs)
- **Prometheus:** `v3.13.2` (Current supported LTS `3.13.x` line; `--web.enable-remote-write-receiver`)
- **Grafana Alloy:** `v1.19.2` (Current supported `1.19.x` line; unified telemetry gateway for OTLP)
- **MinIO mc:** `RELEASE.2025-08-13T08-35-41Z` (Pinned stable MinIO Client release)

## 5. Persistence Model
Local telemetry state is backed by dedicated Docker named volumes and isolated MinIO buckets:
- `grafana_data` $\rightarrow$ `/var/lib/grafana` (user settings, dashboard preferences, plugin installations)
- `prometheus_data` $\rightarrow$ `/prometheus` (local metrics TSDB WAL and blocks)
- `loki_data` $\rightarrow$ `/loki` (local index cache, compaction temp, active WAL)
- `tempo_data` $\rightarrow$ `/var/tempo` (local trace WAL, generator WAL, block cache)
- Dedicated MinIO buckets: `zamk-loki` and `zamk-tempo` (long-term chunk/block objects).

### Persistence Lifecycle Matrix
| Event | Local Named Volumes | MinIO Buckets (`zamk-loki`, `zamk-tempo`) | Telemetry Availability |
|---|---|---|---|
| **Container Restart** | Preserved | Preserved | 100% available immediately |
| **`npm run observability:down` then `up`** | Preserved | Preserved | All historical telemetry, logs, traces, and metrics survive |
| **Explicit Volume Prune (`down -v`)** | Purged | Preserved (in MinIO storage volume) | MinIO chunks survive; local caches rebuilt automatically |

## 6. Retention Policy (Local Dev vs Production)
**Local Dev:** Modest retention / storage to preserve developer machine resources.
**Production (Policy Defaults, not business truth):**
- DEBUG logs: 7d
- INFO logs: 30d
- WARN/ERROR logs: 90d
- Traces: 14–30d
- Metrics: 30–90d

Audit/business records remain governed by PostgreSQL/domain policy. Do not introduce aggressive trace sampling in dev.

## 7. Observability Definition of Done (DoD)

Every future feature, endpoint, mutation, or milestone MUST evaluate and document observability requirements using this mandatory checklist template:

### Mandatory PR / Feature Checklist Template

```markdown
### Observability Definition of Done Checklist
- [ ] **LOG:**
  - Emits meaningful structured log? [YES/NO]
  - Level: [INFO for regular transitions / WARN for recoverable anomalies / ERROR for failures]
  - Bounded context attributes: [e.g. event_name, domain, action, result, actor_id, actor_role, entity IDs]
  - Redaction verified: [Zero PII, zero credentials, zero raw passwords/tokens/cards]
- [ ] **TRACE:**
  - Active span enriched or dedicated span created? [YES/NO]
  - Span event emitted? [YES/NO: event name, bounded attributes, no high-cardinality leak]
- [ ] **METRIC:**
  - Operational counter/histogram updated? [YES/NO]
  - Metric name: [e.g. warehouse_picking_scans_total]
  - Metric type: [Counter / Histogram / Gauge]
  - Low-cardinality labels only: [e.g. action, result, reason_category - NO entity IDs]
- [ ] **DURABLE AUDIT:**
  - Requires persistent legal/financial/transactional record? [YES/NO]
  - Destination table: [e.g. inventory_reconciliation_resolutions, stock_movements]
  - Consistency: [Committed inside the same DB transaction as the business mutation]
- [ ] **PRODUCT ANALYTICS:**
  - Represents user/product behavior for BI funnels? [YES/NO]
  - Schema / destination: [ClickHouse / async event queue]
```

### Reference Decision Matrix
| Scenario | LOG | TRACE | METRIC | DURABLE AUDIT | PRODUCT ANALYTICS |
|---|---|---|---|---|---|
| **Inventory write-off** | YES (INFO, post-commit) | YES (span event) | YES (counter by reason) | YES (`stock_movements`, resolutions) | NO |
| **Picking scan duplicate** | YES (WARN, duplicate scan) | YES (span event) | YES (counter by `already_picked`) | NO | NO |
| **Product viewed in shop** | NO | Optional | Optional (RUM) | NO | YES (ClickHouse event) |
| **Payment refunded** | YES (INFO, post-commit) | YES (span) | YES (counter by method) | YES (`refunds`, `audit_logs`) | YES (BI financial event) |

### Transaction Consistency Rules
1. **Post-Commit Emission:** Business events for database mutations **MUST be emitted only after** the transaction has successfully committed (`tx.Commit` returned `nil`).
2. **Rollback Silence:** If a transaction fails or rolls back, **NO mutation success business event** may be emitted.
3. **Idempotency De-duplication:** If an operation is an idempotent repeat that resulted in no state transition, **do NOT emit duplicate mutation success events**.

New HTTP endpoints should have common middleware providing: `request_id`, `trace_id`, route template, status, duration, panic/error correlation.

## 8. Verification & Smoke Tooling
Developer commands provided in the repository:
- `npm run observability:up`: Spins up the observability stack.
- `npm run observability:down`: Gracefully terminates the observability stack.
- `npm run observability:check`: Executes automated 11-point health check (A–K) verifying all services, provisioned datasources, proxy queries, and S3 buckets.
- `npm run observability:smoke`: Executes real end-to-end OTLP ingestion smoke test through Alloy into Loki, Tempo, and Prometheus, asserting zero cardinality leakage.
