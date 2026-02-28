# tools

Agentic deploy tool used by coding agents (Codex / Claude Code).

## Purpose

Expose `saki_deploy_app`: build from the Saki app template, push to Saki internal registry, then deploy via Saki control plane.

## Prerequisites

- Go 1.24+
- `git` available on `PATH`
- Docker daemon running, with `docker` CLI available
- Access to Saki control plane URL with token query parameter

## Local Development

Install and verify:

```bash
go mod download
make test
```

Run MCP stdio server (used by Codex/Claude Code):

```bash
go run ./cmd/saki-tools-mcp
```

Build binary:

```bash
go build ./cmd/saki-tools-mcp
```

Optional non-MCP process entrypoint:

```bash
go run ./cmd/saki-tools
```

## Environment Variables

### Deploy workflow

- `SAKI_TEMPLATE_REPOSITORY` (optional): fallback template repo if `/apps/prepare` does not return one.
- `SAKI_TEMPLATE_REF` (optional): fallback branch/tag/commit if `/apps/prepare` does not return one.

Default template repository is:

```text
https://github.com/1800agents/saki-app-template
```

### MCP server logging

- `SAKI_TOOLS_MCP_DEBUG` (optional): enable debug-mode flag in logs (`1`/`true`).
- `SAKI_TOOLS_MCP_RAW_LOG` (optional): enable raw MCP transport logging to stderr (`1`/`true`).

### Non-MCP process config (`cmd/saki-tools`)

- `SAKI_TOOLS_ADDR` (optional, default `127.0.0.1:8080`)
- `SAKI_TOOLS_MODE` (optional, default `local`)

## MCP Tool Usage

Tool name: `saki_deploy_app`

Input:

```json
{
  "saki_control_plane_url": "https://<control-plane-host>/api?token=<session-uuid>",
  "name": "my-app",
  "description": "Internal test app"
}
```

Output:

```json
{
  "app_id": "uuid_or_id",
  "deployment_id": "uuid_or_id",
  "image": "registry.internal/user/app:tag",
  "url": "https://app-name--abc123.saki.internal",
  "status": "deploying"
}
```

## Control Plane API Assumptions

This implementation assumes:

- `saki_control_plane_url` must include `token=<session_uuid>` query parameter.
- Tool forwards the same token to control plane API calls.
- `POST /apps/prepare` returns:
  - `repository` (registry repo path)
  - `push_token` (short-lived Docker push credential)
  - `required_tag` (required image tag)
  - optional `template_repository`, `template_ref`
- Tool builds and pushes `repository:required_tag`.
- Tool deploys via `POST /apps` with `{ name, description, image }`.
- `POST /apps` behaves as create-or-update by `(owner, name)`.
- Control plane error envelope is `{ "error": { "code", "message", "details" } }`.

## Deploy Flow

1. Validate input (`name`, `description`, `saki_control_plane_url`).
2. Resolve current git commit (`git rev-parse HEAD`).
3. Call `POST /apps/prepare`.
4. Clone template and write `.env` with only:
   - `NAME=<name>`
   - `DESCRIPTION=<description>`
5. `docker login` using username `token` and `push_token` from prepare response.
6. `docker build` and `docker push`.
7. Call `POST /apps`.
8. Return deployment metadata.

## Guardrails

- Fixed app port: `3000` (platform enforced).
- `DATABASE_URL` is injected by Saki at deploy time.
- No direct Kubernetes calls from this tool.
- Internal registry only (no external image source for deployment).
