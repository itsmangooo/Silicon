package jobs

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/google/uuid"
	"github.com/itsmangooo/Silicon/backend/internal/deployments"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type DeploymentSpec struct {
	DeploymentID   uuid.UUID
	OrganizationID uuid.UUID
	ApplicationID  uuid.UUID
	Repository     string
	Branch         string
	CommitSHA      string
	Image          string
}
type DeploymentExecutor interface {
	Execute(context.Context, DeploymentSpec, func(deployments.State, string) error) error
}
type UnavailableExecutor struct{}

func (UnavailableExecutor) Execute(context.Context, DeploymentSpec, func(deployments.State, string) error) error {
	return errors.New("no runtime deployment executor is configured")
}

type Runner struct {
	Pool     *pgxpool.Pool
	Executor DeploymentExecutor
	Logger   *slog.Logger
	WorkerID string
}

func (r Runner) Run(ctx context.Context) {
	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if err := r.RunOnce(ctx); err != nil && !errors.Is(err, pgx.ErrNoRows) {
				r.Logger.Error("deployment job failed", "error", err)
			}
		}
	}
}

func (r Runner) RunOnce(ctx context.Context) error {
	tx, err := r.Pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	var jobID, orgID uuid.UUID
	var payload json.RawMessage
	err = tx.QueryRow(ctx, `WITH candidate AS (SELECT id FROM jobs WHERE status='queued' AND job_type='deploy_application' AND available_at<=now() ORDER BY created_at FOR UPDATE SKIP LOCKED LIMIT 1) UPDATE jobs j SET status='running',locked_at=now(),locked_by=$1,attempts=attempts+1,updated_at=now() FROM candidate WHERE j.id=candidate.id RETURNING j.id,j.organization_id,j.payload`, r.WorkerID).Scan(&jobID, &orgID, &payload)
	if err != nil {
		return err
	}
	if err = tx.Commit(ctx); err != nil {
		return err
	}
	var input struct {
		DeploymentID uuid.UUID `json:"deploymentId"`
	}
	if err = json.Unmarshal(payload, &input); err != nil || input.DeploymentID == uuid.Nil {
		return r.failJob(ctx, jobID, fmt.Errorf("invalid deployment job payload"))
	}
	conn, err := r.Pool.Acquire(ctx)
	if err != nil {
		return r.failJob(ctx, jobID, err)
	}
	defer conn.Release()
	var spec DeploymentSpec
	var number int64
	var status string
	err = conn.QueryRow(ctx, `SELECT id,organization_id,application_id,number,status,repository,branch,commit_sha,image FROM deployments WHERE id=$1 AND organization_id=$2`, input.DeploymentID, orgID).Scan(&spec.DeploymentID, &spec.OrganizationID, &spec.ApplicationID, &number, &status, &spec.Repository, &spec.Branch, &spec.CommitSHA, &spec.Image)
	if err != nil {
		return r.failJob(ctx, jobID, err)
	}
	if _, err = conn.Exec(ctx, `SELECT pg_advisory_lock(hashtextextended($1,0))`, spec.ApplicationID.String()); err != nil {
		return r.failJob(ctx, jobID, err)
	}
	defer conn.Exec(context.Background(), `SELECT pg_advisory_unlock(hashtextextended($1,0))`, spec.ApplicationID.String())
	var newer bool
	if err = conn.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM deployments WHERE application_id=$1 AND number>$2)`, spec.ApplicationID, number).Scan(&newer); err != nil {
		return r.failJob(ctx, jobID, err)
	}
	if newer {
		if err = r.setState(ctx, conn, spec.DeploymentID, deployments.Superseded, "Superseded before execution by a newer deployment"); err != nil {
			return r.failJob(ctx, jobID, err)
		}
		return r.succeedJob(ctx, jobID)
	}
	if status != "queued" {
		return r.failJob(ctx, jobID, fmt.Errorf("deployment is %s, expected queued", status))
	}
	preparingMessage := "Deployment executor accepted workload"
	if spec.CommitSHA != "" {
		preparingMessage = "Deployment executor accepted exact revision " + spec.CommitSHA
	}
	if err = r.setState(ctx, conn, spec.DeploymentID, deployments.Preparing, preparingMessage); err != nil {
		return r.failJob(ctx, jobID, err)
	}
	executor := r.Executor
	if executor == nil {
		executor = UnavailableExecutor{}
	}
	progress := func(state deployments.State, message string) error {
		return r.setState(ctx, conn, spec.DeploymentID, state, message)
	}
	if err = executor.Execute(ctx, spec, progress); err != nil {
		_ = r.setState(ctx, conn, spec.DeploymentID, deployments.Failed, "Deployment executor failed: "+safeError(err))
		return r.failJob(ctx, jobID, err)
	}
	var final string
	if err = conn.QueryRow(ctx, `SELECT status FROM deployments WHERE id=$1`, spec.DeploymentID).Scan(&final); err != nil {
		return r.failJob(ctx, jobID, err)
	}
	if final != "healthy" {
		err = errors.New("deployment executor returned without a healthy final state")
		_ = r.setState(ctx, conn, spec.DeploymentID, deployments.Failed, err.Error())
		return r.failJob(ctx, jobID, err)
	}
	return r.succeedJob(ctx, jobID)
}

func (r Runner) setState(ctx context.Context, conn *pgxpool.Conn, id uuid.UUID, to deployments.State, message string) error {
	var from string
	if err := conn.QueryRow(ctx, `SELECT status FROM deployments WHERE id=$1`, id).Scan(&from); err != nil {
		return err
	}
	if err := deployments.ValidateTransition(deployments.State(from), to); err != nil {
		return err
	}
	tag, err := conn.Exec(ctx, `WITH changed AS (UPDATE deployments SET status=$2,started_at=CASE WHEN $2='preparing' THEN COALESCE(started_at,now()) ELSE started_at END,finished_at=CASE WHEN $2 IN ('healthy','failed','cancelled','superseded','rolled_back') THEN now() ELSE finished_at END,updated_at=now() WHERE id=$1 AND status=$3 RETURNING id) INSERT INTO deployment_events(deployment_id,from_status,to_status,message) SELECT id,$3,$2,$4 FROM changed`, id, to, from, message)
	if err == nil && tag.RowsAffected() != 1 {
		return errors.New("deployment state changed concurrently")
	}
	return err
}
func (r Runner) succeedJob(ctx context.Context, id uuid.UUID) error {
	_, err := r.Pool.Exec(ctx, `UPDATE jobs SET status='succeeded',locked_at=NULL,locked_by=NULL,updated_at=now() WHERE id=$1`, id)
	return err
}
func (r Runner) failJob(ctx context.Context, id uuid.UUID, cause error) error {
	message := safeError(cause)
	_, err := r.Pool.Exec(ctx, `UPDATE jobs SET status='failed',last_error=$2,locked_at=NULL,locked_by=NULL,updated_at=now() WHERE id=$1`, id, message)
	if err != nil {
		return err
	}
	return cause
}
func safeError(err error) string {
	if err == nil {
		return ""
	}
	message := err.Error()
	if len(message) > 500 {
		message = message[:500]
	}
	return message
}
