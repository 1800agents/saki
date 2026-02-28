# control-plane

Central authority for Saki application lifecycle management.

## Responsibilities
- Register and track applications.
- Provision per-app Postgres database/schema.
- Inject `DATABASE_URL` and enforce `PORT=3000`.
- Deploy container images to the cluster.
- Generate internal app URLs.
- Enforce TTL and resource quotas.
- Expose TL dashboard visibility and stop/delete controls.
- Own and manage the internal Docker registry used by Saki.

## API (contract-level)
### Deploy image
`POST /apps`
```json
{
  "name": "string",
  "description": "string",
  "image": "registry.internal/user/app:tag"
}
```

### Get app
`GET /apps/{app_id}`
```json
{
  "status": "building | deploying | healthy | failed | stopped",
  "url": "string",
  "owner": "uuid",
  "ttl_expiry": "timestamp"
}
```

### List apps
`GET /apps`
- Default: user-owned apps
- Admin mode: all apps

### Stop app
`POST /apps/{app_id}/stop`

### Delete app
`DELETE /apps/{app_id}`

## Guardrails
- Only images in the internal registry can be deployed.
- No external image pulls.
- Per-app isolation.
- TTL auto-expiry.
- Global kill switch support.
