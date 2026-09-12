import http from 'k6/http';
import { check, sleep } from 'k6';
import { Rate, Trend } from 'k6/metrics';

const stockSuccessRate = new Rate('stock_success_rate');
const stockDuration = new Trend('stock_check_duration', true);

const BASE_URL = __ENV.BASE_URL || 'http://traefik:8088';

export const options = {
  stages: [
    { duration: '30s', target: 15 },   // ramp up
    { duration: '1m', target: 15 },    // steady state
    { duration: '30s', target: 30 },   // spike
    { duration: '1m', target: 30 },    // sustain spike
    { duration: '30s', target: 0 },    // ramp down
  ],
  thresholds: {
    http_req_duration: ['p(95)<300'],
    stock_success_rate: ['rate>0.95'],
    stock_check_duration: ['p(95)<300'],
  },
};

function randomProductId() {
  return `prod_${Math.random().toString(36).substring(2, 10)}`;
}

export default function () {
  const productId = randomProductId();
  const params = {
    headers: {
      'Content-Type': 'application/json',
    },
  };

  const res = http.get(`${BASE_URL}/api/inventory/${productId}`, params);

  check(res, {
    'stock check ok (200 or 404)': (r) => r.status === 200 || r.status === 404,
    'stock response time ok': (r) => r.timings.duration < 300,
  });

  stockSuccessRate.add(res.status === 200 || res.status === 404);
  stockDuration.add(res.timings.duration);

  sleep(0.5);
}
