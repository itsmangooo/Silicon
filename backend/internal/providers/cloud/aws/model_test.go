package aws

import (
	"fmt"
	"strings"
	"testing"

	"github.com/aws/smithy-go"
)

func TestSiliconTagsEnforceOwnership(t *testing.T) {
	tags := tagMap(siliconTags("org-1", "project-1", "environment-1", "api", map[string]string{"team": "platform", "silicon:managed": "false"}))
	if tags["silicon:managed"] != "true" || tags["silicon:organization"] != "org-1" || tags["silicon:project"] != "project-1" || tags["team"] != "platform" {
		t.Fatalf("unexpected ownership tags: %#v", tags)
	}
	if ownership(tags) != OwnershipManaged {
		t.Fatalf("ownership=%s", ownership(tags))
	}
}
func TestWorldAccessibleSecurityRuleRequiresDescription(t *testing.T) {
	_, err := permission(SecurityRule{Direction: "ingress", Protocol: "tcp", CIDRs: []string{"0.0.0.0/0"}})
	if err == nil {
		t.Fatal("expected public rule validation error")
	}
	from, to := int32(443), int32(443)
	_, err = permission(SecurityRule{Direction: "ingress", Protocol: "tcp", FromPort: &from, ToPort: &to, CIDRs: []string{"0.0.0.0/0"}, Description: "Public HTTPS"})
	if err != nil {
		t.Fatal(err)
	}
}
func TestSafeErrorDoesNotExposeProviderMessage(t *testing.T) {
	secret := "AKIA-DO-NOT-LOG"
	err := safeError("connect", &smithy.GenericAPIError{Code: "AccessDenied", Message: secret})
	if strings.Contains(err.Error(), secret) {
		t.Fatalf("credential leaked: %v", err)
	}
	if !strings.Contains(err.Error(), "denied") {
		t.Fatalf("unexpected error: %v", err)
	}
	err = safeError("connect", fmt.Errorf("credential=%s", secret))
	if strings.Contains(err.Error(), secret) {
		t.Fatalf("non-SDK credential leaked: %v", err)
	}
}
