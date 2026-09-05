# Revenant — Database Restore Validation

Prove your database backups actually work. Automate snapshot recovery validation: test restorability, measure RTO/RPO, then destroy the test copy.

**Current Status**: Phases 1-4 Complete ✅ (Free CLI fully functional)
- ✅ Local schema validation
- ✅ Assertion engine (5 check types)
- ✅ Direct Postgres connections (local dev)
- ✅ AWS RDS snapshot restoration workflow
- ✅ Orphan cleanup for forgotten sandboxes
- 🚀 Phase 5 Roadmap: Paid control plane with dashboard, scheduler, multi-database orchestration

---

## Quick Start (Local Postgres)

### Prerequisites

| Tool            | Why                                   | Status      |
| --------------- | ------------------------------------- | ----------- |
| **Go 1.22+**    | CLI language                          | `go version` |
| **PostgreSQL**  | Database under test                   | Installed   |
| **AWS CLI**     | (Optional) For AWS testing            | `aws configure` |

### 1. Set up local demo database

```bash
sudo -u postgres psql << 'EOF'
CREATE DATABASE revenant_demo;
EOF

psql -d revenant_demo << 'EOF'
CREATE TABLE customers (
    id SERIAL PRIMARY KEY,
    name TEXT NOT NULL,
    created_at TIMESTAMP DEFAULT NOW()
);

CREATE TABLE orders (
    id SERIAL PRIMARY KEY,
    customer_id INT NOT NULL REFERENCES customers(id),
    amount DECIMAL(10, 2),
    created_at TIMESTAMP DEFAULT NOW()
);

INSERT INTO customers (name) VALUES ('Alice'), ('Bob');
INSERT INTO orders (customer_id, amount) VALUES (1, 100.50), (1, 200.00), (2, 50.25);
EOF
```

### 2. Configure connection

```bash
cp .env.example .env
# Edit .env with your connection string
cat .env
```

Expected:
```
DATABASE_URL=postgres://postgres:PASSWORD@localhost:5432/revenant_demo?sslmode=disable
```

### 3. Build

```bash
go build -o revenant .
```

### 4. Generate config from live database

```bash
./revenant init --plan local-demo --force
```

This creates `revenant.yaml` with scaffolded checks:
- Schema validation
- Row count assertions
- Foreign key integrity
- Golden queries

### 5. Run validation

```bash
./revenant verify
```

Expected output:
```
✓ schema check passed
✓ customers table exists
✓ orders table exists
✓ customers row count 2 >= 2
✓ orders row count 3 >= 3
✓ foreign key orders -> customers intact
✓ golden query returned 3 >= 1

Restore Validation: PASS
Recovery Time (RTO): 245ms
Wrote report.json
Wrote report.md
```

View reports:
```bash
cat report.json  # Machine-readable
cat report.md    # Human-readable
```

---

## AWS RDS Snapshot Testing (Phase 4: Complete)

Revenant now includes full AWS RDS restoration workflow:

### Prerequisites

1. **AWS Account** with RDS Postgres instance
2. **AWS Credentials** configured locally:
   ```bash
   aws configure
   # or set AWS_ACCESS_KEY_ID, AWS_SECRET_ACCESS_KEY, AWS_REGION
   ```
3. **IAM Permissions** (see section below)
4. **Existing snapshots** from your RDS instance

### IAM Policy (Minimal Scoped)

Attach this policy to your CI/CD user or role:

```json
{
  "Version": "2012-10-17",
  "Statement": [
    {
      "Sid": "RevenantRDSRestore",
      "Effect": "Allow",
      "Action": [
        "rds:DescribeDBSnapshots",
        "rds:DescribeDBInstances",
        "rds:CreateDBSnapshot",
        "rds:RestoreDBInstanceFromDBSnapshot",
        "rds:AddTagsToResource",
        "rds:ListTagsForResource",
        "rds:DeleteDBInstance"
      ],
      "Resource": "*"
    }
  ]
}
```

### Configuration (revenant.yaml)

To restore from AWS snapshots, add a `recovery:` section:

```yaml
plan: production-validate

database:
  engine: postgres
  connection: postgres://${SANDBOX_USER}:${SANDBOX_PASSWORD}@${SANDBOX_ENDPOINT}:5432/${SANDBOX_DBNAME}?sslmode=require

recovery:
  engine: aws-rds
  source_identifier: prod-database-instance   # Your RDS instance ID
  region: us-east-1
  sandbox_instance_class: db.t4g.micro        # Cost-effective for testing
  max_sandbox_age: 2h

checks:
  - type: schema
    expect_tables:
      - customers
      - orders
      - invoices

  - type: row_count
    table: customers
    min: 1000

  - type: foreign_key
    table: orders
    references: customers

  - type: golden_query
    query: "SELECT COUNT(*) FROM orders WHERE created_at > NOW() - INTERVAL '7 days'"
    expect_min: 10

  - type: freshness
    table: customers
    column: updated_at
    max_age: 24h
```

### Run AWS Restore Verification

```bash
# Build (if needed)
go build -o revenant .

# Run with AWS restoration
./revenant verify --config revenant-aws-freetier.yaml

# Keep the restored sandbox when checks fail, for debugging with psql
./revenant verify --config revenant-aws-freetier.yaml --keep-sandbox-on-failure

# Expected flow:
# 1. Find latest snapshot of prod-database-instance
# 2. Restore to temporary sandbox (e.g., prod-database-instance-abc123)
# 3. Wait for sandbox to become available (typically 5-15 minutes)
# 4. Extract sandbox endpoint
# 5. Connect and run assertions
# 6. Print pass/fail
# 7. Destroy sandbox (automatic cleanup)
```

### Environment Variables

```bash
# Master user credentials for the restored sandbox
# (Usually same as production, or a read-only replica user)
export SANDBOX_USER=postgres
export SANDBOX_PASSWORD=your-password
export SANDBOX_DBNAME=prod  # Database name in the restored snapshot
```

Or in `.env`:
```
SANDBOX_USER=postgres
SANDBOX_PASSWORD=your-password
SANDBOX_DBNAME=prod
```

`SANDBOX_ENDPOINT` is discovered from the restored RDS instance automatically.
`verify` is read-only: it checks the schema and data already present in the
snapshot; it does not create application tables. Run `revenant migrate` against
the source database before taking the snapshot if the tables do not exist yet.
By default, the temporary sandbox is deleted after the run; use
`--keep-sandbox-on-failure` to inspect a failed restore before deleting it with
`revenant reap` or the AWS console.

### Create A Snapshot From The Checked Source

`verify` does not create the source database, run migrations, or create a
snapshot. To prepare a snapshot from an existing RDS instance, point
`DATABASE_URL` at that source instance and run:

```bash
# Optional demo-only tables: this creates customers and orders.
./revenant migrate

# Check the source, then create and wait for a manual RDS snapshot.
./revenant snapshot --config revenant-aws-freetier.yaml

# Restore the newest available snapshot and validate the restored database.
./revenant verify --config revenant-aws-freetier.yaml
```

The `snapshot` command does not create an RDS instance. Create the RDS source
once through AWS with its VPC, subnet, security group, password, and encryption
settings, then use this command to check it and create snapshots.

### What Revenant Does (AWS Workflow)

1. **Find Latest Snapshot**
   - Queries RDS API for snapshots of your source instance
   - Selects the newest available snapshot
   - Logs snapshot ARN and creation time

2. **Restore Sandbox**
   - Generates unique sandbox ID: `{plan-name}-{random-hex}`
   - Triggers `RestoreDBInstanceFromDBSnapshot`
   - Tags sandbox: `revenant:managed=true`, `revenant:plan={plan-name}`, `revenant:created-at={timestamp}`

3. **Wait for Availability**
   - Polls RDS API every 30 seconds
   - Waits up to 30 minutes for `available` status
   - Logs progress

4. **Extract Endpoint**
   - Gets sandbox endpoint (hostname:port)
   - Updates connection string with actual endpoint

5. **Run Assertions**
   - Connects to sandbox
   - Executes all checks from revenant.yaml
   - Collects results

6. **Destroy Sandbox**
   - Issues `DeleteDBInstance` (skips final snapshot)
   - Handles cleanup failures gracefully
   - Logs cleanup completion

### Testing Locally (Without Real AWS)

If you want to test the CLI locally without provisioning AWS resources:

```bash
# Use local Postgres directly (no recovery section)
./revenant verify --config revenant.yaml

# This skips the AWS restoration workflow and tests the local database instead
```

### Temporary Demo Tables

The `migrate` command in `internal/demo` is a temporary convenience example.
It uses GORM models, `AutoMigrate`, a `Customer` to `Order` foreign key, and
repeatable seed data. Remove `internal/demo` and `cmd/migrate.go` before
deploying if these tables are not part of the product.

```bash
export DATABASE_URL='postgres://postgres:YOUR_PASSWORD@database-1.cxas2mi0obxk.eu-west-2.rds.amazonaws.com:5432/postgres?sslmode=require'

go build -o revenant .
./revenant migrate
```

To inspect PostgreSQL tables directly:

```bash
export RDSHOST="database-1.cxas2mi0obxk.eu-west-2.rds.amazonaws.com"
export PGPASSWORD='your-rds-password'
psql "host=$RDSHOST port=5432 dbname=postgres user=postgres sslmode=verify-full sslrootcert=/absolute/path/global-bundle.pem"
```

For `psql` alone, you can instead keep the password out of the URL with
`export PGPASSWORD='your-rds-password'` and use the equivalent `host=...`
connection string.

Then run:

```sql
\\dt public.*
SELECT table_name
FROM information_schema.tables
WHERE table_schema = 'public' AND table_type = 'BASE TABLE'
ORDER BY table_name;
SELECT * FROM public.customers ORDER BY id;
```

`sslrootcert` must point to the downloaded AWS `global-bundle.pem` file. Use
an absolute path while troubleshooting. The RDS security group must also
allow your current IP on TCP port `5432`.

### Orphan Cleanup (Safety Net)

If a restore verification crashes, the sandbox may linger:

```bash
# Find and destroy orphaned sandboxes older than 2 hours
./revenant reap --max-age 2h --region us-east-1

# Custom max age
./revenant reap --max-age 30m --region us-west-2

# Output
# Scanned 42 RDS instances
# Found 1 orphaned revenant sandbox
# Reaped: production-validate-xyz123
```

---

## Commands Reference

### revenant init
Scans a live database and scaffolds a starter `revenant.yaml`.

```bash
./revenant init [FLAGS]

Flags:
  -o, --output string   Path to write yaml (default "revenant.yaml")
      --plan string     Plan name for yaml (default "local-demo")
      --schema string   Postgres schema to scan (default "public")
      --force           Overwrite existing file
```

### revenant verify
Runs assertions against a live database (or restores + asserts if AWS configured).

```bash
./revenant verify [FLAGS]

Flags:
  -c, --config string     Path to revenant.yaml (default "revenant.yaml")
      --plan string       Optional plan name (must match yaml if set)
  -o, --output string     Path to write report.json (default "report.json")
      --markdown string   Path to write report.md (default "report.md")
```

### revenant reap
Cleans up orphaned sandbox instances in AWS RDS.

```bash
./revenant reap [FLAGS]

Flags:
      --max-age string   Threshold age (default "2h", e.g. "30m", "4h")
      --region string    AWS region (overrides AWS_REGION env var)
```

---

## Check Types

### schema
Verify expected tables exist:
```yaml
- type: schema
  expect_tables:
    - customers
    - orders
```

### row_count
Assert minimum row count:
```yaml
- type: row_count
  table: customers
  min: 100
```

### foreign_key
Verify referential integrity:
```yaml
- type: foreign_key
  table: orders
  references: customers
```

### golden_query
Run a business logic query and assert result count:
```yaml
- type: golden_query
  query: "SELECT id FROM orders WHERE status='completed' AND created_at > NOW() - INTERVAL '7 days'"
  expect_min: 5
```

### freshness
Verify data recency (RPO check):
```yaml
- type: freshness
  table: customers
  column: updated_at
  max_age: 24h  # e.g. 1h, 30m, 7d
```

---

## Reports

### report.json
Machine-readable format for CI/CD integration:

```json
{
  "plan": "production-validate",
  "status": "PASS",
  "duration": "12.345s",
  "checks": [
    {
      "name": "schema check passed",
      "status": "PASS"
    },
    {
      "name": "customers row count 1500 >= 1000",
      "status": "PASS"
    }
  ]
}
```

### report.md
Human-readable summary for team review:

```markdown
# Revenant Restore Validation Report

**Plan**: production-validate  
**Status**: ✅ PASS  
**Duration**: 12.345s

## Checks

- ✅ schema check passed
- ✅ customers row count 1500 >= 1000
- ✅ foreign key orders -> customers intact
- ✅ golden query returned 15 >= 5
- ✅ freshness check: customers.updated_at max age 8h <= 24h

---

Generated: 2026-09-05 14:23:15 UTC
```

---

## GitHub Actions Integration

Example workflow for scheduled restore verification:

```yaml
name: Nightly Restore Validation

on:
  schedule:
    - cron: '0 2 * * *'  # 2 AM UTC daily

jobs:
  validate-restore:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      
      - uses: actions/setup-go@v4
        with:
          go-version: '1.22'
      
      - name: Build Revenant
        run: go build -o revenant .
      
      - name: Run Restore Validation
        env:
          AWS_ACCESS_KEY_ID: ${{ secrets.AWS_ACCESS_KEY_ID }}
          AWS_SECRET_ACCESS_KEY: ${{ secrets.AWS_SECRET_ACCESS_KEY }}
          AWS_REGION: us-east-1
          SANDBOX_USER: ${{ secrets.SANDBOX_USER }}
          SANDBOX_PASSWORD: ${{ secrets.SANDBOX_PASSWORD }}
          SANDBOX_DBNAME: production
        run: |
          ./revenant verify --config revenant-aws.yaml \
                            --output report.json \
                            --markdown report.md
      
      - name: Upload Report
        if: always()
        uses: actions/upload-artifact@v3
        with:
          name: restore-validation-report
          path: |
            report.json
            report.md
      
      - name: Cleanup Orphans
        if: always()
        run: ./revenant reap --max-age 4h --region us-east-1
```

---

## Project Structure

```
revenant-cli/
├── main.go                        # Entry point
├── cmd/
│   ├── root.go                    # CLI root command
│   ├── init.go                    # revenant init
│   ├── verify.go                  # revenant verify (with AWS support)
│   └── reap.go                    # revenant reap
├── internal/
│   ├── config/
│   │   └── loader.go              # YAML parsing + recovery schema
│   ├── database/
│   │   └── postgres.go            # pgx connection wrapper
│   ├── discover/
│   │   ├── schema.go              # Information schema introspection
│   │   └── yaml.go                # YAML scaffolding
│   ├── recovery/
│   │   ├── aws.go                 # AWS RDS SDK (restore, wait, endpoint, cleanup)
│   │   └── reaper.go              # Orphan detection and cleanup
│   ├── checks/
│   │   ├── runner.go              # Dispatcher
│   │   ├── schema.go
│   │   ├── row_count.go
│   │   ├── foreign_key.go
│   │   ├── golden.go
│   │   └── freshness.go
│   └── report/
│       ├── json.go                # report.json generator
│       └── markdown.go            # report.md generator
├── examples/
│   └── revenant.yaml              # Example AWS config
├── go.mod                         # Dependencies
├── revenant.yaml                  # Local demo config
├── .env.example
└── README.md
```

---

## Roadmap

### ✅ Phases 1-4: Free CLI (Complete)
- [x] Schema + rowcount + FK validation
- [x] Golden query checks
- [x] Freshness (RPO) validation
- [x] Direct Postgres connection
- [x] AWS RDS snapshot restoration
- [x] Wait-for-DB polling
- [x] Orphan cleanup (reaper)
- [x] JSON + Markdown reports

### 🚀 Phase 5: Paid Control Plane (In Roadmap)

**Why Phase 5?** Nobody manually runs `revenant verify` for 50 databases. The free CLI is great for 1-5 databases or CI/CD pipelines. For enterprises with fleets of databases, you need:

| Feature | Free CLI | Paid Platform |
|---------|----------|------------------|
| **Cost to user** | $0 | Subscription |
| **How many DBs?** | 1-5 comfortable | 10-1000s |
| **How often?** | Manual or via CI cron | Automatic hourly/daily |
| **History** | This run only | Months/years of trends |
| **Dashboard** | Parse JSON in spreadsheet | Web UI with graphs |
| **Audit proof** | Report on disk | Signed, tamper-proof vault |
| **Multi-cloud?** | CLI runs locally | Orchestrates AWS/GCP/Azure |
| **RBAC/SSO?** | N/A | Yes |
| **Approval gates?** | No | Yes (for regulated orgs) |

**Phase 5 Architecture:**
```
Control Plane (Hosted)
├── Web Dashboard (React/Next.js)
│   └── Visualize trends, pass/fail status, audit evidence
├── API Server (Go)
│   └── Query/store results, manage credentials
├── Scheduler + Job Queue
│   └── When/where to run the FREE CLI on each database
├── Evidence Vault (Postgres)
│   └── Signed reports for compliance (SOC2, ISO27001)
└── Runner Agents
    └── Execute the SAME free CLI logic on customer databases
```

**Components to Build:**
1. Hosted control-plane Postgres database
2. REST API for dashboard queries
3. Scheduler (cron-like job orchestration)
4. Fleet manager (track which DBs, which regions, credentials)
5. Dashboard frontend (React/Next.js)
6. Report signing infrastructure
7. RBAC + SSO (Okta, Azure AD, Google Workspace)
8. Compliance templates (SOC2, ISO27001 export)
9. Multi-cloud credential manager (AWS/GCP/Azure/etc)
10. Audit logging

---

## Contributing

Contributions welcome! Areas for help:
- [ ] Additional database engines (MySQL, PostgreSQL on GCP Cloud SQL, etc)
- [ ] More check types (constraint validation, index presence, etc)
- [ ] Terraform modules for AWS provisioning
- [ ] Docker Compose for local demo
- [ ] Test coverage

---

## License

Apache 2.0 — free to use, modify, redistribute, fork.

---

## Support

- GitHub Issues: Report bugs and request features
- Discussions: Questions about usage and roadmap

