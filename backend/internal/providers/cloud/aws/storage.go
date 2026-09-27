package aws

import (
	"context"
	"errors"

	sdk "github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"
)

func (c *Client) Volumes(ctx context.Context) ([]Volume, error) {
	out, err := c.ec2.DescribeVolumes(ctx, &ec2.DescribeVolumesInput{})
	if err != nil {
		return nil, safeError("list EBS volumes", err)
	}
	items := make([]Volume, 0, len(out.Volumes))
	for _, item := range out.Volumes {
		tags := tagMap(item.Tags)
		volume := Volume{ID: sdk.ToString(item.VolumeId), AvailabilityZone: sdk.ToString(item.AvailabilityZone), SizeGiB: sdk.ToInt32(item.Size), Type: string(item.VolumeType), State: string(item.State), Encrypted: sdk.ToBool(item.Encrypted), Ownership: ownership(tags)}
		if len(item.Attachments) > 0 {
			volume.InstanceID = sdk.ToString(item.Attachments[0].InstanceId)
			volume.Device = sdk.ToString(item.Attachments[0].Device)
		}
		items = append(items, volume)
	}
	return items, nil
}
func (c *Client) CreateVolume(ctx context.Context, input CreateVolumeInput) (Volume, error) {
	if input.Name == "" || input.AvailabilityZone == "" || input.SizeGiB < 1 {
		return Volume{}, errors.New("volume name, availability zone, and positive size are required")
	}
	if input.Type == "" {
		input.Type = "gp3"
	}
	out, err := c.ec2.CreateVolume(ctx, &ec2.CreateVolumeInput{AvailabilityZone: sdk.String(input.AvailabilityZone), Size: sdk.Int32(input.SizeGiB), VolumeType: ec2types.VolumeType(input.Type), Encrypted: sdk.Bool(input.Encrypted), TagSpecifications: []ec2types.TagSpecification{{ResourceType: ec2types.ResourceTypeVolume, Tags: siliconTags(input.OrganizationID, "", "", input.Name, nil)}}})
	if err != nil {
		return Volume{}, safeError("create EBS volume", err)
	}
	return Volume{ID: sdk.ToString(out.VolumeId), AvailabilityZone: sdk.ToString(out.AvailabilityZone), SizeGiB: sdk.ToInt32(out.Size), Type: string(out.VolumeType), State: string(out.State), Encrypted: sdk.ToBool(out.Encrypted), Ownership: OwnershipManaged}, nil
}
func (c *Client) AttachVolume(ctx context.Context, volumeID, instanceID, device string) error {
	_, err := c.ec2.AttachVolume(ctx, &ec2.AttachVolumeInput{VolumeId: sdk.String(volumeID), InstanceId: sdk.String(instanceID), Device: sdk.String(device)})
	return safeError("attach EBS volume", err)
}
func (c *Client) DetachVolume(ctx context.Context, volumeID, instanceID string) error {
	_, err := c.ec2.DetachVolume(ctx, &ec2.DetachVolumeInput{VolumeId: sdk.String(volumeID), InstanceId: sdk.String(instanceID)})
	return safeError("detach EBS volume", err)
}
func (c *Client) DeleteVolume(ctx context.Context, volumeID string) error {
	_, err := c.ec2.DeleteVolume(ctx, &ec2.DeleteVolumeInput{VolumeId: sdk.String(volumeID)})
	return safeError("delete EBS volume", err)
}

func (c *Client) Snapshots(ctx context.Context) ([]Snapshot, error) {
	out, err := c.ec2.DescribeSnapshots(ctx, &ec2.DescribeSnapshotsInput{OwnerIds: []string{"self"}})
	if err != nil {
		return nil, safeError("list EBS snapshots", err)
	}
	items := make([]Snapshot, 0, len(out.Snapshots))
	for _, item := range out.Snapshots {
		tags := tagMap(item.Tags)
		items = append(items, Snapshot{ID: sdk.ToString(item.SnapshotId), VolumeID: sdk.ToString(item.VolumeId), SizeGiB: sdk.ToInt32(item.VolumeSize), State: string(item.State), Encrypted: sdk.ToBool(item.Encrypted), Description: sdk.ToString(item.Description), StartedAt: sdk.ToTime(item.StartTime), Ownership: ownership(tags)})
	}
	return items, nil
}
func (c *Client) CreateSnapshot(ctx context.Context, volumeID, description, organization string) (Snapshot, error) {
	out, err := c.ec2.CreateSnapshot(ctx, &ec2.CreateSnapshotInput{VolumeId: sdk.String(volumeID), Description: sdk.String(description), TagSpecifications: []ec2types.TagSpecification{{ResourceType: ec2types.ResourceTypeSnapshot, Tags: siliconTags(organization, "", "", "snapshot-"+volumeID, nil)}}})
	if err != nil {
		return Snapshot{}, safeError("create EBS snapshot", err)
	}
	return Snapshot{ID: sdk.ToString(out.SnapshotId), VolumeID: sdk.ToString(out.VolumeId), SizeGiB: sdk.ToInt32(out.VolumeSize), State: string(out.State), Encrypted: sdk.ToBool(out.Encrypted), Description: sdk.ToString(out.Description), StartedAt: sdk.ToTime(out.StartTime), Ownership: OwnershipManaged}, nil
}
func (c *Client) DeleteSnapshot(ctx context.Context, id string) error {
	_, err := c.ec2.DeleteSnapshot(ctx, &ec2.DeleteSnapshotInput{SnapshotId: sdk.String(id)})
	return safeError("delete EBS snapshot", err)
}
