# Monee Backend

Go + Gin で構築した Monee 用の backend API です。

## Stack
- Go 1.24
- Gin
- GORM
- SQLite (pure Go driver)
- Docker / Docker Compose

## Features
- Health / readiness endpoint
- Categories CRUD
- Transactions CRUD
- Budgets CRUD
- Savings goals CRUD
- Summary endpoint (`/api/v1/summary`)
- 初回起動時のデフォルトカテゴリ seed

## Local start
```bash
cp .env.example .env
make deps
make test
docker compose up --build -d
```

`docker compose` はデフォルトで `http://127.0.0.1:18080` に公開します。必要なら `.env` の `HOST_PORT` で変更できます。

## API examples
```bash
curl http://127.0.0.1:18080/healthz
curl http://127.0.0.1:18080/api/v1/categories
curl http://127.0.0.1:18080/api/v1/summary?range=month
```

## Main endpoints
- `GET /healthz`
- `GET /readyz`
- `GET /api/v1/summary?range=month`
- `GET|POST|PUT|DELETE /api/v1/categories`
- `GET|POST|PUT|DELETE /api/v1/transactions`
- `GET|POST|PUT|DELETE /api/v1/budgets`
- `GET|POST|PUT|DELETE /api/v1/savings-goals`

## Notes
- DB はデフォルトで `/data/monee.db`
- `SEED_DEMO_DATA=true` なら初回起動時にカテゴリを自動投入
- Fly.io に持っていく場合もこの Dockerfile をベースにできます
