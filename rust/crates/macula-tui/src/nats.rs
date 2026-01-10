//! NATS client for MaculaOS TUI.
//!
//! Handles connection to the local NATS server and provides
//! channels for receiving status updates and sending commands.

use anyhow::{Context, Result};
use async_nats::Client;
use futures::StreamExt;
use serde::{Deserialize, Serialize};
use tokio::sync::mpsc;

/// NATS message topics.
pub mod topics {
    /// Node status updates (published by macula-node)
    pub const NODE_STATUS: &str = "macula.node.status";
    /// Peer discovery events
    pub const PEER_DISCOVERED: &str = "macula.mesh.peer.discovered";
    /// Peer disconnection events
    pub const PEER_DISCONNECTED: &str = "macula.mesh.peer.disconnected";
    /// Log messages from services
    pub const LOGS: &str = "macula.logs.>";
    /// App status changes
    pub const APP_STATUS: &str = "macula.apps.status";
    /// Commands topic (for sending commands)
    pub const COMMANDS: &str = "macula.commands";
}

/// Node status message.
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct NodeStatus {
    pub node_id: String,
    pub realm: String,
    pub uptime_secs: u64,
    pub peer_count: usize,
    pub cpu_percent: f32,
    pub memory_mb: u64,
    pub memory_total_mb: u64,
    pub disk_used_gb: f32,
    pub disk_total_gb: f32,
}

/// Peer information.
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct PeerInfo {
    pub node_id: String,
    pub address: String,
    pub latency_ms: Option<u32>,
    pub connected_at: Option<u64>,
}

/// Log entry from services.
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct LogEntry {
    pub timestamp: u64,
    pub level: String,
    pub service: String,
    pub message: String,
}

/// App status.
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct AppStatus {
    pub app_id: String,
    pub name: String,
    pub status: String, // running, stopped, error
    pub cpu_percent: Option<f32>,
    pub memory_mb: Option<u64>,
}

/// Command to send to the node.
#[derive(Debug, Clone, Serialize, Deserialize)]
#[serde(tag = "type")]
pub enum Command {
    #[serde(rename = "app.start")]
    StartApp { app_id: String },
    #[serde(rename = "app.stop")]
    StopApp { app_id: String },
    #[serde(rename = "app.restart")]
    RestartApp { app_id: String },
    #[serde(rename = "node.restart")]
    RestartNode,
}

/// Events received from NATS.
#[derive(Debug, Clone)]
pub enum NatsEvent {
    NodeStatus(NodeStatus),
    PeerDiscovered(PeerInfo),
    PeerDisconnected(String),
    Log(LogEntry),
    AppStatus(AppStatus),
    Connected,
    Disconnected,
    Error(String),
}

/// NATS connection manager.
pub struct NatsManager {
    client: Option<Client>,
    url: String,
}

impl NatsManager {
    pub fn new(url: &str) -> Self {
        Self {
            client: None,
            url: url.to_string(),
        }
    }

    /// Connect to NATS server and start receiving events.
    pub async fn connect(&mut self, tx: mpsc::Sender<NatsEvent>) -> Result<()> {
        let client = async_nats::connect(&self.url)
            .await
            .context("Failed to connect to NATS")?;

        self.client = Some(client.clone());
        tx.send(NatsEvent::Connected).await.ok();

        // Subscribe to status topics
        let mut status_sub = client
            .subscribe(topics::NODE_STATUS.to_string())
            .await
            .context("Failed to subscribe to node status")?;

        let mut peer_discovered_sub = client
            .subscribe(topics::PEER_DISCOVERED.to_string())
            .await
            .context("Failed to subscribe to peer discovered")?;

        let mut peer_disconnected_sub = client
            .subscribe(topics::PEER_DISCONNECTED.to_string())
            .await
            .context("Failed to subscribe to peer disconnected")?;

        let mut app_status_sub = client
            .subscribe(topics::APP_STATUS.to_string())
            .await
            .context("Failed to subscribe to app status")?;

        // Spawn tasks to handle subscriptions
        let tx1 = tx.clone();
        tokio::spawn(async move {
            while let Some(msg) = status_sub.next().await {
                if let Ok(status) = serde_json::from_slice::<NodeStatus>(&msg.payload) {
                    tx1.send(NatsEvent::NodeStatus(status)).await.ok();
                }
            }
        });

        let tx2 = tx.clone();
        tokio::spawn(async move {
            while let Some(msg) = peer_discovered_sub.next().await {
                if let Ok(peer) = serde_json::from_slice::<PeerInfo>(&msg.payload) {
                    tx2.send(NatsEvent::PeerDiscovered(peer)).await.ok();
                }
            }
        });

        let tx3 = tx.clone();
        tokio::spawn(async move {
            while let Some(msg) = peer_disconnected_sub.next().await {
                if let Ok(node_id) = String::from_utf8(msg.payload.to_vec()) {
                    tx3.send(NatsEvent::PeerDisconnected(node_id)).await.ok();
                }
            }
        });

        let tx4 = tx.clone();
        tokio::spawn(async move {
            while let Some(msg) = app_status_sub.next().await {
                if let Ok(status) = serde_json::from_slice::<AppStatus>(&msg.payload) {
                    tx4.send(NatsEvent::AppStatus(status)).await.ok();
                }
            }
        });

        Ok(())
    }

    /// Send a command to the node.
    pub async fn send_command(&self, command: Command) -> Result<()> {
        let client = self.client.as_ref().context("Not connected to NATS")?;
        let payload = serde_json::to_vec(&command)?;
        client
            .publish(topics::COMMANDS.to_string(), payload.into())
            .await
            .context("Failed to publish command")?;
        Ok(())
    }

    /// Check if connected.
    pub fn is_connected(&self) -> bool {
        self.client.is_some()
    }
}
