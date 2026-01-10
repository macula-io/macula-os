//! Peers view - mesh peer connections.

use macula_tui_common::Theme;
use ratatui::{
    layout::Rect,
    style::{Modifier, Style},
    text::{Line, Span},
    widgets::{Block, Borders, List, ListItem, ListState, Paragraph},
    Frame,
};

use crate::nats::PeerInfo;

/// Peers view state.
pub struct PeersView {
    pub peers: Vec<PeerInfo>,
    pub list_state: ListState,
}

impl Default for PeersView {
    fn default() -> Self {
        Self {
            peers: Vec::new(),
            list_state: ListState::default(),
        }
    }
}

impl PeersView {
    pub fn new() -> Self {
        Self::default()
    }

    pub fn add_peer(&mut self, peer: PeerInfo) {
        // Update existing or add new
        if let Some(existing) = self.peers.iter_mut().find(|p| p.node_id == peer.node_id) {
            *existing = peer;
        } else {
            self.peers.push(peer);
        }
    }

    pub fn remove_peer(&mut self, node_id: &str) {
        self.peers.retain(|p| p.node_id != node_id);
    }

    pub fn select_next(&mut self) {
        let i = match self.list_state.selected() {
            Some(i) => {
                if i >= self.peers.len().saturating_sub(1) {
                    0
                } else {
                    i + 1
                }
            }
            None => 0,
        };
        if !self.peers.is_empty() {
            self.list_state.select(Some(i));
        }
    }

    pub fn select_previous(&mut self) {
        let i = match self.list_state.selected() {
            Some(i) => {
                if i == 0 {
                    self.peers.len().saturating_sub(1)
                } else {
                    i - 1
                }
            }
            None => 0,
        };
        if !self.peers.is_empty() {
            self.list_state.select(Some(i));
        }
    }

    pub fn render(&mut self, frame: &mut Frame, area: Rect, theme: &Theme) {
        let block = Block::default()
            .title(format!("Peers ({})", self.peers.len()))
            .borders(Borders::ALL)
            .border_style(theme.border_style());

        if self.peers.is_empty() {
            let empty_text = Paragraph::new(vec![
                Line::from(""),
                Line::from(Span::styled(
                    "No peers connected",
                    theme.muted_style(),
                )),
                Line::from(""),
                Line::from(Span::styled(
                    "Waiting for mesh discovery...",
                    theme.muted_style(),
                )),
            ])
            .block(block);
            frame.render_widget(empty_text, area);
            return;
        }

        let items: Vec<ListItem> = self
            .peers
            .iter()
            .map(|peer| {
                let latency = peer
                    .latency_ms
                    .map(|l| format!("{}ms", l))
                    .unwrap_or_else(|| "?".to_string());

                let content = Line::from(vec![
                    Span::styled(
                        truncate_did(&peer.node_id, 24),
                        theme.primary_style(),
                    ),
                    Span::raw("  "),
                    Span::styled(&peer.address, Style::default()),
                    Span::raw("  "),
                    Span::styled(
                        latency,
                        latency_style(peer.latency_ms, theme),
                    ),
                ]);

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

fn truncate_did(did: &str, max_len: usize) -> String {
    if did.len() <= max_len {
        did.to_string()
    } else {
        format!("{}...", &did[..max_len - 3])
    }
}

fn latency_style(latency: Option<u32>, theme: &Theme) -> Style {
    match latency {
        Some(l) if l < 50 => theme.success_style(),
        Some(l) if l < 150 => theme.warning_style(),
        Some(_) => theme.error_style(),
        None => theme.muted_style(),
    }
}
