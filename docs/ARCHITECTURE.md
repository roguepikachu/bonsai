
# Architecture

## 1. Components

- **API service** in Go
- **Storage** as in-memory for V1 (can swap to SQLite or Postgres later)
- **Redis** for cache, rate limiting, analytics aggregation, pubsub, streams
- **Worker service** for background aggregation and trend updates
- **Optional Web client** for viewing snippets and live counters

```mermaid
flowchart LR
  subgraph Client
    UI[Web client]
  end

  subgraph API[Go API service]
    H[HTTP handlers]
    MW[Middleware]
    C[Cache layer]
    P[Producer to Streams]
    WS[WebSocket hub]
  end

  subgraph Worker[Go worker]
    CG[Consumer group]
    AGG[Aggregators]
  end

  subgraph Redis
    R[(Redis)]
    K1[snippet_id]
    K2[meta_id]
    K3[views_id_YYYYMMDD]
    K4[uv_id_YYYYMMDD]
    K5[trend_sorted_set]
    K6[rl_route_ip]
    X[x_events_stream]
    PUB[pubsub_channel]
  end

  subgraph Store[Source of truth]
    S[(DB or in memory)]
  end

  UI -->|HTTP JSON| H
  H --> MW
  MW --> C
  C <--> R
  H <--> S
  P --> X
  CG --> X
  CG --> AGG
  AGG --> R
  H <--> WS
  WS <--> PUB
```


## 2. Data Flow

### 2.1 Read Path with Cache Aside and Stampede Guard

```mermaid
sequenceDiagram
  participant U as Client
  participant A as API
  participant R as Redis
  participant S as Store

  U->>A: GET /v1/snippets/{id}
  A->>R: GET snippet:{id}
  alt Cache hit
    R-->>A: JSON
    A->>R: PUBLISH ws:channel increment
    A-->>U: 200 with snippet, X-Cache: HIT
  else Cache miss
    A->>R: SET lock:snippet:{id} NX PX=2000
    alt Lock acquired
      A->>S: Load snippet
      S-->>A: Snippet or not found
      alt Found and not expired
        A->>R: SET snippet:{id} with TTL
        A->>R: SET meta:{id} with TTL
        A->>R: XADD x:events view
        A->>R: PUBLISH ws:channel increment
        A->>R: DEL lock:snippet:{id}
        A-->>U: 200 with snippet, X-Cache: MISS
      else Not found or expired
        A->>R: DEL lock:snippet:{id}
        A-->>U: 404 or 410
      end
    else Lock not acquired
      A->>R: SPIN wait then GET snippet:{id}
      R-->>A: JSON after fill
      A-->>U: 200 with snippet, X-Cache: WARM
    end
  end
```


### 2.2 Write Path with Invalidate on Success

```mermaid
sequenceDiagram
  participant U as Client
  participant A as API
  participant R as Redis
  participant S as Store

  U->>A: POST or PATCH /v1/snippets
  A->>S: Write to source of truth
  S-->>A: OK
  A->>R: DEL snippet:{id}
  A->>R: XADD x:events create or update
  A-->>U: 201 or 200
```


### 2.3 Analytics Aggregation with Streams and Worker Pool

```mermaid
flowchart LR
  A[API] -->|XADD view| X[x:events]
  subgraph Worker
    CG[XREADGROUP] --> W1[Worker 1]
    CG --> W2[Worker 2]
    CG --> Wn[Worker n]
  end
  W1 -->|INCR| V[views:{id}:{date}]
  W1 -->|PFADD| U[uv:{id}:{date}]
  W1 -->|ZINCRBY decay| T[trend]
  W1 -->|ACK| X
  W2 -->|same as above| V
  Wn -->|same as above| V
```


### 2.4 Rate Limiting with Sliding Window

```mermaid
sequenceDiagram
  participant A as API
  participant R as Redis

  A->>R: ZADD rl:{route}:{ip} now now
  A->>R: ZREMRANGEBYSCORE rl key 0 now-window
  A->>R: ZCARD rl key
  alt Count exceeds limit
    A-->>A: return 429 with Retry-After
  else Under limit
    A-->>A: proceed
  end
  A->>R: EXPIRE rl key = window
```


## 3. Redis Keys

| Key                     | Type        | Purpose                               | TTL                          |
| ----------------------- | ----------- | ------------------------------------- | ---------------------------- |
| `snippet:{id}`          | String JSON | Hot snippet payload for reads         | min 24h and remaining expiry |
| `meta:{id}`             | Hash        | Lightweight metadata for quick checks | mirrors snippet TTL          |
| `views:{id}:{yyyyMMdd}` | Integer     | Daily view counter                    | 400 days                     |
| `uv:{id}:{yyyyMMdd}`    | HyperLogLog | Unique visitor estimate per day       | 400 days                     |
| `trend`                 | Sorted Set  | Popularity score for ranking          | none or long TTL             |
| `rl:{route}:{ip}`       | Sorted Set  | Sliding window entries                | equals window                |
| `x:events`              | Stream      | Append only events from API           | capped by trimming           |
| `lock:snippet:{id}`     | String      | Cache refill lock key                 | a few seconds                |

**Key lifecycle notes:**
- Cache keys are refreshed on access with sliding TTL logic.
- Analytics keys are write only by workers to keep API latency low.
- Stream trimming policy can be size based via XTRIM.


## 4. Concurrency Patterns

### 4.1 Stampede Protection

- In-process singleflight groups concurrent GETs for the same id.
- Cross-process lock with SET NX and a short TTL guards the refill.
- Waiting callers poll with jitter and a small bounded backoff.

### 4.2 Worker Pool

- One consumer group name per deployment.
- Each worker goroutine handles an event with context timeout.
- On failure the event is NACKed and moved to a dead letter stream for later inspection.

### 4.3 PubSub and WebSocket

- API publishes a compact increment message for a snippet id.
- WebSocket hub fans out to subscribed clients with bounded channels per connection to avoid unbounded memory use.
- On overflow the oldest message is dropped to preserve liveness.

### 4.4 Graceful Shutdown

- API and worker listen for SIGTERM.
- Stop accepting new HTTP requests or stream claims, drain in-flight work, flush metrics, close Redis connections.


## 5. Data Model

```mermaid
classDiagram
  class Snippet {
    string id
    string content
    time created_at
    time expires_at
    []string tags
  }

  class DailyMetrics {
    string id
    date day
    int views
    int uniques_estimate
  }
```

Source of truth can be swapped without changing cache logic. Keep storage mutations simple and let the cache derive from it.


## 6. API Surface

```mermaid
flowchart TB
  A["GET /v1/health"]
  B["POST /v1/snippets"]
  C["GET /v1/snippets"]
  D["GET /v1/snippets/:id"]
  E["PATCH /v1/snippets/:id"]
  F["DELETE /v1/snippets/:id"]
  G["GET /v1/snippets/:id/metrics"]
  H["GET /v1/metrics"]
```

**Response shape:**
- JSON only
- Consistent error envelope with code, message, details
- `X-Cache` header reports HIT or MISS


## 7. Operational Notes

**Configuration:**
- Redis address
- HTTP listen address
- Log level
- Stream consumer group name

**Metrics to watch:**
- Cache hit and miss rates
- Rate limit denials
- Stream lag and pending counts
- Worker backlog and processing latency
- WebSocket client count

**Alerts:**
- Cache hit rate drops below a threshold
- Stream pending length grows beyond a threshold
- p95 latency crosses target for sustained period


## 8. Performance Targets

- Redirect style reads under 10 ms from cache at p95 on local Docker setup
- Under 50 ms overall for cold reads at p95
- Worker keeps up with event volume with backlog under a few hundred entries
