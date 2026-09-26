//go:build linux

package main

import (
	"bufio"
	"context"
	"os"
	"os/exec"
	"runtime"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/itsmangooo/Silicon/backend/internal/agent"
)

type cpuSample struct{ idle, total uint64 }

func collectHeartbeat(ctx context.Context, previous *cpuSample) agent.Heartbeat {
	hostname, _ := os.Hostname()
	var info syscall.Sysinfo_t
	_ = syscall.Sysinfo(&info)
	var disk syscall.Statfs_t
	_ = syscall.Statfs("/", &disk)
	sample := heartbeatSample()
	cpuUsage := 0.0
	if previous != nil && sample != nil && sample.total > previous.total {
		total, idle := sample.total-previous.total, sample.idle-previous.idle
		cpuUsage = float64(total-idle) * 100 / float64(total)
	}
	dockerAvailable, dockerVersion := dockerInformation(ctx)
	unit := uint64(info.Unit)
	if unit == 0 {
		unit = 1
	}
	memoryTotal := uint64(info.Totalram) * unit
	memoryFree := uint64(info.Freeram+info.Bufferram) * unit
	return agent.Heartbeat{ProtocolVersion: agent.ProtocolVersion, AgentVersion: version, Capabilities: agentCapabilities(dockerAvailable), Hostname: hostname, OperatingSystem: operatingSystem(), Architecture: runtime.GOARCH, UptimeSeconds: info.Uptime, DockerAvailable: dockerAvailable, DockerVersion: dockerVersion, CPUCount: runtime.NumCPU(), CPUUsagePercent: cpuUsage, MemoryTotal: int64(memoryTotal), MemoryUsed: int64(memoryTotal - memoryFree), DiskTotal: int64(disk.Blocks) * int64(disk.Bsize), DiskUsed: int64(disk.Blocks-disk.Bfree) * int64(disk.Bsize)}
}

func heartbeatSample() *cpuSample {
	file, err := os.Open("/proc/stat")
	if err != nil {
		return nil
	}
	defer file.Close()
	scanner := bufio.NewScanner(file)
	if !scanner.Scan() {
		return nil
	}
	fields := strings.Fields(scanner.Text())
	if len(fields) < 5 || fields[0] != "cpu" {
		return nil
	}
	var values []uint64
	for _, field := range fields[1:] {
		value, err := strconv.ParseUint(field, 10, 64)
		if err != nil {
			return nil
		}
		values = append(values, value)
	}
	var total uint64
	for _, value := range values {
		total += value
	}
	idle := values[3]
	if len(values) > 4 {
		idle += values[4]
	}
	return &cpuSample{idle: idle, total: total}
}

func dockerInformation(parent context.Context) (bool, string) {
	ctx, cancel := context.WithTimeout(parent, 5*time.Second)
	defer cancel()
	output, err := exec.CommandContext(ctx, "docker", "version", "--format", "{{.Server.Version}}").Output()
	if err != nil {
		return false, ""
	}
	return true, strings.TrimSpace(string(output))
}

func operatingSystem() string {
	body, err := os.ReadFile("/etc/os-release")
	if err != nil {
		return "linux"
	}
	for _, line := range strings.Split(string(body), "\n") {
		if strings.HasPrefix(line, "PRETTY_NAME=") {
			return strings.Trim(strings.TrimPrefix(line, "PRETTY_NAME="), "\"")
		}
	}
	return "linux"
}
