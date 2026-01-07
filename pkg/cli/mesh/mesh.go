package mesh

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/sirupsen/logrus"
	"github.com/urfave/cli"
	"gopkg.in/yaml.v2"
)

var (
	enableBootstrap bool
	enableGateway   bool
	realm           string
	bootstrapPeers  string
	tlsMode         string
)

// Command returns the `mesh` sub-command for mesh role configuration
func Command() cli.Command {
	return cli.Command{
		Name:  "mesh",
		Usage: "configure Macula mesh roles",
		Description: `
Configure this node's role in the Macula mesh network.

All nodes are implicitly Peers. Additional roles can be enabled:
  - Bootstrap: DHT entry point for new nodes (requires public IP)
  - Gateway:   NAT relay and API ingress (requires public IP)

Common configurations:
  - Home device:     Peer only (default)
  - Cloud entry:     Peer + Bootstrap
  - Edge relay:      Peer + Gateway
  - Infrastructure:  Peer + Bootstrap + Gateway`,
		Subcommands: []cli.Command{
			{
				Name:   "status",
				Usage:  "show current mesh configuration",
				Action: statusAction,
			},
			{
				Name:  "configure",
				Usage: "configure mesh roles interactively",
				Flags: []cli.Flag{
					cli.BoolFlag{
						Name:        "bootstrap",
						Usage:       "enable bootstrap role (DHT entry point)",
						Destination: &enableBootstrap,
					},
					cli.BoolFlag{
						Name:        "gateway",
						Usage:       "enable gateway role (NAT relay)",
						Destination: &enableGateway,
					},
					cli.StringFlag{
						Name:        "realm",
						Usage:       "mesh realm identifier (e.g., io.macula)",
						Value:       "io.macula",
						Destination: &realm,
					},
					cli.StringFlag{
						Name:        "bootstrap-peers",
						Usage:       "comma-separated list of bootstrap peers",
						Value:       "https://boot.macula.io:443",
						Destination: &bootstrapPeers,
					},
					cli.StringFlag{
						Name:        "tls-mode",
						Usage:       "TLS mode: development or production",
						Value:       "development",
						Destination: &tlsMode,
					},
				},
				Action: configureAction,
			},
			{
				Name:   "wizard",
				Usage:  "interactive mesh setup wizard",
				Action: wizardAction,
			},
			{
				Name:   "apply",
				Usage:  "apply mesh configuration (start/restart services)",
				Action: applyAction,
			},
		},
		Action: statusAction,
	}
}

func statusAction(c *cli.Context) error {
	fmt.Println("\033[1;36m=== Mesh Configuration ===\033[0m")

	// Read current config
	cfg, err := readMeshConfig()
	if err != nil {
		fmt.Println("  \033[1;33m!\033[0m No mesh configuration found")
		fmt.Println("    Run: maculaos mesh wizard")
		return nil
	}

	// Display roles
	fmt.Println("  \033[1;32m✓\033[0m Peer role: always enabled")
	if cfg.Roles.Bootstrap {
		fmt.Println("  \033[1;32m✓\033[0m Bootstrap role: enabled")
	} else {
		fmt.Println("  \033[1;90m○\033[0m Bootstrap role: disabled")
	}
	if cfg.Roles.Gateway {
		fmt.Println("  \033[1;32m✓\033[0m Gateway role: enabled")
	} else {
		fmt.Println("  \033[1;90m○\033[0m Gateway role: disabled")
	}

	// Display config
	fmt.Printf("\n  Realm: %s\n", cfg.Realm)
	fmt.Printf("  TLS Mode: %s\n", cfg.TLSMode)
	if len(cfg.BootstrapPeers) > 0 {
		fmt.Println("  Bootstrap Peers:")
		for _, peer := range cfg.BootstrapPeers {
			fmt.Printf("    - %s\n", peer)
		}
	}

	// Check service status
	fmt.Println("\n\033[1;36m=== Service Status ===\033[0m")
	checkService("macula-mesh")
	if cfg.Roles.Bootstrap {
		checkService("macula-bootstrap")
	}
	if cfg.Roles.Gateway {
		checkService("macula-gateway")
	}

	return nil
}

func configureAction(c *cli.Context) error {
	cfg := MeshConfig{
		Roles: MeshRoles{
			Bootstrap: enableBootstrap,
			Gateway:   enableGateway,
		},
		Realm:          realm,
		BootstrapPeers: strings.Split(bootstrapPeers, ","),
		TLSMode:        tlsMode,
	}

	// Validate
	if cfg.Roles.Bootstrap || cfg.Roles.Gateway {
		fmt.Println("\033[1;33mNote:\033[0m Bootstrap and Gateway roles require a public IP address")
	}

	if err := writeMeshConfig(&cfg); err != nil {
		return fmt.Errorf("failed to write config: %v", err)
	}

	fmt.Println("\033[1;32m✓\033[0m Mesh configuration saved")
	fmt.Println("  Run 'maculaos mesh apply' to apply changes")

	return nil
}

func wizardAction(c *cli.Context) error {
	fmt.Println("\033[1;36m╔══════════════════════════════════════════════════════════════╗")
	fmt.Println("║              Macula Mesh Configuration Wizard                 ║")
	fmt.Println("╚══════════════════════════════════════════════════════════════╝\033[0m")
	fmt.Println()

	reader := bufio.NewReader(os.Stdin)

	// Explain roles
	fmt.Println("All nodes are \033[1;32mPeers\033[0m by default. Additional roles available:")
	fmt.Println()
	fmt.Println("  \033[1;36mBootstrap\033[0m - DHT entry point for new nodes")
	fmt.Println("            Requires: Public IP, well-known DNS")
	fmt.Println("            Use case: Cloud VM, datacenter server")
	fmt.Println()
	fmt.Println("  \033[1;36mGateway\033[0m   - NAT relay, external API access")
	fmt.Println("            Requires: Public IP, bandwidth")
	fmt.Println("            Use case: Edge server with public IP")
	fmt.Println()

	cfg := MeshConfig{
		Roles:          MeshRoles{},
		Realm:          "io.macula",
		BootstrapPeers: []string{"https://boot.macula.io:443"},
		TLSMode:        "development",
	}

	// Ask about Bootstrap role
	fmt.Print("Enable \033[1;36mBootstrap\033[0m role? [y/N]: ")
	input, _ := reader.ReadString('\n')
	input = strings.TrimSpace(strings.ToLower(input))
	cfg.Roles.Bootstrap = input == "y" || input == "yes"

	// Ask about Gateway role
	fmt.Print("Enable \033[1;36mGateway\033[0m role? [y/N]: ")
	input, _ = reader.ReadString('\n')
	input = strings.TrimSpace(strings.ToLower(input))
	cfg.Roles.Gateway = input == "y" || input == "yes"

	// Ask about realm
	fmt.Printf("Mesh realm [%s]: ", cfg.Realm)
	input, _ = reader.ReadString('\n')
	input = strings.TrimSpace(input)
	if input != "" {
		cfg.Realm = input
	}

	// Ask about bootstrap peers (only if not a bootstrap node)
	if !cfg.Roles.Bootstrap {
		fmt.Printf("Bootstrap peer URL [%s]: ", cfg.BootstrapPeers[0])
		input, _ = reader.ReadString('\n')
		input = strings.TrimSpace(input)
		if input != "" {
			cfg.BootstrapPeers = strings.Split(input, ",")
		}
	}

	// Ask about TLS mode
	fmt.Printf("TLS mode (development/production) [%s]: ", cfg.TLSMode)
	input, _ = reader.ReadString('\n')
	input = strings.TrimSpace(strings.ToLower(input))
	if input == "production" || input == "prod" {
		cfg.TLSMode = "production"
	}

	// Confirm
	fmt.Println("\n\033[1;36m=== Configuration Summary ===\033[0m")
	fmt.Println()
	fmt.Printf("  Peer:      \033[1;32malways enabled\033[0m\n")
	fmt.Printf("  Bootstrap: %s\n", boolToStatus(cfg.Roles.Bootstrap))
	fmt.Printf("  Gateway:   %s\n", boolToStatus(cfg.Roles.Gateway))
	fmt.Printf("  Realm:     %s\n", cfg.Realm)
	fmt.Printf("  TLS Mode:  %s\n", cfg.TLSMode)
	if !cfg.Roles.Bootstrap {
		fmt.Printf("  Bootstrap Peers: %s\n", strings.Join(cfg.BootstrapPeers, ", "))
	}
	fmt.Println()

	fmt.Print("Save this configuration? [Y/n]: ")
	input, _ = reader.ReadString('\n')
	input = strings.TrimSpace(strings.ToLower(input))
	if input == "n" || input == "no" {
		fmt.Println("Configuration cancelled.")
		return nil
	}

	if err := writeMeshConfig(&cfg); err != nil {
		return fmt.Errorf("failed to write config: %v", err)
	}

	fmt.Println("\n\033[1;32m✓\033[0m Mesh configuration saved!")

	fmt.Print("\nApply configuration now? [Y/n]: ")
	input, _ = reader.ReadString('\n')
	input = strings.TrimSpace(strings.ToLower(input))
	if input != "n" && input != "no" {
		return applyAction(c)
	}

	fmt.Println("  Run 'maculaos mesh apply' to apply changes later")
	return nil
}

func applyAction(c *cli.Context) error {
	fmt.Println("\033[1;36m=== Applying Mesh Configuration ===\033[0m")

	cfg, err := readMeshConfig()
	if err != nil {
		return fmt.Errorf("no mesh configuration found: run 'maculaos mesh wizard' first")
	}

	// Always ensure macula-mesh service exists and is enabled
	fmt.Println("  → Configuring macula-mesh service...")
	if err := enableService("macula-mesh"); err != nil {
		logrus.Warnf("failed to enable macula-mesh: %v", err)
	}

	// Configure bootstrap service
	if cfg.Roles.Bootstrap {
		fmt.Println("  → Enabling macula-bootstrap service...")
		if err := enableService("macula-bootstrap"); err != nil {
			logrus.Warnf("failed to enable macula-bootstrap: %v", err)
		}
	} else {
		fmt.Println("  → Disabling macula-bootstrap service...")
		disableService("macula-bootstrap")
	}

	// Configure gateway service
	if cfg.Roles.Gateway {
		fmt.Println("  → Enabling macula-gateway service...")
		if err := enableService("macula-gateway"); err != nil {
			logrus.Warnf("failed to enable macula-gateway: %v", err)
		}
	} else {
		fmt.Println("  → Disabling macula-gateway service...")
		disableService("macula-gateway")
	}

	// Configure firewall rules
	fmt.Println("  → Configuring firewall rules...")
	if err := configureFirewall(cfg); err != nil {
		logrus.Warnf("failed to configure firewall: %v", err)
	}

	// Restart services
	fmt.Println("  → Restarting mesh services...")
	restartService("macula-mesh")
	if cfg.Roles.Bootstrap {
		restartService("macula-bootstrap")
	}
	if cfg.Roles.Gateway {
		restartService("macula-gateway")
	}

	fmt.Println("\n\033[1;32m✓\033[0m Mesh configuration applied!")
	return nil
}

// MeshConfig mirrors the config package type for local use
type MeshConfig struct {
	Roles          MeshRoles `yaml:"roles,omitempty"`
	BootstrapPeers []string  `yaml:"bootstrapPeers,omitempty"`
	Realm          string    `yaml:"realm,omitempty"`
	TLSMode        string    `yaml:"tlsMode,omitempty"`
}

type MeshRoles struct {
	Bootstrap bool `yaml:"bootstrap,omitempty"`
	Gateway   bool `yaml:"gateway,omitempty"`
}

func readMeshConfig() (*MeshConfig, error) {
	data, err := os.ReadFile("/var/lib/maculaos/mesh.yaml")
	if err != nil {
		return nil, err
	}
	var cfg MeshConfig
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, err
	}
	return &cfg, nil
}

func writeMeshConfig(cfg *MeshConfig) error {
	if err := os.MkdirAll("/var/lib/maculaos", 0755); err != nil {
		return err
	}
	data, err := yaml.Marshal(cfg)
	if err != nil {
		return err
	}
	return os.WriteFile("/var/lib/maculaos/mesh.yaml", data, 0644)
}

func checkService(name string) {
	cmd := exec.Command("rc-service", name, "status")
	if err := cmd.Run(); err != nil {
		fmt.Printf("  \033[1;31m✗\033[0m %s: not running\n", name)
	} else {
		fmt.Printf("  \033[1;32m✓\033[0m %s: running\n", name)
	}
}

func enableService(name string) error {
	// Add to default runlevel
	cmd := exec.Command("rc-update", "add", name, "default")
	return cmd.Run()
}

func disableService(name string) error {
	// Remove from default runlevel
	cmd := exec.Command("rc-update", "del", name, "default")
	return cmd.Run()
}

func restartService(name string) error {
	cmd := exec.Command("rc-service", name, "restart")
	return cmd.Run()
}

func configureFirewall(cfg *MeshConfig) error {
	// Open necessary ports based on roles
	ports := []string{}

	// Mesh always needs outbound
	// Bootstrap needs 443 for QUIC
	if cfg.Roles.Bootstrap {
		ports = append(ports, "443/tcp", "443/udp")
	}

	// Gateway needs 443 and 80
	if cfg.Roles.Gateway {
		ports = append(ports, "443/tcp", "443/udp", "80/tcp")
	}

	// Apply firewall rules using iptables
	for _, port := range ports {
		parts := strings.Split(port, "/")
		if len(parts) != 2 {
			continue
		}
		portNum, proto := parts[0], parts[1]

		cmd := exec.Command("iptables", "-A", "INPUT", "-p", proto, "--dport", portNum, "-j", "ACCEPT")
		if err := cmd.Run(); err != nil {
			logrus.Warnf("failed to add iptables rule for %s: %v", port, err)
		}
	}

	return nil
}

func boolToStatus(b bool) string {
	if b {
		return "\033[1;32menabled\033[0m"
	}
	return "\033[1;90mdisabled\033[0m"
}
