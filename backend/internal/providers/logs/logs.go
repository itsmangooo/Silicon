package logs

import "context"

type Query struct {
	OrganizationID string
	ApplicationID  string
	DeploymentID   string
	Search         string
	Follow         bool
}

type Entry struct {
	Timestamp string `json:"timestamp"`
	Stream    string `json:"stream"`
	Service   string `json:"service"`
	Instance  string `json:"instance"`
	Message   string `json:"message"`
}

type Provider interface {
	Stream(context.Context, Query) (<-chan Entry, error)
}
