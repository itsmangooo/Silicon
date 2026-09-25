package tunnel

import "context"

type Tunnel struct {
	ID     string `json:"id"`
	Name   string `json:"name"`
	Status string `json:"status"`
}
type Route struct {
	Hostname string `json:"hostname"`
	Service  string `json:"service"`
}

type Provider interface {
	List(context.Context, string) ([]Tunnel, error)
	Create(context.Context, string, string) (Tunnel, string, error)
	Routes(context.Context, string, string) ([]Route, error)
	ConfigureRoutes(context.Context, string, string, []Route) error
}
