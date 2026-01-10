//! Macula TUI color theme.

use ratatui::style::{Color, Modifier, Style};

/// Macula color theme for consistent UI styling.
#[derive(Debug, Clone)]
pub struct Theme {
    /// Primary accent color (Macula purple)
    pub primary: Color,
    /// Secondary accent color
    pub secondary: Color,
    /// Success/positive color
    pub success: Color,
    /// Warning color
    pub warning: Color,
    /// Error/danger color
    pub error: Color,
    /// Default text color
    pub text: Color,
    /// Muted/secondary text color
    pub text_muted: Color,
    /// Background color
    pub background: Color,
    /// Surface/card background color
    pub surface: Color,
    /// Border color
    pub border: Color,
}

impl Default for Theme {
    fn default() -> Self {
        Self {
            primary: Color::Rgb(138, 43, 226),    // Macula purple
            secondary: Color::Rgb(100, 149, 237), // Cornflower blue
            success: Color::Rgb(50, 205, 50),     // Lime green
            warning: Color::Rgb(255, 165, 0),     // Orange
            error: Color::Rgb(220, 20, 60),       // Crimson
            text: Color::White,
            text_muted: Color::Gray,
            background: Color::Reset,
            surface: Color::Rgb(30, 30, 30),
            border: Color::Rgb(60, 60, 60),
        }
    }
}

impl Theme {
    /// Style for primary buttons/highlights
    pub fn primary_style(&self) -> Style {
        Style::default().fg(self.primary).add_modifier(Modifier::BOLD)
    }

    /// Style for success messages
    pub fn success_style(&self) -> Style {
        Style::default().fg(self.success)
    }

    /// Style for warning messages
    pub fn warning_style(&self) -> Style {
        Style::default().fg(self.warning)
    }

    /// Style for error messages
    pub fn error_style(&self) -> Style {
        Style::default().fg(self.error)
    }

    /// Style for muted/secondary text
    pub fn muted_style(&self) -> Style {
        Style::default().fg(self.text_muted)
    }

    /// Style for borders
    pub fn border_style(&self) -> Style {
        Style::default().fg(self.border)
    }

    /// Style for focused/selected items
    pub fn focused_style(&self) -> Style {
        Style::default()
            .fg(self.primary)
            .add_modifier(Modifier::BOLD)
    }

    /// Style for selected items in lists
    pub fn selected_style(&self) -> Style {
        Style::default()
            .bg(self.primary)
            .fg(Color::White)
            .add_modifier(Modifier::BOLD)
    }
}
