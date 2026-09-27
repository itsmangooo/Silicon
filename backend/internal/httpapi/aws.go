package httpapi

import (
	"context"
	"encoding/base64"
	"errors"
	"net/http"
	"sort"
	"strings"
	"time"

	"github.com/google/uuid"
	cloudaws "github.com/itsmangooo/Silicon/backend/internal/providers/cloud/aws"
	"github.com/itsmangooo/Silicon/backend/internal/serverconnections"
	"github.com/itsmangooo/Silicon/backend/internal/store"
)

type awsMachineRequest struct {
	Machine             cloudaws.CreateMachineInput `json:"machine"`
	SSHUsername         string                      `json:"sshUsername"`
	EncryptedPrivateKey string                      `json:"encryptedPrivateKey,omitempty"`
}

func awsCredentialContext(organizationID, accountID uuid.UUID, field string) string {
	return "aws-account:" + organizationID.String() + ":" + accountID.String() + ":" + field
}

func awsMachineKeyContext(organizationID, accountID uuid.UUID) string {
	return "aws-machine-key:" + organizationID.String() + ":" + accountID.String()
}

func (a *API) listAWSAccounts(w http.ResponseWriter, r *http.Request) {
	items, err := a.repo.ListAWSAccounts(r.Context(), pathUUID(r, "organizationID"))
	if err != nil {
		a.serverError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"accounts": items})
}

func (a *API) connectAWSAccount(w http.ResponseWriter, r *http.Request) {
	var input struct {
		DisplayName, AccountID, RoleARN, ExternalID, DefaultRegion, AccessKeyID, SecretAccessKey string
		EnabledRegions                                                                           []string `json:"enabledRegions"`
	}
	if !decode(w, r, &input) {
		return
	}
	input.DisplayName = strings.TrimSpace(input.DisplayName)
	input.AccountID = strings.TrimSpace(input.AccountID)
	input.RoleARN = strings.TrimSpace(input.RoleARN)
	input.DefaultRegion = strings.TrimSpace(input.DefaultRegion)
	input.AccessKeyID = strings.TrimSpace(input.AccessKeyID)
	input.SecretAccessKey = strings.TrimSpace(input.SecretAccessKey)
	if input.DisplayName == "" || input.DefaultRegion == "" || (input.AccessKeyID == "") != (input.SecretAccessKey == "") {
		validation(w, "Display name, default region, and both static key fields when used are required.")
		return
	}
	if len(input.EnabledRegions) == 0 {
		input.EnabledRegions = []string{input.DefaultRegion}
	}
	if !stringInSlice(input.DefaultRegion, input.EnabledRegions) {
		input.EnabledRegions = append(input.EnabledRegions, input.DefaultRegion)
	}
	accountUUID := uuid.New()
	var encryptedExternalID, encryptedID, encryptedSecret []byte
	var err error
	if input.ExternalID != "" {
		if a.box == nil {
			writeError(w, http.StatusServiceUnavailable, "encryption_unavailable", "Credential encryption is not configured.")
			return
		}
		encryptedExternalID, err = a.box.Seal([]byte(input.ExternalID), awsCredentialContext(pathUUID(r, "organizationID"), accountUUID, "external-id"))
		if err != nil {
			a.serverError(w, r, err)
			return
		}
	}
	if input.AccessKeyID != "" {
		if a.box == nil {
			writeError(w, http.StatusServiceUnavailable, "encryption_unavailable", "Credential encryption is not configured.")
			return
		}
		encryptedID, err = a.box.Seal([]byte(input.AccessKeyID), awsCredentialContext(pathUUID(r, "organizationID"), accountUUID, "access-key-id"))
		if err == nil {
			encryptedSecret, err = a.box.Seal([]byte(input.SecretAccessKey), awsCredentialContext(pathUUID(r, "organizationID"), accountUUID, "secret-access-key"))
		}
		if err != nil {
			a.serverError(w, r, err)
			return
		}
	}
	provider, err := a.awsFactory.Open(r.Context(), cloudaws.AccountConfig{AccountID: input.AccountID, RoleARN: input.RoleARN, ExternalID: input.ExternalID, Region: input.DefaultRegion, AccessKeyID: input.AccessKeyID, SecretAccessKey: input.SecretAccessKey})
	input.ExternalID = ""
	input.AccessKeyID = ""
	input.SecretAccessKey = ""
	if err != nil {
		writeError(w, http.StatusBadGateway, "aws_connection_failed", err.Error())
		return
	}
	identity, err := provider.Identity(r.Context())
	if err != nil {
		writeError(w, http.StatusBadGateway, "aws_connection_failed", err.Error())
		return
	}
	if input.AccountID != "" && identity.AccountID != input.AccountID {
		writeError(w, http.StatusConflict, "aws_account_mismatch", "STS returned a different AWS account ID.")
		return
	}
	orgID := pathUUID(r, "organizationID")
	item, err := a.repo.CreateAWSAccount(r.Context(), orgID, currentUser(r.Context()).ID, store.AWSAccountInput{ID: accountUUID, DisplayName: input.DisplayName, AccountID: identity.AccountID, RoleARN: input.RoleARN, EncryptedExternalID: encryptedExternalID, EncryptedAccessKeyID: encryptedID, EncryptedSecretAccessKey: encryptedSecret, DefaultRegion: input.DefaultRegion, EnabledRegions: cleanRegions(input.EnabledRegions)}, requestID(r.Context()), clientIP(r))
	if err != nil {
		a.persistenceError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, item)
}

func (a *API) disconnectAWSAccount(w http.ResponseWriter, r *http.Request) {
	err := a.repo.DeleteAWSAccount(r.Context(), pathUUID(r, "organizationID"), pathUUID(r, "accountID"), currentUser(r.Context()).ID, requestID(r.Context()), clientIP(r))
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			writeError(w, http.StatusConflict, "aws_account_in_use", "Disconnect attached AWS resources before removing the account.")
			return
		}
		a.persistenceError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (a *API) testAWSAccount(w http.ResponseWriter, r *http.Request) {
	orgID, accountID := pathUUID(r, "organizationID"), pathUUID(r, "accountID")
	provider, _, err := a.awsProvider(r.Context(), orgID, accountID, "")
	if err != nil {
		_ = a.repo.UpdateAWSAccountHealth(r.Context(), orgID, accountID, "error", err.Error())
		writeError(w, http.StatusBadGateway, "aws_connection_failed", err.Error())
		return
	}
	identity, err := provider.Identity(r.Context())
	if err != nil {
		_ = a.repo.UpdateAWSAccountHealth(r.Context(), orgID, accountID, "error", err.Error())
		writeError(w, http.StatusBadGateway, "aws_connection_failed", err.Error())
		return
	}
	_ = a.repo.UpdateAWSAccountHealth(r.Context(), orgID, accountID, "connected", "")
	writeJSON(w, http.StatusOK, identity)
}

func (a *API) listAWSRegions(w http.ResponseWriter, r *http.Request) {
	provider, account, err := a.awsProvider(r.Context(), pathUUID(r, "organizationID"), pathUUID(r, "accountID"), "")
	if err != nil {
		a.persistenceError(w, err)
		return
	}
	items, err := provider.Regions(r.Context())
	if err != nil {
		writeError(w, http.StatusBadGateway, "aws_provider_error", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"regions": items, "enabledRegions": account.EnabledRegions})
}

func (a *API) getAWSInventory(w http.ResponseWriter, r *http.Request) {
	orgID, accountID := pathUUID(r, "organizationID"), pathUUID(r, "accountID")
	region := strings.TrimSpace(r.URL.Query().Get("region"))
	provider, account, err := a.awsProvider(r.Context(), orgID, accountID, region)
	if err != nil {
		a.persistenceError(w, err)
		return
	}
	if region == "" {
		region = account.DefaultRegion
	}
	if !regionEnabled(account, region) {
		writeError(w, http.StatusUnprocessableEntity, "region_disabled", "The selected region is not enabled for this AWS account.")
		return
	}
	instances, err := provider.Instances(r.Context())
	if err != nil {
		writeError(w, http.StatusBadGateway, "aws_provider_error", err.Error())
		return
	}
	cached, _ := a.repo.ListAWSInstances(r.Context(), orgID, accountID)
	byID := map[string]store.AWSInstance{}
	for _, item := range cached {
		if item.Region == region {
			byID[item.ProviderInstanceID] = item
		}
	}
	for index := range instances {
		if item, ok := byID[instances[index].ID]; ok {
			instances[index].Ownership = cloudaws.Ownership(item.Ownership)
			if item.ServerID != nil {
				instances[index].SiliconServerID = item.ServerID.String()
			}
		}
	}
	vpcs, err := provider.VPCs(r.Context())
	if err != nil {
		writeError(w, http.StatusBadGateway, "aws_provider_error", err.Error())
		return
	}
	subnets, err := provider.Subnets(r.Context())
	if err != nil {
		writeError(w, http.StatusBadGateway, "aws_provider_error", err.Error())
		return
	}
	groups, err := provider.SecurityGroups(r.Context())
	if err != nil {
		writeError(w, http.StatusBadGateway, "aws_provider_error", err.Error())
		return
	}
	addresses, err := provider.ElasticIPs(r.Context())
	if err != nil {
		writeError(w, http.StatusBadGateway, "aws_provider_error", err.Error())
		return
	}
	volumes, err := provider.Volumes(r.Context())
	if err != nil {
		writeError(w, http.StatusBadGateway, "aws_provider_error", err.Error())
		return
	}
	snapshots, err := provider.Snapshots(r.Context())
	if err != nil {
		writeError(w, http.StatusBadGateway, "aws_provider_error", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"region": region, "instances": instances, "vpcs": vpcs, "subnets": subnets, "securityGroups": groups, "elasticIps": addresses, "volumes": volumes, "snapshots": snapshots})
}

func (a *API) getAWSInstanceType(w http.ResponseWriter, r *http.Request) {
	provider, _, err := a.awsProvider(r.Context(), pathUUID(r, "organizationID"), pathUUID(r, "accountID"), strings.TrimSpace(r.URL.Query().Get("region")))
	if err != nil {
		a.persistenceError(w, err)
		return
	}
	item, err := provider.InstanceType(r.Context(), r.PathValue("instanceType"))
	if err != nil {
		writeError(w, http.StatusBadGateway, "aws_provider_error", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, item)
}

func (a *API) estimateAWSMachine(w http.ResponseWriter, r *http.Request) {
	var input cloudaws.CreateMachineInput
	if !decode(w, r, &input) {
		return
	}
	provider, account, err := a.awsProvider(r.Context(), pathUUID(r, "organizationID"), pathUUID(r, "accountID"), input.Region)
	if err != nil {
		a.persistenceError(w, err)
		return
	}
	if input.Region == "" {
		input.Region = account.DefaultRegion
	}
	estimate, err := provider.Estimate(r.Context(), input)
	if err != nil {
		writeError(w, http.StatusBadGateway, "aws_pricing_unavailable", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, estimate)
}

func (a *API) provisionAWSMachine(w http.ResponseWriter, r *http.Request) {
	var input struct {
		Name, Region, AvailabilityZone, Distribution, Architecture, ImageID, InstanceType, VPCID, SubnetID, RootDiskType, SSHKeyName, InstanceProfile, ConnectionMethod, SSHUsername, PrivateKey string
		SecurityGroupIDs                                                                                                                                                                         []string `json:"securityGroupIds"`
		PublicIPv4, ElasticIP, EncryptRootDisk, DockerBootstrap                                                                                                                                  bool
		RootDiskGiB                                                                                                                                                                              int32
		ProjectID, EnvironmentID                                                                                                                                                                 *uuid.UUID
		ServerLabels                                                                                                                                                                             map[string]string
	}
	if !decode(w, r, &input) {
		return
	}
	input.Name = strings.TrimSpace(input.Name)
	input.ConnectionMethod = strings.TrimSpace(input.ConnectionMethod)
	if input.Name == "" || input.InstanceType == "" || !oneOf(input.ConnectionMethod, "ssh", "aws_ssm") {
		validation(w, "Name, instance type, and ssh or aws_ssm connection method are required.")
		return
	}
	if input.ConnectionMethod == "ssh" && (input.SSHUsername == "" || input.PrivateKey == "") {
		validation(w, "SSH provisioning requires a username and private key credential.")
		return
	}
	if input.ConnectionMethod == "ssh" && !validLinuxUsername(input.SSHUsername) {
		validation(w, "SSH username must be a valid Linux account name.")
		return
	}
	if input.ConnectionMethod == "aws_ssm" && strings.TrimSpace(input.InstanceProfile) == "" {
		validation(w, "SSM provisioning requires an EC2 instance profile name or ARN with Systems Manager access.")
		return
	}
	orgID, accountID := pathUUID(r, "organizationID"), pathUUID(r, "accountID")
	if err := a.repo.ValidateAWSResourceScope(r.Context(), orgID, input.ProjectID, input.EnvironmentID); err != nil {
		a.persistenceError(w, err)
		return
	}
	blocked, err := a.repo.ProvisioningBlocked(r.Context(), orgID, accountID, input.ProjectID, input.EnvironmentID)
	if err != nil {
		a.serverError(w, r, err)
		return
	}
	if blocked {
		writeError(w, http.StatusConflict, "budget_policy_blocked", "A Silicon budget policy currently prevents new provisioning.")
		return
	}
	_, account, err := a.awsProvider(r.Context(), orgID, accountID, input.Region)
	if err != nil {
		a.persistenceError(w, err)
		return
	}
	if input.Region == "" {
		input.Region = account.DefaultRegion
	}
	if !regionEnabled(account, input.Region) {
		validation(w, "The selected region is not enabled for this AWS account.")
		return
	}
	machine := cloudaws.CreateMachineInput{Name: input.Name, Region: input.Region, AvailabilityZone: input.AvailabilityZone, Distribution: input.Distribution, Architecture: input.Architecture, ImageID: input.ImageID, InstanceType: input.InstanceType, VPCID: input.VPCID, SubnetID: input.SubnetID, SecurityGroupIDs: input.SecurityGroupIDs, PublicIPv4: input.PublicIPv4, ElasticIP: input.ElasticIP, RootDiskGiB: input.RootDiskGiB, RootDiskType: input.RootDiskType, EncryptRootDisk: input.EncryptRootDisk, SSHKeyName: input.SSHKeyName, InstanceProfile: input.InstanceProfile, BootstrapUser: input.SSHUsername, DockerBootstrap: input.DockerBootstrap, OrganizationID: orgID.String(), ServerLabels: input.ServerLabels, ConnectionMethod: input.ConnectionMethod}
	if input.ProjectID != nil {
		machine.ProjectID = input.ProjectID.String()
	}
	if input.EnvironmentID != nil {
		machine.EnvironmentID = input.EnvironmentID.String()
	}
	request := awsMachineRequest{Machine: machine, SSHUsername: input.SSHUsername}
	if input.PrivateKey != "" {
		if a.box == nil {
			writeError(w, http.StatusServiceUnavailable, "encryption_unavailable", "Credential encryption is not configured.")
			return
		}
		encrypted, sealErr := a.box.Seal([]byte(input.PrivateKey), awsMachineKeyContext(orgID, accountID))
		input.PrivateKey = ""
		if sealErr != nil {
			a.serverError(w, r, sealErr)
			return
		}
		request.EncryptedPrivateKey = base64.StdEncoding.EncodeToString(encrypted)
	}
	operation, err := a.repo.CreateAWSOperation(r.Context(), orgID, accountID, currentUser(r.Context()).ID, "provision_machine", "provision_aws_machine", request, requestID(r.Context()), clientIP(r))
	if err != nil {
		a.persistenceError(w, err)
		return
	}
	writeJSON(w, http.StatusAccepted, operation)
}

func (a *API) awsInstanceAction(w http.ResponseWriter, r *http.Request) {
	orgID, accountID := pathUUID(r, "organizationID"), pathUUID(r, "accountID")
	instanceID, action := r.PathValue("instanceID"), r.PathValue("action")
	provider, account, err := a.awsProvider(r.Context(), orgID, accountID, strings.TrimSpace(r.URL.Query().Get("region")))
	if err != nil {
		a.persistenceError(w, err)
		return
	}
	region := r.URL.Query().Get("region")
	if region == "" {
		region = account.DefaultRegion
	}
	if action != "import" {
		if _, ok := a.requireAWSOwnership(w, r, orgID, accountID, region, "instance", instanceID, "managed", "imported"); !ok {
			return
		}
	}
	switch action {
	case "start":
		err = provider.StartInstance(r.Context(), instanceID)
	case "stop":
		err = provider.StopInstance(r.Context(), instanceID)
	case "reboot":
		err = provider.RebootInstance(r.Context(), instanceID)
	case "terminate":
		operation, queueErr := a.repo.CreateAWSOperation(r.Context(), orgID, accountID, currentUser(r.Context()).ID, "terminate_machine", "terminate_aws_machine", map[string]any{"instanceId": instanceID, "region": region}, requestID(r.Context()), clientIP(r))
		if queueErr != nil {
			a.persistenceError(w, queueErr)
			return
		}
		writeJSON(w, http.StatusAccepted, operation)
		return
	case "import":
		var input struct{ ConnectionMethod, SSHUsername, PrivateKey string }
		if !decode(w, r, &input) {
			return
		}
		if !oneOf(input.ConnectionMethod, "ssh", "aws_ssm") {
			validation(w, "Connection method must be ssh or aws_ssm.")
			return
		}
		instance, inspectErr := provider.Instance(r.Context(), instanceID)
		if inspectErr != nil {
			writeError(w, http.StatusBadGateway, "aws_provider_error", inspectErr.Error())
			return
		}
		saved, saveErr := a.saveImportedAWSInstance(r.Context(), orgID, accountID, instance, input.ConnectionMethod, input.SSHUsername, input.PrivateKey, requestID(r.Context()), clientIP(r))
		input.PrivateKey = ""
		if saveErr != nil {
			a.persistenceError(w, saveErr)
			return
		}
		writeJSON(w, http.StatusCreated, saved)
		return
	default:
		writeError(w, http.StatusNotFound, "not_found", "AWS instance action is not supported.")
		return
	}
	if err != nil {
		writeError(w, http.StatusBadGateway, "aws_provider_error", err.Error())
		return
	}
	_ = a.repo.RecordOrganizationAudit(r.Context(), orgID, currentUser(r.Context()).ID, "aws.instance_"+action, "aws_instance", nil, requestID(r.Context()), map[string]any{"instanceId": instanceID, "region": region}, clientIP(r))
	w.WriteHeader(http.StatusNoContent)
}

func (a *API) saveImportedAWSInstance(ctx context.Context, orgID, accountID uuid.UUID, instance cloudaws.Instance, method, user, privateKey string, request uuid.UUID, ip []byte) (store.AWSInstance, error) {
	serverID := uuid.New()
	var encrypted []byte
	var err error
	if method == "ssh" {
		if user == "" || privateKey == "" {
			return store.AWSInstance{}, errors.New("SSH imports require username and private key")
		}
		if a.box == nil {
			return store.AWSInstance{}, errors.New("credential encryption is unavailable")
		}
		encrypted, err = a.box.Seal([]byte(privateKey), serverconnections.CredentialContext(orgID, serverID))
		if err != nil {
			return store.AWSInstance{}, err
		}
	}
	return a.repo.SaveAWSInstance(ctx, orgID, accountID, currentUser(ctx).ID, serverID, instance, "imported", method, encrypted, user, request, ip)
}

func (a *API) createAWSVPC(w http.ResponseWriter, r *http.Request) {
	var input cloudaws.CreateVPCInput
	if !decode(w, r, &input) {
		return
	}
	a.withAWSResourceCreate(w, r, "vpc", input.Name, func(provider cloudaws.Provider, org string) (string, any, error) {
		input.OrganizationID = org
		item, err := provider.CreateVPC(r.Context(), input)
		return item.ID, item, err
	})
}
func (a *API) deleteAWSVPC(w http.ResponseWriter, r *http.Request) {
	a.withManagedAWSDelete(w, r, "vpc", func(provider cloudaws.Provider, id string) error { return provider.DeleteVPC(r.Context(), id) })
}
func (a *API) createAWSSubnet(w http.ResponseWriter, r *http.Request) {
	var input cloudaws.CreateSubnetInput
	if !decode(w, r, &input) {
		return
	}
	a.withAWSResourceCreate(w, r, "subnet", input.Name, func(provider cloudaws.Provider, org string) (string, any, error) {
		input.OrganizationID = org
		item, err := provider.CreateSubnet(r.Context(), input)
		return item.ID, item, err
	})
}
func (a *API) createAWSSecurityGroup(w http.ResponseWriter, r *http.Request) {
	var input cloudaws.CreateSecurityGroupInput
	if !decode(w, r, &input) {
		return
	}
	a.withAWSResourceCreate(w, r, "security_group", input.Name, func(provider cloudaws.Provider, org string) (string, any, error) {
		input.OrganizationID = org
		item, err := provider.CreateSecurityGroup(r.Context(), input)
		return item.ID, item, err
	})
}
func (a *API) addAWSSecurityRule(w http.ResponseWriter, r *http.Request) {
	var input cloudaws.SecurityRule
	if !decode(w, r, &input) {
		return
	}
	orgID, accountID := pathUUID(r, "organizationID"), pathUUID(r, "accountID")
	provider, account, err := a.awsProvider(r.Context(), orgID, accountID, r.URL.Query().Get("region"))
	region := r.URL.Query().Get("region")
	if region == "" {
		region = account.DefaultRegion
	}
	if err == nil {
		_, ok := a.requireAWSOwnership(w, r, orgID, accountID, region, "security_group", r.PathValue("resourceID"), "managed", "imported")
		if !ok {
			return
		}
	}
	if err == nil {
		err = provider.AddSecurityRule(r.Context(), r.PathValue("resourceID"), input)
	}
	if err != nil {
		writeError(w, http.StatusBadGateway, "aws_provider_error", err.Error())
		return
	}
	_ = a.repo.RecordOrganizationAudit(r.Context(), orgID, currentUser(r.Context()).ID, "aws.security_rule_added", "aws_security_group", nil, requestID(r.Context()), map[string]any{"groupId": r.PathValue("resourceID"), "direction": input.Direction, "protocol": input.Protocol, "cidrs": input.CIDRs}, clientIP(r))
	w.WriteHeader(http.StatusNoContent)
}
func (a *API) removeAWSSecurityRule(w http.ResponseWriter, r *http.Request) {
	var input cloudaws.SecurityRule
	if !decode(w, r, &input) {
		return
	}
	orgID, accountID := pathUUID(r, "organizationID"), pathUUID(r, "accountID")
	provider, account, err := a.awsProvider(r.Context(), orgID, accountID, r.URL.Query().Get("region"))
	region := r.URL.Query().Get("region")
	if region == "" {
		region = account.DefaultRegion
	}
	if err == nil {
		_, ok := a.requireAWSOwnership(w, r, orgID, accountID, region, "security_group", r.PathValue("resourceID"), "managed", "imported")
		if !ok {
			return
		}
	}
	if err == nil {
		err = provider.RemoveSecurityRule(r.Context(), r.PathValue("resourceID"), input)
	}
	if err != nil {
		writeError(w, http.StatusBadGateway, "aws_provider_error", err.Error())
		return
	}
	_ = a.repo.RecordOrganizationAudit(r.Context(), orgID, currentUser(r.Context()).ID, "aws.security_rule_removed", "aws_security_group", nil, requestID(r.Context()), map[string]any{"groupId": r.PathValue("resourceID"), "direction": input.Direction, "protocol": input.Protocol, "cidrs": input.CIDRs}, clientIP(r))
	w.WriteHeader(http.StatusNoContent)
}
func (a *API) setAWSInstanceSecurityGroups(w http.ResponseWriter, r *http.Request) {
	var input struct {
		SecurityGroupIDs []string `json:"securityGroupIds"`
	}
	if !decode(w, r, &input) {
		return
	}
	orgID, accountID := pathUUID(r, "organizationID"), pathUUID(r, "accountID")
	provider, account, err := a.awsProvider(r.Context(), orgID, accountID, r.URL.Query().Get("region"))
	region := r.URL.Query().Get("region")
	if region == "" {
		region = account.DefaultRegion
	}
	if err == nil {
		_, ok := a.requireAWSOwnership(w, r, orgID, accountID, region, "instance", r.PathValue("instanceID"), "managed", "imported")
		if !ok {
			return
		}
	}
	if err == nil {
		err = provider.SetInstanceSecurityGroups(r.Context(), r.PathValue("instanceID"), input.SecurityGroupIDs)
	}
	if err != nil {
		writeError(w, http.StatusBadGateway, "aws_provider_error", err.Error())
		return
	}
	_ = a.repo.RecordOrganizationAudit(r.Context(), orgID, currentUser(r.Context()).ID, "aws.instance_security_groups_changed", "aws_instance", nil, requestID(r.Context()), map[string]any{"instanceId": r.PathValue("instanceID"), "region": region, "securityGroupIds": input.SecurityGroupIDs}, clientIP(r))
	w.WriteHeader(http.StatusNoContent)
}
func (a *API) allocateAWSElasticIP(w http.ResponseWriter, r *http.Request) {
	a.withAWSResourceCreate(w, r, "elastic_ip", "Elastic IP", func(provider cloudaws.Provider, org string) (string, any, error) {
		item, err := provider.AllocateElasticIP(r.Context(), org)
		return item.AllocationID, item, err
	})
}
func (a *API) associateAWSElasticIP(w http.ResponseWriter, r *http.Request) {
	var input struct {
		InstanceID string `json:"instanceId"`
	}
	if !decode(w, r, &input) {
		return
	}
	orgID, accountID := pathUUID(r, "organizationID"), pathUUID(r, "accountID")
	provider, account, err := a.awsProvider(r.Context(), orgID, accountID, r.URL.Query().Get("region"))
	region := r.URL.Query().Get("region")
	if region == "" {
		region = account.DefaultRegion
	}
	if err == nil {
		if _, ok := a.requireAWSOwnership(w, r, orgID, accountID, region, "elastic_ip", r.PathValue("resourceID"), "managed"); !ok {
			return
		}
		if _, ok := a.requireAWSOwnership(w, r, orgID, accountID, region, "instance", input.InstanceID, "managed", "imported"); !ok {
			return
		}
	}
	if err == nil {
		err = provider.AssociateElasticIP(r.Context(), r.PathValue("resourceID"), input.InstanceID)
	}
	if err != nil {
		writeError(w, http.StatusBadGateway, "aws_provider_error", err.Error())
		return
	}
	_ = a.repo.RecordOrganizationAudit(r.Context(), orgID, currentUser(r.Context()).ID, "aws.elastic_ip_associated", "aws_elastic_ip", nil, requestID(r.Context()), map[string]any{"allocationId": r.PathValue("resourceID"), "instanceId": input.InstanceID, "region": region}, clientIP(r))
	w.WriteHeader(http.StatusNoContent)
}
func (a *API) disassociateAWSElasticIP(w http.ResponseWriter, r *http.Request) {
	orgID, accountID := pathUUID(r, "organizationID"), pathUUID(r, "accountID")
	provider, account, err := a.awsProvider(r.Context(), orgID, accountID, r.URL.Query().Get("region"))
	region := r.URL.Query().Get("region")
	if region == "" {
		region = account.DefaultRegion
	}
	if err == nil {
		if _, ok := a.requireAWSOwnership(w, r, orgID, accountID, region, "elastic_ip", r.PathValue("resourceID"), "managed"); !ok {
			return
		}
	}
	if err == nil {
		addresses, listErr := provider.ElasticIPs(r.Context())
		if listErr != nil {
			err = listErr
		} else {
			associationID := ""
			for _, address := range addresses {
				if address.AllocationID == r.PathValue("resourceID") {
					associationID = address.AssociationID
					break
				}
			}
			if associationID == "" {
				writeError(w, http.StatusConflict, "elastic_ip_not_associated", "The Elastic IP is not currently associated.")
				return
			}
			err = provider.DisassociateElasticIP(r.Context(), associationID)
		}
	}
	if err != nil {
		writeError(w, http.StatusBadGateway, "aws_provider_error", err.Error())
		return
	}
	_ = a.repo.RecordOrganizationAudit(r.Context(), orgID, currentUser(r.Context()).ID, "aws.elastic_ip_disassociated", "aws_elastic_ip", nil, requestID(r.Context()), map[string]any{"allocationId": r.PathValue("resourceID"), "region": region}, clientIP(r))
	w.WriteHeader(http.StatusNoContent)
}
func (a *API) releaseAWSElasticIP(w http.ResponseWriter, r *http.Request) {
	a.withManagedAWSDelete(w, r, "elastic_ip", func(provider cloudaws.Provider, id string) error { return provider.ReleaseElasticIP(r.Context(), id) })
}
func (a *API) createAWSVolume(w http.ResponseWriter, r *http.Request) {
	var input cloudaws.CreateVolumeInput
	if !decode(w, r, &input) {
		return
	}
	a.withAWSResourceCreate(w, r, "volume", input.Name, func(provider cloudaws.Provider, org string) (string, any, error) {
		input.OrganizationID = org
		item, err := provider.CreateVolume(r.Context(), input)
		return item.ID, item, err
	})
}
func (a *API) attachAWSVolume(w http.ResponseWriter, r *http.Request) {
	var input struct{ InstanceID, Device string }
	if !decode(w, r, &input) {
		return
	}
	orgID, accountID := pathUUID(r, "organizationID"), pathUUID(r, "accountID")
	provider, account, err := a.awsProvider(r.Context(), orgID, accountID, r.URL.Query().Get("region"))
	region := r.URL.Query().Get("region")
	if region == "" {
		region = account.DefaultRegion
	}
	if err == nil {
		if _, ok := a.requireAWSOwnership(w, r, orgID, accountID, region, "volume", r.PathValue("resourceID"), "managed"); !ok {
			return
		}
		if _, ok := a.requireAWSOwnership(w, r, orgID, accountID, region, "instance", input.InstanceID, "managed", "imported"); !ok {
			return
		}
	}
	if err == nil {
		err = provider.AttachVolume(r.Context(), r.PathValue("resourceID"), input.InstanceID, input.Device)
	}
	if err != nil {
		writeError(w, http.StatusBadGateway, "aws_provider_error", err.Error())
		return
	}
	_ = a.repo.RecordOrganizationAudit(r.Context(), orgID, currentUser(r.Context()).ID, "aws.volume_attached", "aws_volume", nil, requestID(r.Context()), map[string]any{"volumeId": r.PathValue("resourceID"), "instanceId": input.InstanceID, "device": input.Device, "region": region}, clientIP(r))
	w.WriteHeader(http.StatusNoContent)
}
func (a *API) detachAWSVolume(w http.ResponseWriter, r *http.Request) {
	var input struct{ InstanceID string }
	if !decode(w, r, &input) {
		return
	}
	orgID, accountID := pathUUID(r, "organizationID"), pathUUID(r, "accountID")
	provider, account, err := a.awsProvider(r.Context(), orgID, accountID, r.URL.Query().Get("region"))
	region := r.URL.Query().Get("region")
	if region == "" {
		region = account.DefaultRegion
	}
	if err == nil {
		if _, ok := a.requireAWSOwnership(w, r, orgID, accountID, region, "volume", r.PathValue("resourceID"), "managed"); !ok {
			return
		}
	}
	if err == nil {
		err = provider.DetachVolume(r.Context(), r.PathValue("resourceID"), input.InstanceID)
	}
	if err != nil {
		writeError(w, http.StatusBadGateway, "aws_provider_error", err.Error())
		return
	}
	_ = a.repo.RecordOrganizationAudit(r.Context(), orgID, currentUser(r.Context()).ID, "aws.volume_detached", "aws_volume", nil, requestID(r.Context()), map[string]any{"volumeId": r.PathValue("resourceID"), "instanceId": input.InstanceID, "region": region}, clientIP(r))
	w.WriteHeader(http.StatusNoContent)
}
func (a *API) deleteAWSVolume(w http.ResponseWriter, r *http.Request) {
	a.withManagedAWSDelete(w, r, "volume", func(provider cloudaws.Provider, id string) error { return provider.DeleteVolume(r.Context(), id) })
}
func (a *API) createAWSSnapshot(w http.ResponseWriter, r *http.Request) {
	var input struct{ VolumeID, Description, Region string }
	if !decode(w, r, &input) {
		return
	}
	orgID, accountID := pathUUID(r, "organizationID"), pathUUID(r, "accountID")
	account, err := a.repo.AWSAccountConnection(r.Context(), orgID, accountID)
	if err != nil {
		a.persistenceError(w, err)
		return
	}
	if input.Region == "" {
		input.Region = account.DefaultRegion
	}
	if _, ok := a.requireAWSOwnership(w, r, orgID, accountID, input.Region, "volume", input.VolumeID, "managed"); !ok {
		return
	}
	operation, err := a.repo.CreateAWSOperation(r.Context(), orgID, accountID, currentUser(r.Context()).ID, "create_snapshot", "create_aws_snapshot", input, requestID(r.Context()), clientIP(r))
	if err != nil {
		a.persistenceError(w, err)
		return
	}
	writeJSON(w, http.StatusAccepted, operation)
}
func (a *API) deleteAWSSnapshot(w http.ResponseWriter, r *http.Request) {
	a.withManagedAWSDelete(w, r, "snapshot", func(provider cloudaws.Provider, id string) error { return provider.DeleteSnapshot(r.Context(), id) })
}

func (a *API) getAWSCosts(w http.ResponseWriter, r *http.Request) {
	accountID, err := uuid.Parse(r.URL.Query().Get("accountId"))
	if err != nil {
		validation(w, "A valid AWS account ID is required.")
		return
	}
	now := time.Now().UTC()
	start := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, time.UTC)
	end := now.AddDate(0, 0, 1)
	provider, _, err := a.awsProvider(r.Context(), pathUUID(r, "organizationID"), accountID, "")
	if err != nil {
		a.persistenceError(w, err)
		return
	}
	report, err := provider.Costs(r.Context(), start, end)
	if err != nil {
		writeError(w, http.StatusBadGateway, "aws_billing_unavailable", err.Error())
		return
	}
	if err = a.repo.ApplyBudgetCost(r.Context(), pathUUID(r, "organizationID"), accountID, report); err != nil {
		a.serverError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, report)
}
func (a *API) listAWSOperations(w http.ResponseWriter, r *http.Request) {
	items, err := a.repo.ListAWSOperations(r.Context(), pathUUID(r, "organizationID"))
	if err != nil {
		a.serverError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"operations": items})
}
func (a *API) listBudgets(w http.ResponseWriter, r *http.Request) {
	items, err := a.repo.ListBudgets(r.Context(), pathUUID(r, "organizationID"))
	if err != nil {
		a.serverError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"budgets": items})
}
func (a *API) createBudget(w http.ResponseWriter, r *http.Request) {
	var item store.Budget
	if !decode(w, r, &item) {
		return
	}
	item.Name = strings.TrimSpace(item.Name)
	if item.Name == "" || item.MonthlyAmount <= 0 || len(item.Thresholds) == 0 {
		validation(w, "Budget name, positive monthly amount, and thresholds are required.")
		return
	}
	if item.ProviderType != "" && item.ProviderType != "silicon" {
		validation(w, "Only Silicon-local budget policies are currently supported.")
		return
	}
	if countBudgetScopes(item) > 1 {
		validation(w, "A budget must target exactly one scope: account, project, or environment.")
		return
	}
	if err := a.repo.ValidateAWSResourceScope(r.Context(), pathUUID(r, "organizationID"), item.ProjectID, item.EnvironmentID); err != nil {
		a.persistenceError(w, err)
		return
	}
	if item.AccountID != nil {
		if _, err := a.repo.AWSAccountConnection(r.Context(), pathUUID(r, "organizationID"), *item.AccountID); err != nil {
			a.persistenceError(w, err)
			return
		}
	}
	for _, value := range item.Thresholds {
		if value <= 0 || value > 1000 {
			validation(w, "Budget thresholds must be positive percentages.")
			return
		}
	}
	sort.Float64s(item.Thresholds)
	saved, err := a.repo.CreateBudget(r.Context(), pathUUID(r, "organizationID"), currentUser(r.Context()).ID, item, requestID(r.Context()), clientIP(r))
	if err != nil {
		a.persistenceError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, saved)
}
func (a *API) deleteBudget(w http.ResponseWriter, r *http.Request) {
	err := a.repo.DeleteBudget(r.Context(), pathUUID(r, "organizationID"), pathUUID(r, "budgetID"), currentUser(r.Context()).ID, requestID(r.Context()), clientIP(r))
	if err != nil {
		a.persistenceError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (a *API) awsProvider(ctx context.Context, organizationID, accountID uuid.UUID, region string) (cloudaws.Provider, store.AWSAccount, error) {
	record, err := a.repo.AWSAccountConnection(ctx, organizationID, accountID)
	if err != nil {
		return nil, store.AWSAccount{}, err
	}
	if region == "" {
		region = record.DefaultRegion
	}
	config := cloudaws.AccountConfig{AccountID: record.AccountID, RoleARN: record.RoleARN, Region: region}
	if len(record.EncryptedExternalID) > 0 {
		if a.box == nil {
			return nil, record.AWSAccount, errors.New("AWS credential encryption is unavailable")
		}
		externalID, openErr := a.box.Open(record.EncryptedExternalID, awsCredentialContext(organizationID, accountID, "external-id"))
		if openErr != nil {
			return nil, record.AWSAccount, errors.New("AWS external ID could not be decrypted")
		}
		config.ExternalID = string(externalID)
		zeroBytes(externalID)
	}
	if len(record.EncryptedAccessKeyID) > 0 {
		if a.box == nil {
			return nil, record.AWSAccount, errors.New("AWS credential encryption is unavailable")
		}
		id, openErr := a.box.Open(record.EncryptedAccessKeyID, awsCredentialContext(organizationID, accountID, "access-key-id"))
		if openErr != nil {
			return nil, record.AWSAccount, errors.New("AWS bootstrap credentials could not be decrypted")
		}
		secret, openErr := a.box.Open(record.EncryptedSecretAccessKey, awsCredentialContext(organizationID, accountID, "secret-access-key"))
		if openErr != nil {
			zeroBytes(id)
			return nil, record.AWSAccount, errors.New("AWS bootstrap credentials could not be decrypted")
		}
		config.AccessKeyID = string(id)
		config.SecretAccessKey = string(secret)
		zeroBytes(id)
		zeroBytes(secret)
	}
	provider, err := a.awsFactory.Open(ctx, config)
	config.ExternalID = ""
	config.AccessKeyID = ""
	config.SecretAccessKey = ""
	return provider, record.AWSAccount, err
}
func (a *API) withAWSResourceCreate(w http.ResponseWriter, r *http.Request, resourceType, name string, create func(cloudaws.Provider, string) (string, any, error)) {
	orgID, accountID := pathUUID(r, "organizationID"), pathUUID(r, "accountID")
	region := r.URL.Query().Get("region")
	provider, account, err := a.awsProvider(r.Context(), orgID, accountID, region)
	if err != nil {
		a.persistenceError(w, err)
		return
	}
	if region == "" {
		region = account.DefaultRegion
	}
	id, item, err := create(provider, orgID.String())
	if err != nil {
		writeError(w, http.StatusBadGateway, "aws_provider_error", err.Error())
		return
	}
	_, err = a.repo.ClaimAWSResource(r.Context(), orgID, accountID, currentUser(r.Context()).ID, region, resourceType, id, "managed", nil, nil, requestID(r.Context()), clientIP(r))
	if err != nil {
		a.persistenceError(w, err)
		return
	}
	_ = name
	writeJSON(w, http.StatusCreated, item)
}
func (a *API) withManagedAWSDelete(w http.ResponseWriter, r *http.Request, resourceType string, remove func(cloudaws.Provider, string) error) {
	orgID, accountID := pathUUID(r, "organizationID"), pathUUID(r, "accountID")
	region := r.URL.Query().Get("region")
	provider, account, err := a.awsProvider(r.Context(), orgID, accountID, region)
	if err != nil {
		a.persistenceError(w, err)
		return
	}
	if region == "" {
		region = account.DefaultRegion
	}
	ownership, err := a.repo.ResourceOwnership(r.Context(), orgID, accountID, region, resourceType, r.PathValue("resourceID"))
	if err != nil || ownership.Ownership != "managed" {
		writeError(w, http.StatusConflict, "resource_not_owned", "Only Silicon-managed AWS resources may be deleted.")
		return
	}
	if err = remove(provider, r.PathValue("resourceID")); err != nil {
		writeError(w, http.StatusBadGateway, "aws_provider_error", err.Error())
		return
	}
	_ = a.repo.RecordOrganizationAudit(r.Context(), orgID, currentUser(r.Context()).ID, "aws."+resourceType+"_deleted", "aws_"+resourceType, &ownership.ID, requestID(r.Context()), map[string]any{"providerResourceId": r.PathValue("resourceID"), "region": region}, clientIP(r))
	w.WriteHeader(http.StatusNoContent)
}

func (a *API) requireAWSOwnership(w http.ResponseWriter, r *http.Request, organizationID, accountID uuid.UUID, region, resourceType, providerID string, allowed ...string) (store.AWSResourceOwnership, bool) {
	ownership, err := a.repo.ResourceOwnership(r.Context(), organizationID, accountID, region, resourceType, providerID)
	if err != nil || !oneOf(ownership.Ownership, allowed...) {
		writeError(w, http.StatusConflict, "resource_not_owned", "External AWS resources are read-only until explicitly imported into Silicon.")
		return store.AWSResourceOwnership{}, false
	}
	return ownership, true
}

func cleanRegions(values []string) []string {
	seen := map[string]bool{}
	result := []string{}
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value != "" && !seen[value] {
			seen[value] = true
			result = append(result, value)
		}
	}
	sort.Strings(result)
	return result
}
func regionEnabled(account store.AWSAccount, region string) bool {
	for _, value := range account.EnabledRegions {
		if value == region {
			return true
		}
	}
	return false
}
func stringInSlice(value string, values []string) bool {
	for _, candidate := range values {
		if strings.TrimSpace(candidate) == value {
			return true
		}
	}
	return false
}
func validLinuxUsername(value string) bool {
	if len(value) < 1 || len(value) > 32 || value[0] == '-' {
		return false
	}
	for _, character := range value {
		if character == '_' || character == '-' || character >= 'a' && character <= 'z' || character >= 'A' && character <= 'Z' || character >= '0' && character <= '9' {
			continue
		}
		return false
	}
	return true
}
func countBudgetScopes(item store.Budget) int {
	count := 0
	if item.AccountID != nil {
		count++
	}
	if item.ProjectID != nil {
		count++
	}
	if item.EnvironmentID != nil {
		count++
	}
	return count
}
func zeroBytes(value []byte) {
	for index := range value {
		value[index] = 0
	}
}
