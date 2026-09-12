import http from 'k6/http';
import { check, sleep } from 'k6';
import { Rate, Trend } from 'k6/metrics';

const orderSuccessRate = new Rate('order_success_rate');
const orderDuration = new Trend('order_create_duration', true);

const BASE_URL = __ENV.BASE_URL || 'http://order-service:8080';

export const options = {
  stages: [
    { duration: '30s', target: 10 },   // ramp up
    { duration: '1m', target: 10 },    // steady state
    { duration: '30s', target: 20 },   // spike
    { duration: '1m', target: 20 },    // sustain spike
    { duration: '30s', target: 0 },    // ramp down
  ],
  thresholds: {
    http_req_duration: ['p(95)<500'],
    order_success_rate: ['rate>0.95'],
    order_create_duration: ['p(95)<500'],
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
  return JSON.stringify({
    customer_id: uuidv4(),
    items: [
      {
        product_id: productId,
        sku: `SKU-${id}`,
        product_name: `Load Test Product ${id}`,
        quantity: Math.floor(Math.random() * 5) + 1,
        unit_price: Math.floor(Math.random() * 100) + 10,
      },
    ],
  });
}

export default function () {
  const payload = createOrderPayload();
  const params = {
    headers: {
      'Content-Type': 'application/json',
      'Idempotency-Key': `idem_${randomId()}_${Date.now()}`,
    },
  };

  const res = http.post(`${BASE_URL}/api/orders`, payload, params);

  check(res, {
    'order created (201)': (r) => r.status === 201,
    'order has id': (r) => {
      try {
        return JSON.parse(r.body).id !== undefined;
      } catch {
        return false;
      }
    },
  });

  orderSuccessRate.add(res.status === 201);
  orderDuration.add(res.timings.duration);

  sleep(1);
}
