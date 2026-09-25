package deployments

import "testing"

func TestDeploymentStateTransitions(t *testing.T) {
	allowed := [][2]State{{Queued, Preparing}, {Preparing, Building}, {Building, Deploying}, {Deploying, Starting}, {Starting, Healthy}, {Healthy, Superseded}}
	for _, pair := range allowed {
		if err := ValidateTransition(pair[0], pair[1]); err != nil {
			t.Errorf("expected %s -> %s: %v", pair[0], pair[1], err)
		}
	}
	denied := [][2]State{{Queued, Healthy}, {Failed, Preparing}, {Healthy, Deploying}, {RolledBack, Queued}}
	for _, pair := range denied {
		if err := ValidateTransition(pair[0], pair[1]); err == nil {
			t.Errorf("expected %s -> %s to fail", pair[0], pair[1])
		}
	}
}
