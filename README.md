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

開発用の環境変数ファイルは `env.local`、本番用は `env.production` を使います。現時点では `env.production` の Supabase 関連値は空のままで構いません。

`docker compose --env-file env.local` は PostgreSQL と API を同時に起動し、デフォルトで `http://127.0.0.1:18080` に公開します。必要なら `env.local` の `HOST_PORT` で変更できます。PostgreSQL は `127.0.0.1:${POSTGRES_PORT:-15432}` で確認できます。

`env.local` ではローカル PostgreSQL を使いつつ、Supabase Auth の JWT を検証するために `SUPABASE_PROJECT_URL` と `SUPABASE_REQUIRE_AUTH=true` を指定しています。これで Simulator / 実機の Mobile から匿名 Supabase セッションで backend を叩けます。

Supabase 側の本番 / 共有 DB へスキーマを反映する SQL は `/Users/yota/Projects/Monee/Backend/scripts/create_monee_backend_schema.sql` に置いてあります。Supabase の SQL Editor へ貼り付けるか、書き込み権限のある MCP / CLI から適用してください。

## Mobile app integration

- iOS シミュレータからは `http://127.0.0.1:18080` をそのまま利用できます
- 実機からは Mac のローカルネットワーク IP を `Backend URL` に設定してください
- Monee アプリの `Settings > Backend sync` から接続確認、取り込み、書き出しを行えます
- Mobile 側は Supabase Auth の匿名セッションを自動作成 / 更新し、その bearer token を backend に付与します
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
- `SEED_DEFAULT_CATEGORIES=true` なら、認証済みユーザー単位で初回アクセス時にカテゴリだけを自動投入します
- 取引・予算・目標のモックデータは backend 側では投入しません
- Fly.io に持っていく場合もこの Dockerfile をベースにできます

## Deploy to Fly.io

`fly.toml` を使って `monee-backend.fly.dev` へデプロイできます。Supabase Auth を使う場合は Fly.io 側に `SUPABASE_PROJECT_URL` と `SUPABASE_REQUIRE_AUTH=true` を入れ、`DATABASE_URL` には Supabase Postgres の接続文字列を secret として注入します。

```bash
cd /Users/yota/Projects/Monee/Backend
fly auth login
fly secrets set \
  DATABASE_URL=<supabase_postgres_url> \
  SUPABASE_PROJECT_URL=https://<project-ref>.supabase.co \
  SUPABASE_REQUIRE_AUTH=true
fly deploy -a monee-backend
```

補足:

- app 名は `monee-backend`
- 公開 URL は `https://monee-backend.fly.dev`
- app は `nrt` リージョンで 1 台常駐させる設定です
- 本番では `env.production` の値を埋めなくても、Fly.io 側の secret で `DATABASE_URL` / `SUPABASE_PROJECT_URL` / 必要なら `SUPABASE_JWT_SECRET` を渡せば起動できます
- deploy 後の確認は `https://monee-backend.fly.dev/healthz` と `https://monee-backend.fly.dev/readyz` を使います

## GitHub Actions deploy

backend repo には `/.github/workflows/fly-deploy.yml` を置いてあり、次の条件で Fly.io へ deploy できます。

- `main` への push
- `workflow_dispatch`
- このセットアップを検証するため、現在の作業ブランチ `codex/feature/mobile_backend/019d870d` への push

必要な GitHub Actions secret:

- `FLY_API_TOKEN`

repo secret の投入例:

```bash
gh secret set FLY_API_TOKEN --repo Suuu-sh/Monee_Backend
```

手動実行:

```bash
gh workflow run "Fly Deploy" --repo Suuu-sh/Monee_Backend
```
