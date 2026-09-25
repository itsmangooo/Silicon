package store

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"time"

	"github.com/google/uuid"
	"github.com/itsmangooo/Silicon/backend/internal/deployments"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

var ErrNotFound = errors.New("record not found")

type Repository struct{ Pool *pgxpool.Pool }

type User struct {
	ID           uuid.UUID `json:"id"`
	Email        string    `json:"email"`
	DisplayName  string    `json:"displayName"`
	PasswordHash string    `json:"-"`
	Status       string    `json:"status"`
	CreatedAt    time.Time `json:"createdAt"`
}

type Organization struct {
	ID        uuid.UUID `json:"id"`
	Name      string    `json:"name"`
	Slug      string    `json:"slug"`
	Role      string    `json:"role,omitempty"`
	CreatedAt time.Time `json:"createdAt"`
}

type Member struct {
	UserID      uuid.UUID `json:"userId"`
	Email       string    `json:"email"`
	DisplayName string    `json:"displayName"`
	Role        string    `json:"role"`
	CreatedAt   time.Time `json:"createdAt"`
}

type Project struct {
	ID             uuid.UUID `json:"id"`
	OrganizationID uuid.UUID `json:"organizationId"`
	Name           string    `json:"name"`
	Slug           string    `json:"slug"`
	Description    string    `json:"description"`
	CreatedAt      time.Time `json:"createdAt"`
	UpdatedAt      time.Time `json:"updatedAt"`
}

type Environment struct {
	ID             uuid.UUID `json:"id"`
	OrganizationID uuid.UUID `json:"organizationId"`
	ProjectID      uuid.UUID `json:"projectId"`
	Name           string    `json:"name"`
	Slug           string    `json:"slug"`
	CreatedAt      time.Time `json:"createdAt"`
}

type Application struct {
	ID             uuid.UUID `json:"id"`
	OrganizationID uuid.UUID `json:"organizationId"`
	ProjectID      uuid.UUID `json:"projectId"`
	EnvironmentID  uuid.UUID `json:"environmentId"`
	Name           string    `json:"name"`
	SourceType     string    `json:"sourceType"`
	Image          *string   `json:"image"`
	InternalPort   *int      `json:"internalPort"`
	HostAddress    *string   `json:"hostAddress"`
	PublishedPort  *int      `json:"publishedPort"`
	CreatedAt      time.Time `json:"createdAt"`
}

type Deployment struct {
	ID                   uuid.UUID  `json:"id"`
	OrganizationID       uuid.UUID  `json:"organizationId"`
	ApplicationID        uuid.UUID  `json:"applicationId"`
	Number               int64      `json:"number"`
	Source               string     `json:"source"`
	SourceRevision       string     `json:"sourceRevision"`
	Image                string     `json:"image"`
	Status               string     `json:"status"`
	TriggeredBy          *uuid.UUID `json:"triggeredBy"`
	PreviousDeploymentID *uuid.UUID `json:"previousDeploymentId"`
	Repository           string     `json:"repository"`
	Branch               string     `json:"branch"`
	CommitSHA            string     `json:"commitSha"`
	TriggerType          string     `json:"triggerType"`
	ExternalDeliveryID   *string    `json:"githubDeliveryId,omitempty"`
	CreatedAt            time.Time  `json:"createdAt"`
	UpdatedAt            time.Time  `json:"updatedAt"`
}

type Server struct {
	ID               uuid.UUID  `json:"id"`
	OrganizationID   uuid.UUID  `json:"organizationId"`
	Name             string     `json:"name"`
	Hostname         string     `json:"hostname"`
	OperatingSystem  string     `json:"operatingSystem"`
	Architecture     string     `json:"architecture"`
	Runtime          string     `json:"runtime"`
	ConnectionStatus string     `json:"connectionStatus"`
	Health           string     `json:"health"`
	ConnectivityType string     `json:"connectivityType"`
	CPUCapacity      *int       `json:"cpuCapacity"`
	MemoryBytes      *int64     `json:"memoryBytes"`
	LastSeenAt       *time.Time `json:"lastSeenAt"`
	CreatedAt        time.Time  `json:"createdAt"`
}

type IdentityProvider struct {
	ID             uuid.UUID       `json:"id"`
	OrganizationID uuid.UUID       `json:"organizationId"`
	Name           string          `json:"name"`
	ProviderType   string          `json:"providerType"`
	IssuerURL      string          `json:"issuerUrl"`
	ClientID       string          `json:"clientId"`
	Scopes         []string        `json:"scopes"`
	CallbackURL    string          `json:"callbackUrl"`
	ClaimMapping   json.RawMessage `json:"claimMapping"`
	Enabled        bool            `json:"enabled"`
	CreatedAt      time.Time       `json:"createdAt"`
}

type AuditEvent struct {
	ID             uuid.UUID       `json:"id"`
	OrganizationID *uuid.UUID      `json:"organizationId"`
	ActorUserID    *uuid.UUID      `json:"actorUserId"`
	ActorName      *string         `json:"actorName"`
	Action         string          `json:"action"`
	ResourceType   string          `json:"resourceType"`
	ResourceID     *uuid.UUID      `json:"resourceId"`
	RequestID      uuid.UUID       `json:"requestId"`
	Metadata       json.RawMessage `json:"metadata"`
	CreatedAt      time.Time       `json:"createdAt"`
}

func (r Repository) CreateUser(ctx context.Context, email, displayName, passwordHash string) (User, error) {
	var user User
	err := r.Pool.QueryRow(ctx, `INSERT INTO users(email,display_name,password_hash) VALUES($1,$2,$3)
		RETURNING id,email,display_name,password_hash,status,created_at`, email, displayName, passwordHash).
		Scan(&user.ID, &user.Email, &user.DisplayName, &user.PasswordHash, &user.Status, &user.CreatedAt)
	return user, err
}

func (r Repository) UserByEmail(ctx context.Context, email string) (User, error) {
	var user User
	err := r.Pool.QueryRow(ctx, `SELECT id,email,display_name,password_hash,status,created_at FROM users WHERE email=$1`, email).
		Scan(&user.ID, &user.Email, &user.DisplayName, &user.PasswordHash, &user.Status, &user.CreatedAt)
	return user, notFound(err)
}

func (r Repository) CreateSession(ctx context.Context, userID uuid.UUID, tokenHash, csrfHash []byte, expiresAt time.Time, userAgent string, ip net.IP) error {
	_, err := r.Pool.Exec(ctx, `INSERT INTO sessions(user_id,token_hash,csrf_token_hash,expires_at,user_agent,ip_address) VALUES($1,$2,$3,$4,$5,$6)`, userID, tokenHash, csrfHash, expiresAt, userAgent, ip)
	return err
}

func (r Repository) UserBySession(ctx context.Context, tokenHash []byte) (User, []byte, error) {
	var user User
	var csrfHash []byte
	err := r.Pool.QueryRow(ctx, `UPDATE sessions s SET last_seen_at=now()
		FROM users u WHERE s.token_hash=$1 AND s.expires_at>now() AND u.id=s.user_id AND u.status='active'
		RETURNING u.id,u.email,u.display_name,u.password_hash,u.status,u.created_at,s.csrf_token_hash`, tokenHash).
		Scan(&user.ID, &user.Email, &user.DisplayName, &user.PasswordHash, &user.Status, &user.CreatedAt, &csrfHash)
	return user, csrfHash, notFound(err)
}

func (r Repository) DeleteSession(ctx context.Context, tokenHash []byte) error {
	_, err := r.Pool.Exec(ctx, `DELETE FROM sessions WHERE token_hash=$1`, tokenHash)
	return err
}

func (r Repository) CreateOrganization(ctx context.Context, actor uuid.UUID, name, slug string, requestID uuid.UUID, ip net.IP) (Organization, error) {
	tx, err := r.Pool.Begin(ctx)
	if err != nil {
		return Organization{}, err
	}
	defer tx.Rollback(ctx)
	var org Organization
	err = tx.QueryRow(ctx, `INSERT INTO organizations(name,slug) VALUES($1,$2) RETURNING id,name,slug,created_at`, name, slug).
		Scan(&org.ID, &org.Name, &org.Slug, &org.CreatedAt)
	if err != nil {
		return Organization{}, err
	}
	org.Role = "owner"
	if _, err = tx.Exec(ctx, `INSERT INTO memberships(organization_id,user_id,role) VALUES($1,$2,'owner')`, org.ID, actor); err != nil {
		return Organization{}, err
	}
	if err = insertAudit(ctx, tx, &org.ID, &actor, "organization.created", "organization", &org.ID, requestID, map[string]any{"name": org.Name}, ip); err != nil {
		return Organization{}, err
	}
	if err = tx.Commit(ctx); err != nil {
		return Organization{}, err
	}
	return org, nil
}

func (r Repository) ListOrganizations(ctx context.Context, userID uuid.UUID) ([]Organization, error) {
	rows, err := r.Pool.Query(ctx, `SELECT o.id,o.name,o.slug,m.role,o.created_at FROM organizations o JOIN memberships m ON m.organization_id=o.id WHERE m.user_id=$1 ORDER BY o.name`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := []Organization{}
	for rows.Next() {
		var item Organization
		if err := rows.Scan(&item.ID, &item.Name, &item.Slug, &item.Role, &item.CreatedAt); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (r Repository) MembershipRole(ctx context.Context, organizationID, userID uuid.UUID) (string, error) {
	var role string
	err := r.Pool.QueryRow(ctx, `SELECT role FROM memberships WHERE organization_id=$1 AND user_id=$2`, organizationID, userID).Scan(&role)
	return role, notFound(err)
}

func (r Repository) ListMembers(ctx context.Context, organizationID uuid.UUID) ([]Member, error) {
	rows, err := r.Pool.Query(ctx, `SELECT u.id,u.email,u.display_name,m.role,m.created_at FROM memberships m JOIN users u ON u.id=m.user_id WHERE m.organization_id=$1 ORDER BY u.display_name,u.email`, organizationID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := []Member{}
	for rows.Next() {
		var item Member
		if err := rows.Scan(&item.UserID, &item.Email, &item.DisplayName, &item.Role, &item.CreatedAt); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (r Repository) AddMember(ctx context.Context, organizationID, actorID uuid.UUID, email, role string, requestID uuid.UUID, ip net.IP) (Member, error) {
	tx, err := r.Pool.Begin(ctx)
	if err != nil {
		return Member{}, err
	}
	defer tx.Rollback(ctx)
	var member Member
	err = tx.QueryRow(ctx, `INSERT INTO memberships(organization_id,user_id,role)
		SELECT $1,id,$3 FROM users WHERE email=$2
		RETURNING user_id,role,created_at`, organizationID, email, role).Scan(&member.UserID, &member.Role, &member.CreatedAt)
	if err != nil {
		return Member{}, notFound(err)
	}
	if err = tx.QueryRow(ctx, `SELECT email,display_name FROM users WHERE id=$1`, member.UserID).Scan(&member.Email, &member.DisplayName); err != nil {
		return Member{}, err
	}
	if err = insertAudit(ctx, tx, &organizationID, &actorID, "member.added", "membership", &member.UserID, requestID, map[string]any{"role": role}, ip); err != nil {
		return Member{}, err
	}
	if err = tx.Commit(ctx); err != nil {
		return Member{}, err
	}
	return member, nil
}

func (r Repository) ListProjects(ctx context.Context, organizationID uuid.UUID) ([]Project, error) {
	rows, err := r.Pool.Query(ctx, `SELECT id,organization_id,name,slug,description,created_at,updated_at FROM projects WHERE organization_id=$1 ORDER BY name`, organizationID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := []Project{}
	for rows.Next() {
		var item Project
		if err := rows.Scan(&item.ID, &item.OrganizationID, &item.Name, &item.Slug, &item.Description, &item.CreatedAt, &item.UpdatedAt); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (r Repository) CreateProject(ctx context.Context, organizationID, actorID uuid.UUID, name, slug, description string, requestID uuid.UUID, ip net.IP) (Project, error) {
	tx, err := r.Pool.Begin(ctx)
	if err != nil {
		return Project{}, err
	}
	defer tx.Rollback(ctx)
	var item Project
	err = tx.QueryRow(ctx, `INSERT INTO projects(organization_id,name,slug,description) VALUES($1,$2,$3,$4) RETURNING id,organization_id,name,slug,description,created_at,updated_at`, organizationID, name, slug, description).Scan(&item.ID, &item.OrganizationID, &item.Name, &item.Slug, &item.Description, &item.CreatedAt, &item.UpdatedAt)
	if err != nil {
		return Project{}, err
	}
	if err = insertAudit(ctx, tx, &organizationID, &actorID, "project.created", "project", &item.ID, requestID, map[string]any{"name": name}, ip); err != nil {
		return Project{}, err
	}
	if err = tx.Commit(ctx); err != nil {
		return Project{}, err
	}
	return item, nil
}

func (r Repository) GetProject(ctx context.Context, organizationID, projectID uuid.UUID) (Project, error) {
	var item Project
	err := r.Pool.QueryRow(ctx, `SELECT id,organization_id,name,slug,description,created_at,updated_at FROM projects WHERE organization_id=$1 AND id=$2`, organizationID, projectID).Scan(&item.ID, &item.OrganizationID, &item.Name, &item.Slug, &item.Description, &item.CreatedAt, &item.UpdatedAt)
	return item, notFound(err)
}

func (r Repository) UpdateProject(ctx context.Context, organizationID, projectID, actorID uuid.UUID, name, slug, description string, requestID uuid.UUID, ip net.IP) (Project, error) {
	tx, err := r.Pool.Begin(ctx)
	if err != nil {
		return Project{}, err
	}
	defer tx.Rollback(ctx)
	var item Project
	err = tx.QueryRow(ctx, `UPDATE projects SET name=$3,slug=$4,description=$5,updated_at=now() WHERE organization_id=$1 AND id=$2 RETURNING id,organization_id,name,slug,description,created_at,updated_at`, organizationID, projectID, name, slug, description).Scan(&item.ID, &item.OrganizationID, &item.Name, &item.Slug, &item.Description, &item.CreatedAt, &item.UpdatedAt)
	if err != nil {
		return Project{}, notFound(err)
	}
	if err = insertAudit(ctx, tx, &organizationID, &actorID, "project.updated", "project", &item.ID, requestID, map[string]any{"name": name}, ip); err != nil {
		return Project{}, err
	}
	if err = tx.Commit(ctx); err != nil {
		return Project{}, err
	}
	return item, nil
}

func (r Repository) DeleteProject(ctx context.Context, organizationID, projectID, actorID, requestID uuid.UUID, ip net.IP) error {
	tx, err := r.Pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	tag, err := tx.Exec(ctx, `DELETE FROM projects WHERE organization_id=$1 AND id=$2`, organizationID, projectID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() != 1 {
		return ErrNotFound
	}
	if err = insertAudit(ctx, tx, &organizationID, &actorID, "project.deleted", "project", &projectID, requestID, nil, ip); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func (r Repository) ListEnvironments(ctx context.Context, organizationID, projectID uuid.UUID) ([]Environment, error) {
	rows, err := r.Pool.Query(ctx, `SELECT id,organization_id,project_id,name,slug,created_at FROM environments WHERE organization_id=$1 AND project_id=$2 ORDER BY name`, organizationID, projectID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := []Environment{}
	for rows.Next() {
		var item Environment
		if err := rows.Scan(&item.ID, &item.OrganizationID, &item.ProjectID, &item.Name, &item.Slug, &item.CreatedAt); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (r Repository) CreateEnvironment(ctx context.Context, organizationID, projectID, actorID uuid.UUID, name, slug string, requestID uuid.UUID, ip net.IP) (Environment, error) {
	tx, err := r.Pool.Begin(ctx)
	if err != nil {
		return Environment{}, err
	}
	defer tx.Rollback(ctx)
	var item Environment
	err = tx.QueryRow(ctx, `INSERT INTO environments(organization_id,project_id,name,slug) SELECT $1,p.id,$3,$4 FROM projects p WHERE p.id=$2 AND p.organization_id=$1 RETURNING id,organization_id,project_id,name,slug,created_at`, organizationID, projectID, name, slug).Scan(&item.ID, &item.OrganizationID, &item.ProjectID, &item.Name, &item.Slug, &item.CreatedAt)
	if err != nil {
		return Environment{}, notFound(err)
	}
	if err = insertAudit(ctx, tx, &organizationID, &actorID, "environment.created", "environment", &item.ID, requestID, map[string]any{"name": name}, ip); err != nil {
		return Environment{}, err
	}
	if err = tx.Commit(ctx); err != nil {
		return Environment{}, err
	}
	return item, nil
}

func (r Repository) ListApplications(ctx context.Context, organizationID, environmentID uuid.UUID) ([]Application, error) {
	rows, err := r.Pool.Query(ctx, `SELECT id,organization_id,project_id,environment_id,name,source_type,image,internal_port,host_bind_address::text,published_port,created_at FROM applications WHERE organization_id=$1 AND environment_id=$2 ORDER BY name`, organizationID, environmentID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := []Application{}
	for rows.Next() {
		var item Application
		if err := rows.Scan(&item.ID, &item.OrganizationID, &item.ProjectID, &item.EnvironmentID, &item.Name, &item.SourceType, &item.Image, &item.InternalPort, &item.HostAddress, &item.PublishedPort, &item.CreatedAt); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (r Repository) ListAllApplications(ctx context.Context, organizationID uuid.UUID) ([]Application, error) {
	rows, err := r.Pool.Query(ctx, `SELECT id,organization_id,project_id,environment_id,name,source_type,image,internal_port,host_bind_address::text,published_port,created_at FROM applications WHERE organization_id=$1 ORDER BY name`, organizationID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := []Application{}
	for rows.Next() {
		var item Application
		if err := rows.Scan(&item.ID, &item.OrganizationID, &item.ProjectID, &item.EnvironmentID, &item.Name, &item.SourceType, &item.Image, &item.InternalPort, &item.HostAddress, &item.PublishedPort, &item.CreatedAt); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (r Repository) CreateApplication(ctx context.Context, organizationID, environmentID, actorID uuid.UUID, name, sourceType string, image *string, internalPort *int, hostAddress *string, publishedPort *int, requestID uuid.UUID, ip net.IP) (Application, error) {
	tx, err := r.Pool.Begin(ctx)
	if err != nil {
		return Application{}, err
	}
	defer tx.Rollback(ctx)
	var item Application
	err = tx.QueryRow(ctx, `INSERT INTO applications(organization_id,project_id,environment_id,name,source_type,image,internal_port,host_bind_address,published_port) SELECT $1,e.project_id,e.id,$3,$4,$5,$6,$7,$8 FROM environments e WHERE e.id=$2 AND e.organization_id=$1 RETURNING id,organization_id,project_id,environment_id,name,source_type,image,internal_port,host_bind_address::text,published_port,created_at`, organizationID, environmentID, name, sourceType, image, internalPort, hostAddress, publishedPort).Scan(&item.ID, &item.OrganizationID, &item.ProjectID, &item.EnvironmentID, &item.Name, &item.SourceType, &item.Image, &item.InternalPort, &item.HostAddress, &item.PublishedPort, &item.CreatedAt)
	if err != nil {
		return Application{}, notFound(err)
	}
	if err = insertAudit(ctx, tx, &organizationID, &actorID, "application.created", "application", &item.ID, requestID, map[string]any{"name": name, "sourceType": sourceType}, ip); err != nil {
		return Application{}, err
	}
	if err = tx.Commit(ctx); err != nil {
		return Application{}, err
	}
	return item, nil
}

func (r Repository) ListDeployments(ctx context.Context, organizationID uuid.UUID, applicationID *uuid.UUID) ([]Deployment, error) {
	query := `SELECT id,organization_id,application_id,number,source,source_revision,image,status,triggered_by,previous_deployment_id,repository,branch,commit_sha,trigger_type,external_delivery_id,created_at,updated_at FROM deployments WHERE organization_id=$1`
	args := []any{organizationID}
	if applicationID != nil {
		query += ` AND application_id=$2`
		args = append(args, *applicationID)
	}
	query += ` ORDER BY created_at DESC LIMIT 200`
	rows, err := r.Pool.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := []Deployment{}
	for rows.Next() {
		var item Deployment
		if err := rows.Scan(&item.ID, &item.OrganizationID, &item.ApplicationID, &item.Number, &item.Source, &item.SourceRevision, &item.Image, &item.Status, &item.TriggeredBy, &item.PreviousDeploymentID, &item.Repository, &item.Branch, &item.CommitSHA, &item.TriggerType, &item.ExternalDeliveryID, &item.CreatedAt, &item.UpdatedAt); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (r Repository) CreateDeployment(ctx context.Context, organizationID, applicationID, actorID uuid.UUID, source, revision, image string, requestID uuid.UUID, ip net.IP) (Deployment, error) {
	tx, err := r.Pool.Begin(ctx)
	if err != nil {
		return Deployment{}, err
	}
	defer tx.Rollback(ctx)
	if _, err = tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtextextended($1,0))`, applicationID.String()); err != nil {
		return Deployment{}, err
	}
	var item Deployment
	err = tx.QueryRow(ctx, `INSERT INTO deployments(organization_id,application_id,number,source,source_revision,image,status,triggered_by,previous_deployment_id,repository,branch,commit_sha,trigger_type) SELECT $1,a.id,COALESCE((SELECT max(d.number)+1 FROM deployments d WHERE d.application_id=a.id),1),$3,$4,$5,'queued',$6,(SELECT id FROM deployments d WHERE d.application_id=a.id ORDER BY number DESC LIMIT 1),$3,'',$4,'manual' FROM applications a WHERE a.id=$2 AND a.organization_id=$1 RETURNING id,organization_id,application_id,number,source,source_revision,image,status,triggered_by,previous_deployment_id,repository,branch,commit_sha,trigger_type,external_delivery_id,created_at,updated_at`, organizationID, applicationID, source, revision, image, actorID).Scan(&item.ID, &item.OrganizationID, &item.ApplicationID, &item.Number, &item.Source, &item.SourceRevision, &item.Image, &item.Status, &item.TriggeredBy, &item.PreviousDeploymentID, &item.Repository, &item.Branch, &item.CommitSHA, &item.TriggerType, &item.ExternalDeliveryID, &item.CreatedAt, &item.UpdatedAt)
	if err != nil {
		return Deployment{}, notFound(err)
	}
	if _, err = tx.Exec(ctx, `INSERT INTO deployment_events(deployment_id,to_status,message) VALUES($1,'queued','Deployment queued')`, item.ID); err != nil {
		return Deployment{}, err
	}
	payload, _ := json.Marshal(map[string]any{"deploymentId": item.ID, "repository": source, "commitSha": revision})
	if _, err = tx.Exec(ctx, `INSERT INTO jobs(organization_id,job_type,payload) VALUES($1,'deploy_application',$2)`, organizationID, payload); err != nil {
		return Deployment{}, err
	}
	if err = insertAudit(ctx, tx, &organizationID, &actorID, "deployment.triggered", "deployment", &item.ID, requestID, map[string]any{"number": item.Number}, ip); err != nil {
		return Deployment{}, err
	}
	if err = tx.Commit(ctx); err != nil {
		return Deployment{}, err
	}
	return item, nil
}

func (r Repository) TransitionDeployment(ctx context.Context, organizationID, deploymentID, actorID uuid.UUID, to deployments.State, message string, requestID uuid.UUID, ip net.IP) (Deployment, error) {
	tx, err := r.Pool.Begin(ctx)
	if err != nil {
		return Deployment{}, err
	}
	defer tx.Rollback(ctx)
	var current string
	if err = tx.QueryRow(ctx, `SELECT status FROM deployments WHERE organization_id=$1 AND id=$2 FOR UPDATE`, organizationID, deploymentID).Scan(&current); err != nil {
		return Deployment{}, notFound(err)
	}
	if err = deployments.ValidateTransition(deployments.State(current), to); err != nil {
		return Deployment{}, err
	}
	var item Deployment
	err = tx.QueryRow(ctx, `UPDATE deployments SET status=$3,updated_at=now(),started_at=CASE WHEN $3='preparing' THEN COALESCE(started_at,now()) ELSE started_at END,finished_at=CASE WHEN $3 IN ('healthy','failed','cancelled','superseded','rolled_back') THEN now() ELSE finished_at END WHERE organization_id=$1 AND id=$2 RETURNING id,organization_id,application_id,number,source,source_revision,image,status,triggered_by,previous_deployment_id,repository,branch,commit_sha,trigger_type,external_delivery_id,created_at,updated_at`, organizationID, deploymentID, to).Scan(&item.ID, &item.OrganizationID, &item.ApplicationID, &item.Number, &item.Source, &item.SourceRevision, &item.Image, &item.Status, &item.TriggeredBy, &item.PreviousDeploymentID, &item.Repository, &item.Branch, &item.CommitSHA, &item.TriggerType, &item.ExternalDeliveryID, &item.CreatedAt, &item.UpdatedAt)
	if err != nil {
		return Deployment{}, err
	}
	if _, err = tx.Exec(ctx, `INSERT INTO deployment_events(deployment_id,from_status,to_status,message) VALUES($1,$2,$3,$4)`, deploymentID, current, to, message); err != nil {
		return Deployment{}, err
	}
	if err = insertAudit(ctx, tx, &organizationID, &actorID, "deployment.state_changed", "deployment", &deploymentID, requestID, map[string]any{"from": current, "to": to}, ip); err != nil {
		return Deployment{}, err
	}
	if err = tx.Commit(ctx); err != nil {
		return Deployment{}, err
	}
	return item, nil
}

func (r Repository) ListServers(ctx context.Context, organizationID uuid.UUID) ([]Server, error) {
	rows, err := r.Pool.Query(ctx, `SELECT id,organization_id,name,hostname,operating_system,architecture,runtime,connection_status,health,connectivity_type,cpu_capacity,memory_bytes,last_seen_at,created_at FROM servers WHERE organization_id=$1 ORDER BY name`, organizationID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := []Server{}
	for rows.Next() {
		var item Server
		if err := rows.Scan(&item.ID, &item.OrganizationID, &item.Name, &item.Hostname, &item.OperatingSystem, &item.Architecture, &item.Runtime, &item.ConnectionStatus, &item.Health, &item.ConnectivityType, &item.CPUCapacity, &item.MemoryBytes, &item.LastSeenAt, &item.CreatedAt); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (r Repository) CreateServer(ctx context.Context, organizationID, actorID uuid.UUID, name, hostname, osName, architecture, connectivityType string, requestID uuid.UUID, ip net.IP) (Server, error) {
	tx, err := r.Pool.Begin(ctx)
	if err != nil {
		return Server{}, err
	}
	defer tx.Rollback(ctx)
	var item Server
	err = tx.QueryRow(ctx, `INSERT INTO servers(organization_id,name,hostname,operating_system,architecture,connectivity_type) VALUES($1,$2,$3,$4,$5,$6) RETURNING id,organization_id,name,hostname,operating_system,architecture,runtime,connection_status,health,connectivity_type,cpu_capacity,memory_bytes,last_seen_at,created_at`, organizationID, name, hostname, osName, architecture, connectivityType).Scan(&item.ID, &item.OrganizationID, &item.Name, &item.Hostname, &item.OperatingSystem, &item.Architecture, &item.Runtime, &item.ConnectionStatus, &item.Health, &item.ConnectivityType, &item.CPUCapacity, &item.MemoryBytes, &item.LastSeenAt, &item.CreatedAt)
	if err != nil {
		return Server{}, err
	}
	if err = insertAudit(ctx, tx, &organizationID, &actorID, "server.created", "server", &item.ID, requestID, map[string]any{"name": name}, ip); err != nil {
		return Server{}, err
	}
	if err = tx.Commit(ctx); err != nil {
		return Server{}, err
	}
	return item, nil
}

func (r Repository) ListIdentityProviders(ctx context.Context, organizationID uuid.UUID) ([]IdentityProvider, error) {
	rows, err := r.Pool.Query(ctx, `SELECT id,organization_id,name,provider_type,issuer_url,client_id,scopes,callback_url,claim_mapping,enabled,created_at FROM identity_providers WHERE organization_id=$1 ORDER BY name`, organizationID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := []IdentityProvider{}
	for rows.Next() {
		var item IdentityProvider
		if err := rows.Scan(&item.ID, &item.OrganizationID, &item.Name, &item.ProviderType, &item.IssuerURL, &item.ClientID, &item.Scopes, &item.CallbackURL, &item.ClaimMapping, &item.Enabled, &item.CreatedAt); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (r Repository) CreateIdentityProvider(ctx context.Context, organizationID, actorID uuid.UUID, name, providerType, issuerURL, clientID, callbackURL string, scopes []string, requestID uuid.UUID, ip net.IP) (IdentityProvider, error) {
	tx, err := r.Pool.Begin(ctx)
	if err != nil {
		return IdentityProvider{}, err
	}
	defer tx.Rollback(ctx)
	var item IdentityProvider
	err = tx.QueryRow(ctx, `INSERT INTO identity_providers(organization_id,name,provider_type,issuer_url,client_id,scopes,callback_url) VALUES($1,$2,$3,$4,$5,$6,$7) RETURNING id,organization_id,name,provider_type,issuer_url,client_id,scopes,callback_url,claim_mapping,enabled,created_at`, organizationID, name, providerType, issuerURL, clientID, scopes, callbackURL).Scan(&item.ID, &item.OrganizationID, &item.Name, &item.ProviderType, &item.IssuerURL, &item.ClientID, &item.Scopes, &item.CallbackURL, &item.ClaimMapping, &item.Enabled, &item.CreatedAt)
	if err != nil {
		return IdentityProvider{}, err
	}
	if err = insertAudit(ctx, tx, &organizationID, &actorID, "identity_provider.created", "identity_provider", &item.ID, requestID, map[string]any{"name": name, "type": providerType}, ip); err != nil {
		return IdentityProvider{}, err
	}
	if err = tx.Commit(ctx); err != nil {
		return IdentityProvider{}, err
	}
	return item, nil
}

func (r Repository) ListAuditEvents(ctx context.Context, organizationID uuid.UUID) ([]AuditEvent, error) {
	rows, err := r.Pool.Query(ctx, `SELECT a.id,a.organization_id,a.actor_user_id,u.display_name,a.action,a.resource_type,a.resource_id,a.request_id,a.metadata,a.created_at FROM audit_events a LEFT JOIN users u ON u.id=a.actor_user_id WHERE a.organization_id=$1 ORDER BY a.created_at DESC LIMIT 250`, organizationID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := []AuditEvent{}
	for rows.Next() {
		var item AuditEvent
		if err := rows.Scan(&item.ID, &item.OrganizationID, &item.ActorUserID, &item.ActorName, &item.Action, &item.ResourceType, &item.ResourceID, &item.RequestID, &item.Metadata, &item.CreatedAt); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (r Repository) RecordAuthAudit(ctx context.Context, actorID *uuid.UUID, action string, requestID uuid.UUID, ip net.IP, metadata map[string]any) error {
	return insertAudit(ctx, r.Pool, nil, actorID, action, "session", nil, requestID, metadata, ip)
}

func insertAudit(ctx context.Context, q interface {
	Exec(context.Context, string, ...any) (pgconn.CommandTag, error)
}, organizationID, actorID *uuid.UUID, action, resourceType string, resourceID *uuid.UUID, requestID uuid.UUID, metadata map[string]any, ip net.IP) error {
	if metadata == nil {
		metadata = map[string]any{}
	}
	body, err := json.Marshal(metadata)
	if err != nil {
		return err
	}
	_, err = q.Exec(ctx, `INSERT INTO audit_events(organization_id,actor_user_id,action,resource_type,resource_id,request_id,metadata,ip_address) VALUES($1,$2,$3,$4,$5,$6,$7,$8)`, organizationID, actorID, action, resourceType, resourceID, requestID, body, ip)
	return err
}

func notFound(err error) error {
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrNotFound
	}
	return err
}

func WrapConstraint(err error) error {
	if err == nil {
		return nil
	}
	return fmt.Errorf("persistence: %w", err)
}
