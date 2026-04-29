# Monee Backend

Go + Gin で構築した Monee 用の backend API です。

## Stack
- Go 1.24
- Gin
- GORM
- PostgreSQL (runtime via Docker Compose)
- SQLite (in-memory test driver)
- Docker / Docker Compose

## Features
- Health / readiness endpoint
- Categories CRUD
- Transactions CRUD
- Budgets CRUD
- Savings goals CRUD
- Subscription records CRUD
- App preferences CRUD
- Summary endpoint (`/api/v1/summary`)
- 初回起動時のデフォルトカテゴリ seed

## Local start
```bash
make deps
make test
docker compose --env-file env.local up --build -d
```

開発用の環境変数ファイルは `env.local`、本番用は `env.production` を使います。現時点では `env.production` は空のままで構いません。

`docker compose --env-file env.local` は PostgreSQL と API を同時に起動し、デフォルトで `http://127.0.0.1:18080` に公開します。必要なら `env.local` の `HOST_PORT` で変更できます。PostgreSQL は `127.0.0.1:${POSTGRES_PORT:-15432}` で確認できます。

## Mobile app integration

- iOS シミュレータからは `http://127.0.0.1:18080` をそのまま利用できます
- 実機からは Mac のローカルネットワーク IP を `Backend URL` に設定してください
- Monee アプリの `Settings > Backend sync` から接続確認、取り込み、書き出しを行えます
- Backend URL を保存すると、Mobile 側は起動時に「空の local store ← backend」または「空の backend ← local store」の初回同期を行えます
- Auto sync を有効にすると、Mobile 側の編集内容を backend に自動反映できます
- 同期対象は app preferences / categories / transactions / budgets / savings goals / subscription records です
- Mobile 側はサンプル取引を自動投入せず、実データだけを PostgreSQL-backed backend と同期します

## API examples
```bash
curl http://127.0.0.1:18080/healthz
curl http://127.0.0.1:18080/api/v1/preferences
curl http://127.0.0.1:18080/api/v1/categories
curl http://127.0.0.1:18080/api/v1/subscriptions
curl http://127.0.0.1:18080/api/v1/summary?range=month
```

## Main endpoints
- `GET /healthz`
- `GET /readyz`
- `GET /api/v1/summary?range=month`
- `GET|POST|PUT|DELETE /api/v1/preferences`
- `GET|POST|PUT|DELETE /api/v1/categories`
- `GET|POST|PUT|DELETE /api/v1/transactions`
- `GET|POST|PUT|DELETE /api/v1/budgets`
- `GET|POST|PUT|DELETE /api/v1/savings-goals`
- `GET|POST|PUT|DELETE /api/v1/subscriptions`

## Notes
- Runtime は PostgreSQL を使い、テストだけ SQLite in-memory を使います
- `SEED_DEFAULT_CATEGORIES=true` なら認証済みユーザーごとに初回カテゴリだけを自動投入します
- 取引・予算・目標のモックデータは backend 側では投入しません
- `/api/v1/*` は Supabase Auth の Bearer token を必須にし、データは Supabase user id ごとに分離します
- production deploy は `render.yaml` と Dockerfile を使います

## Deploy to Render + existing Supabase

Fly.io の常駐 app / Managed Postgres では料金が出やすいため、Go API だけ Render Free Web Service に移し、DB と Auth は既存 Supabase project を使います。

構成:

```text
iOS app
  -> Render Free Web Service / Go API
  -> Supabase Postgres + Supabase Auth
```

この repository には Render Blueprint 用の `render.yaml` を置いてあります。Render Dashboard で Blueprint を作成し、`Suuu-sh/Monee_Backend` を接続してください。

Blueprint 作成時に `DATABASE_URL` を入力します。Supabase Dashboard の connection string を使い、Render からの接続では SSL を有効にしてください。

推奨:

- `DATABASE_URL`: Supabase pooler / session mode の connection string
- `SUPABASE_PROJECT_URL`: `https://azvfsidxfxjnxatjbljm.supabase.co`
- `SUPABASE_PUBLISHABLE_KEY`: app と同じ publishable key
- `DATABASE_DRIVER`: `postgres`
- `SEED_DEFAULT_CATEGORIES`: `true`

Render Free の注意:

- 15 分 idle で sleep し、次の request で起動します
- sleep 中は Free instance hours を消費しません
- 起動直後の request は cold start で遅くなることがあります
- filesystem は ephemeral なので、production では SQLite を使わず Supabase Postgres を使います

確認:

```bash
curl https://monee-backend.onrender.com/healthz
curl https://monee-backend.onrender.com/readyz
```

`/api/v1/*` は Supabase anonymous session の `Authorization: Bearer <access_token>` が必要です。

## Stop Fly.io deploys

この repository から Fly.io へ自動 deploy する GitHub Actions workflow と `fly.toml` は削除済みです。ただし、既存の Fly.io app / Managed Postgres は repository 変更だけでは停止・削除されません。

Render への移行とデータ確認が終わったら、Fly.io 側で app と DB を停止または削除してください。

確認例:

```bash
fly apps list
fly status -a monee-backend
fly mpg list -o personal
```
