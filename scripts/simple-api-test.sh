#!/bin/bash
#
# Simple API Test - Basic functionality without readiness check
#

set -e

# Configuration
API_BASE_URL="${API_BASE_URL:-http://localhost:8080}"

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
BLUE='\033[0;34m'
NC='\033[0m'

# Counters
TESTS_PASSED=0
TESTS_FAILED=0
TOTAL_TESTS=0

# Helper functions
log_info() {
    echo -e "${BLUE}[INFO]${NC} $1"
}

log_success() {
    echo -e "${GREEN}[PASS]${NC} $1"
    ((TESTS_PASSED++))
}

log_error() {
    echo -e "${RED}[FAIL]${NC} $1"
    ((TESTS_FAILED++))
}

# Test helper
simple_test() {
    local test_name="$1"
    local method="$2"
    local endpoint="$3"
    local expected_status="$4"
    local data="$5"

    ((TOTAL_TESTS++))
    log_info "Testing: $test_name"

    local curl_cmd="curl -s -w '%{http_code}' --max-time 10"

    if [ -n "$data" ]; then
        curl_cmd="$curl_cmd -H 'Content-Type: application/json' -d '$data'"
    fi

    curl_cmd="$curl_cmd -X $method $API_BASE_URL$endpoint"

    local response=$(eval $curl_cmd)
    local status_code="${response: -3}"

    if [ "$status_code" = "$expected_status" ]; then
        log_success "$test_name (Status: $status_code)"
        return 0
    else
        log_error "$test_name - Expected: $expected_status, Got: $status_code"
        return 1
    fi
}

# Main tests
run_tests() {
    log_info "Starting Simple API Test"
    log_info "API Base URL: $API_BASE_URL"
    echo "================================"

    # Test 1: Liveness
    simple_test "Health Check - Liveness" "GET" "/v1/livez" "200"

    # Test 2: List snippets
    simple_test "List snippets" "GET" "/v1/snippets" "200"

    # Test 3: Create snippet
    local snippet_response=$(curl -s -X POST "$API_BASE_URL/v1/snippets" \
        -H "Content-Type: application/json" \
        -d '{"content": "console.log(\"Hello World\");", "tags": ["test"]}')

    if echo "$snippet_response" | grep -q '"id"'; then
        log_success "Create snippet"
        ((TESTS_PASSED++))
        SNIPPET_ID=$(echo "$snippet_response" | grep -o '"id":"[^"]*"' | sed 's/"id":"\([^"]*\)"/\1/')
        log_info "Created snippet ID: $SNIPPET_ID"
    else
        log_error "Create snippet - No ID returned"
        ((TESTS_FAILED++))
    fi
    ((TOTAL_TESTS++))

    # Test 4: Get snippet (if created)
    if [ -n "$SNIPPET_ID" ]; then
        simple_test "Get specific snippet" "GET" "/v1/snippets/$SNIPPET_ID" "200"

        # Test 5: Update snippet
        simple_test "Update snippet" "PUT" "/v1/snippets/$SNIPPET_ID" "200" '{"content": "console.log(\"Updated!\");", "tags": ["test", "updated"]}'

        # Test 6: Delete snippet
        simple_test "Delete snippet" "DELETE" "/v1/snippets/$SNIPPET_ID" "204"
    else
        log_error "Skipping snippet tests - creation failed"
        ((TESTS_FAILED+=3))
        ((TOTAL_TESTS+=3))
    fi
}

# Print summary
print_summary() {
    echo ""
    echo "================================"
    log_info "Test Summary"
    echo "Total Tests: $TOTAL_TESTS"
    echo -e "Passed: ${GREEN}$TESTS_PASSED${NC}"
    echo -e "Failed: ${RED}$TESTS_FAILED${NC}"

    if [ $TESTS_FAILED -eq 0 ]; then
        echo -e "${GREEN}✅ All tests passed! API is working.${NC}"
        return 0
    else
        echo -e "${RED}❌ Some tests failed.${NC}"
        return 1
    fi
}

# Run everything
run_tests
print_summary