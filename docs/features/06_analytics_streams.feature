Feature: Analytics aggregation with Streams
  Scenario: View event increments counters
    Given a worker group is running
    And a snippet view event is in x:events
    When the worker processes the event
    Then Redis increments views:{id}:{yyyyMMdd}
    And adds the visitor to uv:{id}:{yyyyMMdd}
    And updates the trend sorted set