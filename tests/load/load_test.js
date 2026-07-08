import http from 'k6/http';
import { check, sleep } from 'k6';

export const options = {
    stages: [
        { duration: '10s', target: 20 }, // Ramp up to 20 virtual users
        { duration: '20s', target: 50 }, // Ramp up to 50 virtual users
        { duration: '15s', target: 0 },  // Ramp down to 0 virtual users
    ],
    thresholds: {
        http_req_failed: ['rate<0.01'], // Less than 1% request failures (excluding 429s)
        http_req_duration: ['p(95)<50'], // 95% of requests should respond under 50ms
    },
};

export default function () {
    // Matches the seeded tenant ID in migration 0005
    const tenantID = 'd04a6cb1-5bb5-4c07-b08e-327c62d08a54';
    const params = {
        headers: {
            'X-Tenant-ID': tenantID,
        },
    };

    // 1. Hit the eventual consistency mode route
    const resEventual = http.get('http://localhost:8000/api/v1/eventual', params);
    check(resEventual, {
        'eventual mode: status is 200 or 429': (r) => r.status === 200 || r.status === 429,
    });

    // 2. Hit the strict mode route
    const resStrict = http.get('http://localhost:8000/api/v1/strict', params);
    check(resStrict, {
        'strict mode: status is 200 or 429': (r) => r.status === 200 || r.status === 429,
    });

    sleep(0.1); // 100ms pacing delay between request cycles
}
