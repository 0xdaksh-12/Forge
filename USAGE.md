## 0. Prerequisites

Before you begin, ensure you have the following installed:

- **Docker**: Required to run CI jobs and Forge itself via compose.
- **Kubernetes (Optional)**: Required for deployment steps. [Kind](https://kind.sigs.k8s.io/) is recommended for local development.
- **Go 1.23+**: Required if building/developing the backend manually.
- **Node.js 22+ & pnpm**: Required for frontend development.
- **Make**: Used for build automation.
- **Docker Socket Access**: Ensure your user has permissions to access `/var/run/docker.sock`.

---

## 1. Getting Started

### Start the Server

The easiest way to run Forge is via Docker Compose:

```bash
docker compose up -d
```

Forge will be available at `http://localhost:8080`.

### Authentication

Forge uses a static API token for security. By default, this is `forge-secret`.

#### Setting the Token

You can customize this token by setting the `FORGE_API_TOKEN` environment variable:

- **Bash**: `export FORGE_API_TOKEN=your-secret`
- **Docker Compose**: Add `- FORGE_API_TOKEN=your-secret` under the `environment` section.

#### Using the API

When using the UI, this token is automatically handled. If you are accessing the API via `curl` or a custom script, include it in the header:

```bash
curl -H "X-Forge-Token: your-secret" http://localhost:8080/api/v1/builds
```

---

## 2. Registering a Pipeline

1.  Open the Forge Dashboard.
2.  Navigate to **Pipelines** in the sidebar.
3.  Click **+ New Pipeline**.
4.  Fill in the details:
    - **Name**: A friendly name (e.g., `my-api`).
    - **Clone URL**: The SSH or HTTPS Git URL (e.g., `https://github.com/user/repo.git`).
    - **GitHub Repo**: The `owner/repo` string (e.g., `octocat/hello-world`).
    - **Webhook Secret**: A secret string of your choice for HMAC validation.
    - **Default Branch**: Usually `main`.

---

## 3. Configuring your Repo (`.forge.yml`)

Forge looks for a `.forge.yml` file in the root of your repository.

**Example Config:**

```yaml
name: forge-demo
on:
  push: { branches: [main] }

jobs:
  test:
    image: node:20-alpine
    steps:
      - name: Install
        run: npm install
      - name: Test
        run: npm test

  deploy:
    needs: [test]
    deploy:
      type: kubernetes
      manifest: k8s/deploy.yaml
      image_tag: ${{ git.sha }}
```

---

## 4. Setting up GitHub Webhooks

To trigger builds automatically on push:

1.  Go to your GitHub Repository **Settings** > **Webhooks**.
2.  Click **Add webhook**.
3.  **Payload URL**: `http://<your-ip>:8080/api/webhooks/github`
4.  **Content type**: `application/json`
5.  **Secret**: The "Webhook Secret" you entered in Forge.
6.  **Events**: Select "Just the push event" or "Let me select individual events" (Push + Pull Request).
7.  Click **Add webhook**.

---

## 5. Running and Monitoring Builds

### Automatic Triggers

Simply push code to your repository. Forge will receive the webhook, validate the signature, and enqueue a build.

### Manual Triggers

On the **Pipelines** page, click the **▶ Run** button next to your pipeline to trigger a build from the default branch immediately.

### Live Logs

Click on any active build to see the **Build Detail** page. Expand a job card to watch the logs stream in real-time as the Docker container executes your steps.

---

## Troubleshooting

- **Build stuck in Pending**: Check if the `FORGE_MAX_WORKERS` limit has been reached.
- **Docker Errors**: Ensure the user running Forge has permission to access `/var/run/docker.sock`.
- **Webhook 401**: Verify that the "Webhook Secret" in GitHub matches the one in Forge exactly.
- **Logs not showing**: Ensure your browser supports Server-Sent Events (SSE).

---

## 6. Observability (Prometheus & Grafana)

Forge comes pre-configured with a modern observability stack.

### Accessing Metrics
- **Prometheus UI**: [http://localhost:9090](http://localhost:9090)
- **Grafana Dashboard**: [http://localhost:3000](http://localhost:3000)

### Custom Metrics
Forge exports several custom metrics to help you monitor build health:
- `forge_active_builds`: Current running build count.
- `forge_queue_depth`: Number of builds waiting for a worker.

### Setting up Grafana
1.  Log in with `admin` / `admin`.
2.  Add a **Data Source**:
    - Name: `Prometheus`
    - URL: `http://prometheus:9090`
3.  Create a **New Dashboard** and add a graph with the query `forge_active_builds`.

---

## 7. Kubernetes Setup (Local Dev)

Forge includes a built-in Kubernetes deployer. For local development, we recommend using **Kind**.

### 1. Install Kind
```bash
# On Linux
curl -Lo ./kind https://kind.sigs.k8s.io/dl/v0.22.0/kind-linux-amd64
chmod +x ./kind
sudo mv ./kind /usr/local/bin/kind
```

### 2. Create a Cluster
Ensure your cluster is running before starting Forge:
```bash
kind create cluster
```

### 3. Connect Forge to Kind
Forge runs in Docker, so it needs to bridge the network to talk to the Kind API on your host.

1.  **Mount Config**: Forge automatically mounts `~/.kube/config` via `docker-compose.yml`.
2.  **Smart Remapping**: Forge detects if it's in a container and automatically remaps `127.0.0.1` in your Kubeconfig to `host.docker.internal` so it can reach the host gateway.
3.  **Restart**: If you create a cluster while Forge is already running, you must restart Forge:
    ```bash
    docker compose restart forge
    ```

### 4. Verification
Check the Forge logs for this line to confirm K8s is ready:
`Kubernetes: Detected local cluster in Docker. Remapping 127.0.0.1 -> host.docker.internal`

---

## 8. API Documentation (Swagger)

Forge provides interactive API documentation via Swagger UI.

- **Access UI**: [http://localhost:8080/swagger/index.html](http://localhost:8080/swagger/index.html)
- **JSON Spec**: [http://localhost:8080/swagger/doc.json](http://localhost:8080/swagger/doc.json)

You can use the "Authorize" button in the UI to add your `X-Forge-Token` and test API endpoints directly from the browser.

---

## 8. Code Quality (Linting)

Forge uses `golangci-lint` to maintain high code standards.

### Running Linters Locally
If you have `golangci-lint` installed, you can run:
```bash
golangci-lint run
```
This will check for style consistency, potential bugs, and performance issues using the rules defined in `.golangci.yml`.

---

## 9. Troubleshooting

### Build stays in "Pending"
- Ensure the Forge server has enough resources.
- Check if the Docker socket is correctly mounted and accessible.
- Verify the orchestrator is running (check backend logs for "Orchestrator started").

### Docker jobs fail with "Permission Denied"
- Forge needs to talk to the Docker daemon. Ensure the user running Forge is in the `docker` group or that the socket permissions allow access.

### Kubernetes deployment fails
- Ensure the `FORGE_KUBECONFIG` environment variable points to a valid config.
- Verify that the Forge server has network access to your cluster's API server.

### Logs are not streaming in real-time
- SSE requires a direct connection. If you are using a proxy (like Nginx), ensure it supports long-lived connections and has buffering disabled:
  ```nginx
  proxy_set_header Connection '';
  proxy_http_version 1.1;
  chunked_transfer_encoding off;
  proxy_buffering off;
  cache_control off;
  ```
