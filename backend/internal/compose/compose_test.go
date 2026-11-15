package compose

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"
)

func TestValidatePreviewsServicesInDependencyOrder(t *testing.T) {
	preview, err := Validate(`services:
  app:
    image: nginx:alpine
    environment:
      WEB_LABEL: demo
    depends_on: [cache]
    networks: [private]
    ports:
      - target: 80
        published: "8080"
        host_ip: 127.0.0.1
        protocol: tcp
  cache:
    image: redis:alpine
    command: ["redis-server", "--appendonly", "yes"]
    healthcheck:
      test: ["CMD", "redis-cli", "ping"]
      interval: 10s
    volumes:
      - type: volume
        source: cache
        target: /data
    networks: [private]
networks:
  private:
    internal: true
volumes:
  cache: {}
`)
	if err != nil {
		t.Fatal(err)
	}
	expected := []ServicePreview{
		{Name: "cache", HasHealthcheck: true},
		{Name: "app", DependsOn: []string{"cache"}},
	}
	if !reflect.DeepEqual(preview.Services, expected) {
		t.Fatalf("preview = %#v", preview)
	}
	raw, err := json.Marshal(preview)
	if err != nil || strings.Contains(string(raw), "demo") {
		t.Fatalf("preview disclosed an environment value or failed to encode: %v", err)
	}
}

func TestValidateRejectsUnsafeOrInvalidManifests(t *testing.T) {
	service := "services:\n  app:\n    image: nginx:alpine\n"
	ports := "    ports:\n      - target: 80\n        published: '8080'\n"
	cases := map[string]string{
		"multiple documents": service + "---\nservices: {}\n",
		"oversized":          service + strings.Repeat(" ", MaxBytes),
		"build":              service + "    build: .\n",
		"include":            service + "include: [../outside.yaml]\n",
		"env file":           service + "    env_file: /etc/environment\n",
		"privileged":         service + "    privileged: true\n",
		"device":             service + "    devices: [/dev/sda]\n",
		"host network":       service + "    network_mode: host\n",
		"bind mount":         service + "    volumes: [{type: bind, source: /etc, target: /staging}]\n",
		"external volume":    service + "volumes: {existing: {external: true}}\n",
		"volume traversal":   service + "    volumes: [{type: volume, source: data, target: /tmp/../data}]\nvolumes: {data: {}}\n",
		"missing volume":     service + "    volumes: [{type: volume, source: data, target: /data}]\n",
		"implicit host IP":   service + ports,
		"all interfaces":     service + ports + "        host_ip: 0.0.0.0\n",
		"invalid port":       service + "    ports: [{target: 70000, published: '8080', host_ip: 127.0.0.1}]\n",
		"missing dependency": service + "    depends_on: [db]\n",
		"dependency cycle":   service + "    depends_on: [cache]\n  cache: {image: redis, depends_on: [app]}\n",
		"duplicate key":      service + "    image: alpine\n",
		"host interpolation": service + "    environment: {WEB_LABEL: '${HOME}'}\n",
		"inherited env":      service + "    environment: {WEB_LABEL: null}\n",
		"YAML alias":         "services:\n  app: &service {image: nginx:alpine}\n  app2: *service\n",
	}
	for name, input := range cases {
		t.Run(name, func(t *testing.T) {
			if _, err := Validate(input); err == nil {
				t.Fatal("validation accepted unsupported or invalid YAML")
			}
		})
	}
}
