package reader

import (
	"testing"

	"go.uber.org/goleak"
)

// FSReader runs every read, glob and command in a goroutine of its own, so a
// test that leaves one behind fails here.
func TestMain(m *testing.M) {
	goleak.VerifyTestMain(m)
}
