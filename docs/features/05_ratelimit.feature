Feature: Sliding window rate limits
  Scenario: Exceed read limit
    Given 60 requests from one IP in one minute
    When the client sends one more GET to v1 snippets {id}
    Then the response status is 429
    And the response has a Retry-After header