# Roadmap

## Milestone 1 — Core CRUD and Cache

**Goal**: Basic snippet lifecycle with Redis cache-aside.
**Features**

* Create, read, update, delete snippets
* Cache snippet payloads in Redis with TTL
* Health endpoint (`/v1/health`)
  **Acceptance**
* Snippets retrievable from cache after first DB hit
* Expired snippets return 410
* Cache invalidated on update/delete

---

## Milestone 2 — Rate Limits and Metrics

**Goal**: Protect API and expose observability.
**Features**

* Sliding window rate limiting per IP
* Prometheus metrics (`/v1/metrics`)
* Counters: requests, cache hits/misses, blocked requests
  **Acceptance**
* Exceeding limits returns 429 with `Retry-After`
* Metrics scrape shows increasing counters

---

## Milestone 3 — Streams-based Analytics

**Goal**: Capture views and compute daily usage stats.
**Features**

* On each GET, emit event to Redis Stream
* Worker processes events → increments views, uniques (HyperLogLog), trend score (Sorted Set)
* Metrics endpoint exposes daily aggregates
  **Acceptance**
* Daily views and uniques increase correctly
* Trend score decays over time

---

## Milestone 4 — WebSocket Realtime

**Goal**: Provide live snippet view counts.
**Features**

* Pub/Sub channel for view events
* WebSocket endpoint streams updates to clients
* Backpressure handling (bounded buffers, drop oldest)
  **Acceptance**
* Multiple clients see live counter increments
* Overloaded connections degrade gracefully

---

## Milestone 5 — Stampede Guard and Load

**Goal**: Ensure stability under concurrent traffic.
**Features**

* Redis-based lock + in-process singleflight on cache miss
* Load testing with k6/Vegeta
* Document p95 latency with and without cache
  **Acceptance**
* Under 500 concurrent GETs, p95 latency < 50ms
* Cache hit rate > 90% after warmup
