Feature: Kiali Service Details page

  User opens the Services page and sees the bookinfo namespaces,
  clicks in the productpage service, and page loads correctly.

  Background:
    Given user is at administrator perspective
    And user is at the details page for the "service" "bookinfo/productpage"

  @service-details-page
  Scenario: See details for productpage
    Then sd::user sees a list with content "Overview"
    Then sd::user sees a list with content "Traffic"
    Then sd::user sees a list with content "Inbound Metrics"
    Then sd::user sees a list with content "Traces"
    Then sd::user sees the actions button

  @service-details-page
  Scenario: See details for service
    Then sd::user sees "productpage" details information for service "v1"
    Then sd::user sees Network card
    Then sd::user sees Istio Config

  @service-details-page
  Scenario: See service minigraph for details app.
    Then sd::user sees a minigraph

  @service-details-page
  Scenario: See service Traffic information
    Then sd::user sees inbound and outbound traffic information

  @service-details-page
  Scenario: See Inbound Metrics for productspage service details
    Then sd::user sees "Request volume" graph
    Then sd::user sees "Request duration" graph
    Then sd::user sees "Request size" graph
    Then sd::user sees "Response size" graph
    Then sd::user sees "Request throughput" graph
    Then sd::user sees "Response throughput" graph
    Then sd::user sees "gRPC received" graph
    Then sd::user sees "gRPC sent" graph
    Then sd::user sees "TCP opened" graph
    Then sd::user sees "TCP closed" graph
    Then sd::user sees "TCP received" graph
    Then sd::user sees "TCP sent" graph

  @service-details-page
  Scenario: See Graph data for productspage service details Inbound Metrics graphs
    Then sd::user does not see No data message in the "Request volume" graph

  @service-details-page
  Scenario: See graph traces for productspage service details
    And user sees trace information
    When user selects a trace
    Then user sees trace details

  @service-details-page
  Scenario: See span info after selecting service span
    And user sees trace information
    When user selects a trace
    Then user sees span details

  @service-details-page
  Scenario: Verify that the Graph type dropdown is disabled when changing to Show node graph
    When user sees a minigraph
    And user chooses the "Show node graph" option
    Then the graph type is disabled
