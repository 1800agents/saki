# tools

Agentic deploy tool used by coding agents (Codex / Claude Code).

## Purpose
Expose a tool call (`saki_deploy_app`) that takes app metadata, builds from the Saki app template, pushes an image to the internal registry, and asks the Saki control plane to deploy it.

## Tool Contract
Tool name: `saki_deploy_app`

Input:
```json
{
  "saki_control_plane_url": "string_with_uuid_token",
  "name": "string",
  "description": "string"
}
```

Output:
```json
{
  "app_id": "uuid",
  "deployment_id": "uuid",
  "image": "registry.internal/user/app:tag",
  "url": "https://app-name--abc123.saki.internal",
  "status": "deploying"
}
```

## Deploy Flow
1. Clone the fixed Saki app template.
2. Generate `.env` with only `NAME` and `DESCRIPTION`.
3. Build Docker image from the template `Dockerfile`.
4. Push image to Saki's internal Docker registry.
5. Call control plane `POST /apps` with `name`, `description`, and `image`.
6. Return deployment metadata to the agent.

## Guardrails
- Fixed app port: `3000` (platform enforced).
- `DATABASE_URL` is injected by Saki at deploy time.
- No direct Kubernetes calls from the agent/tool.
- Internal registry only (no external image source for deployment).
