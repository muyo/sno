package main

import (
	"os"

	"github.com/muyo/rush/chars"
	"github.com/muyo/sno"
)

func generate(in string) {
	c, ok := chars.ParseUint64(in)
	if !ok {
		_, _ = os.Stderr.Write([]byte("Need a valid number of IDs to generate.\n"))
		os.Exit(1)
	}

	metabyte, snapshot := parseGenerateOpts()

	g, err := sno.NewGenerator(snapshot, nil)
	if err != nil {
		_, _ = os.Stderr.Write([]byte("Failed to create a generator.\n"))
		os.Exit(1)
	}

	ids := make([]sno.ID, c)
	for i := 0; i < int(c); i++ {
		ids[i] = g.New(metabyte)
	}

	buf := make([]byte, sno.SizeEncoded+1)
	buf[sno.SizeEncoded] = '\n'

	for i := 0; i < int(c); i++ {
		enc, _ := ids[i].MarshalText()
		copy(buf, enc)
		if _, err := os.Stdout.Write(buf); err != nil {
			os.Exit(1)
		}
	}

	os.Exit(0)
}

func parseGenerateOpts() (metabyte byte, snapshot *sno.GeneratorSnapshot) {
	var ok bool

	if meta != "" {
		if metabyte, ok = chars.ParseUint8(meta); !ok {
			_, _ = os.Stderr.Write([]byte("-meta must be a valid base10 number smaller than 256\n"))
			os.Exit(1)
		}
	}

	if part != "" {
		pu16, ok := chars.ParseUint16(part)
		if !ok {
			_, _ = os.Stderr.Write([]byte("-partition must be a valid base10 number smaller than 65536\n"))
			os.Exit(1)
		}

		var partition sno.Partition
		partition.PutUint16(pu16)

		snapshot = &sno.GeneratorSnapshot{
			Partition: partition,
		}
	}

	return
}
