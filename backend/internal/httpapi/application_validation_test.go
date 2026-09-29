package httpapi

import "testing"

func TestValidateApplicationInput(t *testing.T) {
	validName, dockerSource, gitSource := "api", "docker_image", "git_dockerfile"
	image, invalidImage, loopback, invalidHost := "example/api:1", "-bad", "127.0.0.1", "not-an-ip"
	internal, published := 3000, 8080
	zero, tooLarge := 0, 65536
	tests := []struct {
		name  string
		input applicationInputPayload
		want  string
	}{
		{name: "published port without host", input: applicationInputPayload{Name: validName, SourceType: gitSource, InternalPort: &internal, PublishedPort: &published}, want: "Host address is required when publishing a host port."},
		{name: "host without published port", input: applicationInputPayload{Name: validName, SourceType: gitSource, InternalPort: &internal, HostAddress: &loopback}, want: "Published host port is required when a host address is provided."},
		{name: "published port without internal port", input: applicationInputPayload{Name: validName, SourceType: gitSource, HostAddress: &loopback, PublishedPort: &published}, want: "Published port requires an internal container port."},
		{name: "invalid internal port", input: applicationInputPayload{Name: validName, SourceType: gitSource, InternalPort: &zero}, want: "Internal port must be between 1 and 65535."},
		{name: "invalid published port", input: applicationInputPayload{Name: validName, SourceType: gitSource, InternalPort: &internal, HostAddress: &loopback, PublishedPort: &tooLarge}, want: "Published port must be between 1 and 65535."},
		{name: "invalid host IP", input: applicationInputPayload{Name: validName, SourceType: gitSource, InternalPort: &internal, HostAddress: &invalidHost, PublishedPort: &published}, want: "Host address must be a valid IP address."},
		{name: "valid loopback binding", input: applicationInputPayload{Name: validName, SourceType: gitSource, InternalPort: &internal, HostAddress: &loopback, PublishedPort: &published}},
		{name: "docker image requires image", input: applicationInputPayload{Name: validName, SourceType: dockerSource}, want: "Docker image applications require an image reference."},
		{name: "docker image accepts image", input: applicationInputPayload{Name: validName, SourceType: dockerSource, Image: &image}},
		{name: "invalid image reference", input: applicationInputPayload{Name: validName, SourceType: dockerSource, Image: &invalidImage}, want: "Image reference is invalid."},
		{name: "git source does not require image", input: applicationInputPayload{Name: validName, SourceType: gitSource}},
		{name: "compose unsupported", input: applicationInputPayload{Name: validName, SourceType: "compose"}, want: "Docker Compose applications are not supported yet."},
		{name: "name required", input: applicationInputPayload{SourceType: gitSource}, want: "Application name is required."},
		{name: "source invalid", input: applicationInputPayload{Name: validName, SourceType: "unknown"}, want: "Source type must be docker_image or git_dockerfile."},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := validateApplicationInput(test.input); got != test.want {
				t.Fatalf("validateApplicationInput()=%q want %q", got, test.want)
			}
		})
	}
}
