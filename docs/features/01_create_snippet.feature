Feature: Create snippet
  Scenario: Create with default expiry
    Given a valid request with content "hello"
    When the client POSTs to v1 snippets
    Then the response status is 201
    And the body contains an id and created_at
    And Redis has key snippet:{id} with a TTL

  Scenario: Reject oversized content
    Given content larger than 10 KB
    When the client POSTs to v1 snippets
    Then the response status is 400
