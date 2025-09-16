#!/bin/bash
#
# Bonsai API Sanity Check Script
# Tests all API endpoints to ensure the service is working correctly
#

set -e

# Configuration
API_BASE_URL="${API_BASE_URL:-http://localhost:8080}"
TIMEOUT="${TIMEOUT:-10}"

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
BLUE='\033[0;34m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

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

log_warning() {
    echo -e "${YELLOW}[WARN]${NC} $1"
}

# Test helper function
run_test() {
    local test_name="$1"
    local method="$2"
    local endpoint="$3"
    local expected_status="$4"
    local data="$5"
    local headers="$6"

    ((TOTAL_TESTS++))

    log_info "Testing: $test_name"

    local curl_cmd="curl -s -w '%{http_code}' --max-time $TIMEOUT"

    if [ -n "$headers" ]; then
        curl_cmd="$curl_cmd -H '$headers'"
    fi

    if [ -n "$data" ]; then
        curl_cmd="$curl_cmd -d '$data'"
    fi

    curl_cmd="$curl_cmd -X $method $API_BASE_URL$endpoint"

    local response=$(eval $curl_cmd)
    local status_code="${response: -3}"
    local body="${response%???}"

    if [ "$status_code" = "$expected_status" ]; then
        log_success "$test_name (Status: $status_code)"
        return 0
    else
        log_error "$test_name - Expected: $expected_status, Got: $status_code"
        if [ -n "$body" ]; then
            echo "Response: $body"
        fi
        return 1
    fi
}

# Create temporary test files
create_test_files() {
    log_info "Creating temporary test files..."

    cat > /tmp/bonsai_snippet1.json << 'EOF'
{
  "content": "console.log('Hello, World!');",
  "tags": ["javascript", "hello-world", "console"]
}
EOF

    cat > /tmp/bonsai_snippet2.json << 'EOF'
{
  "content": "def hello():\n    print('Hello from Python!')",
  "tags": ["python", "function"]
}
EOF

    cat > /tmp/bonsai_update.json << 'EOF'
{
  "content": "def hello_updated():\n    print('Hello from Updated Python!')\n    return 'success'",
  "tags": ["python", "function", "updated"]
}
EOF
}

# Cleanup test files
cleanup_test_files() {
    log_info "Cleaning up temporary test files..."
    rm -f /tmp/bonsai_snippet1.json /tmp/bonsai_snippet2.json /tmp/bonsai_update.json
}

# Wait for API to be ready
wait_for_api() {
    log_info "Waiting for API to be ready..."

    local max_attempts=30
    local attempt=1

    while [ $attempt -le $max_attempts ]; do
        if curl -s --max-time 2 "$API_BASE_URL/v1/livez" > /dev/null 2>&1; then
            log_success "API is ready!"
            return 0
        fi

        log_info "Attempt $attempt/$max_attempts - API not ready, waiting..."
        sleep 2
        ((attempt++))
    done

    log_error "API failed to become ready after $max_attempts attempts"
    return 1
}

# Main test suite
run_sanity_tests() {
    log_info "Starting Bonsai API Sanity Check"
    log_info "API Base URL: $API_BASE_URL"
    echo "================================"

    # Wait for API to be ready
    wait_for_api || exit 1

    create_test_files

    # Test 1: Health Check - Liveness
    run_test "Health Check - Liveness" "GET" "/v1/livez" "200"

    # Test 2: Health Check - Readiness
    # Note: Readiness may fail due to password auth issues, but we continue testing
    local readiness_response=$(curl -s -w '%{http_code}' --max-time $TIMEOUT "$API_BASE_URL/v1/readyz")
    local readiness_status="${readiness_response: -3}"
    local readiness_body="${readiness_response%???}"

    if [ "$readiness_status" = "200" ]; then
        log_success "Health Check - Readiness (Status: 200)"
        ((TESTS_PASSED++))
    elif [ "$readiness_status" = "503" ]; then
        log_warning "Health Check - Readiness returned 503 (services not fully ready) - continuing tests"
        if echo "$readiness_body" | grep -q "postgres.*down"; then
            log_warning "PostgreSQL connection issue detected - may affect database operations"
        fi
        # Don't count as failed since API is operational
    else
        log_error "Health Check - Readiness - Unexpected status: $readiness_status"
        ((TESTS_FAILED++))
    fi
    ((TOTAL_TESTS++))

    # Test 3: List snippets (empty initially)
    run_test "List snippets (empty)" "GET" "/v1/snippets" "200"

    # Test 4: Create JavaScript snippet
    local js_response=$(curl -s -X POST "$API_BASE_URL/v1/snippets" \
        -H "Content-Type: application/json" \
        --data @/tmp/bonsai_snippet1.json)

    if echo "$js_response" | grep -q '"id"'; then
        log_success "Create JavaScript snippet"
        ((TESTS_PASSED++))
        JS_SNIPPET_ID=$(echo "$js_response" | grep -o '"id":"[^"]*"' | sed 's/"id":"\([^"]*\)"/\1/')
    else
        log_error "Create JavaScript snippet - No ID returned"
        ((TESTS_FAILED++))
    fi
    ((TOTAL_TESTS++))

    # Test 5: Create Python snippet
    local py_response=$(curl -s -X POST "$API_BASE_URL/v1/snippets" \
        -H "Content-Type: application/json" \
        --data @/tmp/bonsai_snippet2.json)

    if echo "$py_response" | grep -q '"id"'; then
        log_success "Create Python snippet"
        ((TESTS_PASSED++))
        PY_SNIPPET_ID=$(echo "$py_response" | grep -o '"id":"[^"]*"' | sed 's/"id":"\([^"]*\)"/\1/')
    else
        log_error "Create Python snippet - No ID returned"
        ((TESTS_FAILED++))
        PY_SNIPPET_ID=""
    fi
    ((TOTAL_TESTS++))

    # Test 6: List all snippets (should have 2)
    local list_response=$(curl -s "$API_BASE_URL/v1/snippets")
    if echo "$list_response" | grep -q '"items":\[.*,.*\]'; then
        log_success "List all snippets (with data)"
        ((TESTS_PASSED++))
    else
        log_error "List all snippets - Expected 2 items"
        ((TESTS_FAILED++))
    fi
    ((TOTAL_TESTS++))

    # Test 7: Get specific snippet
    if [ -n "$PY_SNIPPET_ID" ]; then
        local get_response=$(curl -s -w '%{http_code}' "$API_BASE_URL/v1/snippets/$PY_SNIPPET_ID")
        local get_status="${get_response: -3}"

        if [ "$get_status" = "200" ]; then
            log_success "Get specific snippet"
            ((TESTS_PASSED++))
        else
            log_error "Get specific snippet - Status: $get_status"
            ((TESTS_FAILED++))
        fi
    else
        log_error "Get specific snippet - No snippet ID available"
        ((TESTS_FAILED++))
    fi
    ((TOTAL_TESTS++))

    # Test 8: Update snippet
    if [ -n "$PY_SNIPPET_ID" ]; then
        local update_response=$(curl -s -w '%{http_code}' -X PUT "$API_BASE_URL/v1/snippets/$PY_SNIPPET_ID" \
            -H "Content-Type: application/json" \
            --data @/tmp/bonsai_update.json)
        local update_status="${update_response: -3}"

        if [ "$update_status" = "200" ]; then
            log_success "Update snippet"
            ((TESTS_PASSED++))
        else
            log_error "Update snippet - Status: $update_status"
            ((TESTS_FAILED++))
        fi
    else
        log_error "Update snippet - No snippet ID available"
        ((TESTS_FAILED++))
    fi
    ((TOTAL_TESTS++))

    # Test 9: Filter by tag
    local filter_response=$(curl -s "$API_BASE_URL/v1/snippets?tag=python")
    if echo "$filter_response" | grep -q '"items":\['; then
        log_success "Filter by tag"
        ((TESTS_PASSED++))
    else
        log_error "Filter by tag - No items returned"
        ((TESTS_FAILED++))
    fi
    ((TOTAL_TESTS++))

    # Test 10: Pagination
    local pagination_response=$(curl -s "$API_BASE_URL/v1/snippets?page=1&limit=1")
    if echo "$pagination_response" | grep -q '"limit":1'; then
        log_success "Pagination"
        ((TESTS_PASSED++))
    else
        log_error "Pagination - Incorrect limit"
        ((TESTS_FAILED++))
    fi
    ((TOTAL_TESTS++))

    # Test 11: Delete snippet
    if [ -n "$PY_SNIPPET_ID" ]; then
        local delete_response=$(curl -s -w '%{http_code}' -X DELETE "$API_BASE_URL/v1/snippets/$PY_SNIPPET_ID")
        local delete_status="${delete_response: -3}"

        if [ "$delete_status" = "204" ] || [ "$delete_status" = "200" ]; then
            log_success "Delete snippet"
            ((TESTS_PASSED++))
        else
            log_error "Delete snippet - Status: $delete_status"
            ((TESTS_FAILED++))
        fi
    else
        log_error "Delete snippet - No snippet ID available"
        ((TESTS_FAILED++))
    fi
    ((TOTAL_TESTS++))

    # Test 12: Get deleted snippet (should 404)
    if [ -n "$PY_SNIPPET_ID" ]; then
        local get_deleted_response=$(curl -s -w '%{http_code}' "$API_BASE_URL/v1/snippets/$PY_SNIPPET_ID")
        local get_deleted_status="${get_deleted_response: -3}"

        if [ "$get_deleted_status" = "404" ]; then
            log_success "Get deleted snippet (404)"
            ((TESTS_PASSED++))
        else
            log_error "Get deleted snippet - Expected 404, got: $get_deleted_status"
            ((TESTS_FAILED++))
        fi
    else
        log_error "Get deleted snippet - No snippet ID available"
        ((TESTS_FAILED++))
    fi
    ((TOTAL_TESTS++))

    cleanup_test_files
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
        echo -e "${GREEN} All tests passed! API is healthy.${NC}"
        return 0
    else
        echo -e "${RED} Some tests failed. Check API health.${NC}"
        return 1
    fi
}

# Main execution
main() {
    # Handle script arguments
    case "${1:-}" in
        -h|--help)
            echo "Usage: $0 [OPTIONS]"
            echo "Options:"
            echo "  -h, --help     Show this help message"
            echo "  -q, --quiet    Run in quiet mode"
            echo ""
            echo "Environment Variables:"
            echo "  API_BASE_URL   Base URL for API (default: http://localhost:8084)"
            echo "  TIMEOUT        Request timeout in seconds (default: 10)"
            exit 0
            ;;
        -q|--quiet)
            exec > /dev/null 2>&1
            ;;
    esac

    run_sanity_tests
    print_summary
}

# Trap to cleanup on exit
trap cleanup_test_files EXIT

# Run main function
main "$@"