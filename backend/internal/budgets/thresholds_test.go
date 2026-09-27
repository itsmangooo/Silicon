package budgets

import (
	"reflect"
	"testing"
)

func TestThresholdsCrossed(t *testing.T) {
	got := ThresholdsCrossed(149, 305, 300, []float64{100, 50, 90, 80})
	want := []float64{50, 80, 90, 100}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %v want %v", got, want)
	}
}
func TestThresholdsCrossedDoesNotRepeat(t *testing.T) {
	if got := ThresholdsCrossed(305, 320, 300, []float64{50, 80, 100}); len(got) != 0 {
		t.Fatalf("repeated thresholds: %v", got)
	}
}
