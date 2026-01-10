//! Logs view - real-time log viewer.

use macula_tui_common::Theme;
use ratatui::{
    layout::Rect,
    style::Style,
    text::{Line, Span},
    widgets::{Block, Borders, Paragraph, Wrap},
    Frame,
};

use crate::nats::LogEntry;

const MAX_LOG_ENTRIES: usize = 1000;

/// Logs view state.
pub struct LogsView {
    pub entries: Vec<LogEntry>,
    pub scroll: u16,
    pub filter_level: Option<String>,
    pub filter_service: Option<String>,
}

impl Default for LogsView {
    fn default() -> Self {
        Self {
            entries: Vec::new(),
            scroll: 0,
            filter_level: None,
            filter_service: None,
        }
    }
}

impl LogsView {
    pub fn new() -> Self {
        Self::default()
    }

    pub fn add_entry(&mut self, entry: LogEntry) {
        self.entries.push(entry);
        // Keep bounded size
        if self.entries.len() > MAX_LOG_ENTRIES {
            self.entries.remove(0);
        }
    }

    pub fn scroll_down(&mut self) {
        self.scroll = self.scroll.saturating_add(1);
    }

    pub fn scroll_up(&mut self) {
        self.scroll = self.scroll.saturating_sub(1);
    }

    pub fn scroll_to_bottom(&mut self) {
        self.scroll = self.entries.len().saturating_sub(20) as u16;
    }

    pub fn clear(&mut self) {
        self.entries.clear();
        self.scroll = 0;
    }

    fn filtered_entries(&self) -> Vec<&LogEntry> {
        self.entries
            .iter()
            .filter(|e| {
                if let Some(level) = &self.filter_level {
                    if &e.level != level {
                        return false;
                    }
                }
                if let Some(service) = &self.filter_service {
                    if &e.service != service {
                        return false;
                    }
                }
                true
            })
            .collect()
    }

    pub fn render(&self, frame: &mut Frame, area: Rect, theme: &Theme) {
        let block = Block::default()
            .title(format!(
                "Logs ({}) - j/k: scroll | G: bottom | c: clear",
                self.entries.len()
            ))
            .borders(Borders::ALL)
            .border_style(theme.border_style());

        let filtered = self.filtered_entries();

        if filtered.is_empty() {
            let empty_text = Paragraph::new(vec![
                Line::from(""),
                Line::from(Span::styled(
                    "No log entries",
                    theme.muted_style(),
                )),
                Line::from(""),
                Line::from(Span::styled(
                    "Waiting for log messages...",
                    theme.muted_style(),
                )),
            ])
            .block(block);
            frame.render_widget(empty_text, area);
            return;
        }

        let lines: Vec<Line> = filtered
            .iter()
            .skip(self.scroll as usize)
            .map(|entry| {
                let level_style = level_style(&entry.level, theme);
                let timestamp = format_timestamp(entry.timestamp);

                Line::from(vec![
                    Span::styled(timestamp, theme.muted_style()),
                    Span::raw(" "),
                    Span::styled(format!("{:5}", entry.level), level_style),
                    Span::raw(" "),
                    Span::styled(format!("[{}]", entry.service), theme.primary_style()),
                    Span::raw(" "),
                    Span::raw(&entry.message),
                ])
            })
            .collect();

        let paragraph = Paragraph::new(lines)
            .block(block)
            .wrap(Wrap { trim: false });

        frame.render_widget(paragraph, area);
    }
}

fn level_style(level: &str, theme: &Theme) -> Style {
    match level.to_lowercase().as_str() {
        "error" | "err" => theme.error_style(),
        "warn" | "warning" => theme.warning_style(),
        "info" => theme.success_style(),
        "debug" => theme.muted_style(),
        "trace" => theme.muted_style(),
        _ => Style::default(),
    }
}

fn format_timestamp(ts: u64) -> String {
    // Simple formatting - just show HH:MM:SS
    let secs = ts % 86400;
    let hours = secs / 3600;
    let minutes = (secs % 3600) / 60;
    let seconds = secs % 60;
    format!("{:02}:{:02}:{:02}", hours, minutes, seconds)
}
