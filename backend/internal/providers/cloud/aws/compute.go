package aws

import (
	"context"
	"encoding/base64"
	"errors"
	"sort"
	"strings"
	"time"

	sdk "github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"
	"github.com/aws/aws-sdk-go-v2/service/ssm"
)

func (c *Client) Regions(ctx context.Context) ([]string, error) {
	out, err := c.ec2.DescribeRegions(ctx, &ec2.DescribeRegionsInput{AllRegions: sdk.Bool(false)})
	if err != nil {
		return nil, safeError("list AWS regions", err)
	}
	regions := make([]string, 0, len(out.Regions))
	for _, item := range out.Regions {
		if value := sdk.ToString(item.RegionName); value != "" {
			regions = append(regions, value)
		}
	}
	sort.Strings(regions)
	return regions, nil
}

func (c *Client) Instances(ctx context.Context) ([]Instance, error) {
	var result []Instance
	var token *string
	for {
		out, err := c.ec2.DescribeInstances(ctx, &ec2.DescribeInstancesInput{NextToken: token})
		if err != nil {
			return nil, safeError("list EC2 instances", err)
		}
		for _, reservation := range out.Reservations {
			for _, item := range reservation.Instances {
				result = append(result, normalizeInstance(c.region, item))
			}
		}
		token = out.NextToken
		if token == nil || sdk.ToString(token) == "" {
			break
		}
	}
	return result, nil
}

func (c *Client) Instance(ctx context.Context, id string) (Instance, error) {
	out, err := c.ec2.DescribeInstances(ctx, &ec2.DescribeInstancesInput{InstanceIds: []string{id}})
	if err != nil {
		return Instance{}, safeError("inspect EC2 instance", err)
	}
	for _, reservation := range out.Reservations {
		for _, item := range reservation.Instances {
			return normalizeInstance(c.region, item), nil
		}
	}
	return Instance{}, errors.New("EC2 instance was not found")
}

func (c *Client) InstanceType(ctx context.Context, name string) (InstanceTypeInfo, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return InstanceTypeInfo{}, errors.New("instance type is required")
	}
	out, err := c.ec2.DescribeInstanceTypes(ctx, &ec2.DescribeInstanceTypesInput{InstanceTypes: []ec2types.InstanceType{ec2types.InstanceType(name)}})
	if err != nil {
		return InstanceTypeInfo{}, safeError("inspect EC2 instance type", err)
	}
	if len(out.InstanceTypes) != 1 {
		return InstanceTypeInfo{}, errors.New("EC2 instance type was not found in the selected region")
	}
	item := out.InstanceTypes[0]
	info := InstanceTypeInfo{Name: string(item.InstanceType)}
	if item.VCpuInfo != nil {
		info.VCPUs = sdk.ToInt32(item.VCpuInfo.DefaultVCpus)
	}
	if item.MemoryInfo != nil {
		info.MemoryMiB = sdk.ToInt64(item.MemoryInfo.SizeInMiB)
	}
	if item.ProcessorInfo != nil {
		for _, architecture := range item.ProcessorInfo.SupportedArchitectures {
			info.Architectures = append(info.Architectures, string(architecture))
		}
	}
	return info, nil
}

func (c *Client) CreateMachine(ctx context.Context, input CreateMachineInput) (Instance, error) {
	if strings.TrimSpace(input.Name) == "" || strings.TrimSpace(input.InstanceType) == "" {
		return Instance{}, errors.New("machine name and instance type are required")
	}
	imageID := strings.TrimSpace(input.ImageID)
	if imageID == "" {
		image, err := c.ResolveImage(ctx, input.Distribution, input.Architecture)
		if err != nil {
			return Instance{}, err
		}
		imageID = image.ID
	}
	if input.RootDiskGiB < 8 {
		input.RootDiskGiB = 20
	}
	if input.RootDiskType == "" {
		input.RootDiskType = "gp3"
	}
	request := &ec2.RunInstancesInput{
		ImageId: sdk.String(imageID), InstanceType: ec2types.InstanceType(input.InstanceType), MinCount: sdk.Int32(1), MaxCount: sdk.Int32(1),
		BlockDeviceMappings: []ec2types.BlockDeviceMapping{{DeviceName: sdk.String("/dev/sda1"), Ebs: &ec2types.EbsBlockDevice{DeleteOnTermination: sdk.Bool(true), Encrypted: sdk.Bool(input.EncryptRootDisk), VolumeSize: sdk.Int32(input.RootDiskGiB), VolumeType: ec2types.VolumeType(input.RootDiskType)}}},
		TagSpecifications:   []ec2types.TagSpecification{{ResourceType: ec2types.ResourceTypeInstance, Tags: siliconTags(input.OrganizationID, input.ProjectID, input.EnvironmentID, input.Name, input.ServerLabels)}, {ResourceType: ec2types.ResourceTypeVolume, Tags: siliconTags(input.OrganizationID, input.ProjectID, input.EnvironmentID, input.Name+"-root", nil)}},
	}
	if input.SSHKeyName != "" {
		request.KeyName = sdk.String(input.SSHKeyName)
	}
	if input.InstanceProfile != "" {
		request.IamInstanceProfile = &ec2types.IamInstanceProfileSpecification{}
		if strings.HasPrefix(input.InstanceProfile, "arn:") {
			request.IamInstanceProfile.Arn = sdk.String(input.InstanceProfile)
		} else {
			request.IamInstanceProfile.Name = sdk.String(input.InstanceProfile)
		}
	}
	if input.AvailabilityZone != "" {
		request.Placement = &ec2types.Placement{AvailabilityZone: sdk.String(input.AvailabilityZone)}
	}
	if input.SubnetID != "" {
		request.NetworkInterfaces = []ec2types.InstanceNetworkInterfaceSpecification{{DeviceIndex: sdk.Int32(0), SubnetId: sdk.String(input.SubnetID), Groups: input.SecurityGroupIDs, AssociatePublicIpAddress: sdk.Bool(input.PublicIPv4), DeleteOnTermination: sdk.Bool(true)}}
	} else {
		request.SecurityGroupIds = input.SecurityGroupIDs
	}
	if input.DockerBootstrap {
		request.UserData = sdk.String(base64.StdEncoding.EncodeToString([]byte(dockerBootstrap(input.Distribution, input.BootstrapUser))))
	}
	out, err := c.ec2.RunInstances(ctx, request)
	if err != nil {
		return Instance{}, safeError("create EC2 instance", err)
	}
	if len(out.Instances) != 1 {
		return Instance{}, errors.New("AWS did not return the created EC2 instance")
	}
	id := sdk.ToString(out.Instances[0].InstanceId)
	waiter := ec2.NewInstanceRunningWaiter(c.ec2)
	if err = waiter.Wait(ctx, &ec2.DescribeInstancesInput{InstanceIds: []string{id}}, 10*time.Minute); err != nil {
		return normalizeInstance(c.region, out.Instances[0]), safeError("wait for EC2 instance", err)
	}
	return c.Instance(ctx, id)
}

func (c *Client) StartInstance(ctx context.Context, id string) error {
	_, err := c.ec2.StartInstances(ctx, &ec2.StartInstancesInput{InstanceIds: []string{id}})
	return safeError("start EC2 instance", err)
}
func (c *Client) StopInstance(ctx context.Context, id string) error {
	_, err := c.ec2.StopInstances(ctx, &ec2.StopInstancesInput{InstanceIds: []string{id}})
	return safeError("stop EC2 instance", err)
}
func (c *Client) RebootInstance(ctx context.Context, id string) error {
	_, err := c.ec2.RebootInstances(ctx, &ec2.RebootInstancesInput{InstanceIds: []string{id}})
	return safeError("reboot EC2 instance", err)
}
func (c *Client) TerminateInstance(ctx context.Context, id string) error {
	_, err := c.ec2.TerminateInstances(ctx, &ec2.TerminateInstancesInput{InstanceIds: []string{id}})
	return safeError("terminate EC2 instance", err)
}

func (c *Client) ResolveImage(ctx context.Context, distribution, architecture string) (Image, error) {
	architecture = strings.ToLower(strings.TrimSpace(architecture))
	if architecture == "" || architecture == "amd64" {
		architecture = "x86_64"
	}
	if architecture == "arm64" {
		architecture = "arm64"
	}
	distribution = strings.ToLower(strings.TrimSpace(distribution))
	parameter := ""
	description := ""
	switch distribution {
	case "", "ubuntu", "ubuntu-24.04":
		distribution, description = "ubuntu", "Ubuntu Server 24.04 LTS"
		parameter = "/aws/service/canonical/ubuntu/server/24.04/stable/current/" + architecture + "/hvm/ebs-gp3/ami-id"
	case "amazon-linux", "amazon-linux-2023", "al2023":
		distribution, description = "amazon-linux", "Amazon Linux 2023"
		parameter = "/aws/service/ami-amazon-linux-latest/al2023-ami-kernel-default-" + architecture
	default:
		return Image{}, errors.New("unsupported Linux image")
	}
	out, err := c.ssm.GetParameter(ctx, &ssm.GetParameterInput{Name: sdk.String(parameter)})
	if err != nil {
		return Image{}, safeError("resolve current Linux AMI", err)
	}
	id := ""
	if out.Parameter != nil {
		id = sdk.ToString(out.Parameter.Value)
	}
	if !strings.HasPrefix(id, "ami-") {
		return Image{}, errors.New("AWS returned an invalid AMI identifier")
	}
	return Image{ID: id, Name: description, Distribution: distribution, Architecture: architecture, Description: "Resolved from the AWS public SSM parameter at provisioning time."}, nil
}

func normalizeInstance(region string, item ec2types.Instance) Instance {
	tags := tagMap(item.Tags)
	groups := make([]string, 0, len(item.SecurityGroups))
	for _, group := range item.SecurityGroups {
		if id := sdk.ToString(group.GroupId); id != "" {
			groups = append(groups, id)
		}
	}
	return Instance{ID: sdk.ToString(item.InstanceId), Name: tags["Name"], State: string(item.State.Name), InstanceType: string(item.InstanceType), Architecture: string(item.Architecture), Region: region, AvailabilityZone: sdk.ToString(item.Placement.AvailabilityZone), ImageID: sdk.ToString(item.ImageId), PrivateIP: sdk.ToString(item.PrivateIpAddress), PublicIP: sdk.ToString(item.PublicIpAddress), VPCID: sdk.ToString(item.VpcId), SubnetID: sdk.ToString(item.SubnetId), SecurityGroupIDs: groups, Tags: tags, LaunchedAt: item.LaunchTime, Ownership: ownership(tags)}
}

func siliconTags(organization, project, environment, name string, extra map[string]string) []ec2types.Tag {
	values := map[string]string{"Name": name, "silicon:managed": "true", "silicon:organization": organization, "silicon:resource": name}
	if project != "" {
		values["silicon:project"] = project
	}
	if environment != "" {
		values["silicon:environment"] = environment
	}
	for key, value := range extra {
		if strings.HasPrefix(key, "silicon:") || key == "Name" {
			continue
		}
		values[key] = value
	}
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	tags := make([]ec2types.Tag, 0, len(keys))
	for _, key := range keys {
		tags = append(tags, ec2types.Tag{Key: sdk.String(key), Value: sdk.String(values[key])})
	}
	return tags
}
func tagMap(tags []ec2types.Tag) map[string]string {
	result := map[string]string{}
	for _, tag := range tags {
		result[sdk.ToString(tag.Key)] = sdk.ToString(tag.Value)
	}
	return result
}
func ownership(tags map[string]string) Ownership {
	if tags["silicon:managed"] == "true" {
		return OwnershipManaged
	}
	return OwnershipExternal
}

func dockerBootstrap(distribution, user string) string {
	grant := ""
	if user != "" {
		grant = "  - [usermod, -aG, docker, " + user + "]\n"
	}
	if strings.HasPrefix(strings.ToLower(strings.TrimSpace(distribution)), "amazon-linux") || strings.EqualFold(strings.TrimSpace(distribution), "al2023") {
		return "#cloud-config\nruncmd:\n  - [dnf, install, -y, docker]\n  - [systemctl, enable, --now, docker]\n" + grant
	}
	return "#cloud-config\npackage_update: true\npackages:\n  - docker.io\nruncmd:\n  - [systemctl, enable, --now, docker]\n" + grant
}
