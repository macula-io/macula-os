package diag

import (
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"syscall"
	"time"

	"github.com/macula-io/macula-os/pkg/version"
	"github.com/sirupsen/logrus"
	"github.com/urfave/cli"
)

var (
	verbose bool
	jsonOut bool
)

// Command returns the `diag` sub-command for system diagnostics
func Command() cli.Command {
	return cli.Command{
		Name:  "diag",
		Usage: "run system diagnostics",
		Flags: []cli.Flag{
			cli.BoolFlag{
				Name:        "verbose,v",
				Usage:       "show detailed output",
				Destination: &verbose,
			},
			cli.BoolFlag{
				Name:        "json",
				Usage:       "output in JSON format",
				Destination: &jsonOut,
			},
		},
		Subcommands: []cli.Command{
			{
				Name:   "all",
				Usage:  "run all diagnostics",
				Action: runAll,
			},
			{
				Name:   "system",
				Usage:  "check system health (CPU, memory, disk)",
				Action: runSystem,
			},
			{
				Name:   "network",
				Usage:  "check network connectivity",
				Action: runNetwork,
			},
			{
				Name:   "k3s",
				Usage:  "check k3s/kubernetes status",
				Action: runK3s,
			},
			{
				Name:   "services",
				Usage:  "check system services",
				Action: runServices,
			},
			{
				Name:   "mesh",
				Usage:  "check Macula mesh connectivity",
				Action: runMesh,
			},
		},
		Action: runAll,
	}
}

func printHeader(title string) {
	fmt.Printf("\n\033[1;36m=== %s ===\033[0m\n", title)
}

func printStatus(name string, ok bool, detail string) {
	status := "\033[1;32m✓\033[0m"
	if !ok {
		status = "\033[1;31m✗\033[0m"
	}
	if detail != "" {
		fmt.Printf("  %s %s: %s\n", status, name, detail)
	} else {
		fmt.Printf("  %s %s\n", status, name)
	}
}

func runAll(c *cli.Context) error {
	fmt.Printf("\033[1;36mMaculaOS Diagnostics\033[0m v%s\n", version.Version)
	fmt.Printf("Time: %s\n", time.Now().Format(time.RFC3339))

	runSystem(c)
	runNetwork(c)
	runK3s(c)
	runServices(c)
	runMesh(c)

	fmt.Println()
	return nil
}

func runSystem(c *cli.Context) error {
	printHeader("System Health")

	// OS info
	hostname, _ := os.Hostname()
	printStatus("Hostname", true, hostname)
	printStatus("Architecture", true, runtime.GOARCH)
	printStatus("MaculaOS Version", true, version.Version)

	// Uptime
	if data, err := os.ReadFile("/proc/uptime"); err == nil {
		parts := strings.Fields(string(data))
		if len(parts) > 0 {
			var uptime float64
			fmt.Sscanf(parts[0], "%f", &uptime)
			hours := int(uptime) / 3600
			mins := (int(uptime) % 3600) / 60
			printStatus("Uptime", true, fmt.Sprintf("%dh %dm", hours, mins))
		}
	}

	// Memory
	var si syscall.Sysinfo_t
	if err := syscall.Sysinfo(&si); err == nil {
		totalMB := si.Totalram / 1024 / 1024
		freeMB := si.Freeram / 1024 / 1024
		usedMB := totalMB - freeMB
		pct := float64(usedMB) / float64(totalMB) * 100
		memOk := pct < 90
		printStatus("Memory", memOk, fmt.Sprintf("%dMB / %dMB (%.1f%%)", usedMB, totalMB, pct))
	}

	// Disk usage for root
	var stat syscall.Statfs_t
	if err := syscall.Statfs("/", &stat); err == nil {
		totalGB := float64(stat.Blocks*uint64(stat.Bsize)) / 1024 / 1024 / 1024
		freeGB := float64(stat.Bfree*uint64(stat.Bsize)) / 1024 / 1024 / 1024
		usedGB := totalGB - freeGB
		pct := usedGB / totalGB * 100
		diskOk := pct < 85
		printStatus("Disk (/)", diskOk, fmt.Sprintf("%.1fGB / %.1fGB (%.1f%%)", usedGB, totalGB, pct))
	}

	// Disk usage for /var/lib/maculaos
	if err := syscall.Statfs("/var/lib/maculaos", &stat); err == nil {
		totalGB := float64(stat.Blocks*uint64(stat.Bsize)) / 1024 / 1024 / 1024
		freeGB := float64(stat.Bfree*uint64(stat.Bsize)) / 1024 / 1024 / 1024
		usedGB := totalGB - freeGB
		pct := usedGB / totalGB * 100
		diskOk := pct < 85
		printStatus("Disk (/var/lib/maculaos)", diskOk, fmt.Sprintf("%.1fGB / %.1fGB (%.1f%%)", usedGB, totalGB, pct))
	}

	// CPU load
	if data, err := os.ReadFile("/proc/loadavg"); err == nil {
		parts := strings.Fields(string(data))
		if len(parts) >= 3 {
			printStatus("Load Average", true, fmt.Sprintf("%s %s %s", parts[0], parts[1], parts[2]))
		}
	}

	// Watchdog status
	if _, err := os.Stat("/dev/watchdog"); err == nil {
		printStatus("Hardware Watchdog", true, "available")
	} else {
		printStatus("Hardware Watchdog", false, "not available")
	}

	return nil
}

func runNetwork(c *cli.Context) error {
	printHeader("Network")

	// Check interfaces
	if out, err := exec.Command("ip", "-br", "addr").Output(); err == nil {
		lines := strings.Split(strings.TrimSpace(string(out)), "\n")
		for _, line := range lines {
			parts := strings.Fields(line)
			if len(parts) >= 3 && parts[0] != "lo" {
				state := parts[1]
				ip := ""
				if len(parts) > 2 {
					ip = parts[2]
				}
				ok := state == "UP"
				printStatus(fmt.Sprintf("Interface %s", parts[0]), ok, fmt.Sprintf("%s %s", state, ip))
			}
		}
	}

	// DNS resolution
	if out, err := exec.Command("getent", "hosts", "boot.macula.io").Output(); err == nil {
		ip := strings.Fields(string(out))[0]
		printStatus("DNS Resolution", true, fmt.Sprintf("boot.macula.io -> %s", ip))
	} else {
		printStatus("DNS Resolution", false, "cannot resolve boot.macula.io")
	}

	// Internet connectivity
	if err := exec.Command("ping", "-c", "1", "-W", "3", "8.8.8.8").Run(); err == nil {
		printStatus("Internet (ping)", true, "reachable")
	} else {
		printStatus("Internet (ping)", false, "unreachable")
	}

	// HTTPS connectivity
	if err := exec.Command("curl", "-s", "-o", "/dev/null", "-w", "", "--max-time", "5", "https://boot.macula.io").Run(); err == nil {
		printStatus("HTTPS (boot.macula.io)", true, "reachable")
	} else {
		printStatus("HTTPS (boot.macula.io)", false, "unreachable")
	}

	return nil
}

func runK3s(c *cli.Context) error {
	printHeader("Kubernetes (k3s)")

	// Check if k3s is running
	if out, err := exec.Command("pgrep", "-x", "k3s-server").Output(); err == nil {
		pid := strings.TrimSpace(string(out))
		printStatus("k3s Server", true, fmt.Sprintf("PID %s", pid))
	} else if out, err := exec.Command("pgrep", "-x", "k3s-agent").Output(); err == nil {
		pid := strings.TrimSpace(string(out))
		printStatus("k3s Agent", true, fmt.Sprintf("PID %s", pid))
	} else {
		printStatus("k3s", false, "not running")
		return nil
	}

	// Check kubectl
	if out, err := exec.Command("kubectl", "get", "nodes", "-o", "wide", "--no-headers").Output(); err == nil {
		lines := strings.Split(strings.TrimSpace(string(out)), "\n")
		for _, line := range lines {
			parts := strings.Fields(line)
			if len(parts) >= 2 {
				ok := parts[1] == "Ready"
				printStatus(fmt.Sprintf("Node %s", parts[0]), ok, parts[1])
			}
		}
	} else {
		if verbose {
			logrus.Debugf("kubectl error: %v", err)
		}
	}

	// Check critical pods
	if out, err := exec.Command("kubectl", "get", "pods", "-A", "--field-selector=status.phase!=Running,status.phase!=Succeeded", "--no-headers").Output(); err == nil {
		lines := strings.Split(strings.TrimSpace(string(out)), "\n")
		if len(lines) == 1 && lines[0] == "" {
			printStatus("All Pods Healthy", true, "")
		} else {
			printStatus("Unhealthy Pods", false, fmt.Sprintf("%d pods not running", len(lines)))
			if verbose {
				for _, line := range lines {
					fmt.Printf("    %s\n", line)
				}
			}
		}
	}

	return nil
}

func runServices(c *cli.Context) error {
	printHeader("System Services")

	services := []string{"k3s", "connman", "sshd", "avahi-daemon", "watchdog"}

	for _, svc := range services {
		// Check if service exists
		if _, err := os.Stat(fmt.Sprintf("/etc/init.d/%s", svc)); os.IsNotExist(err) {
			continue
		}

		// Check if running
		err := exec.Command("rc-service", svc, "status").Run()
		if err == nil {
			printStatus(svc, true, "running")
		} else {
			printStatus(svc, false, "stopped")
		}
	}

	return nil
}

func runMesh(c *cli.Context) error {
	printHeader("Macula Mesh")

	// Check mesh status file
	if data, err := os.ReadFile("/var/lib/maculaos/mesh-status"); err == nil {
		status := strings.TrimSpace(string(data))
		ok := status == "connected" || status == "online"
		printStatus("Mesh Status", ok, status)
	} else {
		printStatus("Mesh Status", false, "not configured")
	}

	// Check mesh role
	if data, err := os.ReadFile("/var/lib/maculaos/mesh-role"); err == nil {
		role := strings.TrimSpace(string(data))
		printStatus("Mesh Role", true, role)
	} else {
		printStatus("Mesh Role", true, "peer (default)")
	}

	// Check realm
	if data, err := os.ReadFile("/var/lib/maculaos/realm"); err == nil {
		realm := strings.TrimSpace(string(data))
		printStatus("Realm", true, realm)
	} else {
		printStatus("Realm", false, "not set")
	}

	// Check pairing status
	if _, err := os.Stat("/var/lib/maculaos/paired"); err == nil {
		printStatus("Portal Pairing", true, "paired")
	} else {
		printStatus("Portal Pairing", false, "not paired")
	}

	return nil
}
