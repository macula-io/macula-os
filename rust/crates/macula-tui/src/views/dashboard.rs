//! Dashboard view - system overview.

use macula_tui_common::Theme;
use ratatui::{
    layout::{Constraint, Direction, Layout, Rect},
    style::{Modifier, Style},
    text::{Line, Span},
    widgets::{Block, Borders, Gauge, Paragraph},
    Frame,
};

use crate::nats::NodeStatus;

/// Dashboard view state.
#[derive(Default)]
pub struct DashboardView {
    pub node_status: Option<NodeStatus>,
    pub peer_count: usize,
    pub connected: bool,
}

impl DashboardView {
    pub fn new() -> Self {
        Self::default()
    }

    pub fn render(&self, frame: &mut Frame, area: Rect, theme: &Theme) {
        let chunks = Layout::default()
            .direction(Direction::Vertical)
            .constraints([
                Constraint::Length(5),  // Node info
                Constraint::Length(5),  // Resources
                Constraint::Min(3),     // Quick stats
            ])
            .split(area);

        self.render_node_info(frame, chunks[0], theme);
        self.render_resources(frame, chunks[1], theme);
        self.render_quick_stats(frame, chunks[2], theme);
    }

    fn render_node_info(&self, frame: &mut Frame, area: Rect, theme: &Theme) {
        let block = Block::default()
            .title("Node Info")
            .borders(Borders::ALL)
            .border_style(theme.border_style());

        let inner = block.inner(area);
        frame.render_widget(block, area);

        let (node_id, realm, uptime) = match &self.node_status {
            Some(status) => (
                status.node_id.clone(),
                status.realm.clone(),
                format_uptime(status.uptime_secs),
            ),
            None => (
                "Unknown".to_string(),
                "Unknown".to_string(),
                "Unknown".to_string(),
            ),
        };

        let connection_status = if self.connected {
            Span::styled("Connected", theme.success_style())
        } else {
            Span::styled("Disconnected", theme.error_style())
        };

        let text = vec![
            Line::from(vec![
                Span::styled("Node ID: ", Style::default().add_modifier(Modifier::BOLD)),
                Span::styled(&node_id, theme.primary_style()),
            ]),
            Line::from(vec![
                Span::styled("Realm: ", Style::default().add_modifier(Modifier::BOLD)),
                Span::raw(&realm),
                Span::raw("  |  "),
                Span::styled("Status: ", Style::default().add_modifier(Modifier::BOLD)),
                connection_status,
                Span::raw("  |  "),
                Span::styled("Uptime: ", Style::default().add_modifier(Modifier::BOLD)),
                Span::raw(&uptime),
            ]),
        ];

        let paragraph = Paragraph::new(text);
        frame.render_widget(paragraph, inner);
    }

    fn render_resources(&self, frame: &mut Frame, area: Rect, theme: &Theme) {
        let block = Block::default()
            .title("Resources")
            .borders(Borders::ALL)
            .border_style(theme.border_style());

        let inner = block.inner(area);
        frame.render_widget(block, area);

        let chunks = Layout::default()
            .direction(Direction::Horizontal)
            .constraints([
                Constraint::Percentage(33),
                Constraint::Percentage(33),
                Constraint::Percentage(34),
            ])
            .split(inner);

        // CPU gauge
        let cpu_percent = self.node_status.as_ref().map(|s| s.cpu_percent).unwrap_or(0.0);
        let cpu_gauge = Gauge::default()
            .block(Block::default().title("CPU"))
            .gauge_style(gauge_style(cpu_percent, theme))
            .percent(cpu_percent as u16)
            .label(format!("{:.1}%", cpu_percent));
        frame.render_widget(cpu_gauge, chunks[0]);

        // Memory gauge
        let (mem_used, mem_total) = self.node_status.as_ref()
            .map(|s| (s.memory_mb, s.memory_total_mb))
            .unwrap_or((0, 1));
        let mem_percent = (mem_used as f32 / mem_total as f32 * 100.0).min(100.0);
        let mem_gauge = Gauge::default()
            .block(Block::default().title("Memory"))
            .gauge_style(gauge_style(mem_percent, theme))
            .percent(mem_percent as u16)
            .label(format!("{}/{} MB", mem_used, mem_total));
        frame.render_widget(mem_gauge, chunks[1]);

        // Disk gauge
        let (disk_used, disk_total) = self.node_status.as_ref()
            .map(|s| (s.disk_used_gb, s.disk_total_gb))
            .unwrap_or((0.0, 1.0));
        let disk_percent = (disk_used / disk_total * 100.0).min(100.0);
        let disk_gauge = Gauge::default()
            .block(Block::default().title("Disk"))
            .gauge_style(gauge_style(disk_percent, theme))
            .percent(disk_percent as u16)
            .label(format!("{:.1}/{:.1} GB", disk_used, disk_total));
        frame.render_widget(disk_gauge, chunks[2]);
    }

    fn render_quick_stats(&self, frame: &mut Frame, area: Rect, theme: &Theme) {
        let block = Block::default()
            .title("Mesh Status")
            .borders(Borders::ALL)
            .border_style(theme.border_style());

        let inner = block.inner(area);
        frame.render_widget(block, area);

        let text = vec![
            Line::from(vec![
                Span::styled("Peers: ", Style::default().add_modifier(Modifier::BOLD)),
                Span::styled(
                    format!("{}", self.peer_count),
                    if self.peer_count > 0 {
                        theme.success_style()
                    } else {
                        theme.warning_style()
                    },
                ),
            ]),
        ];

        let paragraph = Paragraph::new(text);
        frame.render_widget(paragraph, inner);
    }
}

fn format_uptime(secs: u64) -> String {
    let days = secs / 86400;
    let hours = (secs % 86400) / 3600;
    let minutes = (secs % 3600) / 60;

    if days > 0 {
        format!("{}d {}h {}m", days, hours, minutes)
    } else if hours > 0 {
        format!("{}h {}m", hours, minutes)
    } else {
        format!("{}m", minutes)
    }
}

fn gauge_style(percent: f32, theme: &Theme) -> Style {
    if percent > 90.0 {
        theme.error_style()
    } else if percent > 70.0 {
        theme.warning_style()
    } else {
        theme.success_style()
    }
}
