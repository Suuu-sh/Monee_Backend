#!/usr/bin/env bash
set -euo pipefail

base_url="${1:-http://127.0.0.1:18080}"

echo "[1/4] health"
curl -fsS "$base_url/healthz" | jq .

echo "[2/4] categories"
curl -fsS "$base_url/api/v1/categories" | jq '.items | length'

echo "[3/4] create transaction"
transaction_id=$(curl -fsS -X POST "$base_url/api/v1/transactions" \
  -H 'Content-Type: application/json' \
  -d '{"title":"API Lunch","amount":1200,"type":"expense","date":"2026-04-05T12:00:00Z"}' | jq -r '.id')
echo "$transaction_id"

echo "[4/4] summary"
curl -fsS "$base_url/api/v1/summary?range=month" | jq '{expense_total, income_total, transaction_count}'
