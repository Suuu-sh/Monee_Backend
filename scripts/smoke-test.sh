#!/usr/bin/env bash
set -euo pipefail

base_url="${1:-http://127.0.0.1:18080}"

: "${SUPABASE_ACCESS_TOKEN:?Set SUPABASE_ACCESS_TOKEN to a Supabase user access token before running this smoke test.}"
auth_headers=(-H "Authorization: Bearer $SUPABASE_ACCESS_TOKEN")

echo "[1/4] health"
curl -fsS "$base_url/healthz" | jq .

echo "[2/4] categories"
curl -fsS "${auth_headers[@]}" "$base_url/api/v1/categories" | jq '.items | length'

echo "[3/4] create transaction"
transaction_id=$(curl -fsS -X POST "$base_url/api/v1/transactions" \
  "${auth_headers[@]}" \
  -H 'Content-Type: application/json' \
  -d '{"title":"API Lunch","amount":1200,"type":"expense","date":"2026-04-05T12:00:00Z"}' | jq -r '.id')
echo "$transaction_id"

echo "[4/4] summary"
curl -fsS "${auth_headers[@]}" "$base_url/api/v1/summary?range=month" | jq '{expense_total, income_total, transaction_count}'
