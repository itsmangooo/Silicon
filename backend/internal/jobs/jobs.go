package jobs

import (
	"context"
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

type Type string

const (
	BuildApplication     Type = "build_application"
	DeployApplication    Type = "deploy_application"
	RollbackDeployment   Type = "rollback_deployment"
	CollectRuntimeStatus Type = "collect_runtime_status"
)

type Job struct {
	ID             uuid.UUID
	OrganizationID uuid.UUID
	Type           Type
	Payload        json.RawMessage
	Attempts       int
	AvailableAt    time.Time
}

// Queue describes a PostgreSQL-backed job boundary. Milestone 1 persists the
// schema and vocabulary but intentionally starts no worker or runtime action.
type Queue interface {
	Enqueue(context.Context, Job) (uuid.UUID, error)
	Claim(context.Context, string, time.Duration) (Job, error)
	Succeed(context.Context, uuid.UUID) error
	Fail(context.Context, uuid.UUID, string, time.Time) error
}
