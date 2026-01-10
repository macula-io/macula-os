//! Text input widget.

use ratatui::{
    buffer::Buffer,
    layout::Rect,
    style::Style,
    widgets::{Block, Borders, Widget},
};

use crate::Theme;

/// A text input widget with cursor.
pub struct TextInput<'a> {
    /// Current input value
    value: &'a str,
    /// Cursor position
    cursor: usize,
    /// Whether the input is focused
    focused: bool,
    /// Theme for styling
    theme: Theme,
    /// Optional label
    label: Option<&'a str>,
    /// Mask character for password fields
    mask: Option<char>,
}

impl<'a> TextInput<'a> {
    /// Create a new text input with the given value.
    pub fn new(value: &'a str) -> Self {
        Self {
            value,
            cursor: value.len(),
            focused: false,
            theme: Theme::default(),
            label: None,
            mask: None,
        }
    }

    /// Set the cursor position.
    pub fn cursor(mut self, cursor: usize) -> Self {
        self.cursor = cursor.min(self.value.len());
        self
    }

    /// Set whether the input is focused.
    pub fn focused(mut self, focused: bool) -> Self {
        self.focused = focused;
        self
    }

    /// Set the theme.
    pub fn theme(mut self, theme: Theme) -> Self {
        self.theme = theme;
        self
    }

    /// Set an optional label.
    pub fn label(mut self, label: &'a str) -> Self {
        self.label = Some(label);
        self
    }

    /// Set a mask character for password fields.
    pub fn mask(mut self, mask: char) -> Self {
        self.mask = Some(mask);
        self
    }
}

impl<'a> Widget for TextInput<'a> {
    fn render(self, area: Rect, buf: &mut Buffer) {
        // Create block with optional label
        let block = if let Some(label) = self.label {
            Block::default()
                .title(label)
                .borders(Borders::ALL)
                .border_style(if self.focused {
                    self.theme.focused_style()
                } else {
                    self.theme.border_style()
                })
        } else {
            Block::default()
                .borders(Borders::ALL)
                .border_style(if self.focused {
                    self.theme.focused_style()
                } else {
                    self.theme.border_style()
                })
        };

        let inner = block.inner(area);
        block.render(area, buf);

        // Render the text value (masked if needed)
        let display_value: String = if let Some(mask) = self.mask {
            mask.to_string().repeat(self.value.len())
        } else {
            self.value.to_string()
        };

        buf.set_string(inner.x, inner.y, &display_value, Style::default().fg(self.theme.text));

        // Render cursor if focused
        if self.focused && inner.width > 0 {
            let cursor_x = inner.x + self.cursor as u16;
            if cursor_x < inner.x + inner.width {
                buf.get_mut(cursor_x, inner.y)
                    .set_style(Style::default().bg(self.theme.text).fg(self.theme.background));
            }
        }
    }
}
