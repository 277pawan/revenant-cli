# Revenant — Phase 3: Local assertion engine + init

Prove a PostgreSQL database is actually usable: scaffold checks from a live
schema, run them, print PASS/FAIL, write `report.json` and `report.md`.

No AWS. No dashboard. No Docker. Local Postgres only.

## What you need

| Tool | Why | You already have? |
|------|-----|-------------------|
| **Go 1.22+** | CLI language | `go version` — this machine has 1.22.2 (pgx is pinned to v5.6.0 so we do not need Go 1.25) |
| **PostgreSQL** | live database to check | already installed |
| **Cobra** | `revenant verify` / `revenant init` | pulled via `go.mod` |
| **pgx/v5** | Postgres driver | pulled via `go.mod` |
| **yaml.v3** | parse `revenant.yaml` | pulled via `go.mod` |
| **godotenv** | load `.env` locally | pulled via `go.mod` |
| **slog / encoding/json** | logs + report | Go stdlib |

## One-time: demo database

```bash
sudo -u postgres psql
```

```sql
CREATE DATABASE revenant_demo;
```

```bash
psql -d revenant_demo
```

```sql
CREATE TABLE customers (
    id SERIAL PRIMARY KEY,
    name TEXT
);

CREATE TABLE orders (
    id SERIAL PRIMARY KEY,
    customer_id INT REFERENCES customers(id)
);

INSERT INTO customers (name) VALUES ('Pawan');
INSERT INTO orders (customer_id) VALUES (1);
```

## Configure the connection

```bash
cp .env.example .env
```

Edit `.env`:

```
DATABASE_URL=postgres://USER:PASSWORD@localhost:5432/revenant_demo?sslmode=disable
```

## Build

```bash
go build -o revenant .
```

## Phase 3: scaffold yaml from the database

`revenant init` connects with `DATABASE_URL`, reads `information_schema`,
and writes a starter `revenant.yaml` (schema, row counts, FKs, one golden query).

```bash
./revenant init --output /tmp/revenant.yaml
# or overwrite the repo file:
# ./revenant init --force
```

It will not overwrite an existing file unless you pass `--force`.

Then edit any extra golden queries and run:

```bash
./revenant verify
```

Expected terminal:

```
✓ customers table exists
✓ orders table exists
✓ orders row count 1 >= 1
✓ foreign key orders -> customers intact
✓ golden query returned 1 >= 1

Restore Validation: PASS
Wrote report.json
Wrote report.md
```

Flags:

```
./revenant init --plan local-demo --schema public --output revenant.yaml
./revenant verify --config revenant.yaml --plan local-demo
./revenant verify --output report.json --markdown report.md
```

Exit code is `1` if any check fails (ready for GitHub Actions later).

## Layout (keep this small)

```
revenant-cli/
├── main.go
├── cmd/
│   ├── root.go
│   ├── init.go             # revenant init
│   └── verify.go           # revenant verify
├── internal/
│   ├── config/loader.go
│   ├── database/postgres.go
│   ├── discover/           # catalog scan for init
│   ├── checks/
│   └── report/
├── examples/revenant.yaml
├── revenant.yaml
├── go.mod
└── README.md
```

## Roadmap after Phase 3 (not started)

4. **Freshness / RPO** — `type: freshness` on a timestamp column.
5. **AWS restore** — `internal/recovery/` restores a snapshot, then the same `checks.RunAll`.
6. Paid control plane — scheduler, dashboard, evidence vault.

Do not add AWS SDK, Prometheus, a dashboard, or Docker for this milestone.
