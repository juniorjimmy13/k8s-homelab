# k8s-homelab

A bare-metal-style Kubernetes home lab built on a single laptop using k3d, with a full observability stack and a Go service that queries the Kubernetes API.

## What this is

This project simulates the kind of private cloud platform used in game studio infrastructure. It covers the full stack from cluster provisioning to application deployment to observability — the same patterns used in production build farm and game server environments.

## Architecture

- **3-node Kubernetes cluster** — 1 control plane, 2 workers, provisioned with k3d
- **Traefik ingress** — routes external HTTP traffic into the cluster
- **Prometheus + Grafana** — full observability stack with pre-built Kubernetes dashboards
- **cluster-status** — a Go service that queries the Kubernetes API and returns live pod status as JSON

## Why these choices

- **k3d over minikube** — k3d runs real multi-node clusters inside Docker, closer to how K3s is used in edge and private cloud environments
- **Traefik over Nginx** — ships with k3d by default, demonstrates understanding of ingress controllers generally
- **kube-prometheus-stack** — the industry standard for Kubernetes observability, bundles Prometheus, Grafana, and Alertmanager with sane defaults
- **client-go** — the official Kubernetes Go client, the same library used by kubectl itself

## Structure

├── deployment.yaml       # nginx app, 2 replicas, rolling update strategy
├── service.yaml          # ClusterIP service exposing the app internally
├── ingress.yaml          # Traefik ingress rule routing / to the app
└── cluster-status/       # Go service querying the Kubernetes API
├── main.go
├── go.mod
└── go.sum

## Running it

**Prerequisites:** Docker Desktop, WSL2 (Ubuntu), k3d, kubectl, Helm, Go 1.22+

```bash
# Start the cluster
k3d cluster create homelab \
  --agents 2 \
  --port "8080:80@loadbalancer" \
  --port "8443:443@loadbalancer"

# Deploy the app
kubectl apply -f deployment.yaml
kubectl apply -f service.yaml
kubectl apply -f ingress.yaml

# Install observability stack
helm repo add prometheus-community https://prometheus-community.github.io/helm-charts
helm repo update
helm install monitoring prometheus-community/kube-prometheus-stack \
  --namespace monitoring --create-namespace

# Run the cluster-status service
cd cluster-status
go run main.go

# Query live pod status
curl http://localhost:8090/status
```

## What I learned

- How Kubernetes self-heals — deleting a pod triggers immediate replacement to maintain replica count
- Rolling updates — Kubernetes replaces pods gradually so the app stays available during deploys
- The full request path — browser → ingress → service → pod
- How kubectl and client-go both talk to the same Kubernetes API
- Debugging with kubectl describe, kubectl get -w, and kubectl logs

## ci-bridge

A Go HTTP service that acts as middleware between a CI system and Kubernetes. Receives webhook events, validates a secret token, and creates Kubernetes Jobs to run builds.

### Endpoints

- `POST /webhook` — accepts a JSON payload and triggers a Kubernetes Job
- `GET /health` — returns `{"status":"ok"}` for health checks

### Webhook payload

```json
{
  "repo": "my-game",
  "branch": "main",
  "commit": "abc123"
}
```

### Security

Requests must include the `X-Webhook-Secret` header with the correct token. Requests without it or with a wrong token are rejected with 401 Unauthorized.

### Running

```bash
cd ci-bridge
go run main.go
# listening on :9000

# Trigger a build
curl -X POST http://localhost:9000/webhook \
  -H "Content-Type: application/json" \
  -H "X-Webhook-Secret: homelab-secret" \
  -d '{"repo":"my-game","branch":"main","commit":"abc123"}'
```

## rbac

Namespace isolation for two independent teams. Each team gets their own namespace with a Role that allows full control over their own workloads but no access to other namespaces or cluster infrastructure.

### What each team can do

- Create, update, delete pods, deployments, services, configmaps, and jobs in their own namespace
- Nothing outside their own namespace

### What each team cannot do

- Access another team's namespace
- List or modify cluster nodes
- Access the monitoring namespace
- Create or modify cluster-level resources

### Verified with

```bash
kubectl auth can-i list pods --namespace=team-proj1 \
  --as=system:serviceaccount:team-proj1:proj1-developer
# yes

kubectl auth can-i list pods --namespace=team-proj2 \
  --as=system:serviceaccount:team-proj1:proj1-developer
# no
```
