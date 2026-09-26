package agent

import (
	runtimeprovider "github.com/itsmangooo/Silicon/backend/internal/providers/runtime"
	"strings"
)

const (
	ProtocolVersion   = 1
	CapabilityDocker  = "docker.runtime"
	CapabilityLogs    = "runtime.logs"
	CapabilityMetrics = "node.metrics"
)

type Operation string

const (
	OperationDeploy  Operation = "runtime.deploy"
	OperationInspect Operation = "runtime.inspect"
	OperationStatus  Operation = "runtime.status"
	OperationStart   Operation = "runtime.start"
	OperationStop    Operation = "runtime.stop"
	OperationRestart Operation = "runtime.restart"
	OperationRemove  Operation = "runtime.remove"
	OperationLogs    Operation = "runtime.logs"
	OperationBuild   Operation = "runtime.build"
)

type Envelope struct {
	Type      string        `json:"type"`
	Command   *Command      `json:"command,omitempty"`
	Result    *Result       `json:"result,omitempty"`
	Heartbeat *Heartbeat    `json:"heartbeat,omitempty"`
	Archive   *ArchiveChunk `json:"archive,omitempty"`
}

type Command struct {
	ID         string                          `json:"id"`
	Operation  Operation                       `json:"operation"`
	Deployment *runtimeprovider.DeploymentSpec `json:"deployment,omitempty"`
	InstanceID string                          `json:"instanceId,omitempty"`
	Logs       *runtimeprovider.LogRequest     `json:"logs,omitempty"`
	Build      *runtimeprovider.BuildSpec      `json:"build,omitempty"`
}

type Result struct {
	CommandID string                          `json:"commandId"`
	Final     bool                            `json:"final"`
	Status    *runtimeprovider.InstanceStatus `json:"status,omitempty"`
	Log       *runtimeprovider.LogLine        `json:"log,omitempty"`
	Error     string                          `json:"error,omitempty"`
	Image     string                          `json:"image,omitempty"`
}

type ArchiveChunk struct {
	CommandID string `json:"commandId"`
	Data      []byte `json:"data,omitempty"`
	Final     bool   `json:"final"`
}

type Heartbeat struct {
	ProtocolVersion int      `json:"protocolVersion"`
	AgentVersion    string   `json:"agentVersion"`
	Capabilities    []string `json:"capabilities"`
	Hostname        string   `json:"hostname"`
	OperatingSystem string   `json:"operatingSystem"`
	Architecture    string   `json:"architecture"`
	UptimeSeconds   int64    `json:"uptimeSeconds"`
	DockerAvailable bool     `json:"dockerAvailable"`
	DockerVersion   string   `json:"dockerVersion"`
	CPUCount        int      `json:"cpuCount"`
	CPUUsagePercent float64  `json:"cpuUsagePercent"`
	MemoryTotal     int64    `json:"memoryTotalBytes"`
	MemoryUsed      int64    `json:"memoryUsedBytes"`
	DiskTotal       int64    `json:"diskTotalBytes"`
	DiskUsed        int64    `json:"diskUsedBytes"`
}

func Compatibility(protocol int, version, expected string) string {
	if protocol != ProtocolVersion {
		return "incompatible"
	}
	if strings.TrimSpace(version) != strings.TrimSpace(expected) {
		return "outdated"
	}
	return "compatible"
}
