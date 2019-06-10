package main

import (
	"flag"
)

const (
	cmdGenerate = "generate"
	cmdInspect  = "inspect"
	cmdVersion  = "version"
	cmdHelp     = "help"
)

var (
	meta string
	part string
)

func init() {
	flag.StringVar(&meta, "meta", "", "The metabyte to set on generated IDs, given in decimal (base10)")
	flag.StringVar(&part, "partition", "", "The partition to set on generated IDs, given in decimal (base10)")
	flag.Parse()
}

func main() {
	var (
		args  = flag.Args()
		argsN = len(args)
	)

	if argsN < 2 {
		// No args at all or "generate" without arg simply passes on to generate one sno.
		// Opts will still get passed through, if they were given.
		if argsN == 0 || args[0] == cmdGenerate {
			generate("1")
		}

		switch args[0] {
		case cmdVersion:
			version()
		case cmdHelp:
			usage()
		}
	} else if argsN == 2 {
		switch args[0] {
		case cmdGenerate:
			generate(args[1])
		case cmdInspect:
			inspect(args[1])
		}
	}

	usage()
}
