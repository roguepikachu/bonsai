Feature: Cache stampede protection
  Scenario: Single refill under contention
    Given cache for {id} is empty
    And 50 concurrent GET requests start
    When one goroutine acquires lock:snippet:{id}
    Then only one fetches from storage
    And others wait and return cached data
