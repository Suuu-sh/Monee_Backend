create extension if not exists pgcrypto;

create table if not exists public.categories (
  id uuid primary key default gen_random_uuid(),
  user_id text not null,
  name text not null,
  icon text not null,
  type text not null check (type in ('expense', 'income')),
  color text not null,
  is_default boolean not null default false,
  sort_order integer not null default 0,
  created_at timestamptz not null default timezone('utc', now()),
  updated_at timestamptz not null default timezone('utc', now()),
  deleted_at timestamptz
);

create unique index if not exists categories_user_name_type_key
  on public.categories (user_id, name, type)
  where deleted_at is null;

create index if not exists categories_user_type_idx
  on public.categories (user_id, type, sort_order, created_at desc);

create table if not exists public.transactions (
  id uuid primary key default gen_random_uuid(),
  user_id text not null,
  category_id uuid references public.categories(id) on delete set null,
  title text not null,
  amount bigint not null,
  type text not null check (type in ('expense', 'income')),
  occurred_at timestamptz not null,
  note text,
  payment_method text not null default 'other',
  is_subscription boolean not null default false,
  billing_cycle text,
  created_at timestamptz not null default timezone('utc', now()),
  updated_at timestamptz not null default timezone('utc', now()),
  deleted_at timestamptz
);

create index if not exists transactions_user_occurred_idx
  on public.transactions (user_id, occurred_at desc)
  where deleted_at is null;

create index if not exists transactions_user_type_idx
  on public.transactions (user_id, type, occurred_at desc)
  where deleted_at is null;

create table if not exists public.budgets (
  id uuid primary key default gen_random_uuid(),
  user_id text not null,
  category_id uuid references public.categories(id) on delete cascade,
  amount bigint not null,
  month_start date not null,
  created_at timestamptz not null default timezone('utc', now()),
  updated_at timestamptz not null default timezone('utc', now()),
  deleted_at timestamptz,
  unique (user_id, category_id, month_start)
);

create index if not exists budgets_user_month_idx
  on public.budgets (user_id, month_start desc)
  where deleted_at is null;

create table if not exists public.savings_goals (
  id uuid primary key default gen_random_uuid(),
  user_id text not null,
  title text not null,
  target_amount bigint not null,
  current_amount bigint not null default 0,
  target_date date,
  note text,
  created_at timestamptz not null default timezone('utc', now()),
  updated_at timestamptz not null default timezone('utc', now()),
  deleted_at timestamptz
);

create index if not exists savings_goals_user_updated_idx
  on public.savings_goals (user_id, updated_at desc)
  where deleted_at is null;

create table if not exists public.subscription_records (
  id uuid primary key default gen_random_uuid(),
  user_id text not null,
  name text not null,
  amount bigint not null,
  billing_cycle text not null check (billing_cycle in ('monthly', 'yearly')),
  category_id uuid references public.categories(id) on delete set null,
  first_billing_date date not null,
  note text,
  created_at timestamptz not null default timezone('utc', now()),
  updated_at timestamptz not null default timezone('utc', now()),
  deleted_at timestamptz
);

create unique index if not exists subscription_records_user_name_key
  on public.subscription_records (user_id, name)
  where deleted_at is null;

create index if not exists subscription_records_user_updated_idx
  on public.subscription_records (user_id, updated_at desc)
  where deleted_at is null;

create table if not exists public.app_preferences (
  user_id text primary key,
  selected_budget_tab text not null default 'budget',
  preferred_home_tab text not null default 'summary',
  home_time_filter text not null default 'month',
  home_custom_start date,
  home_custom_end date,
  updated_at timestamptz not null default timezone('utc', now())
);

grant usage on schema public to anon, authenticated, service_role;
grant all on all tables in schema public to anon, authenticated, service_role;
grant all on all sequences in schema public to anon, authenticated, service_role;
alter default privileges in schema public grant all on tables to anon, authenticated, service_role;
alter default privileges in schema public grant all on sequences to anon, authenticated, service_role;

alter table public.categories enable row level security;
alter table public.transactions enable row level security;
alter table public.budgets enable row level security;
alter table public.savings_goals enable row level security;
alter table public.subscription_records enable row level security;
alter table public.app_preferences enable row level security;

do $$ begin
  create policy categories_own
    on public.categories
    for all
    using (auth.uid()::text = user_id)
    with check (auth.uid()::text = user_id);
exception when duplicate_object then null; end $$;

do $$ begin
  create policy transactions_own
    on public.transactions
    for all
    using (auth.uid()::text = user_id)
    with check (auth.uid()::text = user_id);
exception when duplicate_object then null; end $$;

do $$ begin
  create policy budgets_own
    on public.budgets
    for all
    using (auth.uid()::text = user_id)
    with check (auth.uid()::text = user_id);
exception when duplicate_object then null; end $$;

do $$ begin
  create policy savings_goals_own
    on public.savings_goals
    for all
    using (auth.uid()::text = user_id)
    with check (auth.uid()::text = user_id);
exception when duplicate_object then null; end $$;

do $$ begin
  create policy subscription_records_own
    on public.subscription_records
    for all
    using (auth.uid()::text = user_id)
    with check (auth.uid()::text = user_id);
exception when duplicate_object then null; end $$;

do $$ begin
  create policy app_preferences_own
    on public.app_preferences
    for all
    using (auth.uid()::text = user_id)
    with check (auth.uid()::text = user_id);
exception when duplicate_object then null; end $$;
