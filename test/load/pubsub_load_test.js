import http from 'k6/http';
import { check, sleep } from 'k6';
import { Counter, Rate, Trend } from 'k6/metrics';
import encoding from 'k6/encoding';

// Custom metrics
const publishErrors = new Counter('publish_errors');
const publishSuccessRate = new Rate('publish_success_rate');
const messageLatency = new Trend('message_latency', true);

// Test configuration
export const options = {
  stages: [
    { duration: '30s', target: 10 },   // Ramp up
    { duration: '2m', target: 50 },    // Load test
    { duration: '1m', target: 100 },   // Peak load
    { duration: '30s', target: 0 },    // Ramp down
  ],
  thresholds: {
    http_req_duration: ['p(95)<500'],  // 95% of requests under 500ms
    publish_success_rate: ['rate>0.95'], // 95% success rate
    publish_errors: ['count<100'],     // Less than 100 errors
  },
};

const BASE_URL = __ENV.PUBSUB_URL || 'http://localhost:8081';

// Test data generators
function generatePayload(size = 1024) {
  const chars = 'ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789';
  let result = '';
  for (let i = 0; i < size; i++) {
    result += chars.charAt(Math.floor(Math.random() * chars.length));
  }
  return encoding.b64encode(result);
}

function getRandomTopic() {
  const topics = ['events', 'logs', 'metrics', 'orders', 'notifications'];
  return topics[Math.floor(Math.random() * topics.length)];
}

function getRandomPriority() {
  const priorities = ['low', 'normal', 'high', 'critical'];
  return priorities[Math.floor(Math.random() * priorities.length)];
}

// Setup: Create topics
export function setup() {
  console.log('Setting up load test...');
  
  const topics = ['events', 'logs', 'metrics', 'orders', 'notifications'];
  
  for (const topic of topics) {
    const payload = {
      name: topic,
      partitions: 4,
      replication: 1
    };
    
    const response = http.post(`${BASE_URL}/v1/topics`, JSON.stringify(payload), {
      headers: { 'Content-Type': 'application/json' },
    });
    
    if (response.status !== 200 && response.status !== 201) {
      console.log(`Failed to create topic ${topic}: ${response.status}`);
    }
  }
  
  console.log('Setup complete');
  return { baseUrl: BASE_URL };
}

// Main test function
export default function(data) {
  const scenarios = [
    { name: 'single_publish', weight: 60 },
    { name: 'batch_publish', weight: 30 },
    { name: 'topic_operations', weight: 10 },
  ];
  
  const scenario = selectScenario(scenarios);
  
  switch (scenario) {
    case 'single_publish':
      testSinglePublish(data.baseUrl);
      break;
    case 'batch_publish':
      testBatchPublish(data.baseUrl);
      break;
    case 'topic_operations':
      testTopicOperations(data.baseUrl);
      break;
  }
  
  sleep(Math.random() * 2); // Random sleep between 0-2 seconds
}

function selectScenario(scenarios) {
  const random = Math.random() * 100;
  let cumulative = 0;
  
  for (const scenario of scenarios) {
    cumulative += scenario.weight;
    if (random <= cumulative) {
      return scenario.name;
    }
  }
  
  return scenarios[0].name;
}

function testSinglePublish(baseUrl) {
  const topic = getRandomTopic();
  const payload = {
    payload: generatePayload(Math.floor(Math.random() * 2048) + 256),
    key: `key-${Math.floor(Math.random() * 1000)}`,
    priority: getRandomPriority(),
    headers: {
      'test-id': `${__VU}-${__ITER}`,
      'timestamp': new Date().toISOString(),
    }
  };
  
  const startTime = Date.now();
  const response = http.post(`${baseUrl}/v1/publish/${topic}`, JSON.stringify(payload), {
    headers: { 'Content-Type': 'application/json' },
    timeout: '10s',
  });
  
  const success = check(response, {
    'publish status is 200': (r) => r.status === 200,
    'response has message_id': (r) => JSON.parse(r.body).data.message_id !== undefined,
  });
  
  publishSuccessRate.add(success);
  if (!success) {
    publishErrors.add(1);
  }
  
  messageLatency.add(Date.now() - startTime);
}

function testBatchPublish(baseUrl) {
  const topic = getRandomTopic();
  const batchSize = Math.floor(Math.random() * 10) + 1; // 1-10 messages
  const messages = [];
  
  for (let i = 0; i < batchSize; i++) {
    messages.push({
      payload: generatePayload(256),
      key: `batch-key-${i}`,
      priority: getRandomPriority(),
      headers: {
        'batch-id': `${__VU}-${__ITER}`,
        'message-index': i.toString(),
      }
    });
  }
  
  const batchPayload = { messages };
  
  const startTime = Date.now();
  const response = http.post(`${baseUrl}/v1/publish/${topic}/batch`, JSON.stringify(batchPayload), {
    headers: { 'Content-Type': 'application/json' },
    timeout: '15s',
  });
  
  const success = check(response, {
    'batch publish status is 200': (r) => r.status === 200,
    'response has messages': (r) => {
      const body = JSON.parse(r.body);
      return body.data && body.data.messages && body.data.messages.length === batchSize;
    },
  });
  
  publishSuccessRate.add(success);
  if (!success) {
    publishErrors.add(1);
  }
  
  messageLatency.add(Date.now() - startTime);
}

function testTopicOperations(baseUrl) {
  // Test topic listing
  const listResponse = http.get(`${baseUrl}/v1/topics`);
  check(listResponse, {
    'topic list status is 200': (r) => r.status === 200,
    'topic list returns data': (r) => JSON.parse(r.body).data !== undefined,
  });
  
  // Test health endpoint
  const healthResponse = http.get(`${baseUrl}/health`);
  check(healthResponse, {
    'health status is 200': (r) => r.status === 200,
    'health status is healthy': (r) => JSON.parse(r.body).status === 'healthy',
  });
  
  // Test metrics endpoint
  const metricsResponse = http.get(`${baseUrl}:9092/metrics`, { timeout: '5s' });
  check(metricsResponse, {
    'metrics endpoint accessible': (r) => r.status === 200,
  });
}

// Teardown
export function teardown(data) {
  console.log('Load test complete');
  console.log(`Publish errors: ${publishErrors.count}`);
  console.log(`Average message latency: ${messageLatency.avg}ms`);
}