package runtime

import (
	"context"
	"io"
	"time"
)

type DeploymentSpec struct {
	DeploymentID   string
	OrganizationID string
	ApplicationID  string
	Application    string
	Image          string
	PullImage      bool
	InternalPort   int
	HostAddress    string
	HostPort       int
	Environment    map[string]string
	ServerID       string
}

type BuildSpec struct {
	DeploymentID   string
	OrganizationID string
	ApplicationID  string
	ServerID       string
	CommitSHA      string
	Image          string
}

type ImageBuilder interface {
	Build(context.Context, BuildSpec, io.Reader) (string, error)
}

type InstanceStatus struct {
	InstanceID  string     `json:"instanceId"`
	Image       string     `json:"image"`
	State       string     `json:"state"`
	Health      string     `json:"health"`
	Healthy     bool       `json:"healthy"`
	HostAddress string     `json:"hostAddress,omitempty"`
	HostPort    int        `json:"hostPort,omitempty"`
	CreatedAt   *time.Time `json:"createdAt,omitempty"`
	StartedAt   *time.Time `json:"startedAt,omitempty"`
	FinishedAt  *time.Time `json:"finishedAt,omitempty"`
	ServerID    string     `json:"serverId,omitempty"`
}

// Provider is the boundary between Silicon's deployment domain and a runtime.
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
	Since  time.Time
}

type LogLine struct {
	Timestamp time.Time `json:"timestamp"`
	Stream    string    `json:"stream"`
	Message   string    `json:"message"`
}
