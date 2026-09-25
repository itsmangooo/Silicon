package deployments

import "fmt"

type State string

const (
	Queued     State = "queued"
	Preparing  State = "preparing"
	Building   State = "building"
	Deploying  State = "deploying"
	Starting   State = "starting"
	Healthy    State = "healthy"
	Failed     State = "failed"
	Cancelled  State = "cancelled"
	Superseded State = "superseded"
	RolledBack State = "rolled_back"
)

var transitions = map[State]map[State]bool{
	Queued:     {Preparing: true, Cancelled: true, Superseded: true},
	Preparing:  {Building: true, Deploying: true, Failed: true, Cancelled: true, Superseded: true},
	Building:   {Deploying: true, Failed: true, Cancelled: true, Superseded: true},
	Deploying:  {Starting: true, Failed: true, Cancelled: true, Superseded: true},
	Starting:   {Healthy: true, Failed: true, Cancelled: true, Superseded: true},
	Healthy:    {Superseded: true, RolledBack: true},
	Failed:     {},
	Cancelled:  {},
	Superseded: {RolledBack: true},
	RolledBack: {},
}

func (state State) Valid() bool {
	_, ok := transitions[state]
	return ok
}

func CanTransition(from, to State) bool {
	return transitions[from][to]
}

func ValidateTransition(from, to State) error {
	if !from.Valid() || !to.Valid() {
		return fmt.Errorf("unknown deployment state transition %q to %q", from, to)
	}
	if !CanTransition(from, to) {
		return fmt.Errorf("deployment cannot transition from %q to %q", from, to)
	}
	return nil
}
