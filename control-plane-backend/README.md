# control-plane

Express.js + TypeScript scaffold for the Saki Control Plane API (v1 contract), based on:

- [`spec/SPEC.md`](../spec/SPEC.md)
- [`spec/API.md`](../spec/API.md)

## What is included

- API routes for:
  - `POST /apps/prepare`
  - `POST /apps`
  - `GET /apps`
  - `GET /apps/:appId`
  - `POST /apps/:appId/stop`
  - `POST /apps/:appId/start`
  - `DELETE /apps/:appId`
  - `GET /apps/:appId/logs`
- `?token=<session_uuid>` auth middleware
- Spec-shaped error model
- In-memory app store with overwrite-by-name behavior
- Kubernetes integration scaffold
- Postgres schema provisioner scaffold

## Kubernetes libraries imported

The scaffold imports and initializes [`@kubernetes/client-node`](https://www.npmjs.com/package/@kubernetes/client-node), including:

- `KubeConfig`
- `AppsV1Api`
- `CoreV1Api`
- `NetworkingV1Api`
- `BatchV1Api`
- `KubernetesObjectApi`

These are wired in `src/lib/kubernetes.ts` as no-op placeholders ready for real deploy/stop/delete/log integrations.

## Quick start

```bash
cd control-plane
npm install
cp .env.example .env
npm run dev
# or compile + run production build:
npm run build
npm start
```

Server defaults to `http://0.0.0.0:8080`.

## Environment variables

See `.env.example`:

- `CONTROL_PLANE_HOST`
- `CONTROL_PLANE_PORT`
- `REGISTRY_HOST`
- `APP_BASE_DOMAIN`
- `DEFAULT_APP_TTL_HOURS`
- `ADMIN_TOKENS`
- `POSTGRES_URL`
- `K8S_NAMESPACE`
- `K8S_AUTH_MODE`
- `K8S_KUBECONFIG_PATH`
- `K8S_CONTEXT`
- `K8S_API_SERVER`
- `K8S_TOKEN`
- `K8S_CA_FILE`
- `K8S_CA_DATA`
- `K8S_SKIP_TLS_VERIFY`

## Kubernetes authentication

`src/lib/kubernetes.ts` supports both deployment targets you mentioned.

- Local (outside cluster): default `K8S_AUTH_MODE=auto` tries `K8S_KUBECONFIG_PATH` (if set), then standard kubeconfig resolution (`$KUBECONFIG` or `~/.kube/config`).
- Pod (inside cluster): `K8S_AUTH_MODE=auto` detects service account credentials and uses in-cluster auth.
- Explicit in-cluster: set `K8S_AUTH_MODE=incluster`.
- Explicit kubeconfig: set `K8S_AUTH_MODE=kubeconfig` and optional `K8S_KUBECONFIG_PATH`, `K8S_CONTEXT`.
- Explicit token auth: set `K8S_AUTH_MODE=token` with `K8S_API_SERVER` and `K8S_TOKEN` (optionally `K8S_CA_FILE` or `K8S_CA_DATA`).

On startup, the service logs which auth source was selected.

## Docker build and push

Build local image:

```bash
cd control-plane
npm run docker:build
```

Build and push to a repository:

```bash
cd control-plane
scripts/build-and-push.sh <repository> [tag]
```

Example (replace with your registry URL when supplied):

```bash
scripts/build-and-push.sh registry.example.com/saki/control-plane
```

Optional script env vars:

- `DOCKER_PLATFORM=linux/amd64` to use `docker buildx build --push`
- `PUSH_LATEST=1` to also push `:latest`

## Project structure

```text
control-plane/
  Dockerfile
  .dockerignore
  scripts/
    build-and-push.sh
  src/
    app.ts
    index.ts
    config/env.ts
    middleware/
      auth.ts
      error-handler.ts
    routes/apps.routes.ts
    services/apps.service.ts
    repositories/in-memory-store.ts
    lib/
      kubernetes.ts
      postgres.ts
    types/
      app.ts
      express.d.ts
    utils/validation.ts
```

## Notes

- This is intentionally scaffold-first: Kubernetes and Postgres integrations are structured but not fully provisioned with cluster-specific manifests/migrations yet.
- Apps are in-memory for now, so data resets when the process restarts.
