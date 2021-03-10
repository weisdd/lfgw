#!/bin/bash
set -o errexit
set -o nounset
set -o pipefail

TARGET=./cmd/lfgw

echo "Running tests:"
go test ./...
echo

echo -n "Checking go vet: "
ERRS=$(go vet ${TARGET} 2>&1 || true)
if [ -n "${ERRS}" ]; then
    echo "FAIL"
    echo "${ERRS}"
    echo
    exit 1
fi
echo "PASS"
echo

echo -n "Checking golint: "
ERRS=$(golint ${TARGET} 2>&1 || true)
if [ -n "${ERRS}" ]; then
    echo "FAIL"
    echo "${ERRS}"
    echo
    exit 1
fi
echo "PASS"
echo

echo -n "Checking golangci-lint: "
ERRS=$(golangci-lint run  ${TARGET} 2>&1 || true)
if [ -n "${ERRS}" ]; then
    echo "FAIL"
    echo "${ERRS}"
    echo
    exit 1
fi
echo "PASS"
echo
