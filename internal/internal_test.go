package internal

import "testing"

func Test(t *testing.T) {
	t.Run("cpu", testCPU)
	t.Run("time", testSnotime)
	t.Run("encoding", testEncoding)
}
