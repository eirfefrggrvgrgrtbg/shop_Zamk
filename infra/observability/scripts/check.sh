#!/bin/bash
set -e

echo "=================================================="
echo "ZAMK Observability Stack Health & Verification"
echo "=================================================="

# A. Grafana HTTP Health (port 3003)
echo -n "Checking Grafana (http://127.0.0.1:3003/api/health)... "
GRAFANA_HEALTH=$(curl -s http://127.0.0.1:3003/api/health || true)
if echo "$GRAFANA_HEALTH" | grep -q '"database": "ok"'; then
  echo "✅ PASS"
else
  echo "❌ FAIL: Grafana not healthy: $GRAFANA_HEALTH"
  exit 1
fi

# B. Loki Readiness
echo -n "Checking Loki readiness (http://127.0.0.1:3100/ready)... "
LOKI_READY=$(curl -s http://127.0.0.1:3100/ready || true)
if echo "$LOKI_READY" | grep -q "ready"; then
  echo "✅ PASS"
else
  echo "❌ FAIL: Loki not ready: $LOKI_READY"
  exit 1
fi

# C. Tempo Readiness
echo -n "Checking Tempo readiness (http://127.0.0.1:3200/ready)... "
TEMPO_READY=$(curl -s http://127.0.0.1:3200/ready || true)
if echo "$TEMPO_READY" | grep -q "ready"; then
  echo "✅ PASS"
else
  echo "❌ FAIL: Tempo not ready: $TEMPO_READY"
  exit 1
fi

# D. Prometheus Health
echo -n "Checking Prometheus health (http://127.0.0.1:9090/-/healthy)... "
PROM_HEALTH=$(curl -s http://127.0.0.1:9090/-/healthy || true)
if echo "$PROM_HEALTH" | grep -q "Prometheus Server is Healthy"; then
  echo "✅ PASS"
else
  echo "❌ FAIL: Prometheus not healthy: $PROM_HEALTH"
  exit 1
fi

# E. Alloy Health
echo -n "Checking Alloy health (http://127.0.0.1:12345/-/healthy)... "
ALLOY_CODE=$(curl -s -o /dev/null -w "%{http_code}" http://127.0.0.1:12345/-/healthy || true)
if [ "$ALLOY_CODE" = "200" ]; then
  echo "✅ PASS"
else
  echo "❌ FAIL: Alloy status code $ALLOY_CODE"
  exit 1
fi

# F. Grafana Provisioned Datasources
echo -n "Checking Grafana provisioned datasources (Prometheus, Loki, Tempo)... "
DATASOURCES=$(curl -s http://127.0.0.1:3003/api/datasources || true)
HAS_PROM=$(echo "$DATASOURCES" | jq -r '.[] | select(.name=="Prometheus" and .type=="prometheus") | .uid')
HAS_LOKI=$(echo "$DATASOURCES" | jq -r '.[] | select(.name=="Loki" and .type=="loki") | .uid')
HAS_TEMPO=$(echo "$DATASOURCES" | jq -r '.[] | select(.name=="Tempo" and .type=="tempo") | .uid')

if [ -n "$HAS_PROM" ] && [ -n "$HAS_LOKI" ] && [ -n "$HAS_TEMPO" ]; then
  echo "✅ PASS (Prometheus UID: $HAS_PROM, Loki UID: $HAS_LOKI, Tempo UID: $HAS_TEMPO)"
else
  echo "❌ FAIL: Missing datasources in Grafana: $DATASOURCES"
  exit 1
fi

# G. Grafana Datasource Health Checks
echo -n "Checking Grafana Prometheus datasource health check... "
PROM_DS_HEALTH=$(curl -s http://127.0.0.1:3003/api/datasources/uid/prometheus/health || true)
if echo "$PROM_DS_HEALTH" | jq -e '.status == "OK"' > /dev/null 2>&1; then
  echo "✅ PASS"
else
  echo "❌ FAIL: Prometheus datasource check failed: $PROM_DS_HEALTH"
  exit 1
fi

echo -n "Checking Grafana Loki datasource health check... "
LOKI_DS_HEALTH=$(curl -s http://127.0.0.1:3003/api/datasources/uid/loki/health || true)
if echo "$LOKI_DS_HEALTH" | jq -e '.status == "OK"' > /dev/null 2>&1; then
  echo "✅ PASS"
else
  echo "❌ FAIL: Loki datasource check failed: $LOKI_DS_HEALTH"
  exit 1
fi

echo -n "Checking Grafana Tempo proxy connectivity... "
TEMPO_DS_PROXY=$(curl -s http://127.0.0.1:3003/api/datasources/proxy/uid/tempo/ready || true)
if echo "$TEMPO_DS_PROXY" | grep -q "ready"; then
  echo "✅ PASS"
else
  echo "❌ FAIL: Tempo datasource proxy check failed: $TEMPO_DS_PROXY"
  exit 1
fi

# H. Prometheus Can Execute a Real Query
echo -n "Checking Prometheus real query execution (query=up)... "
PROM_QUERY=$(curl -s 'http://127.0.0.1:9090/api/v1/query?query=up' || true)
if echo "$PROM_QUERY" | jq -e '.status == "success" and (.data.result | length) > 0' > /dev/null 2>&1; then
  RESULTS_COUNT=$(echo "$PROM_QUERY" | jq '.data.result | length')
  echo "✅ PASS ($RESULTS_COUNT targets reporting)"
else
  echo "❌ FAIL: Prometheus query failed: $PROM_QUERY"
  exit 1
fi

# I. Loki Can Execute a Harmless Real Query/API Check
echo -n "Checking Loki API query execution (labels & query_range)... "
LOKI_LABELS=$(curl -s 'http://127.0.0.1:3100/loki/api/v1/labels' || true)
LOKI_QUERY=$(curl -s --get 'http://127.0.0.1:3100/loki/api/v1/query_range' --data-urlencode 'query={job="test"}' || true)
if echo "$LOKI_LABELS" | jq -e '.status == "success"' > /dev/null 2>&1 && echo "$LOKI_QUERY" | jq -e '.status == "success"' > /dev/null 2>&1; then
  echo "✅ PASS"
else
  echo "❌ FAIL: Loki query check failed: labels=$LOKI_LABELS, query=$LOKI_QUERY"
  exit 1
fi

# J. Tempo API Query-Capable & Reachable
echo -n "Checking Tempo API query capability (/ready & /status)... "
TEMPO_STATUS=$(curl -s http://127.0.0.1:3200/status || true)
if echo "$TEMPO_STATUS" | grep -q "storage" && echo "$TEMPO_READY" | grep -q "ready"; then
  echo "✅ PASS"
else
  echo "❌ FAIL: Tempo status check failed"
  exit 1
fi

# K. MinIO Buckets Exist (Verified via MinIO S3 API semantics)
echo -n "Checking MinIO dedicated observability buckets (S3 API check via mc stat)... "
MC_STAT_OUT=$(docker run --rm --entrypoint /bin/sh --network backend_default minio/mc:RELEASE.2025-08-13T08-35-41Z -c \
  "/usr/bin/mc alias set zamk_minio http://minio:9000 zamk_minio zamk_minio_password >/dev/null && /usr/bin/mc stat zamk_minio/zamk-loki >/dev/null && /usr/bin/mc stat zamk_minio/zamk-tempo >/dev/null && echo OK" 2>&1 || true)

if echo "$MC_STAT_OUT" | grep -q "OK"; then
  echo "✅ PASS (zamk-loki and zamk-tempo verified via S3 protocol)"
else
  echo "❌ FAIL: S3 bucket verification failed: $MC_STAT_OUT"
  exit 1
fi

echo "=================================================="
echo "All Observability Verifications PASSED (A–K)"
echo "=================================================="
