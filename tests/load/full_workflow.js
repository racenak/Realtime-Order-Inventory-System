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
const JWT_TOKEN = __ENV.JWT_TOKEN;

export const options = {
  stages: [
    { duration: '30s', target: 10 },
    { duration: '1m', target: 25 },
    { duration: '2m', target: 25 },
    { duration: '30s', target: 50 },
    { duration: '1m', target: 50 },
    { duration: '30s', target: 0 },
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

function uuidv4() {
  return 'xxxxxxxx-xxxx-4xxx-yxxx-xxxxxxxxxxxx'.replace(/[xy]/g, function(c) {
    var r = Math.random() * 16 | 0, v = c == 'x' ? r : (r & 0x3 | 0x8);
    return v.toString(16);
  });
}

function createOrderPayload() {
  const id = randomId();
  const productId = uuidv4();
  const qty = Math.floor(Math.random() * 5) + 1;
  const price = Math.floor(Math.random() * 100) + 10;
  return {
    body: JSON.stringify({
      customer_id: uuidv4(),
      items: [
        {
          product_id: productId,
          sku: `SKU-${id}`,
          product_name: `Workflow Test Product ${id}`,
          quantity: qty,
          unit_price: price,
        },
      ],
    }),
    productId: productId,
  };
}

export default function () {
  const scenario = Math.random();

  if (scenario < 0.4) {
    const { body, productId } = createOrderPayload();
    const params = {
      headers: {
        'Content-Type': 'application/json',
        'Authorization': `Bearer ${JWT_TOKEN}`,
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
      sleep(0.5);
      const stockRes = http.get(`${BASE_URL}/api/inventory/${productId}`, {
        headers: { 'Authorization': `Bearer ${JWT_TOKEN}` },
      });
      check(stockRes, { 'stock check ok': (r) => r.status === 200 || r.status === 404 });
      stockChecks.add(1);
      stockDuration.add(stockRes.timings.duration);
    } else {
      ordersFailed.add(1);
      successRate.add(false);
    }

    orderDuration.add(res.timings.duration);

  } else if (scenario < 0.7) {
    const productId = uuidv4();
    const res = http.get(`${BASE_URL}/api/inventory/${productId}`, {
      headers: { 'Authorization': `Bearer ${JWT_TOKEN}` },
    });

    check(res, {
      'stock check ok': (r) => r.status === 200 || r.status === 404,
    });

    stockChecks.add(1);
    stockDuration.add(res.timings.duration);
    successRate.add(true);

  } else {
    const orderId = randomId();
    const res = http.get(`${BASE_URL}/api/orders/${orderId}`, {
      headers: { 'Authorization': `Bearer ${JWT_TOKEN}` },
    });

    check(res, {
      'order get ok': (r) => r.status === 200 || r.status === 404,
    });

    successRate.add(res.status === 200 || res.status === 404);
  }

  sleep(1);
}
