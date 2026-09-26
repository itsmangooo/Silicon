package sourcebuild

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
)

const maxArchiveBytes int64 = 1 << 30

func BuildDockerArchive(ctx context.Context, binary, image, organizationID, applicationID, commitSHA string, archive io.Reader) error {
	workdir, err := os.MkdirTemp("", "silicon-build-*")
	if err != nil {
		return err
	}
	defer os.RemoveAll(workdir)
	if err = ExtractArchive(archive, workdir); err != nil {
		return fmt.Errorf("extract source archive: %w", err)
	}
	if strings.TrimSpace(binary) == "" {
		binary = "docker"
	}
	command := exec.CommandContext(ctx, binary, "build", "--pull", "--label", "silicon.managed=true", "--label", "silicon.organization_id="+organizationID, "--label", "silicon.application_id="+applicationID, "--label", "silicon.commit_sha="+commitSHA, "-t", image, workdir)
	stdout, stderr := &boundedBuffer{limit: 1 << 20}, &boundedBuffer{limit: 1 << 20}
	command.Stdout, command.Stderr = stdout, stderr
	err = command.Run()
	if err != nil {
		message := strings.TrimSpace(stderr.String())
		if message == "" {
			message = strings.TrimSpace(stdout.String())
		}
		if message == "" {
			message = err.Error()
		}
		message = strings.ReplaceAll(strings.ReplaceAll(message, "\r", " "), "\n", " ")
		if stdout.exceeded || stderr.exceeded {
			message += " (output truncated)"
		}
		if len(message) > 500 {
			message = message[:500]
		}
		return fmt.Errorf("Docker build failed: %s", message)
	}
	return nil
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

func ExtractArchive(source io.Reader, destination string) error {
	gz, err := gzip.NewReader(io.LimitReader(source, maxArchiveBytes+1))
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
