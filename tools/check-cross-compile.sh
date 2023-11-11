#!/usr/bin/env bash

set -euo pipefail

SCRIPT_DIR=$( cd -- "$( dirname -- "${BASH_SOURCE[0]}" )" &> /dev/null && pwd )

TOP_DIR=${SCRIPT_DIR}/..

cd ${TOP_DIR}

# Check that tests pass on local os / architecture.

go test ./...

# Check that the library builds on all supported OSs.
#
# Note that "plan9" is in this list to test gateway_unimplemented.go
for os in "darwin" "dragonfly" "freebsd" "netbsd" "openbsd" "plan9" "solaris" "windows"
do
    GOOS=${os} GOARCH=amd64 go build
done
