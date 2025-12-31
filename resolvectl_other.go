//go:build !linux
// +build !linux

package main

import "log"

func runResolvectlHelper() {
	log.Fatal("resolvectl-helper is only supported on Linux")
}

func runResolvectlShim() {
	log.Fatal("resolvectl-shim is only supported on Linux")
}
