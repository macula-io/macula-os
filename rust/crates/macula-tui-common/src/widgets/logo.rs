//! Macula ASCII logo widget.

use ratatui::{
    buffer::Buffer,
    layout::Rect,
    style::Style,
    text::{Line, Span},
    widgets::Widget,
};

use crate::Theme;

/// ASCII art logo for Macula.
pub struct Logo {
    theme: Theme,
}

impl Default for Logo {
    fn default() -> Self {
        Self {
            theme: Theme::default(),
        }
    }
}

impl Logo {
    /// Create a new logo widget with the given theme.
    pub fn new(theme: Theme) -> Self {
        Self { theme }
    }

    /// The ASCII art lines.
    fn lines() -> &'static [&'static str] {
        &[
            r"  __  __                  _       ",
            r" |  \/  | __ _  ___ _   _| | __ _ ",
            r" | |\/| |/ _` |/ __| | | | |/ _` |",
            r" | |  | | (_| | (__| |_| | | (_| |",
            r" |_|  |_|\__,_|\___|\__,_|_|\__,_|",
            r"                                  ",
            r"     Decentralized Edge Platform  ",
        ]
    }
}

impl Widget for Logo {
    fn render(self, area: Rect, buf: &mut Buffer) {
        let lines = Self::lines();
        let style = self.theme.primary_style();

        for (i, line) in lines.iter().enumerate() {
            if i as u16 >= area.height {
                break;
            }
            let y = area.y + i as u16;
            let x = area.x;

            // Center the logo if the area is wider
            let offset = if area.width > line.len() as u16 {
                (area.width - line.len() as u16) / 2
            } else {
                0
            };

            buf.set_string(x + offset, y, *line, style);
        }
    }
}
