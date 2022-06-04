#!/bin/bash

SHA=$(git rev-parse --short HEAD)

LDFLAGS=(
	"-X 'main.GitSHA=$SHA'"
)

echo "Building tg-resize-sticker-images..."
go build -ldflags="${LDFLAGS[*]}" -o tg-resize-sticker-images
echo "Build completed!"