drop table if exists public.transactions cascade;
drop table if exists public.budgets cascade;
drop table if exists public.savings_goals cascade;
drop table if exists public.subscription_records cascade;
drop table if exists public.app_preferences cascade;
drop table if exists public.categories cascade;

create table public.categories (
  id text primary key,
  user_id text not null,
  slug text not null,
  name text not null,
  type text not null,
  icon text not null,
  color_token text not null,
  "order" integer not null default 0,
  is_active boolean not null default true,
  created_at timestamptz not null default timezone('utc', now()),
  updated_at timestamptz not null default timezone('utc', now())
);

create unique index idx_categories_user_slug
  on public.categories (user_id, slug);

create index idx_categories_type
  on public.categories (type);

create index idx_categories_user_id
  on public.categories (user_id);

create table public.transactions (
  id text primary key,
  user_id text not null,
  title text not null,
  amount double precision not null,
  type text not null,
  date timestamptz not null,
  note text,
  merchant_name text,
  category_id text references public.categories(id) on delete set null,
  is_subscription_candidate boolean not null default false,
  recurrence_hint text,
  created_at timestamptz not null default timezone('utc', now()),
  updated_at timestamptz not null default timezone('utc', now())
);

create index idx_transactions_user_id
  on public.transactions (user_id);

create index idx_transactions_type
  on public.transactions (type);

create index idx_transactions_date
  on public.transactions (date);

create index idx_transactions_category_id
  on public.transactions (category_id);

create table public.budgets (
  id text primary key,
  user_id text not null,
  name text not null,
  scope text not null,
  monthly_limit double precision not null,
  category_id text references public.categories(id) on delete set null,
  is_active boolean not null default true,
  created_at timestamptz not null default timezone('utc', now()),
  updated_at timestamptz not null default timezone('utc', now())
);

create index idx_budgets_user_id
  on public.budgets (user_id);

create index idx_budgets_scope
  on public.budgets (scope);

create index idx_budgets_category_id
  on public.budgets (category_id);

create table public.savings_goals (
  id text primary key,
  user_id text not null,
  name text not null,
  target_amount double precision not null,
  saved_amount double precision not null default 0,
  target_date timestamptz,
  is_active boolean not null default true,
  created_at timestamptz not null default timezone('utc', now()),
  updated_at timestamptz not null default timezone('utc', now())
);

create index idx_savings_goals_user_id
  on public.savings_goals (user_id);

create table public.subscription_records (
  id text primary key,
  user_id text not null,
  merchant_key text not null,
  display_name text not null,
  label text not null,
  average_amount double precision not null,
  cadence text not null,
  state text not null,
  estimated_next_billing_date timestamptz,
  last_charge_date timestamptz,
  monthly_equivalent_amount double precision not null,
  yearly_equivalent_amount double precision not null,
  latest_transaction_title text,
  created_at timestamptz not null default timezone('utc', now()),
  updated_at timestamptz not null default timezone('utc', now())
);

create unique index idx_subscriptions_user_merchant_key
  on public.subscription_records (user_id, merchant_key);

create index idx_subscription_records_user_id
  on public.subscription_records (user_id);

create index idx_subscription_records_cadence
  on public.subscription_records (cadence);

create index idx_subscription_records_state
  on public.subscription_records (state);

create table public.app_preferences (
  id text primary key,
  user_id text not null,
  currency_code text not null,
  month_start_day integer not null,
  is_ai_summaries_enabled boolean not null default true,
  appearance_raw text not null,
  language_raw text,
  home_summary_range_raw text,
  home_selected_date timestamptz,
  home_range_start_date timestamptz,
  home_range_end_date timestamptz,
  budget_warning_threshold double precision not null default 0.8,
  seed_scenario_raw text not null default 'balanced',
  created_at timestamptz not null default timezone('utc', now()),
  updated_at timestamptz not null default timezone('utc', now())
);

create index idx_app_preferences_user_id
  on public.app_preferences (user_id);
