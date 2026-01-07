package reset

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/sirupsen/logrus"
	"github.com/urfave/cli"
)

var (
	force     bool
	keepNet   bool
	reboot    bool
	dataOnly  bool
)

// Command returns the `factory-reset` sub-command
func Command() cli.Command {
	return cli.Command{
		Name:    "factory-reset",
		Aliases: []string{"reset"},
		Usage:   "reset system to factory defaults",
		Description: `
Factory reset erases all user data and configuration, returning the system
to its initial state. This includes:

  - Kubernetes state and workloads
  - Console pairing and credentials
  - User-installed applications
  - Custom configurations

Network settings can optionally be preserved with --keep-network.

WARNING: This operation is irreversible!`,
		Flags: []cli.Flag{
			cli.BoolFlag{
				Name:        "force,f",
				Usage:       "skip confirmation prompt",
				Destination: &force,
			},
			cli.BoolFlag{
				Name:        "keep-network",
				Usage:       "preserve network configuration",
				Destination: &keepNet,
			},
			cli.BoolFlag{
				Name:        "reboot",
				Usage:       "reboot after reset",
				Destination: &reboot,
			},
			cli.BoolFlag{
				Name:        "data-only",
				Usage:       "only reset user data, keep k3s state",
				Destination: &dataOnly,
			},
		},
		Action: run,
	}
}

func run(c *cli.Context) error {
	// Check if running as root
	if os.Geteuid() != 0 {
		return fmt.Errorf("factory-reset requires root privileges")
	}

	// Confirmation prompt
	if !force {
		fmt.Println("\033[1;31m╔══════════════════════════════════════════════════════════════╗")
		fmt.Println("║                    ⚠️  WARNING ⚠️                              ║")
		fmt.Println("║                                                              ║")
		fmt.Println("║  Factory reset will PERMANENTLY DELETE:                      ║")
		fmt.Println("║    - All Kubernetes workloads and data                       ║")
		fmt.Println("║    - Console pairing and credentials                         ║")
		fmt.Println("║    - User configurations and customizations                  ║")
		if !keepNet {
			fmt.Println("║    - Network settings and WiFi passwords                     ║")
		}
		fmt.Println("║                                                              ║")
		fmt.Println("║  This operation CANNOT be undone!                            ║")
		fmt.Println("╚══════════════════════════════════════════════════════════════╝\033[0m")
		fmt.Println()

		fmt.Print("Type 'RESET' to confirm factory reset: ")
		reader := bufio.NewReader(os.Stdin)
		input, _ := reader.ReadString('\n')
		input = strings.TrimSpace(input)

		if input != "RESET" {
			fmt.Println("Factory reset cancelled.")
			return nil
		}
	}

	fmt.Println("\n\033[1;33mStarting factory reset...\033[0m")

	// Stop k3s first
	fmt.Println("  → Stopping k3s...")
	exec.Command("rc-service", "k3s", "stop").Run()

	// Kill any remaining k3s processes
	exec.Command("pkill", "-9", "k3s").Run()
	exec.Command("pkill", "-9", "containerd").Run()

	// Paths to clean
	pathsToDelete := []string{
		"/var/lib/maculaos/paired",
		"/var/lib/maculaos/mesh-status",
		"/var/lib/maculaos/mesh-role",
		"/var/lib/maculaos/realm",
		"/var/lib/maculaos/console-token",
		"/var/lib/maculaos/api-key",
	}

	if !dataOnly {
		// Full reset including k3s
		pathsToDelete = append(pathsToDelete,
			"/var/lib/rancher/k3s",
			"/etc/rancher/k3s",
			"/var/lib/maculaos/k3s",
		)
	}

	if !keepNet {
		// Also reset network configuration
		pathsToDelete = append(pathsToDelete,
			"/var/lib/connman",
			"/var/lib/maculaos/network",
		)
	}

	// Delete paths
	for _, path := range pathsToDelete {
		if _, err := os.Stat(path); err == nil {
			fmt.Printf("  → Removing %s\n", path)
			if err := os.RemoveAll(path); err != nil {
				logrus.Warnf("failed to remove %s: %v", path, err)
			}
		}
	}

	// Reset hostname to regenerate on next boot
	fmt.Println("  → Resetting hostname...")
	os.Remove("/var/lib/maculaos/hostname")
	os.Remove("/etc/hostname")

	// Create marker file for first-boot wizard
	fmt.Println("  → Enabling first-boot wizard...")
	os.MkdirAll("/var/lib/maculaos", 0755)
	os.WriteFile("/var/lib/maculaos/first-boot", []byte("1"), 0644)

	// Sync filesystem
	fmt.Println("  → Syncing filesystem...")
	exec.Command("sync").Run()

	fmt.Println("\n\033[1;32m✓ Factory reset complete!\033[0m")

	if reboot {
		fmt.Println("\nRebooting in 5 seconds...")
		exec.Command("sleep", "5").Run()
		exec.Command("reboot").Run()
	} else {
		fmt.Println("\nPlease reboot to complete the reset:")
		fmt.Println("  sudo reboot")
	}

	return nil
}
