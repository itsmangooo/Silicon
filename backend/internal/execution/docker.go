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
	"github.com/jackc/pgx/v5/pgxpool"
)

const maxArchiveBytes int64 = 1 << 30

type LocalDockerExecutor struct {
	Pool         *pgxpool.Pool
	Git          gitprovider.Provider
	DockerBinary string
}

func (e LocalDockerExecutor) Execute(ctx context.Context, spec jobs.DeploymentSpec, progress func(deployments.State, string) error) error {
	var installationID int64
	var repository, sourceType string
	err := e.Pool.QueryRow(ctx, `SELECT g.installation_id,s.repository_full_name,a.source_type FROM application_git_sources s JOIN github_integrations g ON g.id=s.integration_id AND g.organization_id=s.organization_id JOIN applications a ON a.id=s.application_id AND a.organization_id=s.organization_id WHERE s.application_id=$1 AND s.organization_id=$2 AND g.status='connected'`, spec.ApplicationID, spec.OrganizationID).Scan(&installationID, &repository, &sourceType)
	if err != nil {
		return fmt.Errorf("load deployment source: %w", err)
	}
	if sourceType != "git_dockerfile" {
		return fmt.Errorf("source type %q is not supported by the local Docker executor", sourceType)
	}
	if repository != spec.Repository {
		return errors.New("deployment repository no longer matches the application source")
	}
	archive, err := e.Git.Archive(ctx, installationID, repository, spec.CommitSHA)
	if err != nil {
		return fmt.Errorf("fetch exact GitHub revision: %w", err)
	}
	defer archive.Close()
	workdir, err := os.MkdirTemp("", "silicon-build-*")
	if err != nil {
		return err
	}
	defer os.RemoveAll(workdir)
	if err = extractGitHubArchive(archive, workdir); err != nil {
		return fmt.Errorf("extract source archive: %w", err)
	}
	if err = progress(deployments.Building, "Building exact GitHub revision "+spec.CommitSHA); err != nil {
		return err
	}
	image := "silicon/" + spec.ApplicationID.String() + ":" + shortSHA(spec.CommitSHA)
	binary := e.DockerBinary
	if binary == "" {
		binary = "docker"
	}
	if err = run(ctx, binary, "build", "--pull", "--label", "silicon.managed=true", "--label", "silicon.application_id="+spec.ApplicationID.String(), "--label", "silicon.commit_sha="+spec.CommitSHA, "-t", image, workdir); err != nil {
		return fmt.Errorf("docker build failed: %w", err)
	}
	if err = progress(deployments.Deploying, "Starting image built from exact revision "+spec.CommitSHA); err != nil {
		return err
	}
	name := "silicon-" + compact(spec.ApplicationID.String(), 12) + "-" + compact(spec.DeploymentID.String(), 12)
	output, err := runOutput(ctx, binary, "run", "--detach", "--restart", "unless-stopped", "--name", name, "--label", "silicon.managed=true", "--label", "silicon.application_id="+spec.ApplicationID.String(), "--label", "silicon.deployment_id="+spec.DeploymentID.String(), "--label", "silicon.commit_sha="+spec.CommitSHA, image)
	if err != nil {
		return fmt.Errorf("docker run failed: %w", err)
	}
	containerID := strings.TrimSpace(output)
	if containerID == "" {
		return errors.New("docker returned an empty container ID")
	}
	if err = progress(deployments.Starting, "Verifying the new container is running"); err != nil {
		return err
	}
	if err = waitContainerReady(ctx, binary, containerID); err != nil {
		_ = run(ctx, binary, "rm", "--force", containerID)
		return err
	}
	old, err := runOutput(ctx, binary, "ps", "--all", "--quiet", "--filter", "label=silicon.managed=true", "--filter", "label=silicon.application_id="+spec.ApplicationID.String())
	if err != nil {
		_ = run(ctx, binary, "rm", "--force", containerID)
		return fmt.Errorf("list previous containers: %w", err)
	}
	for _, id := range strings.Fields(old) {
		if id == containerID || strings.HasPrefix(containerID, id) || strings.HasPrefix(id, containerID) {
			continue
		}
		if err = run(ctx, binary, "rm", "--force", id); err != nil {
			_ = run(ctx, binary, "rm", "--force", containerID)
			return fmt.Errorf("remove superseded container: %w", err)
		}
	}
	return progress(deployments.Healthy, "Container is running from exact revision "+spec.CommitSHA)
}

func waitContainerReady(ctx context.Context, binary, containerID string) error {
	deadline := time.NewTimer(90 * time.Second)
	defer deadline.Stop()
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()
	for {
		state, err := runOutput(ctx, binary, "inspect", "--format", "{{.State.Running}} {{if .State.Health}}{{.State.Health.Status}}{{else}}none{{end}}", containerID)
		if err != nil {
			return fmt.Errorf("inspect new container: %w", err)
		}
		fields := strings.Fields(state)
		if len(fields) == 2 && fields[0] == "true" {
			switch fields[1] {
			case "none", "healthy":
				return nil
			case "unhealthy":
				return errors.New("new container health check failed")
			}
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-deadline.C:
			return errors.New("new container did not become healthy within 90 seconds")
		case <-ticker.C:
		}
	}
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

func run(ctx context.Context, binary string, args ...string) error {
	command := exec.CommandContext(ctx, binary, args...)
	command.Stdout = io.Discard
	command.Stderr = io.Discard
	return command.Run()
}
func runOutput(ctx context.Context, binary string, args ...string) (string, error) {
	command := exec.CommandContext(ctx, binary, args...)
	output, err := command.Output()
	if len(output) > 1<<20 {
		return "", errors.New("docker output exceeded one MiB")
	}
	return string(output), err
}
func shortSHA(value string) string {
	if len(value) > 12 {
		return value[:12]
	}
	return value
}
func compact(value string, length int) string {
	value = strings.ReplaceAll(value, "-", "")
	if len(value) > length {
		return value[:length]
	}
	return value
}
