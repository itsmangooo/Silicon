package runtime

import (
	"context"
	"errors"
	"io"
	"strings"
)

type Dispatcher struct {
	Local  Provider
	Server Provider
}

func (d Dispatcher) Build(ctx context.Context, spec BuildSpec, archive io.Reader) (string, error) {
	if spec.ServerID == "" {
		return "", errors.New("remote build requires a target server")
	}
	builder, ok := d.Server.(ImageBuilder)
	if !ok {
		return "", errors.New("remote runtime does not support exact-revision builds")
	}
	return builder.Build(ctx, spec, archive)
}

func (d Dispatcher) Deploy(ctx context.Context, spec DeploymentSpec) (InstanceStatus, error) {
	if spec.ServerID != "" {
		if d.Server == nil {
			return InstanceStatus{}, errors.New("server runtime provider is unavailable")
		}
		return d.Server.Deploy(ctx, spec)
	}
	if d.Local == nil {
		return InstanceStatus{}, errors.New("local Docker runtime provider is not enabled; select a connected server")
	}
	return d.Local.Deploy(ctx, spec)
}

func (d Dispatcher) provider(id string) (Provider, error) {
	if strings.HasPrefix(id, "server:") {
		if d.Server == nil {
			return nil, errors.New("server runtime provider is unavailable")
		}
		return d.Server, nil
	}
	if d.Local == nil {
		return nil, errors.New("local Docker runtime provider is unavailable")
	}
	return d.Local, nil
}

func (d Dispatcher) Start(ctx context.Context, id string) error {
	p, e := d.provider(id)
	if e != nil {
		return e
	}
	return p.Start(ctx, id)
}
func (d Dispatcher) Stop(ctx context.Context, id string) error {
	p, e := d.provider(id)
	if e != nil {
		return e
	}
	return p.Stop(ctx, id)
}
func (d Dispatcher) Restart(ctx context.Context, id string) error {
	p, e := d.provider(id)
	if e != nil {
		return e
	}
	return p.Restart(ctx, id)
}
func (d Dispatcher) Remove(ctx context.Context, id string) error {
	p, e := d.provider(id)
	if e != nil {
		return e
	}
	return p.Remove(ctx, id)
}
func (d Dispatcher) Inspect(ctx context.Context, id string) (InstanceStatus, error) {
	p, e := d.provider(id)
	if e != nil {
		return InstanceStatus{}, e
	}
	return p.Inspect(ctx, id)
}
func (d Dispatcher) Status(ctx context.Context, id string) (InstanceStatus, error) {
	p, e := d.provider(id)
	if e != nil {
		return InstanceStatus{}, e
	}
	return p.Status(ctx, id)
}
func (d Dispatcher) Logs(ctx context.Context, id string, request LogRequest) (<-chan LogLine, error) {
	p, e := d.provider(id)
	if e != nil {
		return nil, e
	}
	return p.Logs(ctx, id, request)
}

var _ Provider = Dispatcher{}
var _ ImageBuilder = Dispatcher{}
