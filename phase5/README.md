# Revenant Cloud (Phase 5)

Paid control plane for fleet-wide PostgreSQL restore validation.

**Free tier (existing):** CLI + GitHub Action in sibling repos.  
**This folder:** SaaS product — React dashboard + Node API.

## Planned structure

```
phase5/
├── README.md
├── FIGMA_DESIGN_PROMPT.md    # Paste into Figma — full UI spec
├── apps/
│   ├── web/                  # React (Vite or Next.js) — ops console
│   └── api/                  # Node.js (Express/Fastify) — REST API
├── packages/
│   └── shared/               # Types, API client, validation schemas (optional)
└── docs/
    └── API.md                # OpenAPI spec (from PHASE5_ARCHITECTURE.md)
```

## Stack (target)

| Layer | Choice |
|-------|--------|
| Frontend | React + TypeScript, TanStack Query, shadcn/ui or plain MUI |
| API | Node.js + TypeScript + Fastify or Express |
| DB | PostgreSQL (control-plane metadata) |
| Queue | PostgreSQL or Redis (TBD) |
| Auth | JWT + API tokens; SSO later |
| Runners | Poll API, execute existing `revenant` CLI binary |

## Design

See [FIGMA_DESIGN_PROMPT.md](./FIGMA_DESIGN_PROMPT.md) for the complete Figma brief (AWS-console style, all pages).

## Architecture reference

Parent repo: [PHASE5_ARCHITECTURE.md](../PHASE5_ARCHITECTURE.md)
