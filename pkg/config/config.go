package config

import (
	"fmt"
	"os"
	"strconv"
)

type Maculaos struct {
	DataSources    []string          `json:"dataSources,omitempty"`
	Modules        []string          `json:"modules,omitempty"`
	Sysctls        map[string]string `json:"sysctls,omitempty"`
	NTPServers     []string          `json:"ntpServers,omitempty"`
	DNSNameservers []string          `json:"dnsNameservers,omitempty"`
	Wifi           []Wifi            `json:"wifi,omitempty"`
	Password       string            `json:"password,omitempty"`
	ServerURL      string            `json:"serverUrl,omitempty"`
	Token          string            `json:"token,omitempty"`
	Labels         map[string]string `json:"labels,omitempty"`
	K3sArgs        []string          `json:"k3sArgs,omitempty"`
	Environment    map[string]string `json:"environment,omitempty"`
	Taints         []string          `json:"taints,omitempty"`
	Install        *Install          `json:"install,omitempty"`
	Mesh           *MeshConfig       `json:"mesh,omitempty"`
	GitOps         *GitOpsConfig     `json:"gitops,omitempty"`
	Health         *HealthConfig     `json:"health,omitempty"`
	Backup         *BackupConfig     `json:"backup,omitempty"`
}

// MeshConfig defines the Macula mesh role configuration
type MeshConfig struct {
	Roles          MeshRoles `json:"roles,omitempty"`
	BootstrapPeers []string  `json:"bootstrapPeers,omitempty"`
	Realm          string    `json:"realm,omitempty"`
	TLSMode        string    `json:"tlsMode,omitempty"` // "development" or "production"
}

// MeshRoles defines which mesh roles are enabled
type MeshRoles struct {
	Bootstrap bool `json:"bootstrap,omitempty"` // DHT bootstrap registry endpoint
	Gateway   bool `json:"gateway,omitempty"`   // NAT relay / API ingress gateway
}

// GitOpsConfig defines local GitOps server configuration
type GitOpsConfig struct {
	Enabled      bool   `json:"enabled,omitempty"`
	Server       string `json:"server,omitempty"`       // soft-serve, gitea, or git-daemon
	Port         int    `json:"port,omitempty"`         // SSH port for soft-serve
	DataPath     string `json:"dataPath,omitempty"`     // /var/lib/maculaos/git
	UpstreamSync *GitOpsSyncConfig `json:"upstreamSync,omitempty"`
}

// GitOpsSyncConfig defines upstream Git sync settings
type GitOpsSyncConfig struct {
	Enabled  bool   `json:"enabled,omitempty"`
	URL      string `json:"url,omitempty"`
	Interval string `json:"interval,omitempty"` // e.g., "5m"
}

// HealthConfig defines service health check configuration
type HealthConfig struct {
	Checks []HealthCheck `json:"checks,omitempty"`
}

// HealthCheck defines a single health check
type HealthCheck struct {
	Name             string `json:"name,omitempty"`
	Type             string `json:"type,omitempty"` // process, http, disk
	Process          string `json:"process,omitempty"`
	URL              string `json:"url,omitempty"`
	Path             string `json:"path,omitempty"`
	Interval         string `json:"interval,omitempty"`
	Timeout          string `json:"timeout,omitempty"`
	Threshold        string `json:"threshold,omitempty"`
	RestartOnFailure bool   `json:"restartOnFailure,omitempty"`
	MaxRestarts      int    `json:"maxRestarts,omitempty"`
	Action           string `json:"action,omitempty"` // alert, cleanup, restart
}

// BackupConfig defines backup and restore configuration
type BackupConfig struct {
	Enabled   bool     `json:"enabled,omitempty"`
	Schedule  string   `json:"schedule,omitempty"`  // Cron expression
	Retention int      `json:"retention,omitempty"` // Number of backups to keep
	Target    string   `json:"target,omitempty"`    // mesh, s3, local
	Include   []string `json:"include,omitempty"`
	Exclude   []string `json:"exclude,omitempty"`
	MeshBackup *MeshBackupConfig `json:"mesh,omitempty"`
	S3Backup   *S3BackupConfig   `json:"s3,omitempty"`
}

// MeshBackupConfig defines mesh-based backup settings
type MeshBackupConfig struct {
	ReplicationFactor int `json:"replicationFactor,omitempty"`
}

// S3BackupConfig defines S3-compatible backup settings
type S3BackupConfig struct {
	Endpoint string `json:"endpoint,omitempty"`
	Bucket   string `json:"bucket,omitempty"`
	Prefix   string `json:"prefix,omitempty"`
}

type Wifi struct {
	Name       string `json:"name,omitempty"`
	Passphrase string `json:"passphrase,omitempty"`
}

type Install struct {
	ForceEFI  bool   `json:"forceEfi,omitempty"`
	Device    string `json:"device,omitempty"`
	ConfigURL string `json:"configUrl,omitempty"`
	Silent    bool   `json:"silent,omitempty"`
	ISOURL    string `json:"isoUrl,omitempty"`
	PowerOff  bool   `json:"powerOff,omitempty"`
	NoFormat  bool   `json:"noFormat,omitempty"`
	Debug     bool   `json:"debug,omitempty"`
	TTY       string `json:"tty,omitempty"`
}

type CloudConfig struct {
	SSHAuthorizedKeys []string `json:"sshAuthorizedKeys,omitempty"`
	WriteFiles        []File   `json:"writeFiles,omitempty"`
	Hostname          string   `json:"hostname,omitempty"`
	Maculaos          Maculaos `json:"maculaos,omitempty"`
	Runcmd            []string `json:"runCmd,omitempty"`
	Bootcmd           []string `json:"bootCmd,omitempty"`
	Initcmd           []string `json:"initCmd,omitempty"`
}

type File struct {
	Encoding           string `json:"encoding"`
	Content            string `json:"content"`
	Owner              string `json:"owner"`
	Path               string `json:"path"`
	RawFilePermissions string `json:"permissions"`
}

func (f *File) Permissions() (os.FileMode, error) {
	if f.RawFilePermissions == "" {
		return os.FileMode(0644), nil
	}
	// parse string representation of file mode as integer
	perm, err := strconv.ParseInt(f.RawFilePermissions, 8, 32)
	if err != nil {
		return 0, fmt.Errorf("unable to parse file permissions %q as integer", f.RawFilePermissions)
	}
	return os.FileMode(perm), nil
}
