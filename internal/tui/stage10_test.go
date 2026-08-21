package tui

import (
	"testing"
)

func TestAssistantMeasureCap(t *testing.T) {
	if got := assistantMeasure(200); got != assistantMeasureCap {
		t.Errorf("wide: assistantMeasure(200) = %d, want %d", got, assistantMeasureCap)
	}
	if got := assistantMeasure(80); got != 80 {
		t.Errorf("under cap: assistantMeasure(80) = %d, want 80", got)
	}
	if got := assistantMeasure(5); got != 16 {
		t.Errorf("floor: assistantMeasure(5) = %d, want 16", got)
	}
	if assistantMeasureCap < 110 || assistantMeasureCap > 120 {
		t.Errorf("cap %d outside the 110-120 readability range", assistantMeasureCap)
	}
}
