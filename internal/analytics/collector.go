package analytics

import (
	"bufio"
	"log"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/JustinTDCT/CineVault/internal/models"
	"github.com/JustinTDCT/CineVault/internal/repository"
	"github.com/JustinTDCT/CineVault/internal/stream"
)

// Collector periodically gathers system metrics and stores them.
type Collector struct {
	repo       *repository.AnalyticsRepository
	transcoder *stream.Transcoder
	mediaPaths []string
	interval   time.Duration
	stopCh     chan struct{}
	hasGPU     bool
}

// NewCollector creates a new system metrics collector.
func NewCollector(repo *repository.AnalyticsRepository, transcoder *stream.Transcoder, mediaPaths []string) *Collector {
	hasGPU := detectNVIDIA()
	return &Collector{
		repo:       repo,
		transcoder: transcoder,
		mediaPaths: mediaPaths,
		interval:   60 * time.Second,
		stopCh:     make(chan struct{}),
		hasGPU:     hasGPU,
	}
}

// Start begins the periodic collection loop. Call from a goroutine.
func (c *Collector) Start() {
	log.Println("Analytics collector started (60s interval)")
	ticker := time.NewTicker(c.interval)
	defer ticker.Stop()

	// Collect immediately on start
	c.collect()

	for {
		select {
		case <-ticker.C:
			c.collect()
		case <-c.stopCh:
			log.Println("Analytics collector stopped")
			return
		}
	}
}

// Stop halts the collector.
func (c *Collector) Stop() {
	close(c.stopCh)
}

func (c *Collector) collect() {
	m := &models.SystemMetric{}

	// CPU
	m.CPUPercent = readCPUPercent()

	// Memory
	memPercent, memUsedMB := readMemory()
	m.MemoryPercent = memPercent
	m.MemoryUsedMB = memUsedMB

	// GPU (NVIDIA only)
	if c.hasGPU {
		enc, mem, temp := readNVIDIAGPU()
		if enc >= 0 {
			m.GPUEncoderPercent = &enc
		}
		if mem >= 0 {
			m.GPUMemoryPercent = &mem
		}
		if temp >= 0 {
			m.GPUTempCelsius = &temp
		}
	}

	// Disk usage (first media path or root)
	diskPath := "/"
	if len(c.mediaPaths) > 0 && c.mediaPaths[0] != "" {
		diskPath = c.mediaPaths[0]
	}
	total, used, free := readDiskUsage(diskPath)
	m.DiskTotalGB = total
	m.DiskUsedGB = used
	m.DiskFreeGB = free

	// Active streams/transcodes
	activeStreams, _ := c.repo.CountActiveStreams()
	m.ActiveStreams = activeStreams
	m.ActiveTranscodes = c.transcoder.ActiveSessionCount()

	if err := c.repo.RecordSystemMetrics(m); err != nil {
		log.Printf("Analytics collector: failed to record metrics: %v", err)
	}

	// Cleanup old metrics (keep 30 days)
	_ = c.repo.CleanupOldMetrics(30)
}

// ── CPU ──

func readCPUPercent() float32 {
	f, err := os.Open("/proc/stat")
	if err != nil {
		return 0
	}
	defer f.Close()
	scanner := bufio.NewScanner(f)
	if !scanner.Scan() {
		return 0
	}
	line := scanner.Text()
	if !strings.HasPrefix(line, "cpu ") {
		return 0
	}
	fields := strings.Fields(line)
	if len(fields) < 5 {
		return 0
	}
	user, _ := strconv.ParseFloat(fields[1], 64)
	nice, _ := strconv.ParseFloat(fields[2], 64)
	system, _ := strconv.ParseFloat(fields[3], 64)
	idle, _ := strconv.ParseFloat(fields[4], 64)
	total := user + nice + system + idle
	if total == 0 {
		return 0
	}
	return float32((total - idle) / total * 100)
}

// ── Memory ──

func readMemory() (float32, int) {
	f, err := os.Open("/proc/meminfo")
	if err != nil {
		return 0, 0
	}
	defer f.Close()

	var totalKB, availableKB int64
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, "MemTotal:") {
			totalKB = parseMemValue(line)
		} else if strings.HasPrefix(line, "MemAvailable:") {
			availableKB = parseMemValue(line)
		}
	}
	if totalKB == 0 {
		return 0, 0
	}
	usedKB := totalKB - availableKB
	percent := float32(usedKB) / float32(totalKB) * 100
	usedMB := int(usedKB / 1024)
	return percent, usedMB
}

func parseMemValue(line string) int64 {
	fields := strings.Fields(line)
	if len(fields) < 2 {
		return 0
	}
	val, _ := strconv.ParseInt(fields[1], 10, 64)
	return val
}

// ── GPU (NVIDIA) ──

func detectNVIDIA() bool {
	_, err := exec.LookPath("nvidia-smi")
	return err == nil
}

func readNVIDIAGPU() (encoder, memory, temp float32) {
	encoder, memory, temp = -1, -1, -1
	cmd := exec.Command("nvidia-smi",
		"--query-gpu=utilization.encoder,utilization.memory,temperature.gpu",
		"--format=csv,noheader,nounits")
	out, err := cmd.Output()
	if err != nil {
		return
	}
	line := strings.TrimSpace(string(out))
	parts := strings.Split(line, ",")
	if len(parts) >= 3 {
		if v, err := strconv.ParseFloat(strings.TrimSpace(parts[0]), 32); err == nil {
			encoder = float32(v)
		}
		if v, err := strconv.ParseFloat(strings.TrimSpace(parts[1]), 32); err == nil {
			memory = float32(v)
		}
		if v, err := strconv.ParseFloat(strings.TrimSpace(parts[2]), 32); err == nil {
			temp = float32(v)
		}
	}
	return
}

// ── Disk ──

func readDiskUsage(path string) (totalGB, usedGB, freeGB float32) {
	var stat syscall.Statfs_t
	if err := syscall.Statfs(path, &stat); err != nil {
		return 0, 0, 0
	}
	totalBytes := stat.Blocks * uint64(stat.Bsize)
	freeBytes := stat.Bavail * uint64(stat.Bsize)
	usedBytes := totalBytes - freeBytes

	const gb = 1024 * 1024 * 1024
	totalGB = float32(totalBytes) / gb
	usedGB = float32(usedBytes) / gb
	freeGB = float32(freeBytes) / gb
	return
}
