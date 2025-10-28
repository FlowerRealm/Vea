#!/bin/sh
rm -rf artifacts/core/xray/xray-plugin.tmp
go build ./cmd/server
echo done
