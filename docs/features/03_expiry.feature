Feature: Expiry handling
  Scenario: Expired snippet is gone
    Given snippet {id} expired in storage
    When the client GETs v1 snippets {id}
    Then the response status is 410
