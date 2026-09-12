import http from 'k6/http';
import { check, sleep } from 'k6';
import { Counter, Rate, Trend } from 'k6/metrics';

const ordersCreated = new Counter('orders_created');
const ordersFailed = new Counter('orders_failed');
const stockChecks = new Counter('stock_checks');
const successRate = new Rate('workflow_success_rate');
const orderDuration = new Trend('workflow_order_duration', true);
const stockDuration = new Trend('workflow_stock_duration', true);

const BASE_URL = __ENV.BASE_URL || 'http://traefik:8088';

export const options = {
  stages: [
    { duration: '30s', target: 10 },   // warm up
    { duration: '1m', target: 25 },    // ramp to target
    { duration: '2m', target: 25 },    // sustain
    { duration: '30s', target: 50 },   // spike
    { duration: '1m', target: 50 },    // sustain spike
    { duration: '30s', target: 0 },    // cool down
  ],
  thresholds: {
    http_req_duration: ['p(95)<500'],
    workflow_success_rate: ['rate>0.90'],
    workflow_order_duration: ['p(95)<500'],
  },
};

function randomId() {
  return Math.random().toString(36).substring(2, 15);
}

function createOrderPayload() {
  const id = randomId();
  const qty = Math.floor(Math.random() * 5) + 1;
  const price = Math.floor(Math.random() * 100) + 10;
  return {
    body: JSON.stringify({
      customer_id: `cust_wf_${id}`,
      items: [
        {
          product_id: `prod_${id}`,
          sku: `SKU-${id}`,
          product_name: `Workflow Test Product ${id}`,
          quantity: qty,
          unit_price: price,
        },
      ],
    }),
    productId: `prod_${id}`,
  };
}

export default function () {
  const scenario = Math.random();

  if (scenario < 0.4) {
    // 40% — Create order
    const { body, productId } = createOrderPayload();
    const params = {
      headers: {
        'Content-Type': 'application/json',
        'Idempotency-Key': `idem_wf_${randomId()}_${Date.now()}`,
      },
    };

    const res = http.post(`${BASE_URL}/api/orders`, body, params);

    check(res, {
      'order created': (r) => r.status === 201,
    });

    if (res.status === 201) {
      ordersCreated.add(1);
      successRate.add(true);

      // After creating, check stock
      sleep(0.5);
      const stockRes = http.get(`${BASE_URL}/api/inventory/${productId}`);
      check(stockRes, { 'stock check ok': (r) => r.status === 200 || r.status === 404 });
      stockChecks.add(1);
      stockDuration.add(stockRes.timings.duration);
    } else {
      ordersFailed.add(1);
      successRate.add(false);
    }

    orderDuration.add(res.timings.duration);

  } else if (scenario < 0.7) {
    // 30% — Check stock only (high-frequency read)
    const productId = `prod_${randomId()}`;
    const res = http.get(`${BASE_URL}/api/inventory/${productId}`);

    check(res, {
      'stock check ok': (r) => r.status === 200 || r.status === 404,
    });

    stockChecks.add(1);
    stockDuration.add(res.timings.duration);
    successRate.add(true);

  } else {
    // 30% — Read existing order
    const orderId = randomId();
    const res = http.get(`${BASE_URL}/api/orders/${orderId}`);

    check(res, {
      'order get ok': (r) => r.status === 200 || r.status === 404,
    });

    successRate.add(res.status === 200 || res.status === 404);
  }

  sleep(1);
}
