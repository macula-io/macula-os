//! TUI view implementations.
//!
//! Each view handles a specific aspect of system management:
//! - Dashboard: System overview and quick stats
//! - Peers: Mesh peer connections
//! - Apps: Installed application management
//! - Logs: Real-time log viewer
//! - Config: System configuration

mod dashboard;
mod peers;
mod apps;
mod logs;

pub use dashboard::DashboardView;
pub use peers::PeersView;
pub use apps::AppsView;
pub use logs::LogsView;
