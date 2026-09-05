# AWS Free Tier Setup Guide (Zero Cost for 12 Months)

## Quick Overview

**Cost**: $0/month for first 12 months (AWS free tier)  
**After 12 months**: ~$0.15-$0.50 per validation run (you choose frequency)  
**What you get**: Automatic database restore validation + proof it works

---

## Step 1: AWS Account Setup (5 minutes)

### 1.1 Create AWS Free Tier Account

- Go to: https://aws.amazon.com/free/
- Click "Create a free account"
- Provide email, password, AWS account name
- Add payment method (won't be charged for 12 months)
- Verify your account

### 1.2 Log into AWS Console

- https://console.aws.amazon.com
- Search for "RDS" in the search bar
- Click "RDS" service

---

## Step 2: Create RDS Postgres Instance (10 minutes)

### 2.1 Create Database

1. Left sidebar → Click **Databases**
2. Click **Create database**
3. Choose:
   - **Engine**: PostgreSQL
   - **Version**: 15 (or latest)
   - **Template**: Free tier
   - **DB instance identifier**: `prod-database` (or your name)
   - **Master username**: `postgres`
   - **Master password**: Strong password (save this!)
  - **DB instance class**: `db.t3.micro` (Free Tier eligible and compatible with encrypted snapshots)
   - **Storage**: 20 GB (FREE)
   - **Backup retention**: 7 days (default, free)

4. Click **Create database**
5. Wait 5-10 minutes for creation

### 2.2 Verify It's Running

- Go to **Databases** page
- Should see your database with status "available"

### 2.3 Get Database Endpoint

- Click your database name
- Copy **Endpoint** (looks like: `prod-database.c9akciq.us-east-1.rds.amazonaws.com`)
- Save this!

---

## Step 3: AWS Credentials (5 minutes)

### 3.1 Create IAM User

1. Search for "IAM" in AWS Console
2. Left sidebar → **Users**
3. Click **Create user**
   - Name: `revenant-cli`
   - No AWS console access needed
4. Click **Next**

### 3.2 Add Permissions

1. Click **Attach policies directly**
2. Search for "AmazonRDSFullAccess"
3. Select it
4. Click **Next** → **Create user**

### 3.3 Create Access Keys

1. Click the user `revenant-cli`
2. Tab: **Security credentials**
3. Click **Create access key**
4. Choose **Command line interface (CLI)**
5. Acknowledge warning
6. Click **Create access key**
7. **Copy and save**:
   - Access Key ID: `AKIA...`
   - Secret Access Key: `wJalr...`

---

## Step 4: Configure Revenant (10 minutes)

### 4.1 GitHub Secrets (for CI/CD)

If using GitHub Actions:

1. Go to your GitHub repo
2. Settings → **Secrets and variables** → **Actions**
3. Click **New repository secret**
4. Add these secrets:

| Secret Name             | Value                      |
| ----------------------- | -------------------------- |
| `AWS_ACCESS_KEY_ID`     | `AKIA...` (from Step 3.3)  |
| `AWS_SECRET_ACCESS_KEY` | `wJalr...` (from Step 3.3) |
| `SANDBOX_USER`          | `postgres`                 |
| `SANDBOX_PASSWORD`      | Your RDS master password   |

### 4.2 Update Config File

Edit `revenant-aws-freetier.yaml`:

```yaml
recovery:
  source_identifier: prod-database # ← Your RDS instance ID
  region: us-east-1 # ← Your AWS region
  use_freetier: true # ← Free Tier-compatible class (db.t3.micro)
```

---

## Step 5: Run It! (15-20 minutes)

### 5.1 Local Test (One-Time)

```bash
# Set environment variables
export AWS_SECRET_ACCESS_KEY="wJalr..."
export AWS_ACCESS_KEY_ID="AKIA..."
export AWS_REGION="us-east-1"
export SANDBOX_USER="postgres"
export SANDBOX_PASSWORD="your-password"
export SANDBOX_DBNAME="postgres"

# Build Revenant
go build -o revenant .

# Run restore validation
./revenant verify --config revenant-aws-freetier.yaml
```

The snapshot must already contain the tables and data referenced by `checks`.
`verify` does not run migrations. If this is a new demo database, point
`DATABASE_URL` at the source RDS database, run `./revenant migrate`, and then
create a fresh snapshot before running the restore validation.

You can now automate the source check and snapshot creation:

```bash
export DATABASE_URL="postgres://postgres:PASSWORD@database-1...rds.amazonaws.com:5432/postgres?sslmode=require"
./revenant migrate
./revenant snapshot --config revenant-aws-freetier.yaml
./revenant verify --config revenant-aws-freetier.yaml
```

`snapshot` checks the source database first and creates a manual RDS snapshot
only when all configured checks pass. It does not create the RDS instance
itself; that requires AWS networking and security settings.

Expected output:

```
→ Finding latest snapshot...
→ Found snapshot: arn:aws:rds:us-east-1:123456789:db:prod-database-20240905-003419
→ Restoring to sandbox: aws-free-tier-validation-abc123
→ Waiting for sandbox to become available...
  [polling every 30 seconds...]
→ Sandbox is now available!
→ Extracted endpoint: sandbox-db.c9akciq.us-east-1.rds.amazonaws.com:5432
→ ✓ schema check passed
→ ✓ customers table exists
→ ✓ row count check passed
→ ✓ foreign key integrity check passed

Restore Validation: PASS
Recovery Time (RTO): 12m 34s
Wrote report.json
Wrote report.md

→ Destroying sandbox: aws-free-tier-validation-abc123
→ Sandbox destroyed
```

### 5.2 Automated Test (GitHub Actions)

Push your config to GitHub:

```bash
# Add files
git add revenant-aws-freetier.yaml
git add .github/workflows/free-restore-test.yml
git commit -m "Add free tier restore validation"
git push
```

GitHub Actions will automatically run daily at 2 AM UTC (completely free!).

Check results: GitHub Actions tab → Click latest run → Download `restore-report` artifact

---

## Cost Breakdown

### During Free Tier (12 months)

```
Database instance (db.t3.micro):  $0 when eligible
Storage (20 GB):                  $0
Backup retention (7 days):        $0
Restore from snapshot:            $0
Temporary test instance:          $0
─────────────────────────────────
Monthly cost:                     $0 ✅
```

### After Free Tier (Optional)

If you continue:

```
Database instance (db.t3.micro):    standard RDS pricing after Free Tier
Storage (20 GB):                    $2/month
Each restore test (15 min):         $0.15
─────────────────────────────────
If you run 1 test/week:             ~$12/month
If you run 1 test/day:              ~$15/month
```

**Your choice**: Keep using free tier AWS or pay minimal cost for more frequent tests.

---

## Safety Features (Prevent Accidental Charges)

### 1. Auto-Cleanup (Built-in)

Temporary sandboxes are automatically destroyed after validation.

### 2. Reaper Command (Safety Net)

```bash
# Clean up any orphaned sandboxes older than 2 hours
./revenant reap --max-age 2h --region us-east-1
```

### 3. GitHub Actions Cleanup

The workflow includes automatic cleanup:

```yaml
- name: Cleanup Orphaned Sandboxes
  if: always()
  run: ./revenant reap --max-age 4h
```

### 4. AWS Billing Alerts

Set up in AWS Console:

1. Search "Billing"
2. Click "Billing Preferences"
3. Enable "Receive Billing Alerts"
4. Set budget to $1/month (alerts you if close to charges)

---

## Troubleshooting

### Problem: "no available RDS snapshot found"

**Solution**: Create a snapshot manually:

1. AWS Console → RDS → Databases
2. Click your database → "Actions" → "Create snapshot"
3. Wait for snapshot to complete
4. Try `revenant verify` again

### Problem: "timeout waiting for DB to become available"

**Solution**: Takes 5-15 minutes for restoration. The timeout is 30 min, so be patient.

### Problem: "access denied" error

**Solution**: Check AWS credentials:

```bash
aws sts get-caller-identity
# Should show your account info
```

---

## Next Steps

1. ✅ Create AWS free tier account
2. ✅ Create RDS Postgres instance
3. ✅ Create snapshot
4. ✅ Get AWS credentials
5. ✅ Update `revenant-aws-freetier.yaml`
6. ✅ Run `revenant verify` locally
7. ✅ Push to GitHub
8. ✅ GitHub Actions runs automatically daily
9. ✅ Check reports in GitHub Actions artifacts

---

## Summary

**Cost**: $0 for first 12 months  
**After**: Optional minimal cost (~$0.15 per test)  
**Time**: ~1 hour to setup  
**Benefit**: Proof your backups actually work! ✅
