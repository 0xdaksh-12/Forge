# Forge Configuration Examples

This document provides several `.forge.yml` examples for different programming languages and deployment scenarios.

## 1. Go (Standard API)

Standard test, build, and deploy flow for a Go service.

```yaml
name: auth-service
on:
  push: { branches: [main] }

jobs:
  lint:
    image: golangci/golangci-lint:v1.60-alpine
    steps:
      - name: Lint
        run: golangci-lint run ./...

  test:
    image: golang:1.23-alpine
    needs: [lint]
    steps:
      - name: Unit Tests
        run: go test -v -race ./...

  deploy:
    needs: [test]
    deploy:
      type: kubernetes
      manifest: deploy/k8s.yaml
      image_tag: ${{ git.sha }}
```

## 2. Node.js (React/Vite App)

Example for building a frontend and deploying to a static site host or K8s.

```yaml
name: frontend-dashboard
on:
  push: { branches: [master] }

jobs:
  build:
    image: node:20-alpine
    steps:
      - name: Install dependencies
        run: npm ci
      - name: Build production bundle
        run: npm run build
    artifacts:
      - "dist/*"
      - "package.json"
```

## 3. Python (FastAPI)

Example with multi-job parallel testing.

```yaml
name: core-api
jobs:
  test-v310:
    image: python:3.10-slim
    steps:
      - name: Install & Test
        run: |
          pip install -r requirements.txt
          pytest

  test-v311:
    image: python:3.11-slim
    steps:
      - name: Install & Test
        run: |
          pip install -r requirements.txt
          pytest

  notify:
    image: alpine:latest
    needs: [test-v310, test-v311]
    steps:
      - name: Slack Notification
        run: curl -X POST -H 'Content-type: application/json' --data '{"text":"Build Successful!"}' ${{ secrets.SLACK_WEBHOOK }}
```

## 4. Docker Build & Push

How to build a Docker image using a sidecar or Docker-in-Docker (requires the Forge runner to have access to the host Docker socket, which it does by default if configured).

```yaml
name: image-builder
jobs:
  publish:
    image: docker:latest
    steps:
      - name: Login
        run: echo "${{ secrets.REGISTRY_PASS }}" | docker login -u "${{ secrets.REGISTRY_USER }}" --password-stdin
      - name: Build
        run: docker build -t my-reg.io/repo:${{ git.sha }} .
      - name: Push
        run: docker push my-reg.io/repo:${{ git.sha }}
```

## Configuration Reference

| Key                | Description                                          |
| ------------------ | ---------------------------------------------------- |
| `name`             | The unique name for the pipeline.                    |
| `on`               | Trigger conditions (`push`, `pull_request`).         |
| `env`              | Global environment variables.                        |
| `jobs`             | Map of job definitions.                              |
| `jobs.<id>.needs`  | List of job IDs that must finish successfully first. |
| `jobs.<id>.image`  | The Docker image to run the job in.                  |
| `jobs.<id>.steps`  | List of sequential shell commands.                   |
| `jobs.<id>.artifacts`| List of glob patterns for files to save to S3.       |
| `jobs.<id>.deploy` | Kubernetes deployment configuration.                 |

## 5. Multi-Job DAG with Deployment

A complex example showing job dependencies and a K8s deploy.

```yaml
name: core-service
on:
  push: { branches: [main] }

jobs:
  lint:
    image: golangci/golangci-lint:v1.60-alpine
    steps:
      - name: Lint
        run: golangci-lint run ./...

  test:
    image: golang:1.23-alpine
    needs: [lint]
    steps:
      - name: Unit Tests
        run: go test -v ./...

  build:
    image: docker:latest
    needs: [test]
    steps:
      - name: Build & Push
        run: |
          docker build -t my-reg.io/core:${{ git.sha }} .
          docker push my-reg.io/core:${{ git.sha }}

  deploy:
    needs: [build]
    deploy:
      type: kubernetes
      manifest: deploy/k8s.yaml
      image_tag: ${{ git.sha }}
```

## 6. Rust (Cargo Build & Cache)
Example for a Rust project with testing and binary building.

```yaml
name: rust-app
on:
  push: { branches: [main] }

jobs:
  test:
    image: rust:1.80-slim
    steps:
      - name: Unit Tests
        run: cargo test

  build:
    image: rust:1.80-slim
    needs: [test]
    steps:
      - name: Build Release
        run: cargo build --release
    artifacts:
      - "target/release/my-app"
```

## 7. Monorepo (Path Filtering)
Example for a monorepo where you only want to run jobs if specific directories change. Note: Forge currently triggers on all pushes, but you can use shell logic for filtering.

```yaml
name: monorepo-service
on:
  push: { branches: [main] }

jobs:
  service-a:
    image: node:20-alpine
    steps:
      - name: Check Changes
        run: |
          if git diff --name-only ${{ git.before }} ${{ git.sha }} | grep "^services/a/"; then
            cd services/a && npm install && npm test
          else
            echo "No changes in Service A, skipping."
          fi

## 8. Full Kubernetes Deployment

Example showing how to use the built-in Kubernetes deployer with a manifest file.

### `.forge.yml`
```yaml
name: k8s-deploy-demo
jobs:
  deploy:
    deploy:
      type: kubernetes
      manifest: deploy/deployment.yaml
      image_tag: ${{ git.sha }}
```

### `deploy/deployment.yaml`
```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: my-app
spec:
  replicas: 3
  selector:
    matchLabels:
      app: my-app
  template:
    metadata:
      labels:
        app: my-app
    spec:
      containers:
      - name: my-app
        image: my-registry.io/app:${{ git.sha }}
        ports:
        - containerPort: 8080
```
```
