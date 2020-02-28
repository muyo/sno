package main

import (
	"os"
)

const (
	ver = "1.0.0"
)

func version() {
	_, _ = os.Stdout.Write([]byte(ver))
	os.Exit(0)
}
