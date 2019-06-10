package main

import (
	"os"
)

const usageFmt = `
sno generates compact, unique IDs with embedded metadata.

Usage: 

    sno <command> [parameters ...]

Commands:

    inspect   Displays information about an ID and its components

              sno inspect <ID>

    generate  Generates one or more IDs

              sno generate [options...] [number of IDs to generate]
                  --meta=<decimal>        The metabyte to set on generated IDs, in decimal, max 255
                  --partition=<decimal>   The partition to set on generated IDs, in decimal, max 65535

    version   Displays the version of this program
    help      Displays this information
`

func usage() {
	_, _ = os.Stdout.Write([]byte(usageFmt))
	os.Exit(0)
}
