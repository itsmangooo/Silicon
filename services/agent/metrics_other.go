//go:build !linux

package main

import (
	"context"
	"os"
	"runtime"

	"github.com/itsmangooo/Silicon/backend/internal/agent"
)

type cpuSample struct{}

func heartbeatSample() *cpuSample { return &cpuSample{} }
func collectHeartbeat(context.Context, *cpuSample) agent.Heartbeat {
	hostname, _ := os.Hostname()
	return agent.Heartbeat{ProtocolVersion: agent.ProtocolVersion, AgentVersion: version, Capabilities: agentCapabilities(false), Hostname: hostname, OperatingSystem: runtime.GOOS, Architecture: runtime.GOARCH, CPUCount: runtime.NumCPU()}
}
