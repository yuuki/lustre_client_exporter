package parser

import (
	"reflect"
	"testing"
)

// assertObservations fails unless got equals want element by element, in
// order. A nil want also requires a nil got, so that "no observations" stays
// distinguishable from an empty, allocated slice.
func assertObservations(t *testing.T, got, want []Observation) {
	t.Helper()
	if (want == nil) != (got == nil) {
		t.Fatalf("got nil=%v (%d observations), want nil=%v", got == nil, len(got), want == nil)
	}
	n := max(len(got), len(want))
	for i := range n {
		switch {
		case i >= len(got):
			t.Errorf("[%d] missing, want %+v", i, want[i])
		case i >= len(want):
			t.Errorf("[%d] unexpected %+v", i, got[i])
		case !reflect.DeepEqual(got[i], want[i]):
			t.Errorf("[%d]\n got %+v\nwant %+v", i, got[i], want[i])
		}
	}
}

// assertParseError fails unless err has exactly the text want and the parser
// returned no observations.
func assertParseError(t *testing.T, obs []Observation, err error, want string) {
	t.Helper()
	if err == nil || err.Error() != want {
		t.Fatalf("err = %v, want %q", err, want)
	}
	if obs != nil {
		t.Fatalf("got %d observations alongside the error, want nil", len(obs))
	}
}
