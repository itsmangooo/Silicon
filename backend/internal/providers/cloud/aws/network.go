package aws

import (
	"context"
	"errors"

	sdk "github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"
)

func (c *Client) VPCs(ctx context.Context) ([]VPC, error) {
	out, err := c.ec2.DescribeVpcs(ctx, &ec2.DescribeVpcsInput{})
	if err != nil {
		return nil, safeError("list VPCs", err)
	}
	items := make([]VPC, 0, len(out.Vpcs))
	for _, item := range out.Vpcs {
		tags := tagMap(item.Tags)
		items = append(items, VPC{ID: sdk.ToString(item.VpcId), CIDR: sdk.ToString(item.CidrBlock), State: string(item.State), IsDefault: sdk.ToBool(item.IsDefault), Tags: tags, Ownership: ownership(tags)})
	}
	return items, nil
}
func (c *Client) CreateVPC(ctx context.Context, input CreateVPCInput) (VPC, error) {
	if input.CIDR == "" || input.Name == "" {
		return VPC{}, errors.New("VPC name and CIDR are required")
	}
	out, err := c.ec2.CreateVpc(ctx, &ec2.CreateVpcInput{CidrBlock: sdk.String(input.CIDR), TagSpecifications: []ec2types.TagSpecification{{ResourceType: ec2types.ResourceTypeVpc, Tags: siliconTags(input.OrganizationID, input.ProjectID, input.EnvironmentID, input.Name, nil)}}})
	if err != nil {
		return VPC{}, safeError("create VPC", err)
	}
	item := out.Vpc
	tags := tagMap(item.Tags)
	return VPC{ID: sdk.ToString(item.VpcId), CIDR: sdk.ToString(item.CidrBlock), State: string(item.State), Tags: tags, Ownership: OwnershipManaged}, nil
}
func (c *Client) DeleteVPC(ctx context.Context, id string) error {
	_, err := c.ec2.DeleteVpc(ctx, &ec2.DeleteVpcInput{VpcId: sdk.String(id)})
	return safeError("delete VPC", err)
}

func (c *Client) Subnets(ctx context.Context) ([]Subnet, error) {
	out, err := c.ec2.DescribeSubnets(ctx, &ec2.DescribeSubnetsInput{})
	if err != nil {
		return nil, safeError("list subnets", err)
	}
	items := make([]Subnet, 0, len(out.Subnets))
	for _, item := range out.Subnets {
		tags := tagMap(item.Tags)
		items = append(items, Subnet{ID: sdk.ToString(item.SubnetId), VPCID: sdk.ToString(item.VpcId), CIDR: sdk.ToString(item.CidrBlock), AvailabilityZone: sdk.ToString(item.AvailabilityZone), Public: sdk.ToBool(item.MapPublicIpOnLaunch), State: string(item.State), Tags: tags, Ownership: ownership(tags)})
	}
	return items, nil
}
func (c *Client) CreateSubnet(ctx context.Context, input CreateSubnetInput) (Subnet, error) {
	if input.Name == "" || input.VPCID == "" || input.CIDR == "" {
		return Subnet{}, errors.New("subnet name, VPC, and CIDR are required")
	}
	request := &ec2.CreateSubnetInput{VpcId: sdk.String(input.VPCID), CidrBlock: sdk.String(input.CIDR), TagSpecifications: []ec2types.TagSpecification{{ResourceType: ec2types.ResourceTypeSubnet, Tags: siliconTags(input.OrganizationID, "", "", input.Name, nil)}}}
	if input.AvailabilityZone != "" {
		request.AvailabilityZone = sdk.String(input.AvailabilityZone)
	}
	out, err := c.ec2.CreateSubnet(ctx, request)
	if err != nil {
		return Subnet{}, safeError("create subnet", err)
	}
	id := sdk.ToString(out.Subnet.SubnetId)
	if input.Public {
		_, err = c.ec2.ModifySubnetAttribute(ctx, &ec2.ModifySubnetAttributeInput{SubnetId: sdk.String(id), MapPublicIpOnLaunch: &ec2types.AttributeBooleanValue{Value: sdk.Bool(true)}})
		if err != nil {
			return Subnet{}, safeError("configure public subnet", err)
		}
	}
	return Subnet{ID: id, VPCID: input.VPCID, CIDR: input.CIDR, AvailabilityZone: sdk.ToString(out.Subnet.AvailabilityZone), Public: input.Public, State: string(out.Subnet.State), Tags: tagMap(out.Subnet.Tags), Ownership: OwnershipManaged}, nil
}

func (c *Client) SecurityGroups(ctx context.Context) ([]SecurityGroup, error) {
	out, err := c.ec2.DescribeSecurityGroups(ctx, &ec2.DescribeSecurityGroupsInput{})
	if err != nil {
		return nil, safeError("list security groups", err)
	}
	items := make([]SecurityGroup, 0, len(out.SecurityGroups))
	for _, item := range out.SecurityGroups {
		tags := tagMap(item.Tags)
		group := SecurityGroup{ID: sdk.ToString(item.GroupId), VPCID: sdk.ToString(item.VpcId), Name: sdk.ToString(item.GroupName), Description: sdk.ToString(item.Description), Tags: tags, Ownership: ownership(tags)}
		group.Rules = append(group.Rules, normalizeRules("ingress", item.IpPermissions)...)
		group.Rules = append(group.Rules, normalizeRules("egress", item.IpPermissionsEgress)...)
		items = append(items, group)
	}
	return items, nil
}
func (c *Client) CreateSecurityGroup(ctx context.Context, input CreateSecurityGroupInput) (SecurityGroup, error) {
	if input.Name == "" || input.VPCID == "" {
		return SecurityGroup{}, errors.New("security group name and VPC are required")
	}
	out, err := c.ec2.CreateSecurityGroup(ctx, &ec2.CreateSecurityGroupInput{GroupName: sdk.String(input.Name), Description: sdk.String(input.Description), VpcId: sdk.String(input.VPCID), TagSpecifications: []ec2types.TagSpecification{{ResourceType: ec2types.ResourceTypeSecurityGroup, Tags: siliconTags(input.OrganizationID, "", "", input.Name, nil)}}})
	if err != nil {
		return SecurityGroup{}, safeError("create security group", err)
	}
	return SecurityGroup{ID: sdk.ToString(out.GroupId), VPCID: input.VPCID, Name: input.Name, Description: input.Description, Ownership: OwnershipManaged}, nil
}
func (c *Client) SetInstanceSecurityGroups(ctx context.Context, instanceID string, groupIDs []string) error {
	if instanceID == "" || len(groupIDs) == 0 {
		return errors.New("instance and at least one security group are required")
	}
	_, err := c.ec2.ModifyInstanceAttribute(ctx, &ec2.ModifyInstanceAttributeInput{InstanceId: sdk.String(instanceID), Groups: groupIDs})
	return safeError("attach security groups", err)
}
func (c *Client) AddSecurityRule(ctx context.Context, id string, rule SecurityRule) error {
	permission, err := permission(rule)
	if err != nil {
		return err
	}
	if rule.Direction == "egress" {
		_, err = c.ec2.AuthorizeSecurityGroupEgress(ctx, &ec2.AuthorizeSecurityGroupEgressInput{GroupId: sdk.String(id), IpPermissions: []ec2types.IpPermission{permission}})
	} else {
		_, err = c.ec2.AuthorizeSecurityGroupIngress(ctx, &ec2.AuthorizeSecurityGroupIngressInput{GroupId: sdk.String(id), IpPermissions: []ec2types.IpPermission{permission}})
	}
	return safeError("add security group rule", err)
}
func (c *Client) RemoveSecurityRule(ctx context.Context, id string, rule SecurityRule) error {
	permission, err := permission(rule)
	if err != nil {
		return err
	}
	if rule.Direction == "egress" {
		_, err = c.ec2.RevokeSecurityGroupEgress(ctx, &ec2.RevokeSecurityGroupEgressInput{GroupId: sdk.String(id), IpPermissions: []ec2types.IpPermission{permission}})
	} else {
		_, err = c.ec2.RevokeSecurityGroupIngress(ctx, &ec2.RevokeSecurityGroupIngressInput{GroupId: sdk.String(id), IpPermissions: []ec2types.IpPermission{permission}})
	}
	return safeError("remove security group rule", err)
}

func permission(rule SecurityRule) (ec2types.IpPermission, error) {
	if rule.Protocol == "" || len(rule.CIDRs) == 0 {
		return ec2types.IpPermission{}, errors.New("protocol and at least one CIDR are required")
	}
	ranges := make([]ec2types.IpRange, 0, len(rule.CIDRs))
	for _, cidr := range rule.CIDRs {
		if cidr == "0.0.0.0/0" && rule.Description == "" {
			return ec2types.IpPermission{}, errors.New("world-accessible rules require an explicit description")
		}
		ranges = append(ranges, ec2types.IpRange{CidrIp: sdk.String(cidr), Description: sdk.String(rule.Description)})
	}
	return ec2types.IpPermission{IpProtocol: sdk.String(rule.Protocol), FromPort: rule.FromPort, ToPort: rule.ToPort, IpRanges: ranges}, nil
}
func normalizeRules(direction string, rules []ec2types.IpPermission) []SecurityRule {
	result := []SecurityRule{}
	for _, rule := range rules {
		cidrs := []string{}
		description := ""
		for _, item := range rule.IpRanges {
			cidrs = append(cidrs, sdk.ToString(item.CidrIp))
			if description == "" {
				description = sdk.ToString(item.Description)
			}
		}
		result = append(result, SecurityRule{Direction: direction, Protocol: sdk.ToString(rule.IpProtocol), FromPort: rule.FromPort, ToPort: rule.ToPort, CIDRs: cidrs, Description: description})
	}
	return result
}

func (c *Client) ElasticIPs(ctx context.Context) ([]ElasticIP, error) {
	out, err := c.ec2.DescribeAddresses(ctx, &ec2.DescribeAddressesInput{})
	if err != nil {
		return nil, safeError("list Elastic IPs", err)
	}
	items := make([]ElasticIP, 0, len(out.Addresses))
	for _, item := range out.Addresses {
		tags := tagMap(item.Tags)
		instanceID := sdk.ToString(item.InstanceId)
		items = append(items, ElasticIP{AllocationID: sdk.ToString(item.AllocationId), AssociationID: sdk.ToString(item.AssociationId), PublicIP: sdk.ToString(item.PublicIp), InstanceID: instanceID, Ownership: ownership(tags), Unused: instanceID == ""})
	}
	return items, nil
}
func (c *Client) AllocateElasticIP(ctx context.Context, organization string) (ElasticIP, error) {
	out, err := c.ec2.AllocateAddress(ctx, &ec2.AllocateAddressInput{Domain: ec2types.DomainTypeVpc, TagSpecifications: []ec2types.TagSpecification{{ResourceType: ec2types.ResourceTypeElasticIp, Tags: siliconTags(organization, "", "", "silicon-eip", nil)}}})
	if err != nil {
		return ElasticIP{}, safeError("allocate Elastic IP", err)
	}
	return ElasticIP{AllocationID: sdk.ToString(out.AllocationId), PublicIP: sdk.ToString(out.PublicIp), Ownership: OwnershipManaged, Unused: true}, nil
}
func (c *Client) AssociateElasticIP(ctx context.Context, allocationID, instanceID string) error {
	_, err := c.ec2.AssociateAddress(ctx, &ec2.AssociateAddressInput{AllocationId: sdk.String(allocationID), InstanceId: sdk.String(instanceID)})
	return safeError("associate Elastic IP", err)
}
func (c *Client) DisassociateElasticIP(ctx context.Context, associationID string) error {
	_, err := c.ec2.DisassociateAddress(ctx, &ec2.DisassociateAddressInput{AssociationId: sdk.String(associationID)})
	return safeError("disassociate Elastic IP", err)
}
func (c *Client) ReleaseElasticIP(ctx context.Context, allocationID string) error {
	_, err := c.ec2.ReleaseAddress(ctx, &ec2.ReleaseAddressInput{AllocationId: sdk.String(allocationID)})
	return safeError("release Elastic IP", err)
}
