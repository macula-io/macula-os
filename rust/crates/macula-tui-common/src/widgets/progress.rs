//! Progress bar widget.

use ratatui::{
    buffer::Buffer,
    layout::Rect,
    style::Style,
    widgets::Widget,
};

use crate::Theme;

/// A simple progress bar widget.
pub struct ProgressBar {
    /// Progress value between 0.0 and 1.0
    progress: f64,
    /// Theme for styling
    theme: Theme,
    /// Optional label
    label: Option<String>,
}

impl ProgressBar {
    /// Create a new progress bar with the given progress (0.0 to 1.0).
    pub fn new(progress: f64) -> Self {
        Self {
            progress: progress.clamp(0.0, 1.0),
            theme: Theme::default(),
            label: None,
        }
    }

    /// Set the theme.
    pub fn theme(mut self, theme: Theme) -> Self {
        self.theme = theme;
        self
    }

    /// Set an optional label.
    pub fn label(mut self, label: impl Into<String>) -> Self {
        self.label = Some(label.into());
        self
    }
}

impl Widget for ProgressBar {
    fn render(self, area: Rect, buf: &mut Buffer) {
        if area.height == 0 || area.width == 0 {
            return;
        }

        // Calculate filled width
        let filled_width = (area.width as f64 * self.progress) as u16;

        // Draw filled portion
        let filled_style = Style::default().bg(self.theme.primary);
        for x in area.x..area.x + filled_width {
            buf.get_mut(x, area.y).set_style(filled_style);
            buf.get_mut(x, area.y).set_char(' ');
        }

        // Draw empty portion
        let empty_style = Style::default().bg(self.theme.surface);
        for x in area.x + filled_width..area.x + area.width {
            buf.get_mut(x, area.y).set_style(empty_style);
            buf.get_mut(x, area.y).set_char(' ');
        }

        // Draw label if present
        if let Some(label) = &self.label {
            let label_x = area.x + (area.width.saturating_sub(label.len() as u16)) / 2;
            buf.set_string(label_x, area.y, label, Style::default().fg(self.theme.text));
        }
    }
}
