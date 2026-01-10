//! MaculaOS TUI Management Console
//!
//! This TUI provides real-time monitoring and management of a MaculaOS node.
//! It connects to the local NATS server to receive status updates and
//! send commands.
//!
//! Features:
//! - Dashboard: Overview of node status, mesh peers, resource usage
//! - Logs: Real-time log streaming from system services
//! - Apps: Manage installed applications (start, stop, restart)
//! - Config: View and edit system configuration
//!
//! Usage:
//!   macula-tui                    # Connect to default NATS (localhost:4222)
//!   macula-tui --nats <url>       # Connect to specific NATS server
//!   macula-tui --debug            # Enable debug logging

mod app;
mod nats;
mod views;

use anyhow::Result;
use clap::Parser;
use tracing_subscriber::{layer::SubscriberExt, util::SubscriberInitExt};

/// MaculaOS TUI Management Console
#[derive(Parser, Debug)]
#[command(name = "macula-tui")]
#[command(about = "TUI management console for MaculaOS")]
#[command(version)]
struct Args {
    /// NATS server URL
    #[arg(long, default_value = "nats://localhost:4222")]
    nats: String,

    /// Enable debug logging
    #[arg(long)]
    debug: bool,

    /// Configuration directory
    #[arg(long, default_value = "/var/lib/maculaos")]
    config_dir: String,
}

#[tokio::main]
async fn main() -> Result<()> {
    let args = Args::parse();

    // Initialize logging
    let filter = if args.debug { "debug" } else { "warn" };
    tracing_subscriber::registry()
        .with(tracing_subscriber::fmt::layer())
        .with(tracing_subscriber::EnvFilter::new(filter))
        .init();

    tracing::info!("Starting MaculaOS TUI");

    // Check if system is configured
    let marker_path = format!("{}/.configured", args.config_dir);
    if !std::path::Path::new(&marker_path).exists() {
        eprintln!("MaculaOS is not configured. Run macula-wizard first.");
        std::process::exit(1);
    }

    // Run the TUI
    app::run(&args.nats, &args.config_dir).await
}
