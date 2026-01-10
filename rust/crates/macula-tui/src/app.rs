//! Main TUI application.

use anyhow::Result;
use crossterm::{
    event::{self, DisableMouseCapture, EnableMouseCapture, Event, KeyCode, KeyEventKind},
    execute,
    terminal::{disable_raw_mode, enable_raw_mode, EnterAlternateScreen, LeaveAlternateScreen},
};
use macula_tui_common::{widgets::Logo, Theme};
use ratatui::{
    backend::CrosstermBackend,
    layout::{Constraint, Direction, Layout, Rect},
    style::Style,
    text::{Line, Span},
    widgets::{Block, Borders, Paragraph, Tabs},
    Frame, Terminal,
};
use std::io;
use tokio::sync::mpsc;

use crate::nats::{Command, NatsEvent, NatsManager};
use crate::views::{AppsView, DashboardView, LogsView, PeersView};

/// Active tab.
#[derive(Debug, Clone, Copy, PartialEq, Eq)]
pub enum Tab {
    Dashboard,
    Peers,
    Apps,
    Logs,
}

impl Tab {
    fn all() -> &'static [Tab] {
        &[Tab::Dashboard, Tab::Peers, Tab::Apps, Tab::Logs]
    }

    fn title(&self) -> &'static str {
        match self {
            Tab::Dashboard => "Dashboard",
            Tab::Peers => "Peers",
            Tab::Apps => "Apps",
            Tab::Logs => "Logs",
        }
    }

    fn index(&self) -> usize {
        Tab::all().iter().position(|t| t == self).unwrap_or(0)
    }
}

/// Application state.
pub struct App {
    tab: Tab,
    theme: Theme,
    should_quit: bool,
    nats_manager: NatsManager,
    event_rx: Option<mpsc::Receiver<NatsEvent>>,
    // Views
    dashboard: DashboardView,
    peers: PeersView,
    apps: AppsView,
    logs: LogsView,
}

impl App {
    pub fn new(nats_url: &str) -> Self {
        Self {
            tab: Tab::Dashboard,
            theme: Theme::default(),
            should_quit: false,
            nats_manager: NatsManager::new(nats_url),
            event_rx: None,
            dashboard: DashboardView::new(),
            peers: PeersView::new(),
            apps: AppsView::new(),
            logs: LogsView::new(),
        }
    }

    async fn connect(&mut self) -> Result<()> {
        let (tx, rx) = mpsc::channel(100);
        self.event_rx = Some(rx);
        self.nats_manager.connect(tx).await?;
        Ok(())
    }

    fn process_events(&mut self) {
        if let Some(rx) = &mut self.event_rx {
            while let Ok(event) = rx.try_recv() {
                match event {
                    NatsEvent::NodeStatus(status) => {
                        self.dashboard.node_status = Some(status);
                    }
                    NatsEvent::PeerDiscovered(peer) => {
                        self.peers.add_peer(peer);
                        self.dashboard.peer_count = self.peers.peers.len();
                    }
                    NatsEvent::PeerDisconnected(node_id) => {
                        self.peers.remove_peer(&node_id);
                        self.dashboard.peer_count = self.peers.peers.len();
                    }
                    NatsEvent::Log(entry) => {
                        self.logs.add_entry(entry);
                    }
                    NatsEvent::AppStatus(status) => {
                        self.apps.update_app(status);
                    }
                    NatsEvent::Connected => {
                        self.dashboard.connected = true;
                    }
                    NatsEvent::Disconnected => {
                        self.dashboard.connected = false;
                    }
                    NatsEvent::Error(e) => {
                        tracing::error!("NATS error: {}", e);
                    }
                }
            }
        }
    }

    fn next_tab(&mut self) {
        let tabs = Tab::all();
        let current_idx = self.tab.index();
        self.tab = tabs[(current_idx + 1) % tabs.len()];
    }

    fn prev_tab(&mut self) {
        let tabs = Tab::all();
        let current_idx = self.tab.index();
        self.tab = tabs[(current_idx + tabs.len() - 1) % tabs.len()];
    }

    async fn handle_key(&mut self, key: KeyCode) {
        // Global keys
        match key {
            KeyCode::Char('q') | KeyCode::Esc => {
                self.should_quit = true;
                return;
            }
            KeyCode::Tab => {
                self.next_tab();
                return;
            }
            KeyCode::BackTab => {
                self.prev_tab();
                return;
            }
            KeyCode::Char('1') => {
                self.tab = Tab::Dashboard;
                return;
            }
            KeyCode::Char('2') => {
                self.tab = Tab::Peers;
                return;
            }
            KeyCode::Char('3') => {
                self.tab = Tab::Apps;
                return;
            }
            KeyCode::Char('4') => {
                self.tab = Tab::Logs;
                return;
            }
            _ => {}
        }

        // Tab-specific keys
        match self.tab {
            Tab::Dashboard => {
                // No specific keys for dashboard
            }
            Tab::Peers => match key {
                KeyCode::Down | KeyCode::Char('j') => self.peers.select_next(),
                KeyCode::Up | KeyCode::Char('k') => self.peers.select_previous(),
                _ => {}
            },
            Tab::Apps => match key {
                KeyCode::Down | KeyCode::Char('j') => self.apps.select_next(),
                KeyCode::Up | KeyCode::Char('k') => self.apps.select_previous(),
                KeyCode::Char('s') => {
                    // Start selected app
                    if let Some(app_id) = self.apps.selected_app_id() {
                        let _ = self
                            .nats_manager
                            .send_command(Command::StartApp {
                                app_id: app_id.to_string(),
                            })
                            .await;
                    }
                }
                KeyCode::Char('x') => {
                    // Stop selected app
                    if let Some(app_id) = self.apps.selected_app_id() {
                        let _ = self
                            .nats_manager
                            .send_command(Command::StopApp {
                                app_id: app_id.to_string(),
                            })
                            .await;
                    }
                }
                KeyCode::Char('r') => {
                    // Restart selected app
                    if let Some(app_id) = self.apps.selected_app_id() {
                        let _ = self
                            .nats_manager
                            .send_command(Command::RestartApp {
                                app_id: app_id.to_string(),
                            })
                            .await;
                    }
                }
                _ => {}
            },
            Tab::Logs => match key {
                KeyCode::Down | KeyCode::Char('j') => self.logs.scroll_down(),
                KeyCode::Up | KeyCode::Char('k') => self.logs.scroll_up(),
                KeyCode::Char('G') => self.logs.scroll_to_bottom(),
                KeyCode::Char('c') => self.logs.clear(),
                _ => {}
            },
        }
    }

    fn render(&mut self, frame: &mut Frame) {
        let chunks = Layout::default()
            .direction(Direction::Vertical)
            .constraints([
                Constraint::Length(7),  // Logo
                Constraint::Length(3),  // Tabs
                Constraint::Min(10),    // Content
                Constraint::Length(3),  // Help
            ])
            .split(frame.area());

        // Render logo
        let logo = Logo::new(self.theme.clone());
        frame.render_widget(logo, chunks[0]);

        // Render tabs
        self.render_tabs(frame, chunks[1]);

        // Render active view
        self.render_view(frame, chunks[2]);

        // Render help
        self.render_help(frame, chunks[3]);
    }

    fn render_tabs(&self, frame: &mut Frame, area: Rect) {
        let titles: Vec<Line> = Tab::all()
            .iter()
            .enumerate()
            .map(|(i, t)| {
                let style = if *t == self.tab {
                    self.theme.selected_style()
                } else {
                    Style::default()
                };
                Line::from(Span::styled(format!(" {} {} ", i + 1, t.title()), style))
            })
            .collect();

        let tabs = Tabs::new(titles)
            .block(Block::default().borders(Borders::BOTTOM).border_style(self.theme.border_style()))
            .select(self.tab.index())
            .highlight_style(self.theme.selected_style());

        frame.render_widget(tabs, area);
    }

    fn render_view(&mut self, frame: &mut Frame, area: Rect) {
        match self.tab {
            Tab::Dashboard => self.dashboard.render(frame, area, &self.theme),
            Tab::Peers => self.peers.render(frame, area, &self.theme),
            Tab::Apps => self.apps.render(frame, area, &self.theme),
            Tab::Logs => self.logs.render(frame, area, &self.theme),
        }
    }

    fn render_help(&self, frame: &mut Frame, area: Rect) {
        let help_text = match self.tab {
            Tab::Dashboard => "q: Quit | Tab: Next tab | 1-4: Switch tab",
            Tab::Peers => "q: Quit | Tab: Next tab | j/k: Navigate",
            Tab::Apps => "q: Quit | Tab: Next tab | j/k: Navigate | s: Start | x: Stop | r: Restart",
            Tab::Logs => "q: Quit | Tab: Next tab | j/k: Scroll | G: Bottom | c: Clear",
        };

        let help = Paragraph::new(help_text)
            .style(self.theme.muted_style())
            .block(
                Block::default()
                    .borders(Borders::TOP)
                    .border_style(self.theme.border_style()),
            );

        frame.render_widget(help, area);
    }
}

/// Run the TUI application.
pub async fn run(nats_url: &str, _config_dir: &str) -> Result<()> {
    // Setup terminal
    enable_raw_mode()?;
    let mut stdout = io::stdout();
    execute!(stdout, EnterAlternateScreen, EnableMouseCapture)?;
    let backend = CrosstermBackend::new(stdout);
    let mut terminal = Terminal::new(backend)?;

    // Create app and connect to NATS
    let mut app = App::new(nats_url);

    // Try to connect to NATS (don't fail if not available)
    if let Err(e) = app.connect().await {
        tracing::warn!("Could not connect to NATS: {}", e);
        // Continue anyway - we'll show disconnected state
    }

    // Main loop
    loop {
        // Process NATS events
        app.process_events();

        // Draw UI
        terminal.draw(|f| app.render(f))?;

        // Handle input (with timeout for NATS events)
        if event::poll(std::time::Duration::from_millis(100))? {
            if let Event::Key(key) = event::read()? {
                if key.kind == KeyEventKind::Press {
                    app.handle_key(key.code).await;
                }
            }
        }

        if app.should_quit {
            break;
        }
    }

    // Restore terminal
    disable_raw_mode()?;
    execute!(
        terminal.backend_mut(),
        LeaveAlternateScreen,
        DisableMouseCapture
    )?;
    terminal.show_cursor()?;

    Ok(())
}
