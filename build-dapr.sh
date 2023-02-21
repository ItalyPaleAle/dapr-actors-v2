#!/bin/bash

set -e

DIR=$(dirname "$BASH_SOURCE")
echo "Getting into directory ${DIR}/dapr"
cd "${DIR}/dapr"

CGO_ENABLED=0 \
  go build \
  -ldflags="-X github.com/dapr/dapr/pkg/buildinfo.gitversion=v1-dirty -X github.com/dapr/dapr/pkg/buildinfo.version=dev -X github.com/dapr/kit/logger.DaprVersion=dev -s -w" \
  -o ~/.dapr/bin/daprd \
  ./cmd/daprd/;
echo "Built dapr to ~/.dapr/bin/daprd"
