# Deep Dive: Observability in Real-Time Battle Engines

This document explains structured logging, metrics, distributed tracing, and request correlation IDs in **DSAblitz**.

---

## 1. Structured Logging

In high-concurrency systems, plain text logs (like `fmt.Println`) are impossible to parse at scale. 
* **The Solution**: Use structured JSON logging (via packages like `uber-go/zap` or `sirupsen/logrus`). This allows log aggregation engines (like Elasticsearch or Grafana Loki) to query records by structured key-value filters.

### **Submission Context Mapping**
Every log statement inside the submission lifecycle must carry the context fields:
```json
{
  "level": "info",
  "timestamp": "2026-07-03T19:30:00Z",
  "correlation_id": "c8b417e9-a417-43cf-be79-7df919fb9de1",
  "battle_id": "90b0432c-3965-4f40-84c4-7264a2cbe701",
  "user_id": "a8d29837-12fb-4e1b-b27b-fb49874cb602",
  "question_id": "10000000-0000-0000-0000-000000000001",
  "event": "submission_processed",
  "is_correct": true,
  "elapsed_ms": 350
}
```

---

## 2. Correlation IDs (Tracing Requests)

A **Correlation ID** (or Request ID) is a unique UUID generated at the ingress (HTTP handler or WebSocket connection gate) and propagated through the call stack inside a Go `context.Context`.
* **Purpose**: If a user reports a submission error, searching the log aggregator for that specific `correlation_id` yields the entire call trace:
  1. WebSocket frame received.
  2. Questions Cache read.
  3. ValidateAnswer evaluated.
  4. PostgreSQL row locked.
  5. Transaction committed.
  6. WebSocket success frame emitted.

---

## 3. Metrics (Prometheus / Grafana)

To monitor system health, we track three categories of metrics:

### **A. Latency (Histograms)**
* `battle_submission_duration_seconds`: Tracks the total roundtrip latency of evaluating answers and committing database writes.

### **B. Throughput (Counters)**
* `battle_submissions_total`: Counter tracking the volume of correct vs. incorrect answers.
  * Labels: `status=["correct", "incorrect", "skipped"]`, `question_type=["mcq", "complexity_prediction", "numeric_answer", "algorithm_ordering"]`

### **C. Concurrency (Gauges)**
* `battle_active_matches`: Gauge tracking the current count of concurrent in-flight games.
* `database_pool_active_connections`: Tracks the count of active pgx pool checkouts.

---

## 4. Distributed Tracing (OpenTelemetry)

As **DSAblitz** grows, tracing requests between the monolithed modules helps identify database I/O bottlenecks.
* **Trace Spans**: Every submission generates a root trace span. Within it, child spans trace `ValidateAnswer` cache checks, PostgreSQL locking durations, and transaction commits, making database slow-queries immediately visible in systems like Jaeger.
