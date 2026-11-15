// Package compose validates the image-based Compose subset Silicon can plan.
package compose

import (
	"errors"
	"fmt"
	"io"
	"net/netip"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/itsmangooo/Silicon/backend/internal/providers/runtime"
	"gopkg.in/yaml.v3"
)

const MaxBytes = 64 << 10

type manifest struct {
	Services map[string]service  `yaml:"services"`
	Volumes  map[string]struct{} `yaml:"volumes"`
	Networks map[string]network  `yaml:"networks"`
}

type network struct {
	Internal bool `yaml:"internal"`
}

type service struct {
	Image       string            `yaml:"image"`
	Command     []string          `yaml:"command"`
	Environment map[string]string `yaml:"environment"`
	Volumes     []mount           `yaml:"volumes"`
	Networks    []string          `yaml:"networks"`
	DependsOn   []string          `yaml:"depends_on"`
	Ports       []port            `yaml:"ports"`
	Healthcheck *healthcheck      `yaml:"healthcheck"`
}

type mount struct {
	Type     string `yaml:"type"`
	Source   string `yaml:"source"`
	Target   string `yaml:"target"`
	ReadOnly bool   `yaml:"read_only"`
}

type port struct {
	Target    int    `yaml:"target"`
	Published string `yaml:"published"`
	HostIP    string `yaml:"host_ip"`
	Protocol  string `yaml:"protocol"`
}

type healthcheck struct {
	Test        []string `yaml:"test"`
	Interval    string   `yaml:"interval"`
	Timeout     string   `yaml:"timeout"`
	StartPeriod string   `yaml:"start_period"`
	Retries     int      `yaml:"retries"`
}

type Preview struct {
	Services []ServicePreview `json:"services"`
}

type ServicePreview struct {
	Name           string   `json:"name"`
	DependsOn      []string `json:"dependsOn,omitempty"`
	HasHealthcheck bool     `json:"hasHealthcheck"`
}

var (
	resourceName = regexp.MustCompile(`^[a-z0-9][a-z0-9-]{0,31}$`)
	variableName = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`)
)

// Validate reads a single, self-contained manifest, without host interpolation
// or filesystem access. Its result contains service metadata only.
func Validate(text string) (Preview, error) {
	if len(text) == 0 || len(text) > MaxBytes {
		return Preview{}, errors.New("provide a Compose document of up to 64 KiB")
	}
	decoder := yaml.NewDecoder(strings.NewReader(text))
	var tree yaml.Node
	if err := decoder.Decode(&tree); err != nil {
		return Preview{}, errors.New("provide valid Compose YAML")
	}
	var extra yaml.Node
	if decoder.Decode(&extra) != io.EOF {
		return Preview{}, errors.New("provide exactly one Compose YAML document")
	}
	if err := validateNode(&tree, 0); err != nil {
		return Preview{}, err
	}
	var config manifest
	decoder = yaml.NewDecoder(strings.NewReader(text))
	decoder.KnownFields(true)
	if err := decoder.Decode(&config); err != nil {
		return Preview{}, errors.New("the Compose document contains an unknown field, duplicate key, or value with an unsupported type")
	}
	return config.preview()
}

func validateNode(node *yaml.Node, depth int) error {
	if depth > 16 {
		return errors.New("Compose YAML cannot be nested more than 16 levels deep")
	}
	if node.Anchor != "" || node.Kind == yaml.AliasNode || node.Tag == "!!merge" {
		return fmt.Errorf("line %d: YAML anchors, aliases, and merge keys are not supported", node.Line)
	}
	if node.Kind == yaml.ScalarNode {
		switch node.ShortTag() {
		case "!!str", "!!int", "!!float", "!!bool":
		default:
			return fmt.Errorf("line %d: use an explicit string, number, or boolean", node.Line)
		}
		if strings.ContainsRune(node.Value, 0) {
			return fmt.Errorf("line %d: NUL bytes are not supported", node.Line)
		}
		if strings.ContainsRune(strings.ReplaceAll(node.Value, "$$", ""), '$') {
			return fmt.Errorf("line %d: host variables are not expanded; use $$ for a literal dollar sign", node.Line)
		}
	}
	for _, child := range node.Content {
		if err := validateNode(child, depth+1); err != nil {
			return err
		}
	}
	return nil
}

func (c manifest) preview() (Preview, error) {
	if len(c.Services) == 0 || len(c.Services) > 16 || len(c.Volumes) > 16 || len(c.Networks) > 16 {
		return Preview{}, errors.New("use 1–16 services and at most 16 volumes and networks")
	}
	for name := range c.Volumes {
		if !resourceName.MatchString(name) {
			return Preview{}, errors.New("volume names must use 1–32 lowercase letters, digits, or hyphens and start with a letter or digit")
		}
	}
	for name := range c.Networks {
		if !resourceName.MatchString(name) {
			return Preview{}, errors.New("network names must use 1–32 lowercase letters, digits, or hyphens and start with a letter or digit")
		}
	}
	names := make([]string, 0, len(c.Services))
	for name := range c.Services {
		if !resourceName.MatchString(name) {
			return Preview{}, errors.New("service names must use 1–32 lowercase letters, digits, or hyphens and start with a letter or digit")
		}
		names = append(names, name)
	}
	sort.Strings(names)
	ports := make(map[netip.AddrPort]bool)
	for _, name := range names {
		if err := c.validateService(c.Services[name], ports); err != nil {
			return Preview{}, fmt.Errorf("service %s: %w", name, err)
		}
	}
	result := Preview{Services: make([]ServicePreview, 0, len(names))}
	visiting, added := map[string]bool{}, map[string]bool{}
	var visit func(string) error
	visit = func(name string) error {
		if visiting[name] {
			return fmt.Errorf("service %s participates in a dependency cycle", name)
		}
		if added[name] {
			return nil
		}
		s, ok := c.Services[name]
		if !ok {
			return errors.New("depends_on references an undeclared service")
		}
		visiting[name] = true
		dependencies := make(map[string]bool)
		for _, dependency := range s.DependsOn {
			if dependencies[dependency] {
				return fmt.Errorf("service %s lists a dependency more than once", name)
			}
			dependencies[dependency] = true
			if err := visit(dependency); err != nil {
				return err
			}
		}
		delete(visiting, name)
		added[name] = true
		result.Services = append(result.Services, ServicePreview{Name: name, DependsOn: s.DependsOn, HasHealthcheck: s.Healthcheck != nil})
		return nil
	}
	for _, name := range names {
		if err := visit(name); err != nil {
			return Preview{}, err
		}
	}
	return result, nil
}

func (c manifest) validateService(s service, ports map[netip.AddrPort]bool) error {
	if s.Image == "" || len(s.Image) > 500 || strings.HasPrefix(s.Image, "-") || strings.ContainsAny(s.Image, " \t\r\n$") {
		return errors.New("a Docker image reference is required")
	}
	if len(s.Environment) > 200 || len(s.Ports) > 16 || len(s.DependsOn) > 16 || len(s.Networks) > 16 {
		return errors.New("use at most 200 environment variables and 16 ports, dependencies, and networks")
	}
	for key, value := range s.Environment {
		if !variableName.MatchString(key) || len(value) > 32768 || strings.ContainsAny(value, "\r\n") {
			return errors.New("environment variable names must be valid and values single-line and no more than 32 KiB")
		}
	}
	settings := runtime.Settings{Arguments: s.Command}
	for _, volume := range s.Volumes {
		if volume.Type != "volume" {
			return errors.New("mounts must use type volume")
		}
		if _, ok := c.Volumes[volume.Source]; !ok {
			return errors.New("mounts must reference a declared volume")
		}
		settings.Volumes = append(settings.Volumes, runtime.VolumeMount{Name: volume.Source, Target: volume.Target, ReadOnly: volume.ReadOnly})
	}
	if err := settings.Validate(); err != nil {
		return err
	}
	seenNetworks := make(map[string]bool)
	for _, network := range s.Networks {
		_, declared := c.Networks[network]
		if !declared || seenNetworks[network] {
			return errors.New("network attachments must reference unique declared networks")
		}
		seenNetworks[network] = true
	}
	for _, p := range s.Ports {
		if p.Protocol != "" && p.Protocol != "tcp" {
			return errors.New("published ports must use TCP")
		}
		ip, err := netip.ParseAddr(p.HostIP)
		if err != nil || ip.Zone() != "" || ip.Is4In6() || (!ip.IsLoopback() && !ip.IsPrivate()) {
			return errors.New("ports must bind an explicit loopback or private host IP")
		}
		published, err := strconv.ParseUint(p.Published, 10, 16)
		if err != nil || published == 0 || p.Target < 1 || p.Target > 65535 {
			return errors.New("published and target ports must be between 1 and 65535")
		}
		binding := netip.AddrPortFrom(ip, uint16(published))
		if ports[binding] {
			return errors.New("duplicate host port binding")
		}
		ports[binding] = true
	}
	if h := s.Healthcheck; h != nil {
		if len(h.Test) < 2 || (h.Test[0] != "CMD" && h.Test[0] != "CMD-SHELL") ||
			(h.Test[0] == "CMD-SHELL" && len(h.Test) != 2) || h.Test[1] == "" || h.Retries < 0 || h.Retries > 100 {
			return errors.New("healthcheck requires CMD or CMD-SHELL and at most 100 retries")
		}
		if err := (runtime.Settings{Arguments: h.Test}).Validate(); err != nil {
			return fmt.Errorf("healthcheck: %w", err)
		}
		for _, value := range []string{h.Interval, h.Timeout, h.StartPeriod} {
			if value == "" {
				continue
			}
			if duration, err := time.ParseDuration(value); err != nil || duration <= 0 || duration > time.Hour {
				return errors.New("healthcheck durations must be greater than zero and at most one hour")
			}
		}
	}
	return nil
}
