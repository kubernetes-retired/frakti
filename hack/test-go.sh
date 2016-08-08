#!/bin/bash
set -o errexit
set -o nounset
set -o pipefail

go test -v $(go list -f '{{if .TestGoFiles}}{{.ImportPath}}{{end}}' ./...)

