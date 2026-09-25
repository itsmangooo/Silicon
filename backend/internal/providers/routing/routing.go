package routing

import "context"

type Target struct {
	Host string `json:"host"`
	Port int    `json:"port"`
}

type Route struct {
	Hostname string `json:"hostname"`
	Target   Target `json:"target"`
	TLSMode  string `json:"tlsMode"`
}

type Provider interface {
	Name() string
	Describe(context.Context, Route) (Route, error)
}

type ExternalProvider struct{}

func (ExternalProvider) Name() string { return "external" }

func (ExternalProvider) Describe(_ context.Context, route Route) (Route, error) {
	route.TLSMode = "external"
	return route, nil
}
