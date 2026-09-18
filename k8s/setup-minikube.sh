#!/bin/bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(dirname "$SCRIPT_DIR")"
NS="order-inventory"
CLUSTER_NAME="order-inventory"

RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m'

log()  { echo -e "${GREEN}[✓]${NC} $*"; }
warn() { echo -e "${YELLOW}[!]${NC} $*"; }
err()  { echo -e "${RED}[✗]${NC} $*" >&2; }

# ==========================================
# 1. START MINIKUBE
# ==========================================
if minikube status -p "$CLUSTER_NAME" 2>/dev/null | grep -q "Running"; then
    log "Minikube cluster '$CLUSTER_NAME' is already running"
else
    warn "Starting Minikube with 3 nodes (control-plane + app + observability)..."
    minikube start -p "$CLUSTER_NAME" \
        --driver=docker \
        --nodes=3 \
        --cpus=2 \
        --memory=4096 \
        --disk-size=20g \
        --container-runtime=containerd
    log "Minikube started"
fi

# ==========================================
# 2. LABEL NODES
# ==========================================
log "Labeling nodes..."

# Control plane node
minikube node list -p "$CLUSTER_NAME" | tail -n +2 | head -1 | awk '{print $1}' | while read -r node; do
    kubectl label node "$node" node-role.kubernetes.io/control-plane="" --overwrite 2>/dev/null || true
    log "Control plane: $node"
done

# App node (2nd node)
minikube node list -p "$CLUSTER_NAME" | tail -n +2 | sed -n '2p' | awk '{print $1}' | while read -r node; do
    kubectl label node "$node" node-role.kubernetes.io/app="true" --overwrite 2>/dev/null || true
    log "App node: $node"
done

# Observability node (3rd node)
minikube node list -p "$CLUSTER_NAME" | tail -n +2 | sed -n '3p' | awk '{print $1}' | while read -r node; do
    kubectl label node "$node" node-role.kubernetes.io/observability="true" --overwrite 2>/dev/null || true
    log "Observability node: $node"
done

# ==========================================
# 3. ENABLE ADDONS
# ==========================================
minikube addons enable -p "$CLUSTER_NAME" ingress 2>/dev/null || true
log "Ingress addon enabled"

# ==========================================
# 4. BUILD & LOAD IMAGES (optional - uses GHCR by default)
# ==========================================
if [ "${BUILD_LOCAL:-false}" = "true" ]; then
    warn "Building images locally..."
    eval "$(minikube docker-env -p "$CLUSTER_NAME")"

    for svc in order-service inventory-service websocket-service auth-service; do
        docker build -t "localhost/$svc:latest" -f "cmd/$svc/Dockerfile" "$PROJECT_ROOT"
        log "Built $svc"
    done

    docker build -t localhost/traefik:latest -f cmd/traefik/Dockerfile "$PROJECT_ROOT"
    log "Built traefik"

    # Update image references to use local images
    export IMAGE_PREFIX="localhost/"
    export IMAGE_PULL="Never"
else
    warn "Using GHCR images (set BUILD_LOCAL=true to build locally)"
    export IMAGE_PREFIX="ghcr.io/racenak/realtime-order-inventory-system/"
    export IMAGE_PULL="Always"
fi

# ==========================================
# 5. CREATE NAMESPACE
# ==========================================
kubectl apply -f "$SCRIPT_DIR/base/namespace.yaml"
log "Namespace '$NS' created"

# ==========================================
# 6. APPLY CONFIGMAPS
# ==========================================
kubectl apply -f "$SCRIPT_DIR/base/configmap-otel.yaml"
kubectl apply -f "$SCRIPT_DIR/base/configmap-prometheus.yaml"
kubectl apply -f "$SCRIPT_DIR/base/configmap-tempo.yaml"
kubectl apply -f "$SCRIPT_DIR/base/configmap-loki.yaml"
kubectl apply -f "$SCRIPT_DIR/base/configmap-grafana-dashboards.yaml"
kubectl apply -f "$SCRIPT_DIR/base/configmap-grafana-datasources.yaml"
kubectl apply -f "$SCRIPT_DIR/base/configmap-grafana-dashboards-provider.yaml"
log "All ConfigMaps applied"

# ==========================================
# 7. APPLY DATABASES
# ==========================================
kubectl apply -f "$SCRIPT_DIR/databases/order-db.yaml"
kubectl apply -f "$SCRIPT_DIR/databases/inventory-db.yaml"
kubectl apply -f "$SCRIPT_DIR/databases/redis.yaml"
kubectl apply -f "$SCRIPT_DIR/databases/kafka.yaml"
log "All databases applied"

# ==========================================
# 8. WAIT FOR DATABASES
# ==========================================
warn "Waiting for databases to be ready..."
kubectl -n "$NS" wait --for=condition=available deployment/order-db --timeout=120s 2>/dev/null || warn "order-db not ready yet"
kubectl -n "$NS" wait --for=condition=available deployment/inventory-db --timeout=120s 2>/dev/null || warn "inventory-db not ready yet"
kubectl -n "$NS" wait --for=condition=available deployment/redis --timeout=60s 2>/dev/null || warn "redis not ready yet"
kubectl -n "$NS" wait --for=condition=available deployment/kafka --timeout=120s 2>/dev/null || warn "kafka not ready yet"
log "Databases ready"

# ==========================================
# 9. APPLY APP SERVICES
# ==========================================
if [ "${BUILD_LOCAL:-false}" = "true" ]; then
    sed -i "s|image: ghcr.io/racenak/realtime-order-inventory-system/|image: localhost/|g" \
        "$SCRIPT_DIR/app/"*.yaml
    sed -i "s|imagePullPolicy: Always|imagePullPolicy: Never|g" \
        "$SCRIPT_DIR/app/"*.yaml
fi

kubectl apply -f "$SCRIPT_DIR/app/traefik.yaml"
kubectl apply -f "$SCRIPT_DIR/app/order-service.yaml"
kubectl apply -f "$SCRIPT_DIR/app/inventory-service.yaml"
kubectl apply -f "$SCRIPT_DIR/app/websocket-service.yaml"
kubectl apply -f "$SCRIPT_DIR/app/auth-service.yaml"
log "All app services applied"

# ==========================================
# 10. APPLY OBSERVATION STACK
# ==========================================
kubectl apply -f "$SCRIPT_DIR/obs/otel-collector.yaml"
kubectl apply -f "$SCRIPT_DIR/obs/prometheus.yaml"
kubectl apply -f "$SCRIPT_DIR/obs/tempo.yaml"
kubectl apply -f "$SCRIPT_DIR/obs/loki.yaml"
kubectl apply -f "$SCRIPT_DIR/obs/grafana.yaml"
log "Observation stack applied"

# ==========================================
# 11. WAIT FOR ALL DEPLOYMENTS
# ==========================================
warn "Waiting for all deployments to be ready..."
kubectl -n "$NS" wait --for=condition=available deployment --all --timeout=300s 2>/dev/null || warn "Some deployments not ready yet"
log "All deployments ready"

# ==========================================
# 12. PRINT ACCESS INFO
# ==========================================
echo ""
echo "============================================"
echo "  DEPLOYMENT COMPLETE"
echo "============================================"
echo ""
echo "Namespace: $NS"
echo ""
echo "Node layout:"
minikube node list -p "$CLUSTER_NAME" 2>/dev/null || true
echo ""
echo "Services:"
kubectl -n "$NS" get svc -o wide 2>/dev/null || true
echo ""
echo "Pods:"
kubectl -n "$NS" get pods -o wide 2>/dev/null || true
echo ""
echo "Access URLs:"
echo "  Traefik:      http://$(minikube ip -p "$CLUSTER_NAME"):30080"
echo "  Traefik API:  http://$(minikube ip -p "$CLUSTER_NAME"):30090"
echo "  Grafana:      http://$(minikube ip -p "$CLUSTER_NAME"):30000"
echo ""
echo "Port forward commands:"
echo "  kubectl -n $NS port-forward svc/grafana 3000:3000"
echo "  kubectl -n $NS port-forward svc/prometheus 9090:9090"
echo "  kubectl -n $NS port-forward svc/tempo 3200:3200"
echo "  kubectl -n $NS port-forward svc/loki 3100:3100"
echo ""
