# tools

Agentic deploy tool used by coding agents (Codex / Claude Code).

## Purpose

Expose a tool call (`saki_deploy_app`) that takes app metadata, builds from the Saki app template, pushes an image to the internal registry, and asks the Saki control plane to deploy it.

## User Quickstart

### 1. Get your Saki control plane URL

You need a URL that already contains your session token, for example:

```text
https://saki.internal/api?token=<your-session-uuid>
```

Keep this value ready. You will paste it as `saki_control_plane_url` when calling the tool.

Template source default:
- `https://github.com/1800agents/saki-app-template`

Optional override:
- Set `SAKI_TEMPLATE_REPOSITORY` to use a different template repository.
- Set `SAKI_TEMPLATE_REF` to pin a branch/tag/commit.

### 2. Add this tool server to Codex

In Codex, add this project as a local MCP server:

1. Open Codex MCP/tool settings.
2. Add a local stdio MCP server for this repo.
3. Set the server command to this project’s MCP entrypoint (for example, the `saki-tools-mcp` binary or `go run` command for `cmd/saki-tools-mcp`).
4. Save and refresh tools. You should see `saki_deploy_app`.

### 3. Add this tool server to Claude Code

Add the same MCP server command in Claude Code’s MCP/tool configuration (stdio local server), then refresh tool discovery. You should see `saki_deploy_app`.

### 4. Call the tool

When prompted by Codex/Claude Code (or in a direct tool call), pass:

```json
{
  "saki_control_plane_url": "https://saki.internal/api?token=<your-session-uuid>",
  "name": "my-app",
  "description": "Internal test app"
}
```

The agent should return deployment metadata including `app_id`, `deployment_id`, `image`, `url`, and `status`.

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
