// Package system collects a small, local view of the host running Control
// Plane. It reads kernel-provided files directly so the service does not need
// a resident system-agent dependency.
package system

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"api/internal/config"
)

// Overview is the current host state exposed to authenticated clients.
type Overview struct {
	CapturedAt time.Time `json:"capturedAt"`
	Machine    Machine   `json:"machine"`
	CPU        CPU       `json:"cpu"`
	Memory     Memory    `json:"memory"`
	Storage    Storage   `json:"storage"`
	Network    Network   `json:"network"`
}

type Machine struct {
	Hostname     string `json:"hostname"`
	Distribution string `json:"distribution"`
	Kernel       string `json:"kernel"`
	Architecture string `json:"architecture"`
}

type CPU struct {
	Cores        int     `json:"cores"`
	UsagePercent float64 `json:"usagePercent"`
}

type Memory struct {
	TotalBytes     uint64 `json:"totalBytes"`
	UsedBytes      uint64 `json:"usedBytes"`
	AvailableBytes uint64 `json:"availableBytes"`
}

type Storage struct {
	TotalBytes            uint64 `json:"totalBytes"`
	UsedBytes             uint64 `json:"usedBytes"`
	AvailableBytes        uint64 `json:"availableBytes"`
	ControlPlaneUsedBytes uint64 `json:"controlPlaneUsedBytes"`
}

type Network struct {
	PublicIP   string      `json:"publicIp,omitempty"`
	Interfaces []Interface `json:"interfaces"`
}

type Interface struct {
	Name      string   `json:"name"`
	Addresses []string `json:"addresses"`
}

// MetricSample is a persisted CPU and memory measurement for historical
// charts. Missing capture times are intentionally not synthesized.
type MetricSample struct {
	CapturedAt       time.Time `json:"capturedAt"`
	CPUUsagePercent  float64   `json:"cpuUsagePercent"`
	MemoryTotalBytes uint64    `json:"memoryTotalBytes"`
	MemoryUsedBytes  uint64    `json:"memoryUsedBytes"`
}

// Collector keeps only the previous CPU counters and a cached public address.
// All other information is read on demand from the local machine.
type Collector struct {
	paths  config.Paths
	client *http.Client

	mu          sync.Mutex
	previousCPU cpuCounters
	publicIP    string
	publicIPAt  time.Time
}

type cpuCounters struct {
	total uint64
	idle  uint64
}

func NewCollector(paths config.Paths) *Collector {
	return &Collector{
		paths:  paths,
		client: &http.Client{Timeout: 2 * time.Second},
	}
}

// Collect obtains a complete snapshot. A public-IP lookup is cached for six
// hours and failure to resolve it never prevents local monitoring.
func (c *Collector) Collect(ctx context.Context) (Overview, error) {
	memory, err := readMemory()
	if err != nil {
		return Overview{}, err
	}
	disk, err := readStorage(c.paths.ControlDir)
	if err != nil {
		return Overview{}, err
	}
	hostname, _ := os.Hostname()
	overview := Overview{
		CapturedAt: time.Now().UTC(),
		Machine: Machine{
			Hostname:     hostname,
			Distribution: readDistribution(),
			Kernel:       readKernel(),
			Architecture: runtime.GOARCH,
		},
		CPU: CPU{
			Cores:        runtime.NumCPU(),
			UsagePercent: c.readCPUUsage(),
		},
		Memory:  memory,
		Storage: disk,
		Network: Network{Interfaces: readInterfaces()},
	}
	overview.Storage.ControlPlaneUsedBytes = directorySize(c.paths.ControlDir)
	overview.Network.PublicIP = c.cachedPublicIP(ctx)
	return overview, nil
}

func readMemory() (Memory, error) {
	file, err := os.Open("/proc/meminfo")
	if err != nil {
		return Memory{}, fmt.Errorf("read memory information: %w", err)
	}
	defer file.Close()

	values := make(map[string]uint64)
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		fields := strings.Fields(scanner.Text())
		if len(fields) < 2 {
			continue
		}
		amount, err := strconv.ParseUint(fields[1], 10, 64)
		if err == nil {
			values[strings.TrimSuffix(fields[0], ":")] = amount * 1024
		}
	}
	if err := scanner.Err(); err != nil {
		return Memory{}, fmt.Errorf("scan memory information: %w", err)
	}
	total := values["MemTotal"]
	available := values["MemAvailable"]
	if total == 0 {
		return Memory{}, errors.New("memory total is unavailable")
	}
	if available > total {
		available = total
	}
	return Memory{TotalBytes: total, AvailableBytes: available, UsedBytes: total - available}, nil
}

func (c *Collector) readCPUUsage() float64 {
	current, err := readCPUCounters()
	if err != nil {
		return 0
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	previous := c.previousCPU
	c.previousCPU = current
	if previous.total == 0 || current.total <= previous.total {
		return 0
	}
	totalDelta := current.total - previous.total
	idleDelta := current.idle - previous.idle
	if idleDelta > totalDelta {
		idleDelta = totalDelta
	}
	return float64(totalDelta-idleDelta) * 100 / float64(totalDelta)
}

func readCPUCounters() (cpuCounters, error) {
	file, err := os.Open("/proc/stat")
	if err != nil {
		return cpuCounters{}, err
	}
	defer file.Close()
	scanner := bufio.NewScanner(file)
	if !scanner.Scan() {
		return cpuCounters{}, errors.New("CPU counters are unavailable")
	}
	fields := strings.Fields(scanner.Text())
	if len(fields) < 5 || fields[0] != "cpu" {
		return cpuCounters{}, errors.New("CPU counters have an invalid format")
	}
	var counters cpuCounters
	for index, value := range fields[1:] {
		amount, err := strconv.ParseUint(value, 10, 64)
		if err != nil {
			return cpuCounters{}, err
		}
		counters.total += amount
		if index == 3 || index == 4 { // idle and iowait
			counters.idle += amount
		}
	}
	return counters, nil
}

func readStorage(path string) (Storage, error) {
	var stat syscall.Statfs_t
	if err := syscall.Statfs(path, &stat); err != nil {
		return Storage{}, fmt.Errorf("read storage information: %w", err)
	}
	blockSize := uint64(stat.Bsize)
	total := stat.Blocks * blockSize
	available := stat.Bavail * blockSize
	if available > total {
		available = total
	}
	return Storage{TotalBytes: total, AvailableBytes: available, UsedBytes: total - available}, nil
}

func directorySize(root string) uint64 {
	var total uint64
	_ = filepath.WalkDir(root, func(_ string, entry fs.DirEntry, err error) error {
		if err != nil || entry.IsDir() {
			return nil
		}
		info, err := entry.Info()
		if err == nil && info.Size() > 0 {
			total += uint64(info.Size())
		}
		return nil
	})
	return total
}

func readDistribution() string {
	contents, err := os.ReadFile("/etc/os-release")
	if err != nil {
		return runtime.GOOS
	}
	for _, line := range strings.Split(string(contents), "\n") {
		key, value, found := strings.Cut(line, "=")
		if found && key == "PRETTY_NAME" {
			return strings.Trim(value, "\"")
		}
	}
	return runtime.GOOS
}

func readKernel() string {
	var value syscall.Utsname
	if err := syscall.Uname(&value); err != nil {
		return ""
	}
	characters := make([]byte, 0, len(value.Release))
	for _, character := range value.Release {
		if character == 0 {
			break
		}
		characters = append(characters, byte(character))
	}
	return string(characters)
}

func readInterfaces() []Interface {
	interfaces, err := net.Interfaces()
	if err != nil {
		return []Interface{}
	}
	result := make([]Interface, 0, len(interfaces))
	for _, networkInterface := range interfaces {
		if networkInterface.Flags&net.FlagUp == 0 || networkInterface.Flags&net.FlagLoopback != 0 {
			continue
		}
		addresses, err := networkInterface.Addrs()
		if err != nil {
			continue
		}
		values := make([]string, 0, len(addresses))
		for _, address := range addresses {
			values = append(values, address.String())
		}
		result = append(result, Interface{Name: networkInterface.Name, Addresses: values})
	}
	return result
}

func (c *Collector) cachedPublicIP(ctx context.Context) string {
	c.mu.Lock()
	if time.Since(c.publicIPAt) < 6*time.Hour {
		value := c.publicIP
		c.mu.Unlock()
		return value
	}
	c.mu.Unlock()
	c.mu.Lock()
	c.publicIPAt = time.Now()
	c.mu.Unlock()

	request, err := http.NewRequestWithContext(ctx, http.MethodGet, "https://api.ipify.org", nil)
	if err != nil {
		return ""
	}
	response, err := c.client.Do(request)
	if err != nil {
		return ""
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return ""
	}
	buffer, err := io.ReadAll(io.LimitReader(response.Body, 64))
	if err != nil {
		return ""
	}
	value := strings.TrimSpace(string(buffer))
	if net.ParseIP(value) == nil {
		return ""
	}
	c.mu.Lock()
	c.publicIP = value
	c.mu.Unlock()
	return value
}
