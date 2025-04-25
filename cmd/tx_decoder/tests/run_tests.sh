#!/bin/bash

# Color codes for pretty output
GREEN='\033[0;32m'
RED='\033[0;31m'
BLUE='\033[0;34m'
YELLOW='\033[0;33m'
NC='\033[0m' # No Color

# Print header
echo -e "${BLUE}=========================================${NC}"
echo -e "${BLUE}   Solana WebSocket Monitoring Tests   ${NC}"
echo -e "${BLUE}=========================================${NC}"
echo

# Set environment variables (override with your own if needed)
export TEST_WS_ENDPOINT=${TEST_WS_ENDPOINT:-"wss://api.mainnet-beta.solana.com"}
export TEST_RPC_ENDPOINT=${TEST_RPC_ENDPOINT:-"https://api.mainnet-beta.solana.com"}
export TEST_ACCOUNT=${TEST_ACCOUNT:-"9xQeWvG816bUx9EPjHmaT23yvVM2ZWbrrpZb9PusVFin"}

# Print test configuration
echo -e "${YELLOW}Test Configuration:${NC}"
echo -e "WebSocket Endpoint: ${TEST_WS_ENDPOINT}"
echo -e "RPC Endpoint: ${TEST_RPC_ENDPOINT}"
echo -e "Test Account: ${TEST_ACCOUNT}"
echo

# Function to run a test and report results
run_test() {
    local test_name=$1
    echo -e "${YELLOW}Running test: ${test_name}${NC}"
    if go test -v -run "^${test_name}$" ./; then
        echo -e "${GREEN}✓ ${test_name} passed${NC}"
        return 0
    else
        echo -e "${RED}✗ ${test_name} failed${NC}"
        return 1
    fi
    echo
}

# Function to run all tests
run_all_tests() {
    echo -e "${YELLOW}Running all tests...${NC}"
    if go test -v ./; then
        echo -e "${GREEN}✓ All tests passed${NC}"
        return 0
    else
        echo -e "${RED}✗ Some tests failed${NC}"
        return 1
    fi
    echo
}

# Parse arguments
if [ $# -eq 0 ]; then
    # No arguments, run specific tests in order
    echo -e "${YELLOW}Running WebSocket connection tests...${NC}"
    run_test "TestWebSocketConnection"
    echo
    
    echo -e "${YELLOW}Running debug printing tests...${NC}"
    run_test "TestDebugPrintingIssue"
    echo
    
    echo -e "${YELLOW}Running context timeout tests...${NC}"
    run_test "TestContextTimeout"
    echo
    
    echo -e "${YELLOW}Running transaction visibility tests...${NC}"
    run_test "TestTransactionVisibility"
    echo
    
    echo -e "${YELLOW}Running commitment level tests...${NC}"
    run_test "TestCompareCommitmentLevels"
    echo
    
    echo -e "${YELLOW}Running WebSocket subscription tests...${NC}"
    run_test "TestLogSubscribeMentions"
    echo
    
    # Add more specific test runs as needed
else
    # Arguments provided
    case "$1" in
        "all")
            run_all_tests
            ;;
        "connection")
            run_test "TestWebSocketConnection"
            ;;
        "logs")
            run_test "TestLogSubscribeMentions"
            run_test "TestLogSubscribeAll"
            ;;
        "account")
            run_test "TestAccountSubscribe"
            ;;
        "transaction")
            run_test "TestTransactionVisibility"
            run_test "TestTransactionRetrieval"
            ;;
        "commitment")
            run_test "TestCompareCommitmentLevels"
            run_test "TestCommitmentLevelDelay"
            ;;
        "debug")
            run_test "TestDebugPrintingIssue"
            run_test "TestMockWebSocketResponse"
            ;;
        *)
            # Assume it's a specific test name
            run_test "$1"
            ;;
    esac
fi

echo -e "${BLUE}=========================================${NC}"
echo -e "${BLUE}           Tests Completed              ${NC}"
echo -e "${BLUE}=========================================${NC}" 