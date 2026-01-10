//! Configuration file management for MaculaOS.

use anyhow::{Context, Result};
use ed25519_dalek::{SigningKey, VerifyingKey};
use rand::rngs::OsRng;
use serde::{Deserialize, Serialize};
use std::path::Path;
use tokio::fs;

/// MaculaOS system configuration.
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct MaculaConfig {
    /// Mesh realm (e.g., "io.macula")
    pub realm: String,

    /// Bootstrap peers (e.g., ["https://boot.macula.io:443"])
    pub bootstrap_peers: Vec<String>,

    /// Network configuration
    pub network: NetworkConfig,

    /// Node identity (DID derived from public key)
    pub node_id: Option<String>,
}

/// Network configuration.
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct NetworkConfig {
    /// Use DHCP or static IP
    pub dhcp: bool,

    /// Static IP address (if dhcp = false)
    pub ip_address: Option<String>,

    /// Gateway (if dhcp = false)
    pub gateway: Option<String>,

    /// DNS servers
    pub dns: Vec<String>,
}

impl Default for MaculaConfig {
    fn default() -> Self {
        Self {
            realm: "io.macula".to_string(),
            bootstrap_peers: vec!["https://boot.macula.io:443".to_string()],
            network: NetworkConfig::default(),
            node_id: None,
        }
    }
}

impl Default for NetworkConfig {
    fn default() -> Self {
        Self {
            dhcp: true,
            ip_address: None,
            gateway: None,
            dns: vec!["1.1.1.1".to_string(), "8.8.8.8".to_string()],
        }
    }
}

/// Mesh identity (Ed25519 keypair).
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct MeshIdentity {
    /// Base64-encoded public key
    pub public_key: String,

    /// Base64-encoded private key (sensitive!)
    pub private_key: String,

    /// DID derived from public key
    pub did: String,
}

impl MeshIdentity {
    /// Generate a new Ed25519 keypair.
    pub fn generate() -> Self {
        let mut csprng = OsRng;
        let signing_key = SigningKey::generate(&mut csprng);
        let verifying_key: VerifyingKey = signing_key.verifying_key();

        let public_key = base64::Engine::encode(
            &base64::engine::general_purpose::STANDARD,
            verifying_key.as_bytes(),
        );
        let private_key = base64::Engine::encode(
            &base64::engine::general_purpose::STANDARD,
            signing_key.as_bytes(),
        );

        // Generate DID from public key (simplified format)
        let did = format!(
            "did:macula:{}",
            &base64::Engine::encode(
                &base64::engine::general_purpose::URL_SAFE_NO_PAD,
                &verifying_key.as_bytes()[..16]
            )
        );

        Self {
            public_key,
            private_key,
            did,
        }
    }
}

/// Portal pairing token.
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct PortalToken {
    /// Refresh token for Portal API
    pub refresh_token: String,

    /// User display name
    pub user_name: String,

    /// Organization identity
    pub org_identity: String,
}

/// Write default configuration files.
pub async fn write_defaults(config_dir: &str) -> Result<()> {
    let config = MaculaConfig::default();
    let identity = MeshIdentity::generate();

    write_config(config_dir, &config, &identity, None).await
}

/// Write configuration files.
pub async fn write_config(
    config_dir: &str,
    config: &MaculaConfig,
    identity: &MeshIdentity,
    portal_token: Option<&PortalToken>,
) -> Result<()> {
    let config_path = Path::new(config_dir);

    // Create directory if it doesn't exist
    fs::create_dir_all(config_path)
        .await
        .context("Failed to create config directory")?;

    // Write main config
    let config_file = config_path.join("config.yaml");
    let config_yaml = serde_yaml::to_string(config)?;
    fs::write(&config_file, config_yaml)
        .await
        .context("Failed to write config.yaml")?;
    tracing::info!("Wrote {}", config_file.display());

    // Write identity (with restricted permissions)
    let identity_file = config_path.join("identity.json");
    let identity_json = serde_json::to_string_pretty(identity)?;
    fs::write(&identity_file, identity_json)
        .await
        .context("Failed to write identity.json")?;

    // Set restrictive permissions on identity file (Unix only)
    #[cfg(unix)]
    {
        use std::os::unix::fs::PermissionsExt;
        let mut perms = fs::metadata(&identity_file).await?.permissions();
        perms.set_mode(0o600);
        fs::set_permissions(&identity_file, perms).await?;
    }
    tracing::info!("Wrote {} (mode 0600)", identity_file.display());

    // Write portal token if present
    if let Some(token) = portal_token {
        let token_file = config_path.join("portal-token.json");
        let token_json = serde_json::to_string_pretty(token)?;
        fs::write(&token_file, token_json)
            .await
            .context("Failed to write portal-token.json")?;

        #[cfg(unix)]
        {
            use std::os::unix::fs::PermissionsExt;
            let mut perms = fs::metadata(&token_file).await?.permissions();
            perms.set_mode(0o600);
            fs::set_permissions(&token_file, perms).await?;
        }
        tracing::info!("Wrote {} (mode 0600)", token_file.display());
    }

    // Write configured marker
    let marker_file = config_path.join(".configured");
    fs::write(&marker_file, "")
        .await
        .context("Failed to write .configured marker")?;
    tracing::info!("Wrote {}", marker_file.display());

    Ok(())
}
