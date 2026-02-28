# Saki

**Internal Vibe Coding App Hosting Platform**

---

## Vision

Saki enables any employee — or coding agent — to deploy a **server-rendered internal application** with a live URL and managed Postgres database in minutes, entirely on company-owned infrastructure.

The key insight:

> Agents shouldn’t stop at generating code. They should be able to ship.

---

## Core Principles

- Convention over configuration
- Zero infrastructure knowledge required
- No custom deployment manifests
- Agent-first integration via tool call
- Safe-by-default governance and isolation

---

# System Architecture

---

## 1. Saki Control Plane

The Saki Control Plane is the central authority responsible for application lifecycle management.

### Responsibilities

- Register and track applications
- Provision per-app Postgres database/schema
- Inject fixed `DATABASE_URL`
- Enforce fixed `PORT=3000`
- Deploy container images to the cluster
- Generate internal URLs
- Enforce TTL and resource quotas
- Provide TL dashboard visibility
- Allow stop/delete controls

---

### Mandatory Requirement: Internal Docker Registry

The Saki Control Plane **must provision and manage an internal Docker registry**.

This registry:

- Runs inside company infrastructure
- Is not publicly accessible
- Is the only allowed image source for deployments
- Stores images built by the Saki tool
- Enforces namespace isolation per user/app

All images built by agents must be pushed to this internal registry before deployment.

The control plane owns:

- Cluster connectivity
- Base Postgres instance
- URL routing scheme
- Internal Docker registry
- Resource and policy enforcement

---

## 2. Saki App Template (Fixed Structure)

All applications must be created from a standard template.

There is **no `saki.yaml` or additional abstraction layer**.

### Required Files

#### `agents.md`

Defines:

- Tool call contract
- Environment expectations
- Deployment flow
- Iteration loop guidance for agents

---

#### `Dockerfile`

Builds a production container image for a **server-rendered application**.

---

#### Server-Rendered Application

- Single runtime (no frontend/backend split)
- Listens on fixed port `3000`
- Reads environment variables from `.env`

---

#### `.env` (Fixed Format)

The `.env` file contains only:

```

NAME=<app_name>
DESCRIPTION=<short_description>

```

Not configurable:

- `PORT`
- `DATABASE_URL`

At deployment time, Saki injects:

```

PORT=3000
DATABASE_URL=<generated_per_app_url>

```

Applications must not override these values.

---

## 3. Saki Tool (Agent Tool Call)

Saki integrates with agents via a **tool call**, not a CLI.

The agent receives:

```

SAKI_CONTROL_PLANE_URL_WITH_UUID

```

Example:

```

[https://saki.internal/api?token=](https://saki.internal/api?token=)<uuid>

```

The UUID represents the authenticated user identity.

---

### Tool Name

`saki_deploy_app`

---

### Tool Input

```json
{
  "saki_control_plane_url": "string_with_uuid_token",
  "name": "string",
  "description": "string"
}
```

---

### Tool Responsibilities

When invoked, the tool must:

1. Clone the Saki App Template
2. Write `.env` with:
   - NAME
   - DESCRIPTION

3. Build Docker image from `Dockerfile`
4. Push image to the internal Docker registry managed by Saki
5. Call Saki Control Plane API to deploy the image
6. Return deployment metadata

The agent never communicates directly with Kubernetes.

---

### Tool Output

```json
{
  "app_id": "uuid",
  "deployment_id": "uuid",
  "image": "registry.internal/user/app:tag",
  "url": "https://app-name--abc123.saki.internal",
  "status": "deploying"
}
```

---

# Control Plane API (Contract-Level)

---

## Deploy Image

`POST /apps`

```json
{
  "name": "string",
  "description": "string",
  "image": "registry.internal/user/app:tag"
}
```

Response:

```json
{
  "app_id": "uuid",
  "deployment_id": "uuid",
  "url": "string",
  "status": "deploying"
}
```

---

## Get App

`GET /apps/{app_id}`

```json
{
  "status": "building | deploying | healthy | failed | stopped",
  "url": "string",
  "owner": "uuid",
  "ttl_expiry": "timestamp"
}
```

---

## List Apps

`GET /apps`

- Default: returns user-owned apps
- Admin mode: returns all apps

---

## Stop App

`POST /apps/{app_id}/stop`

---

## Delete App

`DELETE /apps/{app_id}`

---

# Guardrails

- Fixed port: `3000`
- Database URL always injected by platform
- Internal Docker registry only
- No external image pulls
- Per-app isolation
- TTL auto-expiry
- TL dashboard visibility
- Global kill switch

---

# Demo Definition of Done

- Agent calls `saki_deploy_app`
- Template cloned
- `.env` generated
- Docker image built
- Image pushed to internal registry
- App deployed successfully
- Live URL returned
- TL dashboard shows deployment
- TL can stop/delete app
- App expires via TTL

---

# Non-Goals

- No runtime customization
- No arbitrary Kubernetes manifests
- No multi-service apps
- No production CI/CD
- No public container registry usage

---

# Positioning

Saki is:

> Internal Heroku
>
> - Governance
> - Internal Docker Registry
> - Agent-native deployment

It removes the final barrier between:

“I built something”
and
“Here’s the link.”

```

```
