# Saki Control Plane API Contract (v1)

**Audience:** Saki Tool (CLI/Agent) + Control Plane engineers
**Scope:** Single-tenant, internal-only, long-lived apps, 1 replica, shared Postgres with schema isolation.

---

## 1. Core Concepts

### 1.1 Session

A **session** is represented by a UUID token, passed via URL query param. Sessions:

- **do not expire automatically**
- **can be revoked** (admin/control plane action; API for this is out of scope v1)

### 1.2 App

An **app** is uniquely identified by `(owner, name)`.

- `owner` is derived from the session (even in single-tenant mode, we treat it as a namespace boundary)
- creating an app with the same name **overwrites / redeploys** it (v1 behavior)

### 1.3 Deployment

v1 uses a **stable deployment_id per app**:

- redeploys **do not change** `deployment_id`

---

## 2. Authentication

### 2.1 Control Plane URL format (Option B)

The Saki tool is given a control plane URL that contains the session token:

```text
https://<control-plane-domain>?token=<session_uuid>
```

### 2.2 Token forwarding requirement

All API requests from the tool to the control plane **must include the same token**:

```text
?token=<session_uuid>
```

🔎 Note: this is intentionally simple for the hackathon demo; can migrate to header/JWT later.

---

## 3. Registry Push Handshake

The control plane owns the internal registry and issues a **short-lived push token**.

### 3.1 Prepare push

#### `POST /apps/prepare?token=<session_uuid>`

**Purpose:** returns a repository location and short-lived token for pushing the image.

##### Request

```json
{
  "name": "my-app",
  "git_commit": "b7c1a2f5d8e9c0a1b2c3d4e5f6a7b8c9d0e1f2a3"
}
```

##### Response

```json
{
  "repository": "registry.internal/<owner>/my-app",
  "push_token": "short-lived-token",
  "expires_at": "2026-02-28T12:00:00Z",
  "required_tag": "b7c1a2f"
}
```

### 3.2 Image tagging rule (required)

The tool **must** tag the pushed image with:

- `required_tag` from `/apps/prepare`, which is the **truncated git commit hash**.

Example:

```text
registry.internal/<owner>/my-app:b7c1a2f
```

---

## 4. Deploy / Redeploy

### 4.1 Create or Update App (Overwrite-by-name)

#### `POST /apps?token=<session_uuid>`

**Behavior:**

- If `(owner, name)` does not exist → create new app + deploy it.
- If `(owner, name)` exists → **overwrite** (update image/description) + redeploy.
- Redeploy is triggered when `image` tag changes.

##### Request

```json
{
  "name": "my-app",
  "description": "Test agent deployment",
  "image": "registry.internal/<owner>/my-app:b7c1a2f"
}
```

##### Response

```json
{
  "app_id": "app_abc123",
  "deployment_id": "dep_xyz456",
  "url": "https://my-app.<domain>",
  "status": "deploying"
}
```

### 4.2 Validation rules

- `name`: DNS-safe slug
  - lowercase letters, digits, `-`
  - must start/end with alphanumeric
  - max length: 63

- `description`: max 300 chars
- `image`: must match the caller’s namespace (control plane enforces)
- tool guarantees build is complete before calling `POST /apps`

---

## 5. Runtime Configuration Contract

### 5.1 Template guarantees (tool responsibility)

Tool writes only:

- `NAME`
- `DESCRIPTION`

### 5.2 Control plane injection (control plane responsibility)

Control plane injects:

- `PORT` (the platform may set it, but app should listen on `3000` by convention)
- `DATABASE_URL` (points to shared Postgres; app is isolated by schema)

### 5.3 No custom env vars (v1)

The tool/user **cannot** define additional env vars in v1.

---

## 6. App Status & Lifecycle APIs

### 6.1 Status enum

```text
pending
deploying
healthy
failed
stopped
deleting
```

### 6.2 Get an app

#### `GET /apps/{app_id}?token=<session_uuid>`

```json
{
  "app_id": "app_abc123",
  "deployment_id": "dep_xyz456",
  "name": "my-app",
  "description": "Test agent deployment",
  "url": "https://my-app.<domain>",
  "status": "healthy",
  "created_at": "2026-02-28T11:40:00Z",
  "updated_at": "2026-02-28T11:45:00Z",
  "image": "registry.internal/<owner>/my-app:b7c1a2f"
}
```

### 6.3 List apps (user-scoped by default)

#### `GET /apps?token=<session_uuid>`

```json
{
  "data": [
    {
      "app_id": "app_abc123",
      "name": "my-app",
      "status": "healthy",
      "url": "https://my-app.<domain>"
    }
  ]
}
```

### 6.4 List apps (admin can view all)

#### `GET /apps?token=<session_uuid>&all=true`

🔐 Control plane enforces admin access.

Same response shape as 6.3.

---

## 7. Stop vs Delete

### 7.1 Stop an app (reversible)

#### `POST /apps/{app_id}/stop?token=<session_uuid>`

**Behavior:** scales app to 0 / disables routing; keeps resources for restart.

Response:

```json
{
  "app_id": "app_abc123",
  "status": "stopped"
}
```

### 7.2 Start / Resume an app

🔶 (Optional endpoint but recommended to complete “stop is reversible”)

#### `POST /apps/{app_id}/start?token=<session_uuid>`

Response:

```json
{
  "app_id": "app_abc123",
  "status": "deploying"
}
```

### 7.3 Delete an app (irreversible)

#### `DELETE /apps/{app_id}?token=<session_uuid>`

**Behavior:** tears down deployment + schema + routing.

Response:

```json
{
  "app_id": "app_abc123",
  "status": "deleting"
}
```

---

## 8. Logs (Paginated)

### 8.1 Get logs

#### `GET /apps/{app_id}/logs?token=<session_uuid>&cursor=<cursor>&limit=<n>`

**Query params**

- `cursor` (optional): opaque cursor for pagination
- `limit` (optional, default 200, max 1000)

Response:

```json
{
  "data": [
    {
      "timestamp": "2026-02-28T11:42:00Z",
      "stream": "stdout",
      "message": "Server listening on 3000"
    }
  ],
  "next_cursor": "opaque-cursor-or-null"
}
```

---

## 9. Error Model

All errors return:

```json
{
  "error": {
    "code": "invalid_name",
    "message": "App name must be DNS compliant",
    "details": {}
  }
}
```

HTTP status code guidance:

| HTTP | When                                  |
| ---- | ------------------------------------- |
| 400  | validation error                      |
| 401  | invalid / revoked session token       |
| 403  | forbidden (e.g., admin-only all=true) |
| 404  | app not found                         |
| 500  | internal error                        |

### 9.1 Overwrite semantics (no idempotency key)

v1 behavior:

- If two apps of the same name are created, **overwrite** the existing app.
- Control plane should treat `POST /apps` as **create-or-update**.

---

## 10. Non-Goals (Explicitly Out of Scope v1)

- Multi-tenant org/team RBAC
- Custom env vars or secrets
- Autoscaling / replica count configuration
- Resource limits configuration
- Idempotency keys
- Streaming logs
- Session issuance/revocation API

---

# Appendix A: Minimal Tool Flow (Happy Path)

1. Tool receives: `SAKI_CONTROL_PLANE_URL=https://.../?token=<uuid>`
2. Tool runs:
   - `POST /apps/prepare` with `{ name, git_commit }`

3. Tool builds Docker image, tags with `required_tag`, pushes using `push_token`
4. Tool calls:
   - `POST /apps` with `{ name, description, image }`

5. Tool prints:
   - `url`, `status`, `app_id`

---

# Appendix B: Ownership Boundary (for parallel work)

### Tool owns

- Git commit discovery
- Template clone
- Writing `.env` with `NAME` and `DESCRIPTION`
- Docker build
- Calling `/apps/prepare`
- Tagging image with truncated commit
- Pushing image with short-lived token
- Calling `POST /apps` and returning JSON output

### Control Plane owns

- Session validation / revocation checks
- Namespace/owner derivation
- Internal registry + issuing push token
- Shared Postgres schema provisioning + `DATABASE_URL`
- Kubernetes deployment + service + ingress
- DNS / URL routing (`app.domain`)
- App lifecycle state machine
- Logs pagination endpoint
- Admin scoping behavior (`all=true`)
