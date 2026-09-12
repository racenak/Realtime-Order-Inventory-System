import http from 'k6/http';
import { check, sleep } from 'k6';
import { Rate, Trend } from 'k6/metrics';

const stockSuccessRate = new Rate('stock_success_rate');
const stockDuration = new Trend('stock_check_duration', true);

const BASE_URL = __ENV.BASE_URL || 'http://traefik:8088';
const JWT_TOKEN = __ENV.JWT_TOKEN;

export const options = {
  stages: [
    { duration: '30s', target: 15 },
    { duration: '1m', target: 15 },
    { duration: '30s', target: 30 },
    { duration: '1m', target: 30 },
    { duration: '30s', target: 0 },
  ],
  thresholds: {
    http_req_duration: ['p(95)<300'],
    stock_success_rate: ['rate>0.95'],
    stock_check_duration: ['p(95)<300'],
  },
};

function uuidv4() {
  return 'xxxxxxxx-xxxx-4xxx-yxxx-xxxxxxxxxxxx'.replace(/[xy]/g, function(c) {
    var r = Math.random() * 16 | 0, v = c == 'x' ? r : (r & 0x3 | 0x8);
    return v.toString(16);
  });
}

export default function () {
  const productId = uuidv4();
  const params = {
    headers: {
      'Content-Type': 'application/json',
      'Authorization': `Bearer ${JWT_TOKEN}`,
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
