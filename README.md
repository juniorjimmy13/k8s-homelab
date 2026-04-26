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
