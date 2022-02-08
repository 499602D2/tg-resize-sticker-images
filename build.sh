#!/bin/bash

SHA=$(git rev-parse --short HEAD)
go build -ldflags "-X main.GitSHA=$SHA" -o tg-resize-sticker-images