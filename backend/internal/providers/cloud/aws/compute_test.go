package aws

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	sdk "github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
)

func TestEC2DiscoveryAndLifecycleUseNormalizedProvider(t *testing.T) {
	var actions []string
	server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		body, _ := io.ReadAll(request.Body)
		values := string(body)
		action := ""
		for _, part := range strings.Split(values, "&") {
			if strings.HasPrefix(part, "Action=") {
				action = strings.TrimPrefix(part, "Action=")
				break
			}
		}
		actions = append(actions, action)
		response.Header().Set("Content-Type", "text/xml")
		switch action {
		case "DescribeInstances":
			_, _ = io.WriteString(response, `<DescribeInstancesResponse xmlns="http://ec2.amazonaws.com/doc/2016-11-15/"><requestId>req-1</requestId><reservationSet><item><reservationId>r-1</reservationId><instancesSet><item><instanceId>i-123</instanceId><imageId>ami-123</imageId><instanceState><code>16</code><name>running</name></instanceState><privateIpAddress>10.0.1.5</privateIpAddress><ipAddress>203.0.113.5</ipAddress><instanceType>t3.small</instanceType><architecture>x86_64</architecture><subnetId>subnet-1</subnetId><vpcId>vpc-1</vpcId><placement><availabilityZone>eu-central-1a</availabilityZone></placement><groupSet><item><groupId>sg-1</groupId><groupName>web</groupName></item></groupSet><tagSet><item><key>Name</key><value>api-prod</value></item><item><key>silicon:managed</key><value>true</value></item></tagSet></item></instancesSet></item></reservationSet></DescribeInstancesResponse>`)
		case "DescribeInstanceTypes":
			_, _ = io.WriteString(response, `<DescribeInstanceTypesResponse xmlns="http://ec2.amazonaws.com/doc/2016-11-15/"><requestId>req-types</requestId><instanceTypeSet><item><instanceType>t3.small</instanceType><processorInfo><supportedArchitectures><item>x86_64</item></supportedArchitectures></processorInfo><vCpuInfo><defaultVCpus>2</defaultVCpus></vCpuInfo><memoryInfo><sizeInMiB>2048</sizeInMiB></memoryInfo></item></instanceTypeSet></DescribeInstanceTypesResponse>`)
		case "StartInstances", "StopInstances", "RebootInstances":
			_, _ = io.WriteString(response, `<`+action+`Response xmlns="http://ec2.amazonaws.com/doc/2016-11-15/"><requestId>req-2</requestId><instancesSet/></`+action+`Response>`)
		default:
			http.Error(response, "unexpected action", http.StatusBadRequest)
		}
	}))
	defer server.Close()

	config := sdk.Config{
		Region:      "eu-central-1",
		Credentials: sdk.NewCredentialsCache(credentials.NewStaticCredentialsProvider("test", "test", "")),
		HTTPClient:  server.Client(),
	}
	client := &Client{region: config.Region, ec2: ec2.NewFromConfig(config, func(options *ec2.Options) { options.BaseEndpoint = sdk.String(server.URL) })}

	instances, err := client.Instances(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(instances) != 1 || instances[0].ID != "i-123" || instances[0].Name != "api-prod" || instances[0].Ownership != OwnershipManaged || instances[0].PublicIP != "203.0.113.5" {
		t.Fatalf("unexpected normalized instances: %#v", instances)
	}
	typeInfo, err := client.InstanceType(context.Background(), "t3.small")
	if err != nil {
		t.Fatal(err)
	}
	if typeInfo.Name != "t3.small" || typeInfo.VCPUs != 2 || typeInfo.MemoryMiB != 2048 || len(typeInfo.Architectures) != 1 || typeInfo.Architectures[0] != "x86_64" {
		t.Fatalf("unexpected normalized instance type: %#v", typeInfo)
	}
	if err = client.StartInstance(context.Background(), "i-123"); err != nil {
		t.Fatal(err)
	}
	if err = client.StopInstance(context.Background(), "i-123"); err != nil {
		t.Fatal(err)
	}
	if err = client.RebootInstance(context.Background(), "i-123"); err != nil {
		t.Fatal(err)
	}
	want := []string{"DescribeInstances", "DescribeInstanceTypes", "StartInstances", "StopInstances", "RebootInstances"}
	if strings.Join(actions, ",") != strings.Join(want, ",") {
		t.Fatalf("actions=%v want=%v", actions, want)
	}
}

func TestDockerBootstrapMatchesDistribution(t *testing.T) {
	ubuntu := dockerBootstrap("ubuntu", "ubuntu")
	if !strings.Contains(ubuntu, "docker.io") || strings.Contains(ubuntu, "dnf") || !strings.Contains(ubuntu, "usermod, -aG, docker, ubuntu") {
		t.Fatalf("unexpected Ubuntu bootstrap: %s", ubuntu)
	}
	amazon := dockerBootstrap("amazon-linux", "ec2-user")
	if !strings.Contains(amazon, "dnf, install, -y, docker") || strings.Contains(amazon, "docker.io") {
		t.Fatalf("unexpected Amazon Linux bootstrap: %s", amazon)
	}
}
