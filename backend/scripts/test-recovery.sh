#!/bin/bash
# Recovery Codes Test Suite Runner
# This script runs all tests for the recovery codes feature

set -e  # Exit on error

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

echo -e "${GREEN}========================================${NC}"
echo -e "${GREEN}Recovery Codes Test Suite${NC}"
echo -e "${GREEN}========================================${NC}"
echo ""

# Change to backend directory
cd "$(dirname "$0")/.." || exit 1

# Check if sqlmock is installed
echo -e "${YELLOW}Checking dependencies...${NC}"
if ! go list -m github.com/DATA/go-sqlmock > /dev/null 2>&1; then
    echo -e "${YELLOW}Installing go-sqlmock...${NC}"
    go get github.com/DATA/go-sqlmock
fi

# Ensure all dependencies are tidy
echo -e "${YELLOW}Running go mod tidy...${NC}"
go mod tidy

echo ""
echo -e "${GREEN}========================================${NC}"
echo -e "${GREEN}1. Running Recovery Service Tests${NC}"
echo -e "${GREEN}========================================${NC}"
go test ./internal/recovery -v -race -coverprofile=coverage-recovery.out

echo ""
echo -e "${GREEN}========================================${NC}"
echo -e "${GREEN}2. Running API Handler Tests${NC}"
echo -e "${GREEN}========================================${NC}"
go test ./internal/api -v -race -run Recovery -coverprofile=coverage-api.out

echo ""
echo -e "${GREEN}========================================${NC}"
echo -e "${GREEN}3. Coverage Report - Recovery Service${NC}"
echo -e "${GREEN}========================================${NC}"
go tool cover -func=coverage-recovery.out | tail -n 1

echo ""
echo -e "${GREEN}========================================${NC}"
echo -e "${GREEN}4. Coverage Report - API Handlers${NC}"
echo -e "${GREEN}========================================${NC}"
go tool cover -func=coverage-api.out | tail -n 1

echo ""
echo -e "${GREEN}========================================${NC}"
echo -e "${GREEN}5. Running Benchmarks${NC}"
echo -e "${GREEN}========================================${NC}"
go test ./internal/recovery -bench=. -benchmem -run=^$

echo ""
echo -e "${GREEN}========================================${NC}"
echo -e "${GREEN}All Tests Passed! ✓${NC}"
echo -e "${GREEN}========================================${NC}"

# Generate HTML coverage reports
echo ""
echo -e "${YELLOW}Generating HTML coverage reports...${NC}"
go tool cover -html=coverage-recovery.out -o coverage-recovery.html
go tool cover -html=coverage-api.out -o coverage-api.html

echo -e "${GREEN}Coverage reports generated:${NC}"
echo -e "  - coverage-recovery.html"
echo -e "  - coverage-api.html"

# Optional: Open coverage in browser
if command -v xdg-open > /dev/null 2>&1; then
    echo ""
    read -p "Open coverage reports in browser? (y/N) " -n 1 -r
    echo
    if [[ $REPLY =~ ^[Yy]$ ]]; then
        xdg-open coverage-recovery.html
        xdg-open coverage-api.html
    fi
fi

echo ""
echo -e "${GREEN}Test suite complete!${NC}"
