package main

import (
	"os"
)

const (
	ver = "0.1.2"
)

func version() {
	_, _ = os.Stdout.Write([]byte(ver))
	os.Exit(0)
}
