Feature: Update and invalidate
  Scenario: Update clears cache
    Given snippet {id} is cached
    When the client PATCHes v1 snippets {id} with new content
    Then the response status is 200
    And Redis key snippet:{id} is deleted
