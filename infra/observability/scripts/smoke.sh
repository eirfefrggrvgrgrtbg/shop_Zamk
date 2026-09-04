#!/bin/bash
set -e

echo "=================================================="
echo "ZAMK Observability E2E Telemetry & Correlation Smoke"
echo "=================================================="

SMOKE_ID="smoke_$(date +%s)_$RANDOM"
TRACE_ID=$(openssl rand -hex 16)
SPAN_ID=$(openssl rand -hex 8)
NOW_NANO="$(date +%s)000000000"

echo "Smoke Correlation ID: $SMOKE_ID"
echo "Shared Trace ID:      $TRACE_ID"
echo "Shared Span ID:       $SPAN_ID"
echo "--------------------------------------------------"

# 1. SEND DISTRIBUTED TRACE VIA ALLOY OTLP HTTP (:4318)
echo -n "1. Sending distributed trace via Alloy OTLP HTTP (:4318)... "
TRACE_PAYLOAD=$(cat <<JSON
{
  "resourceSpans": [
    {
      "resource": {
        "attributes": [
          { "key": "service.name", "value": { "stringValue": "zamk-observability-smoke" } },
          { "key": "environment", "value": { "stringValue": "local" } }
        ]
      },
      "scopeSpans": [
        {
          "scope": { "name": "smoke" },
          "spans": [
            {
              "traceId": "$TRACE_ID",
              "spanId": "$SPAN_ID",
              "name": "observability.e2e_test",
              "kind": 1,
              "startTimeUnixNano": "$NOW_NANO",
              "endTimeUnixNano": "$NOW_NANO",
              "attributes": [
                { "key": "environment", "value": { "stringValue": "local" } },
                { "key": "component", "value": { "stringValue": "observability_smoke" } },
                { "key": "smoke_id", "value": { "stringValue": "$SMOKE_ID" } }
              ],
              "status": { "code": 1 }
            }
          ]
        }
      ]
    }
  ]
}
JSON
)

TRACE_RESP=$(curl -s -w "\n%{http_code}" -X POST http://127.0.0.1:4318/v1/traces \
  -H "Content-Type: application/json" \
  -d "$TRACE_PAYLOAD")
TRACE_HTTP_CODE=$(echo "$TRACE_RESP" | tail -n 1)

if [ "$TRACE_HTTP_CODE" = "200" ]; then
  echo "✅ Ingested into Alloy"
else
  echo "❌ FAIL: Alloy traces ingest returned HTTP $TRACE_HTTP_CODE: $TRACE_RESP"
  exit 1
fi

echo -n "   Verifying trace retrieval in Tempo API... "
FOUND_TRACE=0
for i in {1..10}; do
  TEMPO_RES=$(curl -s "http://127.0.0.1:3200/api/traces/$TRACE_ID" || true)
  if echo "$TEMPO_RES" | grep -q "observability.e2e_test" && echo "$TEMPO_RES" | grep -q "$SMOKE_ID"; then
    FOUND_TRACE=1
    break
  fi
  sleep 0.5
done

if [ "$FOUND_TRACE" = "1" ]; then
  echo "✅ PASS (Retrieved exact trace $TRACE_ID from Tempo)"
else
  echo "❌ FAIL: Trace not found in Tempo within timeout: $TEMPO_RES"
  exit 1
fi

# 2. SEND CORRELATED INFO LOG VIA ALLOY OTLP HTTP (:4318)
echo -n "2. Sending correlated INFO log with shared trace context... "
INFO_LOG_PAYLOAD=$(cat <<JSON
{
  "resourceLogs": [
    {
      "resource": {
        "attributes": [
          { "key": "service.name", "value": { "stringValue": "zamk-observability-smoke" } },
          { "key": "environment", "value": { "stringValue": "local" } }
        ]
      },
      "scopeLogs": [
        {
          "scope": { "name": "smoke" },
          "logRecords": [
            {
              "timeUnixNano": "$NOW_NANO",
              "traceId": "$TRACE_ID",
              "spanId": "$SPAN_ID",
              "severityText": "INFO",
              "body": { "stringValue": "synthetic observability correlated log [smoke_id=$SMOKE_ID]" },
              "attributes": [
                { "key": "component", "value": { "stringValue": "observability_smoke" } },
                { "key": "operation", "value": { "stringValue": "e2e_test" } },
                { "key": "smoke_id", "value": { "stringValue": "$SMOKE_ID" } }
              ]
            }
          ]
        }
      ]
    }
  ]
}
JSON
)

INFO_RESP=$(curl -s -w "\n%{http_code}" -X POST http://127.0.0.1:4318/v1/logs \
  -H "Content-Type: application/json" \
  -d "$INFO_LOG_PAYLOAD")
INFO_HTTP_CODE=$(echo "$INFO_RESP" | tail -n 1)

if [ "$INFO_HTTP_CODE" = "200" ]; then
  echo "✅ Ingested into Alloy"
else
  echo "❌ FAIL: Alloy logs ingest returned HTTP $INFO_HTTP_CODE: $INFO_RESP"
  exit 1
fi

echo -n "   Verifying INFO log with semantic indexed labels in Loki... "
FOUND_INFO=0
INFO_RECORD=""
for i in {1..10}; do
  LOKI_RES=$(curl -s --get 'http://127.0.0.1:3100/loki/api/v1/query_range' \
    --data-urlencode "query={service_name=\"zamk-observability-smoke\", environment=\"local\", component=\"observability_smoke\", level=\"INFO\"} |= \"$SMOKE_ID\"" || true)
  if echo "$LOKI_RES" | grep -q "$SMOKE_ID"; then
    FOUND_INFO=1
    INFO_RECORD="$LOKI_RES"
    break
  fi
  sleep 0.5
done

if [ "$FOUND_INFO" = "1" ]; then
  echo "✅ PASS (Matched {service_name, environment, component, level=\"INFO\"})"
else
  echo "❌ FAIL: INFO log not queryable by semantic labels: $LOKI_RES"
  exit 1
fi

# 3. VERIFY GRAFANA LOG -> TRACE CORRELATION MATCHER
echo -n "3. Validating Grafana derived-field trace matcher against stored log... "
RAW_LOG_ENTRY=$(echo "$INFO_RECORD" | jq -r '.data.result[0].values[0][1]')
EXTRACTED_TRACE_ID=$(python3 -c "
import re, sys
entry = sys.argv[1]
pattern = r'\"traceid\":\"([a-fA-F0-9]+)\"'
m = re.search(pattern, entry)
print(m.group(1) if m else '')
" "$RAW_LOG_ENTRY")

if [ "$EXTRACTED_TRACE_ID" = "$TRACE_ID" ]; then
  echo "✅ PASS (Extracted $EXTRACTED_TRACE_ID from log matching Tempo trace)"
else
  echo "❌ FAIL: Regex did not extract expected trace ID: got '$EXTRACTED_TRACE_ID', expected '$TRACE_ID'"
  echo "   Raw log entry: $RAW_LOG_ENTRY"
  exit 1
fi

echo -n "   Verifying Grafana Loki datasource derived-field provisioning... "
LOKI_DS_CONFIG=$(curl -s http://127.0.0.1:3003/api/datasources/uid/loki || true)
MATCH_REGEX=$(echo "$LOKI_DS_CONFIG" | jq -r '.jsonData.derivedFields[0].matcherRegex // empty')
TARGET_DS_UID=$(echo "$LOKI_DS_CONFIG" | jq -r '.jsonData.derivedFields[0].datasourceUid // empty')
URL_DISPLAY_LABEL=$(echo "$LOKI_DS_CONFIG" | jq -r '.jsonData.derivedFields[0].urlDisplayLabel // empty')

if [ "$TARGET_DS_UID" = "tempo" ] && [ "$URL_DISPLAY_LABEL" = "View in Tempo" ] && [ "$MATCH_REGEX" = '"traceid":"([a-fA-F0-9]+)"' ]; then
  echo "✅ PASS (Derived field configured: UID=$TARGET_DS_UID, Label='$URL_DISPLAY_LABEL', Regex='$MATCH_REGEX')"
else
  echo "❌ FAIL: Grafana Loki derived field mismatch: $LOKI_DS_CONFIG"
  exit 1
fi

# 4. SEND SYNTHETIC ERROR LOG & VERIFY DASHBOARD QUERY
echo -n "4. Sending synthetic ERROR log for dashboard contract validation... "
ERROR_LOG_PAYLOAD=$(cat <<JSON
{
  "resourceLogs": [
    {
      "resource": {
        "attributes": [
          { "key": "service.name", "value": { "stringValue": "zamk-observability-smoke" } },
          { "key": "environment", "value": { "stringValue": "local" } }
        ]
      },
      "scopeLogs": [
        {
          "scope": { "name": "smoke" },
          "logRecords": [
            {
              "timeUnixNano": "$NOW_NANO",
              "severityText": "ERROR",
              "body": { "stringValue": "synthetic observability error smoke" },
              "attributes": [
                { "key": "component", "value": { "stringValue": "observability_smoke" } },
                { "key": "level", "value": { "stringValue": "ERROR" } },
                { "key": "operation", "value": { "stringValue": "synthetic_error_test" } },
                { "key": "smoke_id", "value": { "stringValue": "$SMOKE_ID" } }
              ]
            }
          ]
        }
      ]
    }
  ]
}
JSON
)

ERROR_RESP=$(curl -s -w "\n%{http_code}" -X POST http://127.0.0.1:4318/v1/logs \
  -H "Content-Type: application/json" \
  -d "$ERROR_LOG_PAYLOAD")
ERROR_HTTP_CODE=$(echo "$ERROR_RESP" | tail -n 1)

if [ "$ERROR_HTTP_CODE" = "200" ]; then
  echo "✅ Ingested into Alloy"
else
  echo "❌ FAIL: Alloy error log ingest returned HTTP $ERROR_HTTP_CODE: $ERROR_RESP"
  exit 1
fi

echo -n "   Verifying ERROR log query {level=\"ERROR\", service_name=...}... "
FOUND_ERROR=0
for i in {1..10}; do
  LOKI_ERR_RES=$(curl -s --get 'http://127.0.0.1:3100/loki/api/v1/query_range' \
    --data-urlencode "query={level=\"ERROR\", service_name=\"zamk-observability-smoke\"} |= \"synthetic observability error smoke\"" || true)
  if echo "$LOKI_ERR_RES" | grep -q "synthetic observability error smoke"; then
    FOUND_ERROR=1
    break
  fi
  sleep 0.5
done

if [ "$FOUND_ERROR" = "1" ]; then
  echo "✅ PASS (Grafana Recent ERROR Logs panel query verified)"
else
  echo "❌ FAIL: ERROR log query failed: $LOKI_ERR_RES"
  exit 1
fi

# 5. SEND OPERATIONAL METRIC VIA ALLOY OTLP HTTP (:4318)
echo -n "5. Sending operational metric via Alloy OTLP HTTP (:4318)... "
METRIC_PAYLOAD=$(cat <<JSON
{
  "resourceMetrics": [
    {
      "resource": {
        "attributes": [
          { "key": "service.name", "value": { "stringValue": "zamk-observability-smoke" } }
        ]
      },
      "scopeMetrics": [
        {
          "scope": { "name": "smoke" },
          "metrics": [
            {
              "name": "zamk_observability_smoke_total",
              "description": "Synthetic smoke metric",
              "sum": {
                "dataPoints": [
                  {
                    "asInt": "1",
                    "timeUnixNano": "$NOW_NANO",
                    "attributes": [
                      { "key": "environment", "value": { "stringValue": "local" } }
                    ]
                  }
                ],
                "aggregationTemporality": 2,
                "isMonotonic": true
              }
            }
          ]
        }
      ]
    }
  ]
}
JSON
)

METRIC_RESP=$(curl -s -w "\n%{http_code}" -X POST http://127.0.0.1:4318/v1/metrics \
  -H "Content-Type: application/json" \
  -d "$METRIC_PAYLOAD")
METRIC_HTTP_CODE=$(echo "$METRIC_RESP" | tail -n 1)

if [ "$METRIC_HTTP_CODE" = "200" ]; then
  echo "✅ Ingested into Alloy"
else
  echo "❌ FAIL: Alloy metrics ingest returned HTTP $METRIC_HTTP_CODE: $METRIC_RESP"
  exit 1
fi

echo -n "   Verifying metric delivery in Prometheus API... "
FOUND_METRIC=0
for i in {1..12}; do
  PROM_RES=$(curl -s --get 'http://127.0.0.1:9090/api/v1/query' \
    --data-urlencode 'query=zamk_observability_smoke_total' || true)
  if echo "$PROM_RES" | grep -q "zamk_observability_smoke_total" && echo "$PROM_RES" | grep -q '"1"'; then
    FOUND_METRIC=1
    break
  fi
  sleep 0.5
done

if [ "$FOUND_METRIC" = "1" ]; then
  echo "✅ PASS (Metric queried successfully in Prometheus)"
else
  echo "❌ FAIL: Metric not found in Prometheus within timeout: $PROM_RES"
  exit 1
fi

# 6. CARDINALITY CONTRACT ENFORCEMENT
echo -n "6. Verifying Cardinality Contract enforcement... "
LOKI_LABELS=$(curl -s http://127.0.0.1:3100/loki/api/v1/labels | jq -r '.data[]?' || true)

# Must contain canonical bounded labels
for required_lbl in service_name environment component level; do
  if ! echo "$LOKI_LABELS" | grep -q "^$required_lbl$"; then
    echo "❌ FAIL: Required bounded label '$required_lbl' missing from Loki: $LOKI_LABELS"
    exit 1
  fi
done

# Must NOT contain high-cardinality attributes
for forbidden_lbl in smoke_id trace_id span_id request_id order_id zmu; do
  if echo "$LOKI_LABELS" | grep -q "^$forbidden_lbl$"; then
    echo "❌ FAIL: High-cardinality attribute leaked as Loki indexed label: $forbidden_lbl"
    exit 1
  fi
done

# Prometheus: must NOT contain high-cardinality attributes
PROM_SERIES=$(curl -s 'http://127.0.0.1:9090/api/v1/series?match[]=zamk_observability_smoke_total' | jq -r '.data[0] | keys[]?' || true)
for forbidden_lbl in smoke_id trace_id span_id request_id order_id zmu; do
  if echo "$PROM_SERIES" | grep -q "^$forbidden_lbl$"; then
    echo "❌ FAIL: High-cardinality attribute leaked as Prometheus metric label: $forbidden_lbl"
    exit 1
  fi
done

echo "✅ PASS (Canonical labels present; high-cardinality labels excluded)"

echo "=================================================="
echo "All E2E Telemetry & Correlation Verifications PASSED"
echo "=================================================="
