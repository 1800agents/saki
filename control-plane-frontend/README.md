# control-plane-frontend

Next.js frontend application.

## Getting started

1. Install dependencies:

```bash
npm install
```

2. Start the dev server:

```bash
npm run dev
```

The app runs at http://localhost:3000.

## Scripts

- `npm run dev`: run development server
- `npm run build`: build production output
- `npm run start`: start production server
- `npm run lint`: run ESLint

## Docker build and push

Build local image:

```bash
cd control-plane-frontend
docker build -t saki-control-plane-frontend:local .
```

Build and push to a repository:

```bash
cd control-plane-frontend
scripts/build-and-push.sh <repository>
```

Example:

```bash
scripts/build-and-push.sh registry.example.com/saki/control-plane-frontend
```

Optional script env vars:

- `DOCKER_PLATFORM=linux/amd64` to use `docker buildx build --push`
- `PUSH_LATEST=1` to also push `:latest`
