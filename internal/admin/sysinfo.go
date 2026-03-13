package admin

import (
	"os"
	"runtime"
	"strconv"
	"strings"
	"time"
)

// SysInfo holds system-level metrics.
type SysInfo struct {
	CPUUsagePercent float64
	CPUCores        int
	MemTotalMB      float64
	MemUsedMB       float64
	MemUsedPercent  float64
}

// GetSysInfo collects system stats.
// On Linux it reads /proc; on other platforms it falls back to Go runtime.
func GetSysInfo() SysInfo {
	if runtime.GOOS == "linux" {
		return getLinuxSysInfo()
	}
	return getFallbackSysInfo()
}

func getLinuxSysInfo() SysInfo {
	info := SysInfo{CPUCores: runtime.NumCPU()}

	// CPU usage: two samples from /proc/stat
	idle1, total1 := readCPUStat()
	time.Sleep(200 * time.Millisecond)
	idle2, total2 := readCPUStat()

	dTotal := total2 - total1
	dIdle := idle2 - idle1
	if dTotal > 0 {
		info.CPUUsagePercent = float64(dTotal-dIdle) / float64(dTotal) * 100
	}

	// Memory from /proc/meminfo
	info.MemTotalMB, info.MemUsedMB = readMemInfo()
	if info.MemTotalMB > 0 {
		info.MemUsedPercent = info.MemUsedMB / info.MemTotalMB * 100
	}

	return info
}

func readCPUStat() (idle, total uint64) {
	data, err := os.ReadFile("/proc/stat")
	if err != nil {
		return 0, 0
	}
	lines := strings.Split(string(data), "\n")
	for _, line := range lines {
		if strings.HasPrefix(line, "cpu ") {
			fields := strings.Fields(line)
			if len(fields) < 5 {
				return 0, 0
			}
			// fields: cpu user nice system idle iowait irq softirq ...
			for i := 1; i < len(fields); i++ {
				v, _ := strconv.ParseUint(fields[i], 10, 64)
				total += v
			}
			v, _ := strconv.ParseUint(fields[4], 10, 64)
			idle = v
			return idle, total
		}
	}
	return 0, 0
}

func readMemInfo() (totalMB, usedMB float64) {
	data, err := os.ReadFile("/proc/meminfo")
	if err != nil {
		return 0, 0
	}
	var totalKB, availKB uint64
	for _, line := range strings.Split(string(data), "\n") {
		if strings.HasPrefix(line, "MemTotal:") {
			totalKB = parseMemInfoValue(line)
		} else if strings.HasPrefix(line, "MemAvailable:") {
			availKB = parseMemInfoValue(line)
		}
	}
	totalMB = float64(totalKB) / 1024.0
	usedMB = float64(totalKB-availKB) / 1024.0
	return totalMB, usedMB
}

func parseMemInfoValue(line string) uint64 {
	// line format: "MemTotal:       16384000 kB"
	fields := strings.Fields(line)
	if len(fields) < 2 {
		return 0
	}
	v, _ := strconv.ParseUint(fields[1], 10, 64)
	return v
}

func getFallbackSysInfo() SysInfo {
	var m runtime.MemStats
	runtime.ReadMemStats(&m)
	totalMB := float64(m.Sys) / 1024.0 / 1024.0
	usedMB := float64(m.Alloc) / 1024.0 / 1024.0
	pct := 0.0
	if totalMB > 0 {
		pct = usedMB / totalMB * 100
	}
	return SysInfo{
		CPUCores:       runtime.NumCPU(),
		MemTotalMB:     totalMB,
		MemUsedMB:      usedMB,
		MemUsedPercent: pct,
	}
}
