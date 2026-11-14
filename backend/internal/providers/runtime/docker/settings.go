package docker

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"

	runtimeprovider "github.com/itsmangooo/Silicon/backend/internal/providers/runtime"
)

const environmentLabel = "silicon.environment_id"

func (p Provider) runtimeOptions(ctx context.Context, spec runtimeprovider.DeploymentSpec) ([]string, error) {
	settings := spec.Settings
	if err := settings.Validate(); err != nil {
		return nil, err
	}
	var args []string
	if settings.MemoryMiB > 0 {
		args = append(args, "--memory", strconv.Itoa(settings.MemoryMiB)+"m")
	}
	if len(settings.Volumes) == 0 && !settings.EnvironmentNetwork {
		return args, nil
	}
	org, err := runtimeprovider.ResourceID(spec.OrganizationID)
	if err != nil {
		return nil, err
	}
	if settings.EnvironmentNetwork {
		environment, err := runtimeprovider.ResourceID(spec.EnvironmentID)
		if err != nil {
			return nil, err
		}
		network := "silicon-env-" + org + "-" + environment
		if err := p.ensureEnvironmentNetwork(ctx, network, org, environment); err != nil {
			return nil, err
		}
		// A UUID-based alias remains stable across deployment replacements.
		application, err := runtimeprovider.ResourceID(spec.ApplicationID)
		if err != nil {
			return nil, err
		}
		args = append(args, "--network", network, "--network-alias", "app-"+application)
	}
	if len(settings.Volumes) == 0 {
		return args, nil
	}
	application, err := runtimeprovider.ResourceID(spec.ApplicationID)
	if err != nil {
		return nil, err
	}
	for _, volume := range settings.Volumes {
		name := "silicon-vol-" + org + "-" + application + "-" + volume.Name
		if err := p.ensureVolume(ctx, name, org, application); err != nil {
			return nil, err
		}
		mount := "type=volume,source=" + name + ",target=" + volume.Target
		if volume.ReadOnly {
			mount += ",readonly"
		}
		args = append(args, "--mount", mount)
	}
	return args, nil
}

func (p Provider) ensureVolume(ctx context.Context, name, org, app string) error {
	if err := p.run(ctx, "volume", "create", "--driver", "local",
		"--label", managedLabel+"=true", "--label", organizationLabel+"="+org,
		"--label", applicationLabel+"="+app, name); err != nil {
		return fmt.Errorf("create application volume: %w", err)
	}
	const format = `{"Driver":{{json .Driver}},"Labels":{{json .Labels}},"Options":{{json .Options}}}`
	out, err := p.output(ctx, "volume", "inspect", "--format", format, "--", name)
	if err != nil {
		return fmt.Errorf("inspect application volume: %w", err)
	}
	var volume struct {
		Driver  string
		Labels  map[string]string
		Options map[string]string
	}
	if err := json.Unmarshal([]byte(out), &volume); err != nil {
		return errors.New("invalid Docker volume inspection")
	}
	if volume.Driver != "local" || len(volume.Options) != 0 || volume.Labels[managedLabel] != "true" ||
		volume.Labels[organizationLabel] != org || volume.Labels[applicationLabel] != app {
		return errors.New("refusing to mount a Docker volume without matching Silicon ownership and local storage")
	}
	return nil
}

func (p Provider) ensureEnvironmentNetwork(ctx context.Context, name, org, environment string) error {
	const format = `{"Name":{{json .Name}},"Driver":{{json .Driver}},"Scope":{{json .Scope}},"Labels":{{json .Labels}}}`
	out, err := p.output(ctx, "network", "inspect", "--format", format, "--", name)
	if err != nil {
		createErr := p.run(ctx, "network", "create", "--driver", "bridge",
			"--label", managedLabel+"=true", "--label", organizationLabel+"="+org,
			"--label", environmentLabel+"="+environment, name)
		// A concurrent deployment can create the same network first.
		out, err = p.output(ctx, "network", "inspect", "--format", format, "--", name)
		if err != nil {
			return fmt.Errorf("ensure environment network: %w", errors.Join(createErr, err))
		}
	}
	var network struct {
		Name, Driver, Scope string
		Labels              map[string]string
	}
	if err := json.Unmarshal([]byte(out), &network); err != nil {
		return errors.New("invalid Docker network inspection")
	}
	if network.Name != name || network.Driver != "bridge" || network.Scope != "local" ||
		network.Labels[managedLabel] != "true" || network.Labels[organizationLabel] != org ||
		network.Labels[environmentLabel] != environment {
		return errors.New("refusing to join a Docker network without matching Silicon environment ownership")
	}
	return nil
}
