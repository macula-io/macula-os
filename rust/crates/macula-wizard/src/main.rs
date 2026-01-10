//! MaculaOS First-Boot Setup Wizard
//!
//! This wizard runs on first boot of MaculaOS, before k3s and other services start.
//! It guides the user through initial configuration:
//!
//! 1. Welcome / EULA
//! 2. Network configuration
//! 3. Mesh identity generation
//! 4. Realm and bootstrap configuration
//! 5. Portal pairing (optional)
//! 6. Summary and confirmation
//!
//! The wizard writes configuration to `/var/lib/maculaos/` and creates a
//! `.configured` marker file to indicate first-boot is complete.

mod app;
mod config;
mod steps;

use anyhow::Result;
use clap::Parser;
use tracing_subscriber::{layer::SubscriberExt, util::SubscriberInitExt};

/// MaculaOS First-Boot Setup Wizard
#[derive(Parser, Debug)]
#[command(name = "macula-wizard")]
#[command(about = "First-boot setup wizard for MaculaOS")]
#[command(version)]
struct Args {
    /// Skip the wizard and use defaults (for testing)
    #[arg(long)]
    auto: bool,

    /// Configuration directory (default: /var/lib/maculaos)
    #[arg(long, default_value = "/var/lib/maculaos")]
    config_dir: String,

    /// Portal URL for pairing (default: https://macula.io)
    #[arg(long, default_value = "https://macula.io")]
    portal_url: String,

    /// Enable debug logging
    #[arg(long)]
    debug: bool,
}

#[tokio::main]
async fn main() -> Result<()> {
    let args = Args::parse();

    // Initialize logging
    let filter = if args.debug { "debug" } else { "info" };
    tracing_subscriber::registry()
        .with(tracing_subscriber::fmt::layer())
        .with(tracing_subscriber::EnvFilter::new(filter))
        .init();

    tracing::info!("Starting MaculaOS Setup Wizard");

    // Check if already configured
    let marker_path = format!("{}/.configured", args.config_dir);
    if std::path::Path::new(&marker_path).exists() {
        tracing::info!("System already configured, exiting");
        println!("MaculaOS is already configured. Delete {} to re-run wizard.", marker_path);
        return Ok(());
    }

    // Run the wizard
    if args.auto {
        tracing::info!("Auto mode: using default configuration");
        config::write_defaults(&args.config_dir).await?;
    } else {
        app::run(&args.config_dir, &args.portal_url).await?;
    }

    tracing::info!("Setup complete");
    Ok(())
}
