# Revenant — Phase 1: Local Restore Verification Engine

Prove a PostgreSQL database is actually usable: connect, run checks from
`revenant.yaml`, print PASS/FAIL, write `report.json`.

No AWS. No dashboard. No Docker. Local Postgres only.

## What you need

| Tool | Why | You already have? |
|------|-----|-------------------|
| **Go 1.22+** | CLI language | `go version` — this machine has 1.22.2 (pgx is pinned to v5.6.0 so we do not need Go 1.25) |
| **PostgreSQL** | live database to check | already installed |
| **Cobra** | `revenant verify` | pulled via `go.mod` |
| **pgx/v5** | Postgres driver | pulled via `go.mod` |
| **yaml.v3** | parse `revenant.yaml` | pulled via `go.mod` |
| **godotenv** | load `.env` locally | pulled via `go.mod` |
| **slog / encoding/json** | logs + report | Go stdlib |

Optional (only if you want the Cobra generator later):

```bash
go install github.com/spf13/cobra-cli@latest
```

## One-time: demo database

Postgres is installed. You still need a database + two tables. If `psql`
asks for a password, create one for your OS user (Ubuntu peer auth):

```bash
sudo -u postgres psql
```

```sql
CREATE DATABASE revenant_demo;

-- If your Linux user cannot connect, give that role a password:
-- CREATE USER your_linux_user WITH PASSWORD 'choose-a-password';
-- GRANT ALL PRIVILEGES ON DATABASE revenant_demo TO your_linux_user;
```

Then:

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

Or skip `.env` and export the same variable in the shell.

## Build and run

```bash
cd /home/pawan-bisht/Documents/revenant/revenant-cli
go mod tidy
go build -o revenant .
./revenant verify
```

Expected terminal:

```
✓ customers table exists
✓ orders table exists
✓ orders row count 1 >= 1

Restore Validation: PASS
Wrote report.json
```

`report.json` looks like:

```json
{
  "plan": "local-demo",
  "status": "PASS",
  "duration": "12ms",
  "checks": [
    { "name": "schema:customers", "status": "PASS", "message": "customers table exists" }
  ]
}
```

Flags:

```
./revenant verify --config revenant.yaml
./revenant verify --plan local-demo
./revenant verify --output report.json
```

Exit code is `1` if any check fails (ready for GitHub Actions later).

## Layout (keep this small)

```
revenant-cli/
├── main.go                 # calls cmd.Execute()
├── cmd/
│   ├── root.go             # `revenant`
│   └── verify.go           # `revenant verify`  ← add init/report here later
├── internal/
│   ├── config/loader.go    # yaml → structs, expands ${DATABASE_URL}
│   ├── database/postgres.go
│   ├── checks/
│   │   ├── runner.go       # switch on check type
│   │   ├── schema.go       # DONE
│   │   ├── rowcount.go     # DONE
│   │   ├── ident.go        # table-name safety
│   │   └── foreignkey.go   # STUB — your next check
│   └── report/json.go
├── examples/revenant.yaml
├── revenant.yaml
├── go.mod
└── README.md
```

## Where to continue (in order)

1. **`internal/checks/foreignkey.go`** — catalog query is sketched in comments.
   Then uncomment the check in `revenant.yaml`.
2. **Golden query** — add fields on `config.Check`, a `case` in `runner.go`,
   a new `golden.go`. Same pattern as `rowcount.go`.
3. **`revenant init`** — new file `cmd/init.go`, introspect `information_schema`,
   write a starter yaml. Not Phase 1.
4. **AWS restore** — new package under `internal/recovery/`. `verify.go` should
   still call `checks.RunAll` unchanged.

Do not add AWS SDK, Prometheus, a dashboard, or Docker for this milestone.
