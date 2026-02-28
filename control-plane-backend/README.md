# control-plane

Express.js + TypeScript backend for the Saki Control Plane API (v1 contract), based on:

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
- Kubernetes-backed app state with overwrite-by-name behavior
- Kubernetes deployment/service/ingress lifecycle integration
- Postgres schema provisioner with per-app `DATABASE_URL`

## Kubernetes libraries imported

The backend imports and initializes [`@kubernetes/client-node`](https://www.npmjs.com/package/@kubernetes/client-node), including:

- `KubeConfig`
- `AppsV1Api`
- `CoreV1Api`
- `NetworkingV1Api`
- `BatchV1Api`
- `KubernetesObjectApi`

These are wired in `src/lib/kubernetes.ts` to create/update/delete app resources and read pod logs.

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
- `APP_INGRESS_CLASS_NAME`
- `DEFAULT_APP_TTL_HOURS`
- `ADMIN_TOKENS`
- `POSTGRES_URL`
- `K8S_NAMESPACE`
- `K8S_KUBECONFIG_PATH`

## Kubernetes authentication

`src/lib/kubernetes.ts` uses a strict two-path setup:

- Inside Kubernetes: service account credentials are auto-detected and in-cluster auth is used (no extra auth env vars).
- Outside Kubernetes: `K8S_KUBECONFIG_PATH` is required and must point to a kubeconfig file.

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
    lib/
      kubernetes.ts
      postgres.ts
    types/
      app.ts
      express.d.ts
    utils/validation.ts
```

## Notes

- App state is sourced from Kubernetes resources (Deployments/Services/Ingresses), not process memory.
