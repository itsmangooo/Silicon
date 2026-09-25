package git

import (
	"context"
	"io"
)

type Repository struct {
	ID            int64  `json:"id"`
	FullName      string `json:"fullName"`
	DefaultBranch string `json:"defaultBranch"`
	Private       bool   `json:"private"`
}

type Installation struct {
	ID      int64  `json:"id"`
	Account string `json:"account"`
}

type Provider interface {
	Installation(context.Context, int64) (Installation, error)
	ListRepositories(context.Context, int64) ([]Repository, error)
	Archive(context.Context, int64, string, string) (io.ReadCloser, error)
}
