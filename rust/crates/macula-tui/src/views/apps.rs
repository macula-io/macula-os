//! Apps view - installed application management.

use macula_tui_common::Theme;
use ratatui::{
    layout::Rect,
    style::{Modifier, Style},
    text::{Line, Span},
    widgets::{Block, Borders, List, ListItem, ListState, Paragraph},
    Frame,
};

use crate::nats::AppStatus;

/// Apps view state.
pub struct AppsView {
    pub apps: Vec<AppStatus>,
    pub list_state: ListState,
}

impl Default for AppsView {
    fn default() -> Self {
        Self {
            apps: Vec::new(),
            list_state: ListState::default(),
        }
    }
}

impl AppsView {
    pub fn new() -> Self {
        Self::default()
    }

    pub fn update_app(&mut self, status: AppStatus) {
        if let Some(existing) = self.apps.iter_mut().find(|a| a.app_id == status.app_id) {
            *existing = status;
        } else {
            self.apps.push(status);
        }
    }

    pub fn selected_app_id(&self) -> Option<&str> {
        self.list_state
            .selected()
            .and_then(|i| self.apps.get(i))
            .map(|a| a.app_id.as_str())
    }

    pub fn select_next(&mut self) {
        let i = match self.list_state.selected() {
            Some(i) => {
                if i >= self.apps.len().saturating_sub(1) {
                    0
                } else {
                    i + 1
                }
            }
            None => 0,
        };
        if !self.apps.is_empty() {
            self.list_state.select(Some(i));
        }
    }

    pub fn select_previous(&mut self) {
        let i = match self.list_state.selected() {
            Some(i) => {
                if i == 0 {
                    self.apps.len().saturating_sub(1)
                } else {
                    i - 1
                }
            }
            None => 0,
        };
        if !self.apps.is_empty() {
            self.list_state.select(Some(i));
        }
    }

    pub fn render(&mut self, frame: &mut Frame, area: Rect, theme: &Theme) {
        let block = Block::default()
            .title(format!("Apps ({})", self.apps.len()))
            .borders(Borders::ALL)
            .border_style(theme.border_style());

        if self.apps.is_empty() {
            let empty_text = Paragraph::new(vec![
                Line::from(""),
                Line::from(Span::styled(
                    "No applications installed",
                    theme.muted_style(),
                )),
                Line::from(""),
                Line::from(Span::styled(
                    "Install apps via Macula Console",
                    theme.muted_style(),
                )),
            ])
            .block(block);
            frame.render_widget(empty_text, area);
            return;
        }

        let items: Vec<ListItem> = self
            .apps
            .iter()
            .map(|app| {
                let status_style = match app.status.as_str() {
                    "running" => theme.success_style(),
                    "stopped" => theme.muted_style(),
                    "error" => theme.error_style(),
                    _ => Style::default(),
                };

                let status_icon = match app.status.as_str() {
                    "running" => "[R]",
                    "stopped" => "[S]",
                    "error" => "[E]",
                    _ => "[?]",
                };

                let resources = match (&app.cpu_percent, &app.memory_mb) {
                    (Some(cpu), Some(mem)) => format!("CPU: {:.1}% | Mem: {} MB", cpu, mem),
                    _ => String::new(),
                };

                let content = vec![
                    Line::from(vec![
                        Span::styled(status_icon, status_style),
                        Span::raw(" "),
                        Span::styled(&app.name, theme.primary_style()),
                        Span::raw("  "),
                        Span::styled(&app.status, status_style),
                    ]),
                    Line::from(vec![
                        Span::raw("   "),
                        Span::styled(&app.app_id, theme.muted_style()),
                        if !resources.is_empty() {
                            Span::styled(format!("  {}", resources), theme.muted_style())
                        } else {
                            Span::raw("")
                        },
                    ]),
                ];

                ListItem::new(content)
            })
            .collect();

        let list = List::new(items)
            .block(block)
            .highlight_style(theme.selected_style().add_modifier(Modifier::BOLD))
            .highlight_symbol("> ");

        frame.render_stateful_widget(list, area, &mut self.list_state);
    }
}
