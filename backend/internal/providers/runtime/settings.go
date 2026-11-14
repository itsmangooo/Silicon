package runtime

import (
	"errors"
	"path"
	"regexp"
	"strings"

	"github.com/google/uuid"
)

type VolumeMount struct {
	Name     string `json:"name"`
	Target   string `json:"target"`
	ReadOnly bool   `json:"readOnly,omitempty"`
}

// Settings describes optional Docker features for an application.
type Settings struct {
	Volumes            []VolumeMount `json:"volumes,omitempty"`
	Arguments          []string      `json:"arguments,omitempty"`
	MemoryMiB          int           `json:"memoryMiB,omitempty"`
	EnvironmentNetwork bool          `json:"environmentNetwork,omitempty"`
}

var volumeNamePattern = regexp.MustCompile(`^[a-z0-9][a-z0-9-]{0,31}$`)
var mountTargetPattern = regexp.MustCompile(`^(/[a-zA-Z0-9._-]+)+$`)

func (s Settings) Validate() error {
	if s.MemoryMiB < 0 || (s.MemoryMiB > 0 && s.MemoryMiB < 64) || s.MemoryMiB > 1048576 {
		return errors.New("memory must be 0 (default) or between 64 and 1048576 MiB")
	}
	if len(s.Volumes) > 16 || len(s.Arguments) > 64 {
		return errors.New("use at most 16 volumes and 64 arguments")
	}
	names, targets := map[string]bool{}, map[string]bool{}
	for _, v := range s.Volumes {
		if !volumeNamePattern.MatchString(v.Name) || names[v.Name] {
			return errors.New("volume names must be unique and use 1–32 lowercase letters, digits or hyphens, starting with a letter or digit")
		}
		if len(v.Target) > 255 || !mountTargetPattern.MatchString(v.Target) || path.Clean(v.Target) != v.Target {
			return errors.New("volume targets must be canonical absolute container paths")
		}
		for target := range targets {
			if v.Target == target || strings.HasPrefix(target, v.Target+"/") || strings.HasPrefix(v.Target, target+"/") {
				return errors.New("volume targets may not overlap")
			}
		}
		names[v.Name], targets[v.Target] = true, true
	}
	for _, argument := range s.Arguments {
		if len(argument) > 2048 || strings.ContainsAny(argument, "\x00\r\n") {
			return errors.New("each container argument must be at most 2048 bytes and contain no line breaks or NUL bytes")
		}
	}
	return nil
}

// ResourceID only accepts canonical UUIDs before forming Docker resource names.
func ResourceID(value string) (string, error) {
	id, err := uuid.Parse(value)
	if err != nil || id == uuid.Nil || id.String() != value {
		return "", errors.New("a canonical resource UUID is required")
	}
	return id.String(), nil
}
