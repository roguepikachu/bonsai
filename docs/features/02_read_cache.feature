Feature: Read with cache aside
  Scenario: Cache hit
    Given snippet {id} exists in Redis
    When the client GETs v1 snippets {id}
    Then the response status is 200
    And the header X-Cache is HIT

  Scenario: Cache miss then fill
    Given snippet {id} is only in storage
    When the client GETs v1 snippets {id}
    Then the response status is 200
    And Redis stores snippet:{id} with TTL

