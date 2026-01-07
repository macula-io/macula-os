package health

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/sirupsen/logrus"
	"github.com/urfave/cli"
	"gopkg.in/yaml.v2"
)

var (
	checkInterval int
	jsonOutput    bool
)

// Command returns the `health` sub-command for service health checks
func Command() cli.Command {
	return cli.Command{
		Name:  "health",
		Usage: "manage service health checks",
		Description: `
Monitor and manage health checks for system services.

Health checks can monitor:
  - Processes: Check if a process is running
  - HTTP endpoints: Check if a URL returns 200 OK
  - Disk usage: Check if disk usage is below threshold

When a check fails, configured actions are taken:
  - alert: Log a warning
  - restart: Restart the service (up to max_restarts)
  - cleanup: Clean up disk space`,
		Subcommands: []cli.Command{
			{
				Name:  "check",
				Usage: "run all health checks once",
				Flags: []cli.Flag{
					cli.BoolFlag{
						Name:        "json",
						Usage:       "output results as JSON",
						Destination: &jsonOutput,
					},
				},
				Action: checkAction,
			},
			{
				Name:  "watch",
				Usage: "continuously monitor health (daemon mode)",
				Flags: []cli.Flag{
					cli.IntFlag{
						Name:        "interval",
						Usage:       "check interval in seconds",
						Value:       30,
						Destination: &checkInterval,
					},
				},
				Action: watchAction,
			},
			{
				Name:   "status",
				Usage:  "show health check configuration and status",
				Action: statusAction,
			},
			{
				Name:      "add",
				Usage:     "add a new health check",
				ArgsUsage: "<name> <type>",
				Flags: []cli.Flag{
					cli.StringFlag{
						Name:  "process",
						Usage: "process name to check (for type=process)",
					},
					cli.StringFlag{
						Name:  "url",
						Usage: "URL to check (for type=http)",
					},
					cli.StringFlag{
						Name:  "path",
						Usage: "path to check (for type=disk)",
					},
					cli.StringFlag{
						Name:  "threshold",
						Usage: "threshold value (e.g., 90% for disk)",
					},
					cli.StringFlag{
						Name:  "action",
						Usage: "action on failure: alert, restart, cleanup",
						Value: "alert",
					},
					cli.IntFlag{
						Name:  "max-restarts",
						Usage: "maximum restart attempts",
						Value: 3,
					},
				},
				Action: addAction,
			},
			{
				Name:      "remove",
				Usage:     "remove a health check",
				ArgsUsage: "<name>",
				Action:    removeAction,
			},
		},
		Action: statusAction,
	}
}

// HealthResult represents the result of a single health check
type HealthResult struct {
	Name    string `json:"name"`
	Type    string `json:"type"`
	Status  string `json:"status"` // "ok", "warn", "fail"
	Message string `json:"message,omitempty"`
}

// HealthConfig represents health check configuration
type HealthConfig struct {
	Checks []HealthCheck `yaml:"checks,omitempty"`
}

// HealthCheck defines a single health check
type HealthCheck struct {
	Name             string `yaml:"name,omitempty"`
	Type             string `yaml:"type,omitempty"` // process, http, disk
	Process          string `yaml:"process,omitempty"`
	URL              string `yaml:"url,omitempty"`
	Path             string `yaml:"path,omitempty"`
	Interval         string `yaml:"interval,omitempty"`
	Timeout          string `yaml:"timeout,omitempty"`
	Threshold        string `yaml:"threshold,omitempty"`
	RestartOnFailure bool   `yaml:"restartOnFailure,omitempty"`
	MaxRestarts      int    `yaml:"maxRestarts,omitempty"`
	Action           string `yaml:"action,omitempty"`
}

func statusAction(c *cli.Context) error {
	fmt.Println("\033[1;36m=== Health Check Configuration ===\033[0m\n")

	cfg, err := readHealthConfig()
	if err != nil {
		fmt.Println("  \033[1;33m!\033[0m No health checks configured")
		fmt.Println("    Add checks with: maculaos health add <name> <type>")
		return nil
	}

	if len(cfg.Checks) == 0 {
		fmt.Println("  \033[1;33m!\033[0m No health checks configured")
		return nil
	}

	fmt.Printf("  Configured checks: %d\n\n", len(cfg.Checks))

	for _, check := range cfg.Checks {
		fmt.Printf("  \033[1;36m%s\033[0m (%s)\n", check.Name, check.Type)
		switch check.Type {
		case "process":
			fmt.Printf("    Process: %s\n", check.Process)
		case "http":
			fmt.Printf("    URL: %s\n", check.URL)
		case "disk":
			fmt.Printf("    Path: %s, Threshold: %s\n", check.Path, check.Threshold)
		}
		fmt.Printf("    Action: %s", check.Action)
		if check.RestartOnFailure {
			fmt.Printf(" (max %d restarts)", check.MaxRestarts)
		}
		fmt.Println()
	}

	return nil
}

func checkAction(c *cli.Context) error {
	cfg, err := readHealthConfig()
	if err != nil {
		// Use default checks if no config
		cfg = defaultHealthConfig()
	}

	results := runAllChecks(cfg)

	if jsonOutput {
		data, _ := json.MarshalIndent(results, "", "  ")
		fmt.Println(string(data))
		return nil
	}

	fmt.Println("\033[1;36m=== Health Check Results ===\033[0m\n")

	allOk := true
	for _, result := range results {
		icon := "\033[1;32m✓\033[0m"
		if result.Status == "warn" {
			icon = "\033[1;33m!\033[0m"
			allOk = false
		} else if result.Status == "fail" {
			icon = "\033[1;31m✗\033[0m"
			allOk = false
		}
		fmt.Printf("  %s %s: %s\n", icon, result.Name, result.Message)
	}

	if allOk {
		fmt.Println("\n\033[1;32mAll health checks passed\033[0m")
	} else {
		fmt.Println("\n\033[1;33mSome health checks require attention\033[0m")
	}

	return nil
}

func watchAction(c *cli.Context) error {
	fmt.Printf("Starting health check daemon (interval: %ds)\n", checkInterval)
	fmt.Println("Press Ctrl+C to stop\n")

	cfg, err := readHealthConfig()
	if err != nil {
		cfg = defaultHealthConfig()
	}

	restartCounts := make(map[string]int)

	for {
		results := runAllChecks(cfg)

		for _, result := range results {
			if result.Status != "ok" {
				logrus.Warnf("Health check failed: %s - %s", result.Name, result.Message)

				// Find the check config
				for _, check := range cfg.Checks {
					if check.Name == result.Name {
						handleFailure(&check, &restartCounts)
						break
					}
				}
			}
		}

		time.Sleep(time.Duration(checkInterval) * time.Second)
	}
}

func addAction(c *cli.Context) error {
	if c.NArg() < 2 {
		return fmt.Errorf("usage: maculaos health add <name> <type>")
	}

	name := c.Args().Get(0)
	checkType := c.Args().Get(1)

	check := HealthCheck{
		Name:             name,
		Type:             checkType,
		Process:          c.String("process"),
		URL:              c.String("url"),
		Path:             c.String("path"),
		Threshold:        c.String("threshold"),
		Action:           c.String("action"),
		RestartOnFailure: c.String("action") == "restart",
		MaxRestarts:      c.Int("max-restarts"),
	}

	// Validate
	switch checkType {
	case "process":
		if check.Process == "" {
			return fmt.Errorf("--process is required for type=process")
		}
	case "http":
		if check.URL == "" {
			return fmt.Errorf("--url is required for type=http")
		}
	case "disk":
		if check.Path == "" {
			return fmt.Errorf("--path is required for type=disk")
		}
		if check.Threshold == "" {
			check.Threshold = "90%"
		}
	default:
		return fmt.Errorf("unknown check type: %s (use: process, http, disk)", checkType)
	}

	cfg, err := readHealthConfig()
	if err != nil {
		cfg = &HealthConfig{}
	}

	// Check for duplicate
	for _, existing := range cfg.Checks {
		if existing.Name == name {
			return fmt.Errorf("health check '%s' already exists", name)
		}
	}

	cfg.Checks = append(cfg.Checks, check)

	if err := writeHealthConfig(cfg); err != nil {
		return fmt.Errorf("failed to save config: %v", err)
	}

	fmt.Printf("\033[1;32m✓\033[0m Added health check: %s\n", name)
	return nil
}

func removeAction(c *cli.Context) error {
	if c.NArg() < 1 {
		return fmt.Errorf("usage: maculaos health remove <name>")
	}

	name := c.Args().Get(0)

	cfg, err := readHealthConfig()
	if err != nil {
		return fmt.Errorf("no health checks configured")
	}

	found := false
	newChecks := []HealthCheck{}
	for _, check := range cfg.Checks {
		if check.Name == name {
			found = true
		} else {
			newChecks = append(newChecks, check)
		}
	}

	if !found {
		return fmt.Errorf("health check '%s' not found", name)
	}

	cfg.Checks = newChecks

	if err := writeHealthConfig(cfg); err != nil {
		return fmt.Errorf("failed to save config: %v", err)
	}

	fmt.Printf("\033[1;32m✓\033[0m Removed health check: %s\n", name)
	return nil
}

func runAllChecks(cfg *HealthConfig) []HealthResult {
	var results []HealthResult

	for _, check := range cfg.Checks {
		result := runCheck(&check)
		results = append(results, result)
	}

	return results
}

func runCheck(check *HealthCheck) HealthResult {
	result := HealthResult{
		Name: check.Name,
		Type: check.Type,
	}

	switch check.Type {
	case "process":
		result = checkProcess(check)
	case "http":
		result = checkHTTP(check)
	case "disk":
		result = checkDisk(check)
	default:
		result.Status = "fail"
		result.Message = fmt.Sprintf("unknown check type: %s", check.Type)
	}

	return result
}

func checkProcess(check *HealthCheck) HealthResult {
	result := HealthResult{
		Name: check.Name,
		Type: "process",
	}

	// Use pgrep to check if process is running
	cmd := exec.Command("pgrep", "-x", check.Process)
	if err := cmd.Run(); err != nil {
		result.Status = "fail"
		result.Message = fmt.Sprintf("process '%s' not running", check.Process)
	} else {
		result.Status = "ok"
		result.Message = fmt.Sprintf("process '%s' is running", check.Process)
	}

	return result
}

func checkHTTP(check *HealthCheck) HealthResult {
	result := HealthResult{
		Name: check.Name,
		Type: "http",
	}

	timeout := 5 * time.Second
	if check.Timeout != "" {
		if d, err := time.ParseDuration(check.Timeout); err == nil {
			timeout = d
		}
	}

	client := &http.Client{Timeout: timeout}
	resp, err := client.Get(check.URL)
	if err != nil {
		result.Status = "fail"
		result.Message = fmt.Sprintf("failed to reach %s: %v", check.URL, err)
		return result
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		result.Status = "ok"
		result.Message = fmt.Sprintf("%s returned %d", check.URL, resp.StatusCode)
	} else {
		result.Status = "fail"
		result.Message = fmt.Sprintf("%s returned %d", check.URL, resp.StatusCode)
	}

	return result
}

func checkDisk(check *HealthCheck) HealthResult {
	result := HealthResult{
		Name: check.Name,
		Type: "disk",
	}

	var stat syscall.Statfs_t
	if err := syscall.Statfs(check.Path, &stat); err != nil {
		result.Status = "fail"
		result.Message = fmt.Sprintf("failed to stat %s: %v", check.Path, err)
		return result
	}

	total := stat.Blocks * uint64(stat.Bsize)
	free := stat.Bfree * uint64(stat.Bsize)
	used := total - free
	usedPct := float64(used) / float64(total) * 100

	threshold := 90.0
	if check.Threshold != "" {
		if t, err := strconv.ParseFloat(strings.TrimSuffix(check.Threshold, "%"), 64); err == nil {
			threshold = t
		}
	}

	if usedPct >= threshold {
		result.Status = "warn"
		result.Message = fmt.Sprintf("%s: %.1f%% used (threshold: %.0f%%)", check.Path, usedPct, threshold)
	} else {
		result.Status = "ok"
		result.Message = fmt.Sprintf("%s: %.1f%% used", check.Path, usedPct)
	}

	return result
}

func handleFailure(check *HealthCheck, restartCounts *map[string]int) {
	switch check.Action {
	case "alert":
		logrus.Warnf("Alert: %s health check failed", check.Name)

	case "restart":
		count := (*restartCounts)[check.Name]
		if count >= check.MaxRestarts {
			logrus.Errorf("Max restarts (%d) reached for %s", check.MaxRestarts, check.Name)
			return
		}

		logrus.Infof("Restarting service: %s (attempt %d/%d)", check.Name, count+1, check.MaxRestarts)
		cmd := exec.Command("rc-service", check.Name, "restart")
		if err := cmd.Run(); err != nil {
			logrus.Errorf("Failed to restart %s: %v", check.Name, err)
		}
		(*restartCounts)[check.Name] = count + 1

	case "cleanup":
		logrus.Infof("Running cleanup for %s", check.Name)
		runCleanup(check.Path)
	}
}

func runCleanup(path string) {
	// Clean old logs
	cmd := exec.Command("find", path, "-name", "*.log", "-mtime", "+7", "-delete")
	cmd.Run()

	// Clean old journal logs
	exec.Command("journalctl", "--vacuum-time=3d").Run()

	// Clean container images
	exec.Command("crictl", "rmi", "--prune").Run()
}

func defaultHealthConfig() *HealthConfig {
	return &HealthConfig{
		Checks: []HealthCheck{
			{
				Name:             "k3s",
				Type:             "process",
				Process:          "k3s-server",
				Action:           "restart",
				RestartOnFailure: true,
				MaxRestarts:      3,
			},
			{
				Name:      "root-disk",
				Type:      "disk",
				Path:      "/",
				Threshold: "90%",
				Action:    "alert",
			},
			{
				Name:      "data-disk",
				Type:      "disk",
				Path:      "/var/lib",
				Threshold: "85%",
				Action:    "cleanup",
			},
		},
	}
}

func readHealthConfig() (*HealthConfig, error) {
	data, err := os.ReadFile("/var/lib/maculaos/health.yaml")
	if err != nil {
		return nil, err
	}
	var cfg HealthConfig
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, err
	}
	return &cfg, nil
}

func writeHealthConfig(cfg *HealthConfig) error {
	if err := os.MkdirAll("/var/lib/maculaos", 0755); err != nil {
		return err
	}
	data, err := yaml.Marshal(cfg)
	if err != nil {
		return err
	}
	return os.WriteFile("/var/lib/maculaos/health.yaml", data, 0644)
}
