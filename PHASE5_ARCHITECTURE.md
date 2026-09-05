# Revenant Phase 5: Paid Control Plane Architecture

**Status**: Pre-implementation (Roadmap)

**Purpose**: Enterprise-grade database restore validation at scale. Orchestrates the free CLI across hundreds of databases, stores audit evidence, and provides compliance-ready dashboards.

---

## Table of Contents

1. [Overview](#overview)
2. [Why Phase 5?](#why-phase-5)
3. [Architecture Components](#architecture-components)
4. [Implementation Plan](#implementation-plan)
5. [Database Schema](#database-schema)
6. [API Specification](#api-specification)
7. [Security Model](#security-model)
8. [Deployment Guide](#deployment-guide)

---

## Overview

Phase 5 transforms Revenant from a single-database CLI tool into a **multi-tenant SaaS control plane**:

```
┌─────────────────────────────────────────────────────────────────┐
│                    REVENANT CLOUD (Paid)                         │
│                                                                   │
│  ┌──────────────────────────────────────────────────────────┐  │
│  │          Web Dashboard (React/Next.js)                  │  │
│  │  • Recovery history & trends                             │  │
│  │  • RTO/RPO metrics                                       │  │
│  │  • Pass/fail status per database                         │  │
│  │  • Audit evidence & compliance reports                   │  │
│  └──────────────────────────────────────────────────────────┘  │
│           ▲                                                       │
│           │ REST API calls                                       │
│           ▼                                                       │
│  ┌──────────────────────────────────────────────────────────┐  │
│  │         API Server + Control Plane (Go)                 │  │
│  │  • Flask/Gin REST API                                    │  │
│  │  • Credential management (encrypted)                     │  │
│  │  • Fleet orchestration                                   │  │
│  │  • Evidence vault (signed reports)                       │  │
│  └──────────────────────────────────────────────────────────┘  │
│           ▲                                                       │
│           │ Job queue                                            │
│           ▼                                                       │
│  ┌────────────────────────────────────────────────────────────┐ │
│  │    Scheduler + Job Queue (Redis/PostgreSQL)              │ │
│  │  • Define frequency (hourly/daily/weekly)                 │ │
│  │  • Track state (pending/running/completed/failed)         │ │
│  │  • Distribute work to runners                             │ │
│  └────────────────────────────────────────────────────────────┘ │
│           ▲           ▲           ▲                              │
│           │           │           │                              │
│    ┌──────┴──┐   ┌────┴───┐   ┌───┴─────┐                       │
│    ▼         ▼   ▼        ▼   ▼         ▼                       │
│  ┌────┐   ┌────┐  ┌────┐   ┌────┐   ┌────┐                     │
│  │Run1│   │Run2│  │Run3│   │Run4│   │Run5│  ... (many runners) │
│  └────┘   └────┘  └────┘   └────┘   └────┘                     │
│    │        │      │        │        │                          │
│    └────────┴──────┴────────┴────────┘                          │
│               │                                                   │
│               ▼ (run FREE CLI against customer DBs)              │
│  Revenant CLI: find snapshot → restore → validate → destroy     │
│                                                                   │
│    Customer A AWS    Customer B GCP    Customer C Azure          │
│    ────────────────  ──────────────    ────────────────          │
│    Prod DB ─────→    Prod DB ─────→    Prod DB ─────→          │
│    Snapshot          Snapshot          Snapshot                 │
│    Sandbox           Sandbox           Sandbox                  │
│    (validates)       (validates)       (validates)              │
│                                                                   │
└─────────────────────────────────────────────────────────────────┘
           │
           │ Results (JSON + signed certificate)
           ▼
    Control Plane DB (Postgres)
    • Execution history
    • Test results
    • Audit trail
    • Evidence vault
```

---

## Why Phase 5?

| Scenario | Free CLI | Phase 5 Paid |
|----------|----------|--------------|
| Test 1 DB quarterly | ✅ Perfect | Overkill |
| Test 5 DBs monthly | ✅ Works | Better with history |
| Test 50 DBs weekly | ❌ Manual nightmare | ✅ Automatic + dashboard |
| Test 200 DBs daily | ❌ Impossible | ✅ Orchestrated at scale |
| Compliance audit | ❌ Spreadsheets | ✅ Signed evidence vault |
| Multi-team access | ❌ CLI only | ✅ Web UI + RBAC |
| Multi-cloud (AWS/GCP/Azure) | ⚠️ Run CLI 3x | ✅ Single dashboard |

**Key wins of Phase 5:**
- 🤖 **Automatic scheduling** — No manual CLI invocation
- 📊 **Dashboard** — See trends, not just pass/fail
- 🔐 **Compliance-ready** — Signed, tamper-proof reports
- 👥 **Multi-tenant** — One platform, many customers/teams
- 🌍 **Multi-cloud** — AWS, GCP, Azure in one view
- 🔑 **Credential management** — Secure storage of DB credentials
- 📈 **Audit trail** — Who ran what, when, with what result

---

## Architecture Components

### 1. Control-Plane Database (PostgreSQL)

Stores metadata, results, and audit trail.

**Tables to create:**
- `organizations` — Multi-tenancy (SaaS)
- `databases` — Managed databases (e.g., prod-db-1, staging-db-2)
- `credentials` — Encrypted cloud provider credentials (AWS/GCP/Azure)
- `schedules` — When/how often to test each database
- `jobs` — Execution history (pending/running/completed/failed)
- `results` — Test results for each job (pass/fail, RTO/RPO)
- `evidence` — Signed report artifacts (for compliance audit)
- `users` — Team members and their roles (RBAC)
- `audit_log` — Who did what, when

See [Database Schema](#database-schema) section below.

### 2. API Server (Go + Gin/Echo)

Serves requests from:
- Dashboard frontend (React queries)
- CLI runners (report results)
- External systems (webhooks, Datadog, Splunk)

**Example endpoints:**
- `GET /api/v1/organizations/{id}` — Get org metadata
- `GET /api/v1/databases/{id}` — Get database config
- `GET /api/v1/jobs?database_id=...` — List job history
- `GET /api/v1/results/{job_id}` — Fetch test results
- `POST /api/v1/jobs` — Trigger manual run
- `POST /api/v1/credentials` — Store encrypted cloud credentials
- `PUT /api/v1/schedules/{id}` — Update test frequency
- `GET /api/v1/reports/{job_id}/signed` — Download signed evidence

### 3. Scheduler + Job Queue

Manages *when* and *where* to run tests.

**Technology options:**
- **Redis + worker pool** — Simple, fast, good for <1000 jobs/day
- **PostgreSQL + polling** — No external dependencies, good for stable loads
- **Temporal/Airflow** — Enterprise-grade, complex DAG scheduling
- **AWS SQS + Lambda** — Serverless, auto-scales (if hosting on AWS)

**Workflow:**
```
Schedule entry: "Test prod-db-1 daily at 2 AM UTC"
    ↓
Scheduler fires job at 2 AM
    ↓
Job inserted into queue (state: "pending")
    ↓
Free runner picks up job
    ↓
Runner updates job state to "running"
    ↓
Runner executes: revenant verify --plan prod-db-1
    ↓
Runner gets exit code + report.json
    ↓
Runner POSTs result to API: POST /api/v1/jobs/{job_id}/complete
    ↓
API updates job state to "completed"
    ↓
Dashboard refreshes and shows result
```

### 4. Runner Agents

Lightweight workers that execute the **same free CLI**.

**What they do:**
- Poll job queue for work
- Download plan config + credentials from API
- Run: `revenant verify --config <plan> [--aws-flags]`
- Collect report.json + metadata
- POST results back to API
- Move to next job

**Where to deploy:**
- On customer's VPN (if they want to run tests from inside their network)
- On Revenant cloud infrastructure (for SaaS)
- On AWS Lambda / GCP Cloud Run (serverless)

**Scaling:**
- 1 runner can handle ~10 tests/hour (depending on DB size)
- For 200 databases tested daily, you'd want 20-30 runners

### 5. Dashboard (React / Next.js)

Web UI for non-technical users to:
- View recovery trends
- See RTO/RPO metrics
- Check pass/fail status
- Download audit evidence
- Manage databases and schedules
- Invite team members

**Example pages:**
- `/dashboard` — Overview (all databases)
- `/databases/{id}` — History + trends for one database
- `/jobs/{id}` — Details of a specific test run
- `/compliance/{id}` — Export signed evidence for audit
- `/settings/databases` — Add/edit/delete databases
- `/settings/schedules` — Configure test frequency
- `/settings/users` — RBAC management

### 6. Evidence Vault

Stores cryptographically signed reports for compliance.

**Why sign reports?**
- Auditors (SOC2, ISO27001) need proof that tests actually ran
- Signatures prove nobody tampered with the report after generation
- Legal requirement in some regulated industries

**Workflow:**
```
1. Runner completes job → generates report.json
2. API signs report with private key:
   - Hash report.json
   - Sign hash with RSA/ECDSA private key
   - Return {report, signature, public_key}
3. Auditor can verify:
   - Hash report.json
   - Verify signature with public_key
   - Confirms report is authentic and unchanged
```

### 7. Credential Manager

Encrypts and stores cloud provider credentials safely.

**What needs encryption:**
- AWS access keys
- GCP service account keys
- Azure credentials
- Postgres master user passwords
- SSH keys for on-prem databases

**Implementation:**
```go
// Example: AES-256-GCM encryption
func EncryptCredential(plaintext []byte, masterKey [32]byte) (encryptedBlob []byte, nonce []byte, error) {
    cipher, _ := aes.NewCipher(masterKey[:])
    gcm, _ := cipher.NewGCM()
    nonce = make([]byte, gcm.NonceSize())
    io.ReadFull(rand.Reader, nonce)
    return gcm.Seal(nil, nonce, plaintext, nil), nonce, nil
}

// Store in DB:
INSERT INTO credentials (org_id, cloud_provider, encrypted_value, nonce, created_at)
VALUES ($1, $2, $3, $4, NOW())
```

---

## Implementation Plan

### Phase 5.0: Foundation (Weeks 1-4)

**Goal**: Get a runnable control plane with basic CRUD.

```
Week 1-2: Backend Foundation
├── Set up project structure (Go + Gin)
├── Create control-plane PostgreSQL schema
├── Implement credential encryption
├── Write migrations (db/migrations/)
└── Basic API endpoints (CRUD operations)

Week 3: Job Queue
├── Implement job queue (Redis or PostgreSQL-backed)
├── Write scheduler (tick every minute, enqueue due jobs)
├── Create runner agent skeleton
└── Local testing with 1 runner

Week 4: Dashboard Scaffold
├── Set up React/Next.js project
├── Create login + authentication
├── Build basic dashboard layout (navigation, menu)
├── Display organization overview
```

### Phase 5.1: Core Features (Weeks 5-8)

```
Week 5: Job Execution
├── Runner connects to API and polls queue
├── Runner downloads config + credentials
├── Runner executes revenant CLI (subprocess)
├── Runner reports results back to API
└── API stores results in database

Week 6: Dashboard Results
├── Show job history for each database
├── Display pass/fail status
├── Plot RTO/RPO trends (chart library: recharts/plotly)
├── View individual test report details

Week 7: Scheduling UI
├── Create schedule editor (frequency, time, databases)
├── Manual trigger: "Run this test now"
├── Pause/resume schedules
├── Database management (add/edit/delete)

Week 8: RBAC
├── Create users and roles (admin, viewer, executor)
├── Implement permission checks in API
├── Dashboard respects permissions
└── Audit log captures who did what
```

### Phase 5.2: Enterprise Features (Weeks 9-12)

```
Week 9: Evidence Vault
├── Implement report signing (RSA-2048)
├── Store signatures in database
├── Create /api/v1/reports/{id}/signed endpoint
└── Audit-ready export (PDF + signature chain)

Week 10: Multi-Cloud
├── Support GCP Cloud SQL (in addition to RDS)
├── Support Azure Database for PostgreSQL
├── Credential manager for each cloud provider
└── E2E test with mock GCP/Azure

Week 11: Compliance Templates
├── SOC2 report template (tests + dates)
├── ISO27001 checklist export
├── Map test results to compliance requirements
└── Export as PDF/CSV for auditor

Week 12: Advanced Features
├── Webhooks (Slack, PagerDuty, Datadog)
├── Email alerts on failure
├── API integrations (Datadog, New Relic ingestion)
└── Terraform module (IaC for deployment)
```

---

## Database Schema

### Organizations (Multi-tenancy)

```sql
CREATE TABLE organizations (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name VARCHAR(255) NOT NULL UNIQUE,
    plan VARCHAR(50) NOT NULL DEFAULT 'starter', -- starter, pro, enterprise
    status VARCHAR(50) NOT NULL DEFAULT 'active', -- active, suspended, deleted
    created_at TIMESTAMP DEFAULT NOW(),
    updated_at TIMESTAMP DEFAULT NOW(),
    deleted_at TIMESTAMP NULL
);
```

### Databases (Managed DBs under test)

```sql
CREATE TABLE databases (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    organization_id UUID NOT NULL REFERENCES organizations(id),
    name VARCHAR(255) NOT NULL, -- e.g., "prod-db-1"
    description TEXT,
    cloud_provider VARCHAR(50) NOT NULL, -- aws, gcp, azure, on-prem
    instance_identifier VARCHAR(255) NOT NULL, -- RDS instance ID or connection string
    region VARCHAR(100), -- us-east-1, europe-west1, etc
    backup_retention_days INT DEFAULT 7,
    status VARCHAR(50) NOT NULL DEFAULT 'active', -- active, testing, maintenance, deleted
    created_at TIMESTAMP DEFAULT NOW(),
    updated_at TIMESTAMP DEFAULT NOW(),
    UNIQUE(organization_id, name)
);
```

### Credentials (Encrypted cloud provider creds)

```sql
CREATE TABLE credentials (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    organization_id UUID NOT NULL REFERENCES organizations(id),
    credential_type VARCHAR(50) NOT NULL, -- aws_access_key, gcp_service_account, azure_sp, postgres_user_pass
    cloud_provider VARCHAR(50) NOT NULL, -- aws, gcp, azure
    encrypted_value BYTEA NOT NULL, -- AES-256-GCM encrypted
    nonce BYTEA NOT NULL, -- 12-byte GCM nonce
    created_at TIMESTAMP DEFAULT NOW(),
    updated_at TIMESTAMP DEFAULT NOW()
);
```

### Schedules (When to test)

```sql
CREATE TABLE schedules (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    database_id UUID NOT NULL REFERENCES databases(id),
    frequency VARCHAR(50) NOT NULL, -- daily, weekly, monthly, hourly
    time_of_day VARCHAR(10) DEFAULT '02:00', -- HH:MM UTC (for daily/weekly)
    day_of_week VARCHAR(3), -- Mon, Tue, etc (for weekly)
    day_of_month INT, -- 1-31 (for monthly)
    timezone VARCHAR(100) DEFAULT 'UTC',
    max_duration_minutes INT DEFAULT 60, -- timeout for test
    status VARCHAR(50) NOT NULL DEFAULT 'active', -- active, paused, disabled
    created_at TIMESTAMP DEFAULT NOW(),
    updated_at TIMESTAMP DEFAULT NOW()
);
```

### Jobs (Test execution history)

```sql
CREATE TABLE jobs (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    organization_id UUID NOT NULL REFERENCES organizations(id),
    database_id UUID NOT NULL REFERENCES databases(id),
    schedule_id UUID REFERENCES schedules(id), -- NULL if manual trigger
    triggered_by UUID REFERENCES users(id),
    state VARCHAR(50) NOT NULL DEFAULT 'pending', -- pending, running, completed, failed, timeout, cancelled
    assigned_to_runner_id UUID, -- Which runner picked this up
    started_at TIMESTAMP,
    completed_at TIMESTAMP,
    error_message TEXT,
    created_at TIMESTAMP DEFAULT NOW(),
    updated_at TIMESTAMP DEFAULT NOW(),
    INDEX(database_id, created_at),
    INDEX(state, created_at)
);
```

### Results (Test results per job)

```sql
CREATE TABLE results (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    job_id UUID NOT NULL REFERENCES jobs(id),
    check_name VARCHAR(255) NOT NULL,
    check_type VARCHAR(50) NOT NULL, -- schema, row_count, foreign_key, golden_query, freshness
    status VARCHAR(50) NOT NULL, -- PASS, FAIL, SKIP, ERROR
    message TEXT,
    duration_ms INT,
    created_at TIMESTAMP DEFAULT NOW(),
    INDEX(job_id)
);
```

### Evidence (Signed reports)

```sql
CREATE TABLE evidence (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    job_id UUID NOT NULL REFERENCES jobs(id),
    report_json JSONB NOT NULL, -- The actual report
    signature_hex VARCHAR(1024) NOT NULL, -- RSA signature (hex encoded)
    public_key_pem TEXT NOT NULL, -- PEM-encoded public key for verification
    compliance_tags VARCHAR[] DEFAULT '{}', -- soc2, iso27001, hipaa, etc
    signed_at TIMESTAMP DEFAULT NOW(),
    verified_at TIMESTAMP,
    created_at TIMESTAMP DEFAULT NOW()
);
```

### Users (RBAC)

```sql
CREATE TABLE users (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    organization_id UUID NOT NULL REFERENCES organizations(id),
    email VARCHAR(255) NOT NULL,
    password_hash VARCHAR(255) NOT NULL, -- bcrypt
    role VARCHAR(50) NOT NULL DEFAULT 'viewer', -- admin, executor, viewer
    status VARCHAR(50) NOT NULL DEFAULT 'active', -- active, suspended, deleted
    sso_provider VARCHAR(50), -- okta, azure_ad, google_workspace
    sso_id VARCHAR(255), -- External ID from SSO provider
    created_at TIMESTAMP DEFAULT NOW(),
    updated_at TIMESTAMP DEFAULT NOW(),
    last_login_at TIMESTAMP,
    UNIQUE(organization_id, email)
);
```

### Audit Log (Who did what, when)

```sql
CREATE TABLE audit_log (
    id BIGSERIAL PRIMARY KEY,
    organization_id UUID NOT NULL REFERENCES organizations(id),
    user_id UUID REFERENCES users(id), -- NULL if system action
    resource_type VARCHAR(50) NOT NULL, -- job, database, schedule, user, credential
    resource_id UUID NOT NULL,
    action VARCHAR(50) NOT NULL, -- create, update, delete, run, download
    details JSONB, -- What changed, parameters, etc
    ip_address INET,
    created_at TIMESTAMP DEFAULT NOW(),
    INDEX(organization_id, created_at),
    INDEX(resource_type, resource_id)
);
```

---

## API Specification

### Authentication

All endpoints require Bearer token:

```bash
curl -H "Authorization: Bearer ${REVENANT_API_TOKEN}" \
     https://api.revenant.io/api/v1/databases
```

### Organizations

```
GET  /api/v1/organizations/{id}
     Get organization metadata

POST /api/v1/organizations
     Create new organization (admin only)

PUT  /api/v1/organizations/{id}
     Update organization (admin only)
```

### Databases

```
GET  /api/v1/organizations/{org_id}/databases
     List all databases for org

GET  /api/v1/databases/{id}
     Get database config + metadata

POST /api/v1/databases
     Create new database under test
     Body: { organization_id, name, cloud_provider, instance_identifier, region }

PUT  /api/v1/databases/{id}
     Update database config

DELETE /api/v1/databases/{id}
       Soft-delete database
```

### Jobs

```
GET  /api/v1/databases/{db_id}/jobs
     List all test runs for database
     Query params: ?limit=50&offset=0&state=completed

GET  /api/v1/jobs/{id}
     Get job details (state, start time, duration, etc)

POST /api/v1/jobs
     Trigger manual test run
     Body: { database_id }

GET  /api/v1/jobs/{id}/results
     Get test results from this run
     Returns: [{ check_name, status, message, duration_ms }]

POST /api/v1/jobs/{id}/cancel
     Cancel a running or pending job
```

### Results

```
GET  /api/v1/jobs/{job_id}/results
     Fetch all test results for job
     Response: [{ check_name, check_type, status, message, duration_ms }]

GET  /api/v1/results/stats?database_id={id}&period=7d
     Get aggregated stats (pass rate, avg RTO/RPO)
     Returns: { total_runs, pass_count, fail_count, avg_rto_seconds, avg_rpo_hours }
```

### Evidence & Compliance

```
GET  /api/v1/jobs/{job_id}/evidence
     Fetch signed report for compliance audit
     Returns: { report_json, signature, public_key, compliance_tags }

GET  /api/v1/jobs/{job_id}/evidence/verify
     Verify signature is authentic
     Returns: { valid: bool }

POST /api/v1/evidence/export/soc2
     Export SOC2 evidence package (PDF)
     Query params: ?from_date=2024-01-01&to_date=2024-12-31
     Returns: PDF file
```

### Schedules

```
GET  /api/v1/databases/{db_id}/schedules
     List all test schedules for database

POST /api/v1/databases/{db_id}/schedules
     Create new schedule
     Body: { frequency, time_of_day, max_duration_minutes }

PUT  /api/v1/schedules/{id}
     Update schedule

DELETE /api/v1/schedules/{id}
       Delete schedule

POST /api/v1/schedules/{id}/pause
     Pause schedule (don't trigger new jobs)

POST /api/v1/schedules/{id}/resume
     Resume schedule
```

---

## Security Model

### Authentication

- **API Tokens**: Long-lived, org-scoped tokens for CI/CD and runners
- **OAuth2/SSO**: For dashboard users (Okta, Azure AD, Google Workspace)
- **Service Accounts**: For runners and automation

### Encryption

- **Credentials at rest**: AES-256-GCM
- **In transit**: TLS 1.3 only
- **Signing key**: RSA-2048 for evidence (or ECDSA-256 for newer deployments)

### Authorization (RBAC)

| Role | Can Do |
|------|--------|
| **Admin** | Everything: create orgs, manage users, view/trigger/download evidence |
| **Executor** | Trigger jobs, view results, download own evidence |
| **Viewer** | Read-only: view results, download evidence |

### Data Isolation

- Each organization's data is siloed (no cross-org leakage)
- Credentials are never logged (only encrypted blobs)
- Audit log tracks all access

### Compliance

- **SOC2 Type 2**: Annual audit-ready architecture
- **ISO27001**: Encryption, RBAC, audit trail
- **HIPAA**: Optional encryption key management (KMS)
- **PCI DSS**: If testing databases that contain payment data

---

## Deployment Guide

### Option 1: Self-Hosted on AWS

```bash
# 1. Set up RDS PostgreSQL for control-plane DB
aws rds create-db-instance \
  --db-instance-class db.t4g.micro \
  --db-instance-identifier revenant-controlplane \
  --engine postgres \
  --allocated-storage 100

# 2. Deploy API server (ECS + ALB)
#    - Docker image: revenant/control-plane:latest
#    - Environment: DATABASE_URL, JWT_SECRET, SIGNING_KEY

# 3. Deploy dashboard (S3 + CloudFront)
#    - Static React build
#    - API URL points to ALB

# 4. Deploy runners (ECS/Lambda or EC2 auto-scaling group)
#    - Poll job queue every 10 seconds
#    - Report results to API
```

### Option 2: Kubernetes

```yaml
# control-plane-api deployment
apiVersion: apps/v1
kind: Deployment
metadata:
  name: revenant-api
spec:
  replicas: 3
  containers:
  - name: api
    image: revenant/control-plane:latest
    ports:
    - containerPort: 8080
    env:
    - name: DATABASE_URL
      valueFrom:
        secretKeyRef:
          name: revenant-secrets
          key: database-url
    - name: JWT_SECRET
      valueFrom:
        secretKeyRef:
          name: revenant-secrets
          key: jwt-secret
    - name: SIGNING_KEY
      valueFrom:
        secretKeyRef:
          name: revenant-secrets
          key: signing-key

---

# runner deployment
apiVersion: batch/v1
kind: Job
metadata:
  name: revenant-runner
spec:
  parallelism: 10  # 10 concurrent runners
  containers:
  - name: runner
    image: revenant/runner:latest
    env:
    - name: API_URL
      value: http://revenant-api:8080
    - name: API_TOKEN
      valueFrom:
        secretKeyRef:
          name: runner-token
          key: token
```

### Option 3: Docker Compose (Local Dev)

```yaml
version: '3.8'

services:
  postgres:
    image: postgres:15
    environment:
      POSTGRES_DB: revenant
      POSTGRES_PASSWORD: dev-password
    volumes:
      - postgres_data:/var/lib/postgresql/data

  api:
    build: ./control-plane-api
    ports:
      - "8080:8080"
    environment:
      DATABASE_URL: postgres://postgres:dev-password@postgres:5432/revenant
      JWT_SECRET: dev-secret
    depends_on:
      - postgres

  dashboard:
    build: ./dashboard
    ports:
      - "3000:3000"
    environment:
      REACT_APP_API_URL: http://localhost:8080

  runner1:
    build: ./runner
    environment:
      API_URL: http://api:8080
      API_TOKEN: dev-token
    depends_on:
      - api

  runner2:
    build: ./runner
    environment:
      API_URL: http://api:8080
      API_TOKEN: dev-token
    depends_on:
      - api

volumes:
  postgres_data:
```

---

## Testing Phase 5

### Manual Testing

```bash
# 1. Create organization
curl -X POST http://localhost:8080/api/v1/organizations \
  -H "Authorization: Bearer ${TOKEN}" \
  -H "Content-Type: application/json" \
  -d '{"name": "Acme Corp", "plan": "starter"}'

# 2. Add database under test
curl -X POST http://localhost:8080/api/v1/databases \
  -H "Authorization: Bearer ${TOKEN}" \
  -d '{
    "organization_id": "...",
    "name": "prod-db-1",
    "cloud_provider": "aws",
    "instance_identifier": "acme-prod",
    "region": "us-east-1"
  }'

# 3. Store encrypted credentials
curl -X POST http://localhost:8080/api/v1/credentials \
  -H "Authorization: Bearer ${TOKEN}" \
  -d '{
    "credential_type": "aws_access_key",
    "cloud_provider": "aws",
    "access_key_id": "AKIA...",
    "secret_access_key": "..."
  }'

# 4. Create schedule
curl -X POST http://localhost:8080/api/v1/schedules \
  -H "Authorization: Bearer ${TOKEN}" \
  -d '{
    "database_id": "...",
    "frequency": "daily",
    "time_of_day": "02:00"
  }'

# 5. Trigger manual run
curl -X POST http://localhost:8080/api/v1/jobs \
  -H "Authorization: Bearer ${TOKEN}" \
  -d '{"database_id": "..."}'

# 6. Check job status
curl -X GET http://localhost:8080/api/v1/jobs/{job_id} \
  -H "Authorization: Bearer ${TOKEN}"

# 7. View results
curl -X GET http://localhost:8080/api/v1/jobs/{job_id}/results \
  -H "Authorization: Bearer ${TOKEN}"

# 8. Download signed evidence
curl -X GET http://localhost:8080/api/v1/jobs/{job_id}/evidence \
  -H "Authorization: Bearer ${TOKEN}" > evidence.json
```

### Integration Tests

```bash
# Test full workflow: create org → database → schedule → trigger job → get results
pytest tests/integration/test_full_workflow.py

# Test credential encryption
pytest tests/unit/test_credential_encryption.py

# Test report signing
pytest tests/unit/test_report_signing.py

# Test RBAC enforcement
pytest tests/integration/test_rbac.py

# Test multi-tenancy isolation
pytest tests/integration/test_multi_tenant.py
```

---

## Success Metrics

When Phase 5 is complete and live:

- ✅ Can add and manage 100+ databases via dashboard (no CLI)
- ✅ Automatic daily testing via web UI (no manual scheduling)
- ✅ RTO/RPO trends visible on dashboard (6-month history)
- ✅ Compliance evidence exportable as PDF (audit-ready)
- ✅ RBAC enforced (viewers can't trigger tests)
- ✅ Multi-cloud support (AWS + GCP + Azure simultaneously)
- ✅ Runners scale horizontally (add more runners = handle more databases)
- ✅ 99.5% uptime (control plane HA)
- ✅ <1s API response time (dashboard snappy)
- ✅ <5% test failure rate (reliability)

---

## Next Steps

1. **Finalize tech stack** (Gin vs Echo, Redis vs PostgreSQL queue)
2. **Design database schema** (review with team)
3. **Create Figma mockups** (dashboard UI/UX)
4. **Write API spec** (OpenAPI/Swagger)
5. **Set up CI/CD pipeline** (GitHub Actions or similar)
6. **Begin Phase 5.0 foundation work** (Week 1-4)
