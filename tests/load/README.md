# k6 Load Testing

This directory contains k6 load test scripts for the Realtime Order Inventory System.

## Prerequisites

- k6 must be installed (or use Docker: `grafana/k6:0.50.0-with-browser`)
- Services must be running: `podman compose up -d`
- Traefik gateway must be accessible

## Running Tests

### Via Docker Compose (recommended)

```bash
# Run order creation load test
podman compose run --rm k6-loadtester run /scripts/order_create.js

# Run inventory check load test
podman compose run --rm k6-loadtester run /scripts/inventory_check.js

# Run full workflow load test
podman compose run --rm k6-loadtester run /scripts/full_workflow.js
```

### Via Makefile

```bash
make load-test-order    # Order creation
make load-test-stock    # Inventory check
make load-test-full     # Full workflow
```

### Direct (if k6 installed locally)

```bash
k6 run tests/load/order_create.js
k6 run tests/load/inventory_check.js
k6 run tests/load/full_workflow.js
```

## Test Scenarios

### order_create.js
- **Goal**: Measure order creation throughput
- **Load**: 10 → 20 VUs over 3.5 min
- **Thresholds**: p95 < 500ms, success rate > 95%

### inventory_check.js
- **Goal**: Measure stock check read performance
- **Load**: 15 → 30 VUs over 3.5 min
- **Thresholds**: p95 < 300ms, success rate > 95%

### full_workflow.js
- **Goal**: Realistic mixed workload
- **Load**: 10 → 25 → 50 VUs over 5.5 min
- **Mix**: 40% order creation, 30% stock checks, 30% order reads
- **Thresholds**: p95 < 500ms, success rate > 90%

## Output

k6 outputs metrics to stdout. For JSON output:

```bash
k6 run --out json=results.json tests/load/full_workflow.js
```

## Grafana Integration

k6 metrics can be pushed to Prometheus for dashboard visualization.
