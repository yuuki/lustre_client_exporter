package collector

import (
	"testing"

	"go.uber.org/goleak"
)

// Registry.Collect runs each collector in a goroutine of its own and must not
// return before all of them have finished.
func TestMain(m *testing.M) {
	goleak.VerifyTestMain(m)
}
