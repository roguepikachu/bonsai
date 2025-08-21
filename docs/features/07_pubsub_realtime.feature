Feature: Realtime view count
  Scenario: Live increment broadcast
    Given a WebSocket client is subscribed for {id}
    When a view is recorded
    Then the client receives a message with the new count
