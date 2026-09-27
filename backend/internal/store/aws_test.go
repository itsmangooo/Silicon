package store

import (
	"testing"

	cloudaws "github.com/itsmangooo/Silicon/backend/internal/providers/cloud/aws"
)

func TestCostGroupValuesNeverAttributesUnallocatedSpend(t *testing.T) {
	values := costGroupValues([]cloudaws.CostGroup{{Name: "project-1", Amount: 10.25}, {Name: "project-1", Amount: 2.75}, {Name: "Unallocated", Amount: 40}, {Name: "", Amount: 8}})
	if len(values) != 1 || values["project-1"] != 13 {
		t.Fatalf("unexpected attribution: %#v", values)
	}
}
