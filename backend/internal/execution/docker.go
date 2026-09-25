package execution

import (
	"archive/tar"
	"compress/gzip"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/itsmangooo/Silicon/backend/internal/deployments"
	"github.com/itsmangooo/Silicon/backend/internal/jobs"
	gitprovider "github.com/itsmangooo/Silicon/backend/internal/providers/git"
	runtimeprovider "github.com/itsmangooo/Silicon/backend/internal/providers/runtime"
	secretprovider "github.com/itsmangooo/Silicon/backend/internal/providers/secrets"
	"github.com/itsmangooo/Silicon/backend/internal/store"
	"github.com/jackc/pgx/v5/pgxpool"
)

const maxArchiveBytes int64 = 1 << 30

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
		image, err = e.buildGitHubRevision(ctx, spec, progress)
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

func (e DockerDeploymentExecutor) buildGitHubRevision(ctx context.Context, spec jobs.DeploymentSpec, progress func(deployments.State, string) error) (string, error) {
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
	workdir, err := os.MkdirTemp("", "silicon-build-*")
	if err != nil {
		return "", err
	}
	defer os.RemoveAll(workdir)
	if err = extractGitHubArchive(archive, workdir); err != nil {
		return "", fmt.Errorf("extract source archive: %w", err)
	}
	if err = progress(deployments.Building, "Building exact GitHub revision "+spec.CommitSHA); err != nil {
		return "", err
	}
	image := "silicon/" + spec.ApplicationID.String() + ":" + shortSHA(spec.CommitSHA)
	binary := e.DockerBinary
	if binary == "" {
		binary = "docker"
	}
	if _, err = runOutput(ctx, binary, "build", "--pull", "--label", "silicon.managed=true", "--label", "silicon.application_id="+spec.ApplicationID.String(), "--label", "silicon.commit_sha="+spec.CommitSHA, "-t", image, workdir); err != nil {
		return "", fmt.Errorf("Docker build failed: %w", err)
	}
	return image, nil
}

func extractGitHubArchive(source io.Reader, destination string) error {
	limited := io.LimitReader(source, maxArchiveBytes+1)
	gz, err := gzip.NewReader(limited)
	if err != nil {
		return err
	}
	defer gz.Close()
	reader := tar.NewReader(gz)
	var total int64
	for {
		header, err := reader.Next()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return err
		}
		parts := strings.Split(filepath.ToSlash(header.Name), "/")
		if len(parts) < 2 {
			continue
		}
		relative := filepath.Clean(filepath.FromSlash(strings.Join(parts[1:], "/")))
		if relative == "." {
			continue
		}
		if filepath.IsAbs(relative) || relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
			return errors.New("archive contains an unsafe path")
		}
		target := filepath.Join(destination, relative)
		if !strings.HasPrefix(target, destination+string(filepath.Separator)) {
			return errors.New("archive path escapes build directory")
		}
		switch header.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(target, 0750); err != nil {
				return err
			}
		case tar.TypeReg:
			total += header.Size
			if total > maxArchiveBytes {
				return errors.New("source archive exceeds one GiB")
			}
			if err := os.MkdirAll(filepath.Dir(target), 0750); err != nil {
				return err
			}
			mode := os.FileMode(header.Mode) & 0777
			if mode&0111 == 0 {
				mode = 0640
			}
			file, err := os.OpenFile(target, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, mode)
			if err != nil {
				return err
			}
			_, copyErr := io.CopyN(file, reader, header.Size)
			closeErr := file.Close()
			if copyErr != nil {
				return copyErr
			}
			if closeErr != nil {
				return closeErr
			}
		default:
			return fmt.Errorf("archive entry type %d is not allowed", header.Typeflag)
		}
	}
	return nil
}

func runOutput(ctx context.Context, binary string, args ...string) (string, error) {
	command := exec.CommandContext(ctx, binary, args...)
	stdout, stderr := &boundedBuffer{limit: 1 << 20}, &boundedBuffer{limit: 1 << 20}
	command.Stdout, command.Stderr = stdout, stderr
	err := command.Run()
	if err != nil {
		message := strings.TrimSpace(stderr.String())
		if message == "" {
			message = err.Error()
		}
		message = strings.ReplaceAll(strings.ReplaceAll(message, "\r", " "), "\n", " ")
		if len(message) > 500 {
			message = message[:500]
		}
		return "", errors.New(message)
	}
	return stdout.String(), nil
}

type boundedBuffer struct {
	data     []byte
	limit    int
	exceeded bool
}

func (b *boundedBuffer) Write(value []byte) (int, error) {
	remaining := b.limit - len(b.data)
	if remaining > 0 {
		if remaining > len(value) {
			remaining = len(value)
		}
		b.data = append(b.data, value[:remaining]...)
	}
	if remaining < len(value) {
		b.exceeded = true
	}
	return len(value), nil
}

func (b *boundedBuffer) String() string { return string(b.data) }
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
