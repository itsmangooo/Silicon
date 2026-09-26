package connection

import (
	"context"
	"errors"
	"io"
	"time"
)

var (
	ErrNotConfigured        = errors.New("server connection is not configured")
	ErrUnreachable          = errors.New("server is unreachable")
	ErrAuthenticationFailed = errors.New("server authentication failed")
	ErrDockerUnavailable    = errors.New("Docker is unavailable")
)

type Config struct {
	ServerID           string
	OrganizationID     string
	Type               string
	Host               string
	Port               int
	Username           string
	PrivateKey         []byte
	HostKeyFingerprint string
	PublicAddress      string
}

type Status struct {
	Reachable       bool      `json:"reachable"`
	DockerAvailable bool      `json:"dockerAvailable"`
	DockerVersion   string    `json:"dockerVersion"`
	OperatingSystem string    `json:"operatingSystem"`
	Architecture    string    `json:"architecture"`
	CheckedAt       time.Time `json:"checkedAt"`
}

type Stream struct {
	Stdout io.Reader
	Stderr io.Reader
	Wait   func() error
}

// CommandExecutor is an internal typed-operation primitive. It is deliberately
// not exposed through the HTTP API; callers provide fixed program names and
// validated arguments defined by Silicon providers.
type CommandExecutor interface {
	Output(context.Context, io.Reader, string, ...string) (string, error)
	Run(context.Context, io.Reader, string, ...string) error
	Start(context.Context, string, ...string) (Stream, error)
	WriteFile(context.Context, string, []byte, uint32) (string, func(context.Context) error, error)
}

type TunnelInstallation struct {
	Name  string
	Token []byte
}

type Provider interface {
	Check(context.Context, Config) (Status, error)
	Executor(context.Context, Config) (CommandExecutor, error)
	InstallTunnel(context.Context, Config, TunnelInstallation) error
}

type HostKeyError struct {
	Fingerprint string
	Changed     bool
}

func (e *HostKeyError) Error() string {
	if e.Changed {
		return "SSH host key changed unexpectedly"
	}
	return "SSH host key is not trusted"
}
