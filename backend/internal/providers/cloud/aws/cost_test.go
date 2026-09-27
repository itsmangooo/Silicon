package aws

import (
	"testing"

	sdk "github.com/aws/aws-sdk-go-v2/aws"
	costtypes "github.com/aws/aws-sdk-go-v2/service/costexplorer/types"
)

func TestNormalizeTagGroupsKeepsRealAttributionAndUnallocated(t *testing.T) {
	groups := normalizeTagGroups("silicon:project", []costtypes.ResultByTime{{Groups: []costtypes.Group{
		{Keys: []string{"silicon:project$11111111-1111-4111-8111-111111111111"}, Metrics: map[string]costtypes.MetricValue{"UnblendedCost": {Amount: sdk.String("12.345"), Unit: sdk.String("USD")}}},
		{Keys: []string{"silicon:project$"}, Metrics: map[string]costtypes.MetricValue{"UnblendedCost": {Amount: sdk.String("7.00"), Unit: sdk.String("USD")}}},
	}}})
	if len(groups) != 2 || groups[0].Name != "11111111-1111-4111-8111-111111111111" || groups[0].Amount != 12.35 || groups[1].Name != "Unallocated" {
		t.Fatalf("unexpected tag cost groups: %#v", groups)
	}
}
