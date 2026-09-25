package runtime

import "context"

type DeploymentSpec struct {
	DeploymentID string
	Application  string
	Image        string
	InternalPort int
	Environment  map[string]string
	SecretNames  []string
}

type InstanceStatus struct {
	InstanceID string
	State      string
	Healthy    bool
}

// Provider is the boundary between Silicon's deployment domain and a runtime.
// Milestone 1 defines the contract but deliberately ships no execution adapter.
type Provider interface {
	Deploy(context.Context, DeploymentSpec) (InstanceStatus, error)
	Start(context.Context, string) error
	Stop(context.Context, string) error
	Restart(context.Context, string) error
	Remove(context.Context, string) error
	Inspect(context.Context, string) (InstanceStatus, error)
	Status(context.Context, string) (InstanceStatus, error)
	Logs(context.Context, string, LogRequest) (<-chan LogLine, error)
}

type LogRequest struct {
	Follow bool
	Tail   int
}

type LogLine struct {
	Timestamp string
	Stream    string
	Message   string
}
