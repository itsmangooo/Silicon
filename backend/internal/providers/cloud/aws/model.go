package aws

import (
	"context"
	"time"

	"github.com/google/uuid"
)

type Ownership string

const (
	OwnershipExternal Ownership = "external"
	OwnershipImported Ownership = "imported"
	OwnershipManaged  Ownership = "managed"
)

type AccountConfig struct {
	AccountID       string
	RoleARN         string
	ExternalID      string
	Region          string
	AccessKeyID     string
	SecretAccessKey string
}

type Identity struct {
	AccountID string `json:"accountId"`
	ARN       string `json:"arn"`
	UserID    string `json:"userId"`
}

type Instance struct {
	ID               string            `json:"id"`
	Name             string            `json:"name"`
	State            string            `json:"state"`
	InstanceType     string            `json:"instanceType"`
	Architecture     string            `json:"architecture"`
	Region           string            `json:"region"`
	AvailabilityZone string            `json:"availabilityZone"`
	ImageID          string            `json:"imageId"`
	PrivateIP        string            `json:"privateIp"`
	PublicIP         string            `json:"publicIp"`
	VPCID            string            `json:"vpcId"`
	SubnetID         string            `json:"subnetId"`
	SecurityGroupIDs []string          `json:"securityGroupIds"`
	Tags             map[string]string `json:"tags"`
	LaunchedAt       *time.Time        `json:"launchedAt,omitempty"`
	Ownership        Ownership         `json:"ownership"`
	SiliconServerID  string            `json:"siliconServerId,omitempty"`
}

type VPC struct {
	ID        string            `json:"id"`
	CIDR      string            `json:"cidr"`
	State     string            `json:"state"`
	IsDefault bool              `json:"isDefault"`
	Tags      map[string]string `json:"tags"`
	Ownership Ownership         `json:"ownership"`
}

type Subnet struct {
	ID               string            `json:"id"`
	VPCID            string            `json:"vpcId"`
	CIDR             string            `json:"cidr"`
	AvailabilityZone string            `json:"availabilityZone"`
	Public           bool              `json:"public"`
	State            string            `json:"state"`
	Tags             map[string]string `json:"tags"`
	Ownership        Ownership         `json:"ownership"`
}

type SecurityRule struct {
	Direction   string   `json:"direction"`
	Protocol    string   `json:"protocol"`
	FromPort    *int32   `json:"fromPort,omitempty"`
	ToPort      *int32   `json:"toPort,omitempty"`
	CIDRs       []string `json:"cidrs"`
	Description string   `json:"description"`
}

type SecurityGroup struct {
	ID          string            `json:"id"`
	VPCID       string            `json:"vpcId"`
	Name        string            `json:"name"`
	Description string            `json:"description"`
	Rules       []SecurityRule    `json:"rules"`
	Tags        map[string]string `json:"tags"`
	Ownership   Ownership         `json:"ownership"`
}

type ElasticIP struct {
	AllocationID  string    `json:"allocationId"`
	AssociationID string    `json:"associationId,omitempty"`
	PublicIP      string    `json:"publicIp"`
	InstanceID    string    `json:"instanceId,omitempty"`
	Ownership     Ownership `json:"ownership"`
	Unused        bool      `json:"unused"`
}

type Volume struct {
	ID               string    `json:"id"`
	AvailabilityZone string    `json:"availabilityZone"`
	SizeGiB          int32     `json:"sizeGiB"`
	Type             string    `json:"type"`
	State            string    `json:"state"`
	Encrypted        bool      `json:"encrypted"`
	InstanceID       string    `json:"instanceId,omitempty"`
	Device           string    `json:"device,omitempty"`
	Ownership        Ownership `json:"ownership"`
}

type Snapshot struct {
	ID          string    `json:"id"`
	VolumeID    string    `json:"volumeId"`
	SizeGiB     int32     `json:"sizeGiB"`
	State       string    `json:"state"`
	Encrypted   bool      `json:"encrypted"`
	Description string    `json:"description"`
	StartedAt   time.Time `json:"startedAt"`
	Ownership   Ownership `json:"ownership"`
}

type Image struct {
	ID           string `json:"id"`
	Name         string `json:"name"`
	Distribution string `json:"distribution"`
	Architecture string `json:"architecture"`
	Description  string `json:"description"`
}

type InstanceTypeInfo struct {
	Name          string   `json:"name"`
	VCPUs         int32    `json:"vcpus"`
	MemoryMiB     int64    `json:"memoryMiB"`
	Architectures []string `json:"architectures"`
}

type CreateMachineInput struct {
	Name             string
	Region           string
	AvailabilityZone string
	Distribution     string
	Architecture     string
	ImageID          string
	InstanceType     string
	VPCID            string
	SubnetID         string
	SecurityGroupIDs []string
	PublicIPv4       bool
	ElasticIP        bool
	RootDiskGiB      int32
	RootDiskType     string
	EncryptRootDisk  bool
	SSHKeyName       string
	InstanceProfile  string
	BootstrapUser    string
	DockerBootstrap  bool
	OrganizationID   string
	ProjectID        string
	EnvironmentID    string
	ServerLabels     map[string]string
	ConnectionMethod string
}

type CreateVPCInput struct{ Name, CIDR, OrganizationID, ProjectID, EnvironmentID string }
type CreateSubnetInput struct {
	Name, VPCID, CIDR, AvailabilityZone, OrganizationID string
	Public                                              bool
}
type CreateSecurityGroupInput struct{ Name, Description, VPCID, OrganizationID string }
type CreateVolumeInput struct {
	Name, AvailabilityZone, Type, OrganizationID string
	SizeGiB                                      int32
	Encrypted                                    bool
}

type CostPoint struct {
	Start  time.Time `json:"start"`
	End    time.Time `json:"end"`
	Amount float64   `json:"amount"`
	Unit   string    `json:"unit"`
}

type CostGroup struct {
	Name   string  `json:"name"`
	Amount float64 `json:"amount"`
	Unit   string  `json:"unit"`
}

type CostReport struct {
	PeriodStart                     time.Time   `json:"periodStart"`
	PeriodEnd                       time.Time   `json:"periodEnd"`
	Daily                           []CostPoint `json:"daily"`
	Services                        []CostGroup `json:"services"`
	Regions                         []CostGroup `json:"regions"`
	Projects                        []CostGroup `json:"projects"`
	Environments                    []CostGroup `json:"environments"`
	ProjectAttributionAvailable     bool        `json:"projectAttributionAvailable"`
	EnvironmentAttributionAvailable bool        `json:"environmentAttributionAvailable"`
	Total                           float64     `json:"total"`
	Unit                            string      `json:"unit"`
	PreviousPeriodTotal             *float64    `json:"previousPeriodTotal,omitempty"`
	Forecast                        *float64    `json:"forecast,omitempty"`
	Freshness                       string      `json:"freshness"`
}

type Estimate struct {
	ComputeMonthly float64  `json:"computeMonthly"`
	StorageMonthly float64  `json:"storageMonthly"`
	TotalMonthly   float64  `json:"totalMonthly"`
	Currency       string   `json:"currency"`
	Exclusions     []string `json:"exclusions"`
	Source         string   `json:"source"`
}

type Provider interface {
	Identity(context.Context) (Identity, error)
	Regions(context.Context) ([]string, error)
	Instances(context.Context) ([]Instance, error)
	Instance(context.Context, string) (Instance, error)
	InstanceType(context.Context, string) (InstanceTypeInfo, error)
	CreateMachine(context.Context, CreateMachineInput) (Instance, error)
	StartInstance(context.Context, string) error
	StopInstance(context.Context, string) error
	RebootInstance(context.Context, string) error
	TerminateInstance(context.Context, string) error
	VPCs(context.Context) ([]VPC, error)
	CreateVPC(context.Context, CreateVPCInput) (VPC, error)
	DeleteVPC(context.Context, string) error
	Subnets(context.Context) ([]Subnet, error)
	CreateSubnet(context.Context, CreateSubnetInput) (Subnet, error)
	SecurityGroups(context.Context) ([]SecurityGroup, error)
	CreateSecurityGroup(context.Context, CreateSecurityGroupInput) (SecurityGroup, error)
	SetInstanceSecurityGroups(context.Context, string, []string) error
	AddSecurityRule(context.Context, string, SecurityRule) error
	RemoveSecurityRule(context.Context, string, SecurityRule) error
	ElasticIPs(context.Context) ([]ElasticIP, error)
	AllocateElasticIP(context.Context, string) (ElasticIP, error)
	AssociateElasticIP(context.Context, string, string) error
	DisassociateElasticIP(context.Context, string) error
	ReleaseElasticIP(context.Context, string) error
	Volumes(context.Context) ([]Volume, error)
	CreateVolume(context.Context, CreateVolumeInput) (Volume, error)
	AttachVolume(context.Context, string, string, string) error
	DetachVolume(context.Context, string, string) error
	DeleteVolume(context.Context, string) error
	Snapshots(context.Context) ([]Snapshot, error)
	CreateSnapshot(context.Context, string, string, string) (Snapshot, error)
	DeleteSnapshot(context.Context, string) error
	ResolveImage(context.Context, string, string) (Image, error)
	Costs(context.Context, time.Time, time.Time) (CostReport, error)
	Estimate(context.Context, CreateMachineInput) (Estimate, error)
}

type Factory interface {
	Open(context.Context, AccountConfig) (Provider, error)
}

type CommandResult struct {
	Stdout string
	Stderr string
	Status string
}

type SSMRunner interface {
	RunSSMCommand(context.Context, string, []string, time.Duration) (CommandResult, error)
}

func CredentialContext(organizationID, accountID uuid.UUID, field string) string {
	return "aws-account:" + organizationID.String() + ":" + accountID.String() + ":" + field
}

func MachineKeyContext(organizationID, accountID uuid.UUID) string {
	return "aws-machine-key:" + organizationID.String() + ":" + accountID.String()
}
