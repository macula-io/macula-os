package backup

import (
	"archive/tar"
	"compress/gzip"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/sirupsen/logrus"
	"github.com/urfave/cli"
	"gopkg.in/yaml.v2"
)

var (
	backupTarget    string
	restoreSource   string
	restoreDate     string
	restoreLatest   bool
	dryRun          bool
	includeUserData bool
)

const (
	backupDir    = "/var/lib/maculaos/backups"
	configDir    = "/var/lib/maculaos"
	defaultPaths = "/var/lib/maculaos"
)

// Command returns the `backup` sub-command for backup and restore
func Command() cli.Command {
	return cli.Command{
		Name:  "backup",
		Usage: "backup and restore system state",
		Description: `
Backup and restore MaculaOS configuration and state.

By default, backups include:
  - /var/lib/maculaos/ (config, credentials, pairing)

Optionally include:
  - User data from /var/lib/data/

Backup targets:
  - local: Store backups in /var/lib/maculaos/backups/
  - usb:   Store backups on mounted USB drive
  - s3:    Store backups in S3-compatible storage (requires config)`,
		Subcommands: []cli.Command{
			{
				Name:  "create",
				Usage: "create a new backup",
				Flags: []cli.Flag{
					cli.StringFlag{
						Name:        "target,t",
						Usage:       "backup target: local, usb, s3",
						Value:       "local",
						Destination: &backupTarget,
					},
					cli.BoolFlag{
						Name:        "include-data",
						Usage:       "include user data from /var/lib/data/",
						Destination: &includeUserData,
					},
					cli.BoolFlag{
						Name:        "dry-run",
						Usage:       "show what would be backed up without creating backup",
						Destination: &dryRun,
					},
				},
				Action: createAction,
			},
			{
				Name:  "restore",
				Usage: "restore from a backup",
				Flags: []cli.Flag{
					cli.StringFlag{
						Name:        "from,f",
						Usage:       "restore source: local, usb, s3",
						Value:       "local",
						Destination: &restoreSource,
					},
					cli.StringFlag{
						Name:        "date,d",
						Usage:       "restore from specific date (YYYY-MM-DD)",
						Destination: &restoreDate,
					},
					cli.BoolFlag{
						Name:        "latest",
						Usage:       "restore from most recent backup",
						Destination: &restoreLatest,
					},
					cli.BoolFlag{
						Name:        "dry-run",
						Usage:       "show what would be restored without restoring",
						Destination: &dryRun,
					},
				},
				Action: restoreAction,
			},
			{
				Name:   "list",
				Usage:  "list available backups",
				Action: listAction,
			},
			{
				Name:      "delete",
				Usage:     "delete a backup",
				ArgsUsage: "<backup-name>",
				Action:    deleteAction,
			},
			{
				Name:   "status",
				Usage:  "show backup configuration and schedule",
				Action: statusAction,
			},
			{
				Name:  "schedule",
				Usage: "configure automatic backups",
				Flags: []cli.Flag{
					cli.StringFlag{
						Name:  "cron",
						Usage: "cron expression for backup schedule",
						Value: "0 2 * * *",
					},
					cli.IntFlag{
						Name:  "retention",
						Usage: "number of backups to keep",
						Value: 7,
					},
				},
				Action: scheduleAction,
			},
		},
		Action: statusAction,
	}
}

// BackupConfig represents backup configuration
type BackupConfig struct {
	Enabled   bool     `yaml:"enabled,omitempty"`
	Schedule  string   `yaml:"schedule,omitempty"`
	Retention int      `yaml:"retention,omitempty"`
	Target    string   `yaml:"target,omitempty"`
	Include   []string `yaml:"include,omitempty"`
	Exclude   []string `yaml:"exclude,omitempty"`
}

func statusAction(c *cli.Context) error {
	fmt.Println("\033[1;36m=== Backup Status ===\033[0m\n")

	cfg, err := readBackupConfig()
	if err != nil {
		fmt.Println("  \033[1;33m!\033[0m Automatic backups not configured")
		fmt.Println("    Configure with: maculaos backup schedule")
	} else {
		if cfg.Enabled {
			fmt.Println("  \033[1;32m✓\033[0m Automatic backups: enabled")
			fmt.Printf("    Schedule: %s\n", cfg.Schedule)
			fmt.Printf("    Retention: %d backups\n", cfg.Retention)
			fmt.Printf("    Target: %s\n", cfg.Target)
		} else {
			fmt.Println("  \033[1;90m○\033[0m Automatic backups: disabled")
		}
	}

	// Show local backups
	fmt.Println("\n\033[1;36m=== Local Backups ===\033[0m\n")
	backups, err := listLocalBackups()
	if err != nil || len(backups) == 0 {
		fmt.Println("  No local backups found")
	} else {
		for _, b := range backups {
			fmt.Printf("  • %s\n", b)
		}
	}

	// Show backup paths
	fmt.Println("\n\033[1;36m=== Backup Paths ===\033[0m\n")
	fmt.Println("  Default paths included:")
	fmt.Println("    • /var/lib/maculaos/ (config, credentials)")
	fmt.Println("  Optional paths:")
	fmt.Println("    • /var/lib/data/ (user data, use --include-data)")
	fmt.Println("  Excluded paths:")
	fmt.Println("    • /var/lib/rancher/k3s/agent/containerd/ (container layers)")

	return nil
}

func createAction(c *cli.Context) error {
	fmt.Println("\033[1;36m=== Creating Backup ===\033[0m\n")

	// Determine paths to backup
	paths := []string{"/var/lib/maculaos"}
	if includeUserData {
		if _, err := os.Stat("/var/lib/data"); err == nil {
			paths = append(paths, "/var/lib/data")
		}
	}

	// Exclusions
	excludes := []string{
		"/var/lib/maculaos/backups",
		"*.log",
		"*.tmp",
	}

	if dryRun {
		fmt.Println("  Dry run - would backup:")
		for _, p := range paths {
			fmt.Printf("    • %s\n", p)
		}
		fmt.Println("\n  Excluded patterns:")
		for _, e := range excludes {
			fmt.Printf("    • %s\n", e)
		}
		return nil
	}

	// Create backup filename
	timestamp := time.Now().Format("2006-01-02_15-04-05")
	hostname, _ := os.Hostname()
	backupName := fmt.Sprintf("maculaos-%s-%s.tar.gz", hostname, timestamp)

	// Ensure backup directory exists
	if err := os.MkdirAll(backupDir, 0700); err != nil {
		return fmt.Errorf("failed to create backup directory: %v", err)
	}

	backupPath := filepath.Join(backupDir, backupName)

	fmt.Printf("  → Backing up to: %s\n", backupPath)

	// Create tarball
	if err := createTarball(backupPath, paths, excludes); err != nil {
		return fmt.Errorf("backup failed: %v", err)
	}

	// Get file size
	info, _ := os.Stat(backupPath)
	size := formatSize(info.Size())

	fmt.Printf("  → Backup size: %s\n", size)

	// Handle target-specific actions
	switch backupTarget {
	case "usb":
		if err := copyToUSB(backupPath); err != nil {
			logrus.Warnf("failed to copy to USB: %v", err)
		}
	case "s3":
		if err := uploadToS3(backupPath); err != nil {
			logrus.Warnf("failed to upload to S3: %v", err)
		}
	}

	// Apply retention policy
	if err := applyRetention(); err != nil {
		logrus.Warnf("retention cleanup failed: %v", err)
	}

	fmt.Println("\n\033[1;32m✓\033[0m Backup created successfully!")
	return nil
}

func restoreAction(c *cli.Context) error {
	fmt.Println("\033[1;36m=== Restoring Backup ===\033[0m\n")

	var backupPath string

	switch restoreSource {
	case "local":
		backups, err := listLocalBackups()
		if err != nil || len(backups) == 0 {
			return fmt.Errorf("no local backups found")
		}

		if restoreLatest {
			backupPath = filepath.Join(backupDir, backups[len(backups)-1])
		} else if restoreDate != "" {
			for _, b := range backups {
				if strings.Contains(b, restoreDate) {
					backupPath = filepath.Join(backupDir, b)
					break
				}
			}
			if backupPath == "" {
				return fmt.Errorf("no backup found for date: %s", restoreDate)
			}
		} else {
			// Interactive selection
			fmt.Println("  Available backups:")
			for i, b := range backups {
				fmt.Printf("    %d. %s\n", i+1, b)
			}
			return fmt.Errorf("specify --latest or --date to select backup")
		}

	case "usb":
		backupPath, _ = findUSBBackup()
		if backupPath == "" {
			return fmt.Errorf("no backup found on USB")
		}

	case "s3":
		return fmt.Errorf("S3 restore not yet implemented")

	default:
		return fmt.Errorf("unknown restore source: %s", restoreSource)
	}

	fmt.Printf("  → Restoring from: %s\n", backupPath)

	if dryRun {
		fmt.Println("\n  Dry run - would restore files from backup")
		return nil
	}

	// Confirm
	fmt.Println("\n  \033[1;33mWarning:\033[0m This will overwrite existing configuration!")
	fmt.Print("  Continue? [y/N]: ")
	var confirm string
	fmt.Scanln(&confirm)
	if strings.ToLower(confirm) != "y" {
		fmt.Println("  Restore cancelled")
		return nil
	}

	// Extract tarball
	if err := extractTarball(backupPath, "/"); err != nil {
		return fmt.Errorf("restore failed: %v", err)
	}

	fmt.Println("\n\033[1;32m✓\033[0m Restore completed!")
	fmt.Println("  \033[1;33mNote:\033[0m You may need to reboot for all changes to take effect")

	return nil
}

func listAction(c *cli.Context) error {
	fmt.Println("\033[1;36m=== Available Backups ===\033[0m\n")

	// Local backups
	fmt.Println("  \033[1;36mLocal:\033[0m")
	backups, err := listLocalBackups()
	if err != nil || len(backups) == 0 {
		fmt.Println("    No local backups")
	} else {
		for _, b := range backups {
			info, _ := os.Stat(filepath.Join(backupDir, b))
			size := formatSize(info.Size())
			fmt.Printf("    • %s (%s)\n", b, size)
		}
	}

	// USB backups
	fmt.Println("\n  \033[1;36mUSB:\033[0m")
	if usbPath, _ := findUSBBackup(); usbPath != "" {
		fmt.Printf("    • %s\n", filepath.Base(usbPath))
	} else {
		fmt.Println("    No USB backups found")
	}

	return nil
}

func deleteAction(c *cli.Context) error {
	if c.NArg() < 1 {
		return fmt.Errorf("usage: maculaos backup delete <backup-name>")
	}

	name := c.Args().Get(0)
	backupPath := filepath.Join(backupDir, name)

	if _, err := os.Stat(backupPath); os.IsNotExist(err) {
		return fmt.Errorf("backup not found: %s", name)
	}

	if err := os.Remove(backupPath); err != nil {
		return fmt.Errorf("failed to delete backup: %v", err)
	}

	fmt.Printf("\033[1;32m✓\033[0m Deleted backup: %s\n", name)
	return nil
}

func scheduleAction(c *cli.Context) error {
	cfg := &BackupConfig{
		Enabled:   true,
		Schedule:  c.String("cron"),
		Retention: c.Int("retention"),
		Target:    "local",
		Include:   []string{"/var/lib/maculaos"},
		Exclude:   []string{"/var/lib/maculaos/backups"},
	}

	if err := writeBackupConfig(cfg); err != nil {
		return fmt.Errorf("failed to save config: %v", err)
	}

	// Install cron job
	cronEntry := fmt.Sprintf("%s root /usr/bin/maculaos backup create --target=local\n", cfg.Schedule)
	if err := os.WriteFile("/etc/cron.d/maculaos-backup", []byte(cronEntry), 0644); err != nil {
		logrus.Warnf("failed to install cron job: %v", err)
	}

	fmt.Println("\033[1;32m✓\033[0m Backup schedule configured!")
	fmt.Printf("  Schedule: %s\n", cfg.Schedule)
	fmt.Printf("  Retention: %d backups\n", cfg.Retention)
	fmt.Println("  Cron job installed to /etc/cron.d/maculaos-backup")

	return nil
}

// Helper functions

func createTarball(dest string, paths []string, excludes []string) error {
	file, err := os.Create(dest)
	if err != nil {
		return err
	}
	defer file.Close()

	gzw := gzip.NewWriter(file)
	defer gzw.Close()

	tw := tar.NewWriter(gzw)
	defer tw.Close()

	for _, path := range paths {
		err := filepath.Walk(path, func(file string, fi os.FileInfo, err error) error {
			if err != nil {
				return nil // Skip files we can't read
			}

			// Check exclusions
			for _, exclude := range excludes {
				if strings.Contains(file, exclude) || matchGlob(filepath.Base(file), exclude) {
					return nil
				}
			}

			header, err := tar.FileInfoHeader(fi, file)
			if err != nil {
				return nil
			}

			header.Name = file

			if err := tw.WriteHeader(header); err != nil {
				return err
			}

			if !fi.IsDir() {
				data, err := os.Open(file)
				if err != nil {
					return nil
				}
				defer data.Close()
				if _, err := io.Copy(tw, data); err != nil {
					return err
				}
			}
			return nil
		})
		if err != nil {
			return err
		}
	}

	return nil
}

func extractTarball(src, dest string) error {
	file, err := os.Open(src)
	if err != nil {
		return err
	}
	defer file.Close()

	gzr, err := gzip.NewReader(file)
	if err != nil {
		return err
	}
	defer gzr.Close()

	tr := tar.NewReader(gzr)

	for {
		header, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}

		target := filepath.Join(dest, header.Name)

		switch header.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(target, 0755); err != nil {
				return err
			}
		case tar.TypeReg:
			if err := os.MkdirAll(filepath.Dir(target), 0755); err != nil {
				return err
			}
			f, err := os.OpenFile(target, os.O_CREATE|os.O_RDWR, os.FileMode(header.Mode))
			if err != nil {
				return err
			}
			if _, err := io.Copy(f, tr); err != nil {
				f.Close()
				return err
			}
			f.Close()
		}
	}

	return nil
}

func listLocalBackups() ([]string, error) {
	entries, err := os.ReadDir(backupDir)
	if err != nil {
		return nil, err
	}

	var backups []string
	for _, e := range entries {
		if strings.HasSuffix(e.Name(), ".tar.gz") {
			backups = append(backups, e.Name())
		}
	}

	sort.Strings(backups)
	return backups, nil
}

func copyToUSB(src string) error {
	// Find mounted USB
	usbMount := findUSBMount()
	if usbMount == "" {
		return fmt.Errorf("no USB drive mounted")
	}

	dest := filepath.Join(usbMount, "maculaos-backups", filepath.Base(src))
	if err := os.MkdirAll(filepath.Dir(dest), 0755); err != nil {
		return err
	}

	cmd := exec.Command("cp", src, dest)
	return cmd.Run()
}

func findUSBMount() string {
	// Check common USB mount points
	candidates := []string{"/mnt/usb", "/media/usb", "/run/media"}
	for _, c := range candidates {
		if _, err := os.Stat(c); err == nil {
			return c
		}
	}
	return ""
}

func findUSBBackup() (string, error) {
	usbMount := findUSBMount()
	if usbMount == "" {
		return "", fmt.Errorf("no USB mounted")
	}

	backupDir := filepath.Join(usbMount, "maculaos-backups")
	entries, err := os.ReadDir(backupDir)
	if err != nil {
		return "", err
	}

	for _, e := range entries {
		if strings.HasSuffix(e.Name(), ".tar.gz") {
			return filepath.Join(backupDir, e.Name()), nil
		}
	}

	return "", fmt.Errorf("no backup found")
}

func uploadToS3(src string) error {
	// TODO: Implement S3 upload using aws cli or native Go SDK
	return fmt.Errorf("S3 upload not yet implemented")
}

func applyRetention() error {
	cfg, err := readBackupConfig()
	if err != nil || cfg.Retention <= 0 {
		return nil
	}

	backups, err := listLocalBackups()
	if err != nil {
		return err
	}

	// Delete oldest backups if over retention limit
	for len(backups) > cfg.Retention {
		oldest := backups[0]
		if err := os.Remove(filepath.Join(backupDir, oldest)); err != nil {
			logrus.Warnf("failed to delete old backup %s: %v", oldest, err)
		}
		backups = backups[1:]
	}

	return nil
}

func readBackupConfig() (*BackupConfig, error) {
	data, err := os.ReadFile("/var/lib/maculaos/backup.yaml")
	if err != nil {
		return nil, err
	}
	var cfg BackupConfig
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, err
	}
	return &cfg, nil
}

func writeBackupConfig(cfg *BackupConfig) error {
	if err := os.MkdirAll("/var/lib/maculaos", 0755); err != nil {
		return err
	}
	data, err := yaml.Marshal(cfg)
	if err != nil {
		return err
	}
	return os.WriteFile("/var/lib/maculaos/backup.yaml", data, 0644)
}

func formatSize(bytes int64) string {
	const unit = 1024
	if bytes < unit {
		return fmt.Sprintf("%d B", bytes)
	}
	div, exp := int64(unit), 0
	for n := bytes / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(bytes)/float64(div), "KMGTPE"[exp])
}

func matchGlob(name, pattern string) bool {
	if strings.HasPrefix(pattern, "*") {
		return strings.HasSuffix(name, pattern[1:])
	}
	return name == pattern
}
