package main

import (
	"fmt"
	"os"

	"github.com/azhai/sno"
)

const inspectFmt = `
-- Representations

    Encoded: %v
      Bytes: %v

-- Components

       Time: %v
  Timestamp: %v
       Meta: %v
  Partition: %v
   Sequence: %v

`

func inspect(in string) {
	id, err := sno.FromEncodedString(in)
	if err != nil {
		_, _ = os.Stderr.Write([]byte(fmt.Sprintf("Failed to inspect: [%s] does not appear to be a valid sno.\n", in)))
		os.Exit(1)
	}

	fmt.Printf(inspectFmt,
		id.String(),
		id[:],
		id.Time().UTC(),
		id.Timestamp(),
		id.Meta(),
		id.Partition().AsUint16(),
		id.Sequence(),
	)

	os.Exit(0)
}
