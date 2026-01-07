package encrypt

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
	force    bool
	password string
	keyFile  string
)

// Command returns the `encrypt` sub-command for disk encryption
func Command() cli.Command {
	return cli.Command{
		Name:  "encrypt",
		Usage: "manage disk encryption (LUKS)",
		Description: `
Manage LUKS disk encryption for the MaculaOS data partition.

Encryption protects data at rest - if the disk is stolen, the data
cannot be read without the encryption key.

IMPORTANT: Store your passphrase securely. If lost, data cannot be recovered!`,
		Subcommands: []cli.Command{
			{
				Name:  "status",
				Usage: "show encryption status",
				Action: statusAction,
			},
			{
				Name:  "enable",
				Usage: "enable encryption on data partition",
				Flags: []cli.Flag{
					cli.BoolFlag{
						Name:        "force,f",
						Usage:       "skip confirmation",
						Destination: &force,
					},
					cli.StringFlag{
						Name:        "key-file",
						Usage:       "path to key file (optional, instead of passphrase)",
						Destination: &keyFile,
					},
				},
				Action: enableAction,
			},
			{
				Name:  "change-passphrase",
				Usage: "change encryption passphrase",
				Action: changePassphraseAction,
			},
			{
				Name:  "add-key",
				Usage: "add an additional key/passphrase",
				Flags: []cli.Flag{
					cli.StringFlag{
						Name:        "key-file",
						Usage:       "path to new key file",
						Destination: &keyFile,
					},
				},
				Action: addKeyAction,
			},
		},
		Action: statusAction,
	}
}

func statusAction(c *cli.Context) error {
	fmt.Println("\033[1;36m=== Encryption Status ===\033[0m\n")

	// Check if cryptsetup is available
	if _, err := exec.LookPath("cryptsetup"); err != nil {
		fmt.Println("  \033[1;31m✗\033[0m cryptsetup not installed")
		fmt.Println("    Encryption support requires cryptsetup package")
		return nil
	}
	fmt.Println("  \033[1;32m✓\033[0m cryptsetup available")

	// Check if data partition is encrypted
	dataPartition := findDataPartition()
	if dataPartition == "" {
		fmt.Println("  \033[1;33m!\033[0m Data partition not found")
		return nil
	}

	// Check if it's a LUKS device
	out, err := exec.Command("cryptsetup", "isLuks", dataPartition).CombinedOutput()
	if err != nil {
		fmt.Printf("  \033[1;33m!\033[0m Data partition (%s) is NOT encrypted\n", dataPartition)
		fmt.Println("\n  To enable encryption, run:")
		fmt.Println("    sudo maculaos encrypt enable")
		return nil
	}

	fmt.Printf("  \033[1;32m✓\033[0m Data partition (%s) is encrypted\n", dataPartition)

	// Show LUKS info
	if verbose, err := exec.Command("cryptsetup", "luksDump", dataPartition).Output(); err == nil {
		lines := strings.Split(string(verbose), "\n")
		for _, line := range lines {
			if strings.HasPrefix(line, "Version:") ||
				strings.HasPrefix(line, "Cipher:") ||
				strings.HasPrefix(line, "Key Slots:") ||
				strings.Contains(line, "ENABLED") {
				fmt.Printf("    %s\n", strings.TrimSpace(line))
			}
		}
	}

	// Check if unlocked
	if _, err := os.Stat("/dev/mapper/maculaos-data"); err == nil {
		fmt.Println("  \033[1;32m✓\033[0m Encrypted volume is unlocked")
	} else {
		fmt.Println("  \033[1;31m✗\033[0m Encrypted volume is locked")
	}

	_ = out // silence unused warning
	return nil
}

func enableAction(c *cli.Context) error {
	// Check root
	if os.Geteuid() != 0 {
		return fmt.Errorf("encryption requires root privileges")
	}

	// Check cryptsetup
	if _, err := exec.LookPath("cryptsetup"); err != nil {
		return fmt.Errorf("cryptsetup not installed - please install it first")
	}

	dataPartition := findDataPartition()
	if dataPartition == "" {
		return fmt.Errorf("could not find data partition (MACULAOS_STATE)")
	}

	// Check if already encrypted
	if err := exec.Command("cryptsetup", "isLuks", dataPartition).Run(); err == nil {
		return fmt.Errorf("partition %s is already encrypted", dataPartition)
	}

	// Warning and confirmation
	if !force {
		fmt.Println("\033[1;31m╔══════════════════════════════════════════════════════════════╗")
		fmt.Println("║                    ⚠️  WARNING ⚠️                              ║")
		fmt.Println("║                                                              ║")
		fmt.Println("║  Enabling encryption will:                                   ║")
		fmt.Println("║    1. Backup existing data                                   ║")
		fmt.Println("║    2. Format the partition with LUKS encryption              ║")
		fmt.Println("║    3. Restore data to encrypted volume                       ║")
		fmt.Println("║                                                              ║")
		fmt.Println("║  You will need to enter a passphrase at boot.                ║")
		fmt.Println("║                                                              ║")
		fmt.Println("║  ⚠️  IF YOU FORGET YOUR PASSPHRASE, DATA CANNOT BE RECOVERED! ║")
		fmt.Println("╚══════════════════════════════════════════════════════════════╝\033[0m")
		fmt.Println()

		fmt.Print("Type 'ENCRYPT' to proceed: ")
		reader := bufio.NewReader(os.Stdin)
		input, _ := reader.ReadString('\n')
		if strings.TrimSpace(input) != "ENCRYPT" {
			fmt.Println("Encryption cancelled.")
			return nil
		}
	}

	// Get passphrase
	var passphrase string
	if keyFile != "" {
		// Use key file
		if _, err := os.Stat(keyFile); err != nil {
			return fmt.Errorf("key file not found: %s", keyFile)
		}
	} else {
		// Get passphrase interactively
		fmt.Print("\nEnter encryption passphrase: ")
		reader := bufio.NewReader(os.Stdin)
		pass1, _ := reader.ReadString('\n')
		pass1 = strings.TrimSpace(pass1)

		fmt.Print("Confirm passphrase: ")
		pass2, _ := reader.ReadString('\n')
		pass2 = strings.TrimSpace(pass2)

		if pass1 != pass2 {
			return fmt.Errorf("passphrases do not match")
		}
		if len(pass1) < 8 {
			return fmt.Errorf("passphrase must be at least 8 characters")
		}
		passphrase = pass1
	}

	fmt.Println("\n\033[1;33mStarting encryption process...\033[0m")

	// 1. Create backup directory
	backupDir := "/tmp/maculaos-data-backup"
	fmt.Println("  → Creating backup of existing data...")
	os.MkdirAll(backupDir, 0700)

	// 2. Copy data to backup
	if err := exec.Command("rsync", "-a", "/var/lib/maculaos/", backupDir+"/").Run(); err != nil {
		logrus.Warnf("backup warning: %v", err)
	}

	// 3. Unmount if mounted
	fmt.Println("  → Unmounting data partition...")
	exec.Command("umount", "/var/lib/maculaos").Run()

	// 4. Format with LUKS
	fmt.Println("  → Formatting with LUKS encryption...")
	var cmd *exec.Cmd
	if keyFile != "" {
		cmd = exec.Command("cryptsetup", "luksFormat", "--type", "luks2",
			"--cipher", "aes-xts-plain64", "--key-size", "512",
			"--hash", "sha256", "--iter-time", "2000",
			"--key-file", keyFile, dataPartition)
	} else {
		cmd = exec.Command("cryptsetup", "luksFormat", "--type", "luks2",
			"--cipher", "aes-xts-plain64", "--key-size", "512",
			"--hash", "sha256", "--iter-time", "2000",
			dataPartition)
		cmd.Stdin = strings.NewReader(passphrase + "\n")
	}
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("LUKS format failed: %v", err)
	}

	// 5. Open encrypted volume
	fmt.Println("  → Opening encrypted volume...")
	if keyFile != "" {
		cmd = exec.Command("cryptsetup", "open", "--key-file", keyFile, dataPartition, "maculaos-data")
	} else {
		cmd = exec.Command("cryptsetup", "open", dataPartition, "maculaos-data")
		cmd.Stdin = strings.NewReader(passphrase + "\n")
	}
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to open encrypted volume: %v", err)
	}

	// 6. Create filesystem
	fmt.Println("  → Creating filesystem...")
	if err := exec.Command("mkfs.ext4", "-L", "MACULAOS_DATA", "/dev/mapper/maculaos-data").Run(); err != nil {
		return fmt.Errorf("failed to create filesystem: %v", err)
	}

	// 7. Mount and restore data
	fmt.Println("  → Mounting encrypted volume...")
	os.MkdirAll("/var/lib/maculaos", 0755)
	if err := exec.Command("mount", "/dev/mapper/maculaos-data", "/var/lib/maculaos").Run(); err != nil {
		return fmt.Errorf("failed to mount: %v", err)
	}

	fmt.Println("  → Restoring data...")
	if err := exec.Command("rsync", "-a", backupDir+"/", "/var/lib/maculaos/").Run(); err != nil {
		logrus.Warnf("restore warning: %v", err)
	}

	// 8. Update crypttab
	fmt.Println("  → Updating system configuration...")
	uuid, _ := exec.Command("blkid", "-s", "UUID", "-o", "value", dataPartition).Output()
	crypttabEntry := fmt.Sprintf("maculaos-data UUID=%s none luks\n", strings.TrimSpace(string(uuid)))
	os.WriteFile("/etc/crypttab", []byte(crypttabEntry), 0644)

	// 9. Cleanup
	os.RemoveAll(backupDir)

	fmt.Println("\n\033[1;32m✓ Encryption enabled successfully!\033[0m")
	fmt.Println("\nIMPORTANT: You will need to enter your passphrase at every boot.")
	fmt.Println("           Store your passphrase securely - it cannot be recovered!")

	return nil
}

func changePassphraseAction(c *cli.Context) error {
	if os.Geteuid() != 0 {
		return fmt.Errorf("requires root privileges")
	}

	dataPartition := findDataPartition()
	if dataPartition == "" {
		return fmt.Errorf("data partition not found")
	}

	if err := exec.Command("cryptsetup", "isLuks", dataPartition).Run(); err != nil {
		return fmt.Errorf("partition is not encrypted")
	}

	fmt.Println("Changing LUKS passphrase...")
	cmd := exec.Command("cryptsetup", "luksChangeKey", dataPartition)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func addKeyAction(c *cli.Context) error {
	if os.Geteuid() != 0 {
		return fmt.Errorf("requires root privileges")
	}

	dataPartition := findDataPartition()
	if dataPartition == "" {
		return fmt.Errorf("data partition not found")
	}

	if err := exec.Command("cryptsetup", "isLuks", dataPartition).Run(); err != nil {
		return fmt.Errorf("partition is not encrypted")
	}

	fmt.Println("Adding new key to LUKS volume...")
	var cmd *exec.Cmd
	if keyFile != "" {
		cmd = exec.Command("cryptsetup", "luksAddKey", dataPartition, keyFile)
	} else {
		cmd = exec.Command("cryptsetup", "luksAddKey", dataPartition)
	}
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func findDataPartition() string {
	// Try to find MACULAOS_STATE partition
	out, err := exec.Command("blkid", "-L", "MACULAOS_STATE").Output()
	if err == nil {
		return strings.TrimSpace(string(out))
	}

	// Fallback: look for common locations
	candidates := []string{
		"/dev/sda2",
		"/dev/nvme0n1p2",
		"/dev/mmcblk0p2",
	}
	for _, dev := range candidates {
		if _, err := os.Stat(dev); err == nil {
			return dev
		}
	}

	return ""
}
