# Releasing Revenant (for maintainers)

Revenant is a **Go binary**, but **users never need Go**. They install via:

1. **GitHub Releases** — pre-built binaries (Linux/Mac/Windows)
2. **GitHub Action** — `uses: 277pawan/revenant-action@v1` in any repo
3. **Manual download** — from the Releases tab

This doc explains how publishing works.

---

## The two repos

| Repo | What it is | Who uses it |
|------|------------|-------------|
| **revenant-cli** | Go source + CLI | Contributors, releases |
| **revenant-action** | Thin GitHub Action wrapper | Every CI user (any language) |

The action repo has **no database models, no demo tables** — only a script that downloads the binary from `revenant-cli` Releases and runs it.

---

## How GitHub Releases work

1. You finish a version of the CLI locally.
2. You create a git tag: `git tag v0.1.1`
3. You push the tag: `git push origin v0.1.1`
4. GitHub sees the tag → runs `.github/workflows/release.yml`
5. **GoReleaser** builds `revenant` for each OS/arch and uploads `.tar.gz` / `.zip` files to:
   **GitHub → your repo → Releases → v0.1.1**

Users download `revenant_0.1.0_linux_amd64.tar.gz` (or the Action does it for them).

You never upload binaries by hand.

---

## First release checklist

```bash
# 1. Commit everything on main
git add -A
git commit -m "Prepare v0.1.1 release"
git push origin main

# 2. Tag and push (this triggers the Release workflow)
git tag v0.1.1
git push origin v0.1.1

# 3. Watch Actions tab → "Release" job → should go green

# 4. Open GitHub → Releases → confirm binaries are there

# 5. Push revenant-action repo and tag v1 (see revenant-action/README.md)
```

If Release fails, open the workflow log. Common fixes: tests failing, tag format wrong (must be `v0.1.1` not `0.1.0`).

---

## How the GitHub Action works

In **any** repository (React app, Django API, Go microservice):

```yaml
- uses: 277pawan/revenant-action@v1
  with:
    version: v0.1.1
    config: revenant.yaml
  env:
    DATABASE_URL: ${{ secrets.DATABASE_URL }}
```

What happens:

1. Action runs on GitHub's Ubuntu runner.
2. `install.sh` downloads the correct binary for that runner from Releases.
3. Action runs `revenant verify --config revenant.yaml`.
4. If checks fail, the workflow step fails (CI goes red).

No Go. No npm. No connection to your application code.

---

## Versioning rules

- **CLI releases**: `v0.1.1`, `v0.2.0` (semver tags on `revenant-cli`)
- **Action releases**: `v1`, `v1.0.0` — points at a default CLI version in `action.yml`; users can override with `version: v0.1.1`

When you ship CLI `v0.2.0`, users pin `version: v0.2.0` in their workflow. You can bump the action's default later.

---

## Workflows in this repo

| Workflow | Purpose |
|----------|---------|
| `ci.yml` | Local Postgres smoke test on every push/PR |
| `release.yml` | Build + publish binaries on tag push |
| `free-restore-test.yml` | Dogfood AWS restore (your secrets) |

---

## After v0.1.1: switch dogfood AWS workflow to binary

Replace `go build` in `free-restore-test.yml` with:

```yaml
- uses: 277pawan/revenant-action@v1
  with:
    version: v0.1.1
    config: revenant-aws-freetier.yaml
```

Same binary your users get.
