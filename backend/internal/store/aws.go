package store

import (
	"context"
	"encoding/json"
	"errors"
	"net"
	"time"

	"github.com/google/uuid"
	"github.com/itsmangooo/Silicon/backend/internal/budgets"
	cloudaws "github.com/itsmangooo/Silicon/backend/internal/providers/cloud/aws"
)

type AWSAccount struct {
	ID                   uuid.UUID  `json:"id"`
	OrganizationID       uuid.UUID  `json:"organizationId"`
	DisplayName          string     `json:"displayName"`
	AccountID            string     `json:"accountId"`
	RoleARN              string     `json:"roleArn"`
	ExternalIDConfigured bool       `json:"externalIdConfigured"`
	StaticKeysConfigured bool       `json:"staticKeysConfigured"`
	DefaultRegion        string     `json:"defaultRegion"`
	EnabledRegions       []string   `json:"enabledRegions"`
	Status               string     `json:"status"`
	LastError            string     `json:"lastError,omitempty"`
	LastCheckedAt        *time.Time `json:"lastCheckedAt,omitempty"`
	CreatedAt            time.Time  `json:"createdAt"`
}

type AWSAccountConnection struct {
	AWSAccount
	EncryptedExternalID      []byte
	EncryptedAccessKeyID     []byte
	EncryptedSecretAccessKey []byte
}

type AWSAccountInput struct {
	ID                       uuid.UUID
	DisplayName              string
	AccountID                string
	RoleARN                  string
	EncryptedExternalID      []byte
	EncryptedAccessKeyID     []byte
	EncryptedSecretAccessKey []byte
	DefaultRegion            string
	EnabledRegions           []string
}

type AWSResourceOwnership struct {
	ID                 uuid.UUID  `json:"id"`
	OrganizationID     uuid.UUID  `json:"organizationId"`
	AccountID          uuid.UUID  `json:"accountId"`
	Region             string     `json:"region"`
	ResourceType       string     `json:"resourceType"`
	ProviderResourceID string     `json:"providerResourceId"`
	Ownership          string     `json:"ownership"`
	ProjectID          *uuid.UUID `json:"projectId,omitempty"`
	EnvironmentID      *uuid.UUID `json:"environmentId,omitempty"`
}

type AWSInstance struct {
	ID                 uuid.UUID  `json:"id"`
	OrganizationID     uuid.UUID  `json:"organizationId"`
	AccountID          uuid.UUID  `json:"accountId"`
	ServerID           *uuid.UUID `json:"serverId,omitempty"`
	ProviderInstanceID string     `json:"providerInstanceId"`
	Region             string     `json:"region"`
	Name               string     `json:"name"`
	State              string     `json:"state"`
	InstanceType       string     `json:"instanceType"`
	Architecture       string     `json:"architecture"`
	AvailabilityZone   string     `json:"availabilityZone"`
	ImageID            string     `json:"imageId"`
	PrivateIP          string     `json:"privateIp"`
	PublicIP           string     `json:"publicIp"`
	VPCID              string     `json:"vpcId"`
	SubnetID           string     `json:"subnetId"`
	SecurityGroupIDs   []string   `json:"securityGroupIds"`
	Ownership          string     `json:"ownership"`
	ConnectionMethod   string     `json:"connectionMethod"`
	LaunchedAt         *time.Time `json:"launchedAt,omitempty"`
}

type AWSOperation struct {
	ID                 uuid.UUID       `json:"id"`
	OrganizationID     uuid.UUID       `json:"organizationId"`
	AccountID          uuid.UUID       `json:"accountId"`
	JobID              *uuid.UUID      `json:"jobId,omitempty"`
	OperationType      string          `json:"operationType"`
	Status             string          `json:"status"`
	ResourceType       string          `json:"resourceType"`
	ProviderResourceID string          `json:"providerResourceId"`
	Request            json.RawMessage `json:"-"`
	Error              string          `json:"error,omitempty"`
	CreatedBy          *uuid.UUID      `json:"createdBy,omitempty"`
	CreatedAt          time.Time       `json:"createdAt"`
	UpdatedAt          time.Time       `json:"updatedAt"`
	CompletedAt        *time.Time      `json:"completedAt,omitempty"`
}

type Budget struct {
	ID                     uuid.UUID  `json:"id"`
	OrganizationID         uuid.UUID  `json:"organizationId"`
	AccountID              *uuid.UUID `json:"accountId,omitempty"`
	ProjectID              *uuid.UUID `json:"projectId,omitempty"`
	EnvironmentID          *uuid.UUID `json:"environmentId,omitempty"`
	Name                   string     `json:"name"`
	MonthlyAmount          float64    `json:"monthlyAmount"`
	Currency               string     `json:"currency"`
	Thresholds             []float64  `json:"thresholds"`
	PreventNewProvisioning bool       `json:"preventNewProvisioning"`
	ProviderType           string     `json:"providerType"`
	ProviderBudgetName     string     `json:"providerBudgetName,omitempty"`
	LastEvaluatedAmount    *float64   `json:"lastEvaluatedAmount,omitempty"`
	LastEvaluatedAt        *time.Time `json:"lastEvaluatedAt,omitempty"`
	CreatedAt              time.Time  `json:"createdAt"`
}

func (r Repository) ListAWSAccounts(ctx context.Context, organizationID uuid.UUID) ([]AWSAccount, error) {
	rows, err := r.Pool.Query(ctx, `SELECT id,organization_id,display_name,account_id,role_arn,(encrypted_external_id IS NOT NULL),(encrypted_access_key_id IS NOT NULL),default_region,enabled_regions,status,last_error,last_checked_at,created_at FROM aws_accounts WHERE organization_id=$1 ORDER BY display_name`, organizationID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := []AWSAccount{}
	for rows.Next() {
		var item AWSAccount
		if err = rows.Scan(&item.ID, &item.OrganizationID, &item.DisplayName, &item.AccountID, &item.RoleARN, &item.ExternalIDConfigured, &item.StaticKeysConfigured, &item.DefaultRegion, &item.EnabledRegions, &item.Status, &item.LastError, &item.LastCheckedAt, &item.CreatedAt); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (r Repository) AWSAccountConnection(ctx context.Context, organizationID, accountID uuid.UUID) (AWSAccountConnection, error) {
	var item AWSAccountConnection
	err := r.Pool.QueryRow(ctx, `SELECT id,organization_id,display_name,account_id,role_arn,(encrypted_external_id IS NOT NULL),(encrypted_access_key_id IS NOT NULL),default_region,enabled_regions,status,last_error,last_checked_at,created_at,encrypted_external_id,encrypted_access_key_id,encrypted_secret_access_key FROM aws_accounts WHERE organization_id=$1 AND id=$2`, organizationID, accountID).Scan(&item.ID, &item.OrganizationID, &item.DisplayName, &item.AccountID, &item.RoleARN, &item.ExternalIDConfigured, &item.StaticKeysConfigured, &item.DefaultRegion, &item.EnabledRegions, &item.Status, &item.LastError, &item.LastCheckedAt, &item.CreatedAt, &item.EncryptedExternalID, &item.EncryptedAccessKeyID, &item.EncryptedSecretAccessKey)
	return item, notFound(err)
}

func (r Repository) CreateAWSAccount(ctx context.Context, organizationID, actorID uuid.UUID, input AWSAccountInput, requestID uuid.UUID, ip net.IP) (AWSAccount, error) {
	tx, err := r.Pool.Begin(ctx)
	if err != nil {
		return AWSAccount{}, err
	}
	defer tx.Rollback(ctx)
	var item AWSAccount
	err = tx.QueryRow(ctx, `INSERT INTO aws_accounts(id,organization_id,display_name,account_id,role_arn,encrypted_external_id,encrypted_access_key_id,encrypted_secret_access_key,default_region,enabled_regions,connected_by,last_checked_at) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,now()) RETURNING id,organization_id,display_name,account_id,role_arn,(encrypted_external_id IS NOT NULL),(encrypted_access_key_id IS NOT NULL),default_region,enabled_regions,status,last_error,last_checked_at,created_at`, input.ID, organizationID, input.DisplayName, input.AccountID, input.RoleARN, input.EncryptedExternalID, input.EncryptedAccessKeyID, input.EncryptedSecretAccessKey, input.DefaultRegion, input.EnabledRegions, actorID).Scan(&item.ID, &item.OrganizationID, &item.DisplayName, &item.AccountID, &item.RoleARN, &item.ExternalIDConfigured, &item.StaticKeysConfigured, &item.DefaultRegion, &item.EnabledRegions, &item.Status, &item.LastError, &item.LastCheckedAt, &item.CreatedAt)
	if err != nil {
		return AWSAccount{}, err
	}
	if err = insertAudit(ctx, tx, &organizationID, &actorID, "aws.account_connected", "aws_account", &item.ID, requestID, map[string]any{"accountId": item.AccountID, "roleArn": item.RoleARN, "defaultRegion": item.DefaultRegion}, ip); err != nil {
		return AWSAccount{}, err
	}
	return item, tx.Commit(ctx)
}

func (r Repository) UpdateAWSAccountHealth(ctx context.Context, organizationID, accountID uuid.UUID, status, lastError string) error {
	tag, err := r.Pool.Exec(ctx, `UPDATE aws_accounts SET status=$3,last_error=$4,last_checked_at=now(),updated_at=now() WHERE organization_id=$1 AND id=$2`, organizationID, accountID, status, lastError)
	if err == nil && tag.RowsAffected() != 1 {
		return ErrNotFound
	}
	return err
}
func (r Repository) DeleteAWSAccount(ctx context.Context, organizationID, accountID, actorID, requestID uuid.UUID, ip net.IP) error {
	tx, err := r.Pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	tag, err := tx.Exec(ctx, `DELETE FROM aws_accounts WHERE organization_id=$1 AND id=$2 AND NOT EXISTS(SELECT 1 FROM aws_instances WHERE account_id=$2)`, organizationID, accountID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() != 1 {
		return ErrNotFound
	}
	if err = insertAudit(ctx, tx, &organizationID, &actorID, "aws.account_disconnected", "aws_account", &accountID, requestID, nil, ip); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func (r Repository) ResourceOwnership(ctx context.Context, organizationID, accountID uuid.UUID, region, resourceType, providerID string) (AWSResourceOwnership, error) {
	var item AWSResourceOwnership
	err := r.Pool.QueryRow(ctx, `SELECT id,organization_id,account_id,region,resource_type,provider_resource_id,ownership,project_id,environment_id FROM aws_resource_ownership WHERE organization_id=$1 AND account_id=$2 AND region=$3 AND resource_type=$4 AND provider_resource_id=$5`, organizationID, accountID, region, resourceType, providerID).Scan(&item.ID, &item.OrganizationID, &item.AccountID, &item.Region, &item.ResourceType, &item.ProviderResourceID, &item.Ownership, &item.ProjectID, &item.EnvironmentID)
	return item, notFound(err)
}

func (r Repository) ValidateAWSResourceScope(ctx context.Context, organizationID uuid.UUID, projectID, environmentID *uuid.UUID) error {
	if projectID != nil {
		var exists bool
		if err := r.Pool.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM projects WHERE organization_id=$1 AND id=$2)`, organizationID, projectID).Scan(&exists); err != nil {
			return err
		}
		if !exists {
			return ErrNotFound
		}
	}
	if environmentID != nil {
		var environmentProjectID uuid.UUID
		if err := r.Pool.QueryRow(ctx, `SELECT project_id FROM environments WHERE organization_id=$1 AND id=$2`, organizationID, environmentID).Scan(&environmentProjectID); err != nil {
			return notFound(err)
		}
		if projectID != nil && environmentProjectID != *projectID {
			return ErrNotFound
		}
	}
	return nil
}
func (r Repository) ClaimAWSResource(ctx context.Context, organizationID, accountID, actorID uuid.UUID, region, resourceType, providerID, ownership string, projectID, environmentID *uuid.UUID, requestID uuid.UUID, ip net.IP) (AWSResourceOwnership, error) {
	tx, err := r.Pool.Begin(ctx)
	if err != nil {
		return AWSResourceOwnership{}, err
	}
	defer tx.Rollback(ctx)
	var item AWSResourceOwnership
	err = tx.QueryRow(ctx, `INSERT INTO aws_resource_ownership(organization_id,account_id,region,resource_type,provider_resource_id,ownership,project_id,environment_id) VALUES($1,$2,$3,$4,$5,$6,$7,$8) ON CONFLICT(organization_id,account_id,region,resource_type,provider_resource_id) DO UPDATE SET ownership=EXCLUDED.ownership,project_id=EXCLUDED.project_id,environment_id=EXCLUDED.environment_id,updated_at=now() RETURNING id,organization_id,account_id,region,resource_type,provider_resource_id,ownership,project_id,environment_id`, organizationID, accountID, region, resourceType, providerID, ownership, projectID, environmentID).Scan(&item.ID, &item.OrganizationID, &item.AccountID, &item.Region, &item.ResourceType, &item.ProviderResourceID, &item.Ownership, &item.ProjectID, &item.EnvironmentID)
	if err != nil {
		return AWSResourceOwnership{}, err
	}
	action := "aws." + resourceType + "_claimed"
	if ownership == "managed" {
		action = "aws." + resourceType + "_created"
	} else if ownership == "imported" {
		action = "aws." + resourceType + "_imported"
	}
	if err = insertAudit(ctx, tx, &organizationID, &actorID, action, "aws_"+resourceType, &item.ID, requestID, map[string]any{"providerResourceId": providerID, "region": region, "ownership": ownership}, ip); err != nil {
		return AWSResourceOwnership{}, err
	}
	return item, tx.Commit(ctx)
}

func (r Repository) ListAWSInstances(ctx context.Context, organizationID, accountID uuid.UUID) ([]AWSInstance, error) {
	rows, err := r.Pool.Query(ctx, `SELECT id,organization_id,account_id,server_id,provider_instance_id,region,name,state,instance_type,architecture,availability_zone,image_id,private_ip,public_ip,vpc_id,subnet_id,security_group_ids,ownership,connection_method,launched_at FROM aws_instances WHERE organization_id=$1 AND account_id=$2 ORDER BY name,provider_instance_id`, organizationID, accountID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := []AWSInstance{}
	for rows.Next() {
		var item AWSInstance
		if err = rows.Scan(&item.ID, &item.OrganizationID, &item.AccountID, &item.ServerID, &item.ProviderInstanceID, &item.Region, &item.Name, &item.State, &item.InstanceType, &item.Architecture, &item.AvailabilityZone, &item.ImageID, &item.PrivateIP, &item.PublicIP, &item.VPCID, &item.SubnetID, &item.SecurityGroupIDs, &item.Ownership, &item.ConnectionMethod, &item.LaunchedAt); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (r Repository) SaveAWSInstance(ctx context.Context, organizationID, accountID, actorID, serverID uuid.UUID, item cloudaws.Instance, ownership, connectionMethod string, encryptedPrivateKey []byte, sshUser string, requestID uuid.UUID, ip net.IP) (AWSInstance, error) {
	tx, err := r.Pool.Begin(ctx)
	if err != nil {
		return AWSInstance{}, err
	}
	defer tx.Rollback(ctx)
	var alreadyImported bool
	if err = tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM aws_instances WHERE organization_id=$1 AND account_id=$2 AND region=$3 AND provider_instance_id=$4)`, organizationID, accountID, item.Region, item.ID).Scan(&alreadyImported); err != nil {
		return AWSInstance{}, err
	}
	if alreadyImported {
		return AWSInstance{}, errors.New("AWS instance is already attached to Silicon")
	}
	if serverID == uuid.Nil {
		serverID = uuid.New()
	}
	securityGroupIDs := item.SecurityGroupIDs
	if securityGroupIDs == nil {
		securityGroupIDs = []string{}
	}
	address := item.PrivateIP
	connectivity := "private"
	if item.PublicIP != "" {
		address = item.PublicIP
		connectivity = "public"
	}
	connectionType := connectionMethod
	if connectionType == "" {
		connectionType = "aws_ssm"
	}
	hostname := address
	if connectionType == "aws_ssm" {
		hostname = item.ID
	}
	_, err = tx.Exec(ctx, `INSERT INTO servers(id,organization_id,name,hostname,operating_system,architecture,connectivity_type,connection_type,public_address,ssh_port,ssh_username,ssh_private_key_encrypted,connection_status,provider_type,aws_account_id,aws_instance_id,aws_region) VALUES($1,$2,$3,$4,'linux',$5,$6,$7,$8,22,$9,$10,'unreachable','aws',$11,$12,$13)`, serverID, organizationID, item.Name, hostname, item.Architecture, connectivity, connectionType, item.PublicIP, sshUser, encryptedPrivateKey, accountID, item.ID, item.Region)
	if err != nil {
		return AWSInstance{}, err
	}
	var saved AWSInstance
	err = tx.QueryRow(ctx, `INSERT INTO aws_instances(organization_id,account_id,server_id,provider_instance_id,region,name,state,instance_type,architecture,availability_zone,image_id,private_ip,public_ip,vpc_id,subnet_id,security_group_ids,ownership,connection_method,launched_at) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,$17,$18,$19) RETURNING id,organization_id,account_id,server_id,provider_instance_id,region,name,state,instance_type,architecture,availability_zone,image_id,private_ip,public_ip,vpc_id,subnet_id,security_group_ids,ownership,connection_method,launched_at`, organizationID, accountID, serverID, item.ID, item.Region, item.Name, item.State, item.InstanceType, item.Architecture, item.AvailabilityZone, item.ImageID, item.PrivateIP, item.PublicIP, item.VPCID, item.SubnetID, securityGroupIDs, ownership, connectionMethod, item.LaunchedAt).Scan(&saved.ID, &saved.OrganizationID, &saved.AccountID, &saved.ServerID, &saved.ProviderInstanceID, &saved.Region, &saved.Name, &saved.State, &saved.InstanceType, &saved.Architecture, &saved.AvailabilityZone, &saved.ImageID, &saved.PrivateIP, &saved.PublicIP, &saved.VPCID, &saved.SubnetID, &saved.SecurityGroupIDs, &saved.Ownership, &saved.ConnectionMethod, &saved.LaunchedAt)
	if err != nil {
		return AWSInstance{}, err
	}
	_, err = tx.Exec(ctx, `INSERT INTO aws_resource_ownership(organization_id,account_id,region,resource_type,provider_resource_id,ownership) VALUES($1,$2,$3,'instance',$4,$5) ON CONFLICT(organization_id,account_id,region,resource_type,provider_resource_id) DO UPDATE SET ownership=EXCLUDED.ownership,updated_at=now()`, organizationID, accountID, item.Region, item.ID, ownership)
	if err != nil {
		return AWSInstance{}, err
	}
	action := "aws.instance_imported"
	if ownership == "managed" {
		action = "aws.instance_created"
	}
	if err = insertAudit(ctx, tx, &organizationID, &actorID, action, "aws_instance", &saved.ID, requestID, map[string]any{"instanceId": item.ID, "region": item.Region, "ownership": ownership, "serverId": serverID}, ip); err != nil {
		return AWSInstance{}, err
	}
	return saved, tx.Commit(ctx)
}

func (r Repository) CreateAWSOperation(ctx context.Context, organizationID, accountID, actorID uuid.UUID, operationType, jobType string, request any, requestID uuid.UUID, ip net.IP) (AWSOperation, error) {
	body, err := json.Marshal(request)
	if err != nil {
		return AWSOperation{}, err
	}
	tx, err := r.Pool.Begin(ctx)
	if err != nil {
		return AWSOperation{}, err
	}
	defer tx.Rollback(ctx)
	var operation AWSOperation
	err = tx.QueryRow(ctx, `INSERT INTO aws_operations(organization_id,account_id,operation_type,request,created_by) VALUES($1,$2,$3,$4,$5) RETURNING id,organization_id,account_id,job_id,operation_type,status,resource_type,provider_resource_id,request,error,created_by,created_at,updated_at,completed_at`, organizationID, accountID, operationType, body, actorID).Scan(&operation.ID, &operation.OrganizationID, &operation.AccountID, &operation.JobID, &operation.OperationType, &operation.Status, &operation.ResourceType, &operation.ProviderResourceID, &operation.Request, &operation.Error, &operation.CreatedBy, &operation.CreatedAt, &operation.UpdatedAt, &operation.CompletedAt)
	if err != nil {
		return AWSOperation{}, err
	}
	payload, _ := json.Marshal(map[string]any{"operationId": operation.ID})
	var jobID uuid.UUID
	if err = tx.QueryRow(ctx, `INSERT INTO jobs(organization_id,job_type,payload) VALUES($1,$2,$3) RETURNING id`, organizationID, jobType, payload).Scan(&jobID); err != nil {
		return AWSOperation{}, err
	}
	operation.JobID = &jobID
	if _, err = tx.Exec(ctx, `UPDATE aws_operations SET job_id=$3 WHERE id=$1 AND organization_id=$2`, operation.ID, organizationID, jobID); err != nil {
		return AWSOperation{}, err
	}
	if err = insertAudit(ctx, tx, &organizationID, &actorID, "aws.operation_queued", "aws_operation", &operation.ID, requestID, map[string]any{"operationType": operationType}, ip); err != nil {
		return AWSOperation{}, err
	}
	return operation, tx.Commit(ctx)
}
func (r Repository) ListAWSOperations(ctx context.Context, organizationID uuid.UUID) ([]AWSOperation, error) {
	rows, err := r.Pool.Query(ctx, `SELECT id,organization_id,account_id,job_id,operation_type,status,resource_type,provider_resource_id,request,error,created_by,created_at,updated_at,completed_at FROM aws_operations WHERE organization_id=$1 ORDER BY created_at DESC LIMIT 100`, organizationID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := []AWSOperation{}
	for rows.Next() {
		var item AWSOperation
		if err = rows.Scan(&item.ID, &item.OrganizationID, &item.AccountID, &item.JobID, &item.OperationType, &item.Status, &item.ResourceType, &item.ProviderResourceID, &item.Request, &item.Error, &item.CreatedBy, &item.CreatedAt, &item.UpdatedAt, &item.CompletedAt); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (r Repository) ListBudgets(ctx context.Context, organizationID uuid.UUID) ([]Budget, error) {
	rows, err := r.Pool.Query(ctx, `SELECT id,organization_id,account_id,project_id,environment_id,name,monthly_amount::float8,currency,thresholds::float8[],prevent_new_provisioning,provider_type,provider_budget_name,last_evaluated_amount::float8,last_evaluated_at,created_at FROM aws_budgets WHERE organization_id=$1 ORDER BY name`, organizationID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := []Budget{}
	for rows.Next() {
		var item Budget
		if err = rows.Scan(&item.ID, &item.OrganizationID, &item.AccountID, &item.ProjectID, &item.EnvironmentID, &item.Name, &item.MonthlyAmount, &item.Currency, &item.Thresholds, &item.PreventNewProvisioning, &item.ProviderType, &item.ProviderBudgetName, &item.LastEvaluatedAmount, &item.LastEvaluatedAt, &item.CreatedAt); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}
func (r Repository) CreateBudget(ctx context.Context, organizationID, actorID uuid.UUID, item Budget, requestID uuid.UUID, ip net.IP) (Budget, error) {
	if item.Currency == "" {
		item.Currency = "USD"
	}
	if item.ProviderType == "" {
		item.ProviderType = "silicon"
	}
	tx, err := r.Pool.Begin(ctx)
	if err != nil {
		return Budget{}, err
	}
	defer tx.Rollback(ctx)
	err = tx.QueryRow(ctx, `INSERT INTO aws_budgets(organization_id,account_id,project_id,environment_id,name,monthly_amount,currency,thresholds,prevent_new_provisioning,provider_type,provider_budget_name) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11) RETURNING id,organization_id,account_id,project_id,environment_id,name,monthly_amount::float8,currency,thresholds::float8[],prevent_new_provisioning,provider_type,provider_budget_name,last_evaluated_amount::float8,last_evaluated_at,created_at`, organizationID, item.AccountID, item.ProjectID, item.EnvironmentID, item.Name, item.MonthlyAmount, item.Currency, item.Thresholds, item.PreventNewProvisioning, item.ProviderType, item.ProviderBudgetName).Scan(&item.ID, &item.OrganizationID, &item.AccountID, &item.ProjectID, &item.EnvironmentID, &item.Name, &item.MonthlyAmount, &item.Currency, &item.Thresholds, &item.PreventNewProvisioning, &item.ProviderType, &item.ProviderBudgetName, &item.LastEvaluatedAmount, &item.LastEvaluatedAt, &item.CreatedAt)
	if err != nil {
		return Budget{}, err
	}
	if err = insertAudit(ctx, tx, &organizationID, &actorID, "budget.created", "budget", &item.ID, requestID, map[string]any{"name": item.Name, "amount": item.MonthlyAmount, "currency": item.Currency, "thresholds": item.Thresholds, "preventNewProvisioning": item.PreventNewProvisioning}, ip); err != nil {
		return Budget{}, err
	}
	return item, tx.Commit(ctx)
}
func (r Repository) DeleteBudget(ctx context.Context, organizationID, budgetID, actorID, requestID uuid.UUID, ip net.IP) error {
	tx, err := r.Pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	tag, err := tx.Exec(ctx, `DELETE FROM aws_budgets WHERE organization_id=$1 AND id=$2`, organizationID, budgetID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() != 1 {
		return ErrNotFound
	}
	if err = insertAudit(ctx, tx, &organizationID, &actorID, "budget.deleted", "budget", &budgetID, requestID, nil, ip); err != nil {
		return err
	}
	return tx.Commit(ctx)
}
func (r Repository) ProvisioningBlocked(ctx context.Context, organizationID, accountID uuid.UUID, projectID, environmentID *uuid.UUID) (bool, error) {
	var blocked bool
	err := r.Pool.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM aws_budgets WHERE organization_id=$1 AND prevent_new_provisioning AND last_evaluated_amount IS NOT NULL AND last_evaluated_amount>=monthly_amount AND date_trunc('month',last_evaluated_at)=date_trunc('month',now()) AND ((account_id IS NULL AND project_id IS NULL AND environment_id IS NULL) OR account_id=$2 OR project_id=$3 OR environment_id=$4))`, organizationID, accountID, projectID, environmentID).Scan(&blocked)
	return blocked, err
}

func (r Repository) ApplyBudgetCost(ctx context.Context, organizationID, accountID uuid.UUID, report cloudaws.CostReport) error {
	tx, err := r.Pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	projectCosts := costGroupValues(report.Projects)
	environmentCosts := costGroupValues(report.Environments)
	projectJSON, _ := json.Marshal(projectCosts)
	environmentJSON, _ := json.Marshal(environmentCosts)
	if _, err = tx.Exec(ctx, `INSERT INTO aws_cost_snapshots(organization_id,account_id,period_start,period_end,total,project_costs,environment_costs,project_attribution_available,environment_attribution_available) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9) ON CONFLICT(organization_id,account_id,period_start) DO UPDATE SET period_end=EXCLUDED.period_end,total=EXCLUDED.total,project_costs=EXCLUDED.project_costs,environment_costs=EXCLUDED.environment_costs,project_attribution_available=EXCLUDED.project_attribution_available,environment_attribution_available=EXCLUDED.environment_attribution_available,updated_at=now()`, organizationID, accountID, report.PeriodStart, report.PeriodEnd, report.Total, projectJSON, environmentJSON, report.ProjectAttributionAvailable, report.EnvironmentAttributionAvailable); err != nil {
		return err
	}
	type snapshot struct {
		accountID             uuid.UUID
		total                 float64
		projects              map[string]float64
		environments          map[string]float64
		projectsAvailable     bool
		environmentsAvailable bool
	}
	snapshots := []snapshot{}
	rows, err := tx.Query(ctx, `SELECT account_id,total::float8,project_costs,environment_costs,project_attribution_available,environment_attribution_available FROM aws_cost_snapshots WHERE organization_id=$1 AND period_start=$2`, organizationID, report.PeriodStart)
	if err != nil {
		return err
	}
	for rows.Next() {
		var item snapshot
		var projects, environments []byte
		if err = rows.Scan(&item.accountID, &item.total, &projects, &environments, &item.projectsAvailable, &item.environmentsAvailable); err != nil {
			rows.Close()
			return err
		}
		_ = json.Unmarshal(projects, &item.projects)
		_ = json.Unmarshal(environments, &item.environments)
		snapshots = append(snapshots, item)
	}
	if err = rows.Err(); err != nil {
		rows.Close()
		return err
	}
	rows.Close()
	var accountCount int
	if err = tx.QueryRow(ctx, `SELECT count(*) FROM aws_accounts WHERE organization_id=$1 AND status<>'disabled'`, organizationID).Scan(&accountCount); err != nil {
		return err
	}
	completeOrganization := accountCount > 0 && len(snapshots) == accountCount
	rows, err = tx.Query(ctx, `SELECT id,name,account_id,project_id,environment_id,monthly_amount::float8,thresholds::float8[],COALESCE(last_evaluated_amount::float8,0),last_evaluated_at FROM aws_budgets WHERE organization_id=$1 FOR UPDATE`, organizationID)
	if err != nil {
		return err
	}
	type evaluation struct {
		id            uuid.UUID
		name          string
		accountID     *uuid.UUID
		projectID     *uuid.UUID
		environmentID *uuid.UUID
		limit         float64
		thresholds    []float64
		previous      float64
		lastAt        *time.Time
	}
	items := []evaluation{}
	for rows.Next() {
		var item evaluation
		if err = rows.Scan(&item.id, &item.name, &item.accountID, &item.projectID, &item.environmentID, &item.limit, &item.thresholds, &item.previous, &item.lastAt); err != nil {
			rows.Close()
			return err
		}
		items = append(items, item)
	}
	if err = rows.Err(); err != nil {
		rows.Close()
		return err
	}
	rows.Close()
	for _, item := range items {
		amount, available := 0.0, true
		switch {
		case item.accountID != nil:
			available = false
			for _, value := range snapshots {
				if value.accountID == *item.accountID {
					amount, available = value.total, true
					break
				}
			}
		case item.projectID != nil:
			available = completeOrganization
			for _, value := range snapshots {
				available = available && value.projectsAvailable
				amount += value.projects[item.projectID.String()]
			}
		case item.environmentID != nil:
			available = completeOrganization
			for _, value := range snapshots {
				available = available && value.environmentsAvailable
				amount += value.environments[item.environmentID.String()]
			}
		default:
			available = completeOrganization
			for _, value := range snapshots {
				amount += value.total
			}
		}
		if !available {
			continue
		}
		if item.lastAt == nil || item.lastAt.Year() != report.PeriodStart.Year() || item.lastAt.Month() != report.PeriodStart.Month() {
			item.previous = 0
		}
		crossed := budgets.ThresholdsCrossed(item.previous, amount, item.limit, item.thresholds)
		if _, err = tx.Exec(ctx, `UPDATE aws_budgets SET last_evaluated_amount=$3,last_evaluated_at=now(),updated_at=now() WHERE id=$1 AND organization_id=$2`, item.id, organizationID, amount); err != nil {
			return err
		}
		for _, threshold := range crossed {
			if err = insertAudit(ctx, tx, &organizationID, nil, "budget.threshold_reached", "budget", &item.id, uuid.New(), map[string]any{"name": item.name, "threshold": threshold, "amount": amount, "monthlyAmount": item.limit}, nil); err != nil {
				return err
			}
		}
	}
	return tx.Commit(ctx)
}

func costGroupValues(groups []cloudaws.CostGroup) map[string]float64 {
	values := map[string]float64{}
	for _, group := range groups {
		if group.Name == "" || group.Name == "Unallocated" {
			continue
		}
		values[group.Name] += group.Amount
	}
	return values
}

func (r Repository) AWSOperationForUpdate(ctx context.Context, organizationID, operationID uuid.UUID) (AWSOperation, error) {
	var item AWSOperation
	err := r.Pool.QueryRow(ctx, `SELECT id,organization_id,account_id,job_id,operation_type,status,resource_type,provider_resource_id,request,error,created_by,created_at,updated_at,completed_at FROM aws_operations WHERE organization_id=$1 AND id=$2`, organizationID, operationID).Scan(&item.ID, &item.OrganizationID, &item.AccountID, &item.JobID, &item.OperationType, &item.Status, &item.ResourceType, &item.ProviderResourceID, &item.Request, &item.Error, &item.CreatedBy, &item.CreatedAt, &item.UpdatedAt, &item.CompletedAt)
	return item, notFound(err)
}
func (r Repository) UpdateAWSOperation(ctx context.Context, organizationID, operationID uuid.UUID, status, resourceType, providerID, errorMessage string) error {
	tag, err := r.Pool.Exec(ctx, `UPDATE aws_operations SET status=$3,resource_type=COALESCE(NULLIF($4,''),resource_type),provider_resource_id=COALESCE(NULLIF($5,''),provider_resource_id),error=$6,updated_at=now(),completed_at=CASE WHEN $3 IN ('succeeded','failed') THEN now() ELSE completed_at END WHERE organization_id=$1 AND id=$2`, organizationID, operationID, status, resourceType, providerID, errorMessage)
	if err == nil && tag.RowsAffected() != 1 {
		return ErrNotFound
	}
	return err
}
func (r Repository) ClaimAWSJob(ctx context.Context, workerID string) (uuid.UUID, uuid.UUID, json.RawMessage, string, error) {
	var jobID, orgID uuid.UUID
	var payload json.RawMessage
	var jobType string
	err := r.Pool.QueryRow(ctx, `WITH candidate AS (SELECT id FROM jobs WHERE status='queued' AND job_type IN ('provision_aws_machine','terminate_aws_machine','create_aws_snapshot','prepare_aws_machine') AND available_at<=now() ORDER BY created_at FOR UPDATE SKIP LOCKED LIMIT 1) UPDATE jobs j SET status='running',locked_at=now(),locked_by=$1,attempts=attempts+1,updated_at=now() FROM candidate WHERE j.id=candidate.id RETURNING j.id,j.organization_id,j.payload,j.job_type`, workerID).Scan(&jobID, &orgID, &payload, &jobType)
	return jobID, orgID, payload, jobType, err
}
func (r Repository) CompleteAWSJob(ctx context.Context, organizationID, jobID uuid.UUID, status, errorMessage string) error {
	tag, err := r.Pool.Exec(ctx, `UPDATE jobs SET status=$3,last_error=$4,locked_at=NULL,locked_by=NULL,updated_at=now() WHERE organization_id=$1 AND id=$2`, organizationID, jobID, status, errorMessage)
	if err == nil && tag.RowsAffected() != 1 {
		return ErrNotFound
	}
	return err
}

func (r Repository) MarkAWSInstanceTerminated(ctx context.Context, organizationID, accountID uuid.UUID, region, instanceID string) error {
	_, err := r.Pool.Exec(ctx, `WITH changed AS (UPDATE aws_instances SET state='terminated',updated_at=now() WHERE organization_id=$1 AND account_id=$2 AND region=$3 AND provider_instance_id=$4 RETURNING server_id) UPDATE servers SET connection_status='unreachable',health='unhealthy',connection_error='AWS instance was terminated.',updated_at=now() WHERE organization_id=$1 AND id IN (SELECT server_id FROM changed)`, organizationID, accountID, region, instanceID)
	return err
}
