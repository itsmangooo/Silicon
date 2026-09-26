package execution

import (
	"context"
	"errors"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/itsmangooo/Silicon/backend/internal/deployments"
	"github.com/itsmangooo/Silicon/backend/internal/jobs"
	gitprovider "github.com/itsmangooo/Silicon/backend/internal/providers/git"
	runtimeprovider "github.com/itsmangooo/Silicon/backend/internal/providers/runtime"
	secretprovider "github.com/itsmangooo/Silicon/backend/internal/providers/secrets"
	"github.com/itsmangooo/Silicon/backend/internal/sourcebuild"
	"github.com/itsmangooo/Silicon/backend/internal/store"
	"github.com/jackc/pgx/v5/pgxpool"
)

type DockerDeploymentExecutor struct {
	Pool         *pgxpool.Pool
	Git          gitprovider.Provider
	Runtime      runtimeprovider.Provider
	Secrets      secretprovider.Provider
	DockerBinary string
}

func (e DockerDeploymentExecutor) Execute(ctx context.Context, spec jobs.DeploymentSpec, progress func(deployments.State, string) error) error {
	if e.Runtime == nil {
		return errors.New("Docker runtime provider is not configured")
	}
	repository := store.Repository{Pool: e.Pool}
	application, err := repository.ApplicationByID(ctx, spec.OrganizationID, spec.ApplicationID)
	if err != nil {
		return fmt.Errorf("load application: %w", err)
	}
	image := strings.TrimSpace(spec.Image)
	pullImage := false
	targetServer := ""
	if application.ServerID != nil {
		targetServer = application.ServerID.String()
	}
	switch application.SourceType {
	case "docker_image":
		if image == "" && application.Image != nil {
			image = strings.TrimSpace(*application.Image)
		}
		if image == "" {
			return errors.New("Docker image application has no image configured")
		}
		pullImage = true
	case "git_dockerfile":
		image, err = e.buildGitHubRevision(ctx, spec, targetServer, progress)
		if err != nil {
			return err
		}
	case "compose":
		return errors.New("Docker Compose execution is intentionally unsupported")
	default:
		return fmt.Errorf("unsupported application source type %q", application.SourceType)
	}
	environment, err := repository.ListEnvironmentVariables(ctx, spec.OrganizationID, spec.ApplicationID)
	if err != nil {
		return fmt.Errorf("load environment variables: %w", err)
	}
	values := make(map[string]string, len(environment))
	for _, variable := range environment {
		values[variable.Name] = variable.Value
	}
	secrets, err := repository.ListSecretMetadata(ctx, spec.OrganizationID, spec.ApplicationID)
	if err != nil {
		return fmt.Errorf("load secret references: %w", err)
	}
	if len(secrets) > 0 && e.Secrets == nil {
		return errors.New("local encrypted secret provider is unavailable")
	}
	for _, secret := range secrets {
		plaintext, resolveErr := e.Secrets.Resolve(ctx, secretprovider.Reference{OrganizationID: spec.OrganizationID.String(), EnvironmentID: application.EnvironmentID.String(), ApplicationID: application.ID.String(), SecretID: secret.ID.String(), Name: secret.Name})
		if resolveErr != nil {
			return fmt.Errorf("resolve secret %q: %w", secret.Name, resolveErr)
		}
		values[secret.Name] = string(plaintext)
		for index := range plaintext {
			plaintext[index] = 0
		}
	}
	if err = progress(deployments.Deploying, "Deploying image through DockerRuntimeProvider"); err != nil {
		return err
	}
	if err = progress(deployments.Starting, "Creating and starting a Silicon-managed container"); err != nil {
		return err
	}
	runtimeSpec := runtimeprovider.DeploymentSpec{
		DeploymentID: spec.DeploymentID.String(), OrganizationID: spec.OrganizationID.String(), ApplicationID: application.ID.String(), Application: application.Name,
		Image: image, PullImage: pullImage, Environment: values,
	}
	if application.ServerID != nil {
		runtimeSpec.ServerID = application.ServerID.String()
	}
	if application.InternalPort != nil {
		runtimeSpec.InternalPort = *application.InternalPort
	}
	if application.HostAddress != nil {
		runtimeSpec.HostAddress = *application.HostAddress
	}
	if application.PublishedPort != nil {
		runtimeSpec.HostPort = *application.PublishedPort
	}
	previous, err := repository.PreviousRuntimeInstances(ctx, spec.OrganizationID, spec.ApplicationID, spec.DeploymentID)
	if err != nil {
		return fmt.Errorf("load previous runtime instances: %w", err)
	}
	stoppedForBinding := make([]store.RuntimeInstance, 0, len(previous))
	if runtimeSpec.HostPort > 0 {
		for _, old := range previous {
			if err = e.Runtime.Stop(ctx, old.ExternalID); err != nil {
				for _, stopped := range stoppedForBinding {
					_ = e.Runtime.Start(ctx, stopped.ExternalID)
				}
				return fmt.Errorf("stop previous container for fixed port replacement: %w", err)
			}
			stoppedForBinding = append(stoppedForBinding, old)
		}
	}
	status, deployErr := e.Runtime.Deploy(ctx, runtimeSpec)
	for name := range values {
		values[name] = ""
		delete(values, name)
	}
	if status.InstanceID != "" {
		if status.Image == "" {
			status.Image = image
		}
		if _, err = repository.SaveRuntimeInstance(ctx, spec.OrganizationID, spec.ApplicationID, spec.DeploymentID, status, runtimeSpec.InternalPort); err != nil {
			return fmt.Errorf("persist runtime instance: %w", err)
		}
	}
	if deployErr != nil {
		cleanupCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		if status.InstanceID != "" {
			_ = e.Runtime.Remove(cleanupCtx, status.InstanceID)
			if current, findErr := repository.CurrentRuntimeInstance(cleanupCtx, spec.OrganizationID, spec.ApplicationID); findErr == nil && current.DeploymentID == spec.DeploymentID {
				_ = repository.MarkRuntimeRemoved(cleanupCtx, spec.OrganizationID, current.ID)
			}
		}
		for _, stopped := range stoppedForBinding {
			_ = e.Runtime.Start(cleanupCtx, stopped.ExternalID)
		}
		return fmt.Errorf("Docker runtime deploy failed: %w", deployErr)
	}
	if !status.Healthy {
		return fmt.Errorf("Docker reported state %q and health %q", status.State, status.Health)
	}
	if _, err = e.Pool.Exec(ctx, `UPDATE deployments SET image=$2,updated_at=now() WHERE id=$1`, spec.DeploymentID, image); err != nil {
		return fmt.Errorf("store deployed image: %w", err)
	}
	removed, cleanupErrors := 0, 0
	for _, old := range previous {
		if err = e.Runtime.Remove(ctx, old.ExternalID); err != nil {
			cleanupErrors++
			continue
		}
		if err = repository.MarkRuntimeRemoved(ctx, spec.OrganizationID, old.ID); err != nil {
			cleanupErrors++
			continue
		}
		removed++
	}
	message := fmt.Sprintf("Docker container is %s (%s)", status.Health, shortID(status.InstanceID))
	if removed > 0 {
		message += fmt.Sprintf("; removed %d previous managed instance(s)", removed)
	}
	if cleanupErrors > 0 {
		message += fmt.Sprintf("; %d previous instance(s) require cleanup", cleanupErrors)
	}
	return progress(deployments.Healthy, message)
}

func (e DockerDeploymentExecutor) buildGitHubRevision(ctx context.Context, spec jobs.DeploymentSpec, targetServer string, progress func(deployments.State, string) error) (string, error) {
	if e.Git == nil {
		return "", errors.New("GitHub provider is not configured")
	}
	var installationID int64
	var repository string
	err := e.Pool.QueryRow(ctx, `SELECT g.installation_id,s.repository_full_name FROM application_git_sources s JOIN github_integrations g ON g.id=s.integration_id AND g.organization_id=s.organization_id WHERE s.application_id=$1 AND s.organization_id=$2 AND g.status='connected'`, spec.ApplicationID, spec.OrganizationID).Scan(&installationID, &repository)
	if err != nil {
		return "", fmt.Errorf("load GitHub source: %w", err)
	}
	if repository != spec.Repository {
		return "", errors.New("deployment repository no longer matches the application source")
	}
	if len(spec.CommitSHA) != 40 {
		return "", errors.New("an exact 40-character Git commit SHA is required")
	}
	archive, err := e.Git.Archive(ctx, installationID, repository, spec.CommitSHA)
	if err != nil {
		return "", fmt.Errorf("fetch exact GitHub revision: %w", err)
	}
	defer archive.Close()
	if err = progress(deployments.Building, "Building exact GitHub revision "+spec.CommitSHA); err != nil {
		return "", err
	}
	image := "silicon/" + spec.ApplicationID.String() + ":" + shortSHA(spec.CommitSHA)
	if targetServer != "" {
		builder, ok := e.Runtime.(runtimeprovider.ImageBuilder)
		if !ok {
			return "", errors.New("selected remote runtime does not support exact-revision builds")
		}
		return builder.Build(ctx, runtimeprovider.BuildSpec{DeploymentID: spec.DeploymentID.String(), OrganizationID: spec.OrganizationID.String(), ApplicationID: spec.ApplicationID.String(), ServerID: targetServer, CommitSHA: spec.CommitSHA, Image: image}, archive)
	}
	if err = BuildDockerArchive(ctx, e.DockerBinary, image, spec.OrganizationID.String(), spec.ApplicationID.String(), spec.CommitSHA, archive); err != nil {
		return "", err
	}
	return image, nil
}

func BuildDockerArchive(ctx context.Context, binary, image, organizationID, applicationID, commitSHA string, archive io.Reader) error {
	return sourcebuild.BuildDockerArchive(ctx, binary, image, organizationID, applicationID, commitSHA, archive)
}

func extractGitHubArchive(source io.Reader, destination string) error {
	return sourcebuild.ExtractArchive(source, destination)
}
func shortSHA(value string) string {
	if len(value) > 12 {
		return value[:12]
	}
	return value
}
func shortID(value string) string {
	if len(value) > 12 {
		return value[:12]
	}
	return value
}
