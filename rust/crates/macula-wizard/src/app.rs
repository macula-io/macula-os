//! Main TUI application for the setup wizard.

use anyhow::Result;
use crossterm::{
    cursor::MoveTo,
    event::{self, DisableMouseCapture, EnableMouseCapture, Event, KeyCode, KeyEventKind},
    execute,
    terminal::{disable_raw_mode, enable_raw_mode, Clear, ClearType, EnterAlternateScreen, LeaveAlternateScreen},
};
use macula_tui_common::{widgets::Logo, Theme};
use ratatui::{
    backend::CrosstermBackend,
    layout::{Constraint, Direction, Layout, Rect},
    style::{Modifier, Style},
    text::{Line, Span},
    widgets::{Block, Borders, Paragraph},
    Frame, Terminal,
};
use std::io;

use crate::config::{MaculaConfig, MeshIdentity, PortalToken};

/// Wizard steps.
#[derive(Debug, Clone, Copy, PartialEq, Eq)]
pub enum Step {
    Welcome,
    Network,
    Identity,
    Realm,
    Portal,
    Summary,
}

impl Step {
    fn all() -> &'static [Step] {
        &[
            Step::Welcome,
            Step::Network,
            Step::Identity,
            Step::Realm,
            Step::Portal,
            Step::Summary,
        ]
    }

    fn title(&self) -> &'static str {
        match self {
            Step::Welcome => "Welcome",
            Step::Network => "Network",
            Step::Identity => "Identity",
            Step::Realm => "Realm",
            Step::Portal => "Portal",
            Step::Summary => "Summary",
        }
    }

    fn index(&self) -> usize {
        Step::all().iter().position(|s| s == self).unwrap_or(0)
    }
}

/// Application state.
pub struct App {
    /// Current step
    step: Step,
    /// Theme
    theme: Theme,
    /// Configuration being built
    config: MaculaConfig,
    /// Identity being built
    identity: MeshIdentity,
    /// Portal token (optional)
    portal_token: Option<PortalToken>,
    /// Portal URL
    portal_url: String,
    /// Should quit
    should_quit: bool,
    /// Step-specific state
    step_state: StepState,
}

/// Step-specific state.
#[derive(Default)]
struct StepState {
    /// Network: DHCP selected
    dhcp: bool,
    /// Portal: pairing code input
    pairing_code: String,
    /// Portal: pairing in progress
    pairing: bool,
    /// Portal: pairing error
    pairing_error: Option<String>,
    /// Portal: skip pairing
    skip_portal: bool,
}

impl App {
    pub fn new(portal_url: &str) -> Self {
        Self {
            step: Step::Welcome,
            theme: Theme::default(),
            config: MaculaConfig::default(),
            identity: MeshIdentity::generate(),
            portal_token: None,
            portal_url: portal_url.to_string(),
            should_quit: false,
            step_state: StepState {
                dhcp: true,
                ..Default::default()
            },
        }
    }

    fn next_step(&mut self) {
        let steps = Step::all();
        let current_idx = self.step.index();
        if current_idx + 1 < steps.len() {
            self.step = steps[current_idx + 1];
        }
    }

    fn prev_step(&mut self) {
        let steps = Step::all();
        let current_idx = self.step.index();
        if current_idx > 0 {
            self.step = steps[current_idx - 1];
        }
    }

    fn handle_key(&mut self, key: KeyCode) {
        match key {
            KeyCode::Char('q') | KeyCode::Esc => {
                self.should_quit = true;
            }
            KeyCode::Enter | KeyCode::Right => {
                if self.step == Step::Summary {
                    // Finish wizard
                    self.should_quit = true;
                } else {
                    self.next_step();
                }
            }
            KeyCode::Left | KeyCode::Backspace => {
                self.prev_step();
            }
            KeyCode::Tab => {
                // Toggle options in current step
                match self.step {
                    Step::Network => {
                        self.step_state.dhcp = !self.step_state.dhcp;
                        self.config.network.dhcp = self.step_state.dhcp;
                    }
                    Step::Portal => {
                        self.step_state.skip_portal = !self.step_state.skip_portal;
                    }
                    _ => {}
                }
            }
            KeyCode::Char(c) => {
                // Text input for portal pairing code
                if self.step == Step::Portal && !self.step_state.skip_portal {
                    self.step_state.pairing_code.push(c.to_ascii_uppercase());
                }
            }
            _ => {}
        }
    }

    fn render(&self, frame: &mut Frame) {
        let chunks = Layout::default()
            .direction(Direction::Vertical)
            .constraints([
                Constraint::Length(9),  // Logo
                Constraint::Length(3),  // Progress
                Constraint::Min(10),    // Content
                Constraint::Length(3),  // Help
            ])
            .split(frame.area());

        // Render logo
        let logo = Logo::new(self.theme.clone());
        frame.render_widget(logo, chunks[0]);

        // Render progress
        self.render_progress(frame, chunks[1]);

        // Render step content
        self.render_step(frame, chunks[2]);

        // Render help
        self.render_help(frame, chunks[3]);
    }

    fn render_progress(&self, frame: &mut Frame, area: Rect) {
        let steps = Step::all();
        let mut spans: Vec<Span> = Vec::new();

        for (i, s) in steps.iter().enumerate() {
            let style = if *s == self.step {
                self.theme.selected_style()
            } else if s.index() < self.step.index() {
                self.theme.success_style()
            } else {
                self.theme.muted_style()
            };

            let prefix = if s.index() < self.step.index() {
                "[x]"
            } else if *s == self.step {
                "[>]"
            } else {
                "[ ]"
            };

            spans.push(Span::styled(format!("{} {}", prefix, s.title()), style));

            // Add separator between steps
            if i < steps.len() - 1 {
                spans.push(Span::raw("  "));
            }
        }

        let line = Line::from(spans);
        let paragraph = Paragraph::new(line)
            .block(Block::default().borders(Borders::NONE));

        frame.render_widget(paragraph, area);
    }

    fn render_step(&self, frame: &mut Frame, area: Rect) {
        let block = Block::default()
            .title(self.step.title())
            .borders(Borders::ALL)
            .border_style(self.theme.border_style());

        let inner = block.inner(area);
        frame.render_widget(block, area);

        match self.step {
            Step::Welcome => self.render_welcome(frame, inner),
            Step::Network => self.render_network(frame, inner),
            Step::Identity => self.render_identity(frame, inner),
            Step::Realm => self.render_realm(frame, inner),
            Step::Portal => self.render_portal(frame, inner),
            Step::Summary => self.render_summary(frame, inner),
        }
    }

    fn render_welcome(&self, frame: &mut Frame, area: Rect) {
        let text = vec![
            Line::from("Welcome to MaculaOS!"),
            Line::from(""),
            Line::from("This wizard will guide you through the initial setup of your"),
            Line::from("Macula edge node. You'll configure:"),
            Line::from(""),
            Line::from("  - Network settings"),
            Line::from("  - Mesh identity (Ed25519 keypair)"),
            Line::from("  - Realm and bootstrap peers"),
            Line::from("  - Optional: Portal pairing"),
            Line::from(""),
            Line::from(Span::styled(
                "Press Enter or -> to continue",
                self.theme.primary_style(),
            )),
        ];

        let paragraph = Paragraph::new(text);
        frame.render_widget(paragraph, area);
    }

    fn render_network(&self, frame: &mut Frame, area: Rect) {
        let dhcp_style = if self.step_state.dhcp {
            self.theme.selected_style()
        } else {
            Style::default()
        };
        let static_style = if !self.step_state.dhcp {
            self.theme.selected_style()
        } else {
            Style::default()
        };

        let text = vec![
            Line::from("Network Configuration"),
            Line::from(""),
            Line::from(vec![
                Span::raw("  "),
                Span::styled(if self.step_state.dhcp { "[x]" } else { "[ ]" }, dhcp_style),
                Span::styled(" DHCP (automatic)", dhcp_style),
            ]),
            Line::from(vec![
                Span::raw("  "),
                Span::styled(if !self.step_state.dhcp { "[x]" } else { "[ ]" }, static_style),
                Span::styled(" Static IP", static_style),
            ]),
            Line::from(""),
            Line::from(Span::styled(
                "Press Tab to toggle, Enter to continue",
                self.theme.muted_style(),
            )),
        ];

        let paragraph = Paragraph::new(text);
        frame.render_widget(paragraph, area);
    }

    fn render_identity(&self, frame: &mut Frame, area: Rect) {
        let text = vec![
            Line::from("Mesh Identity"),
            Line::from(""),
            Line::from("A new Ed25519 keypair has been generated for this node."),
            Line::from(""),
            Line::from(vec![
                Span::raw("DID: "),
                Span::styled(&self.identity.did, self.theme.primary_style()),
            ]),
            Line::from(""),
            Line::from(Span::styled(
                "This identity will be used to authenticate with the mesh.",
                self.theme.muted_style(),
            )),
        ];

        let paragraph = Paragraph::new(text);
        frame.render_widget(paragraph, area);
    }

    fn render_realm(&self, frame: &mut Frame, area: Rect) {
        let text = vec![
            Line::from("Realm Configuration"),
            Line::from(""),
            Line::from(vec![
                Span::raw("Realm: "),
                Span::styled(&self.config.realm, self.theme.primary_style()),
            ]),
            Line::from(""),
            Line::from(vec![
                Span::raw("Bootstrap: "),
                Span::styled(
                    self.config.bootstrap_peers.join(", "),
                    self.theme.primary_style(),
                ),
            ]),
            Line::from(""),
            Line::from(Span::styled(
                "Using default Macula realm and bootstrap servers.",
                self.theme.muted_style(),
            )),
        ];

        let paragraph = Paragraph::new(text);
        frame.render_widget(paragraph, area);
    }

    fn render_portal(&self, frame: &mut Frame, area: Rect) {
        let skip_style = if self.step_state.skip_portal {
            self.theme.selected_style()
        } else {
            Style::default()
        };
        let pair_style = if !self.step_state.skip_portal {
            self.theme.selected_style()
        } else {
            Style::default()
        };

        let mut text = vec![
            Line::from("Portal Pairing (Optional)"),
            Line::from(""),
            Line::from("Pair with Macula Portal to sync apps and certificates."),
            Line::from(""),
            Line::from(vec![
                Span::raw("  "),
                Span::styled(if self.step_state.skip_portal { "[x]" } else { "[ ]" }, skip_style),
                Span::styled(" Skip pairing", skip_style),
            ]),
            Line::from(vec![
                Span::raw("  "),
                Span::styled(if !self.step_state.skip_portal { "[x]" } else { "[ ]" }, pair_style),
                Span::styled(" Enter pairing code", pair_style),
            ]),
        ];

        if !self.step_state.skip_portal {
            text.push(Line::from(""));
            text.push(Line::from(vec![
                Span::raw("Pairing code: "),
                Span::styled(
                    if self.step_state.pairing_code.is_empty() {
                        "___-___"
                    } else {
                        &self.step_state.pairing_code
                    },
                    self.theme.primary_style(),
                ),
            ]));

            if let Some(error) = &self.step_state.pairing_error {
                text.push(Line::from(""));
                text.push(Line::from(Span::styled(error, self.theme.error_style())));
            }
        }

        text.push(Line::from(""));
        text.push(Line::from(Span::styled(
            "Press Tab to toggle, Enter to continue",
            self.theme.muted_style(),
        )));

        let paragraph = Paragraph::new(text);
        frame.render_widget(paragraph, area);
    }

    fn render_summary(&self, frame: &mut Frame, area: Rect) {
        let text = vec![
            Line::from(Span::styled("Configuration Summary", self.theme.primary_style())),
            Line::from(""),
            Line::from(vec![
                Span::styled("Network: ", Style::default().add_modifier(Modifier::BOLD)),
                Span::raw(if self.config.network.dhcp { "DHCP" } else { "Static" }),
            ]),
            Line::from(vec![
                Span::styled("Realm: ", Style::default().add_modifier(Modifier::BOLD)),
                Span::raw(&self.config.realm),
            ]),
            Line::from(vec![
                Span::styled("Bootstrap: ", Style::default().add_modifier(Modifier::BOLD)),
                Span::raw(self.config.bootstrap_peers.join(", ")),
            ]),
            Line::from(vec![
                Span::styled("Identity: ", Style::default().add_modifier(Modifier::BOLD)),
                Span::raw(&self.identity.did),
            ]),
            Line::from(vec![
                Span::styled("Portal: ", Style::default().add_modifier(Modifier::BOLD)),
                Span::raw(if self.portal_token.is_some() {
                    "Paired"
                } else {
                    "Not paired"
                }),
            ]),
            Line::from(""),
            Line::from(Span::styled(
                "Press Enter to save and complete setup",
                self.theme.success_style(),
            )),
        ];

        let paragraph = Paragraph::new(text);
        frame.render_widget(paragraph, area);
    }

    fn render_help(&self, frame: &mut Frame, area: Rect) {
        let help_text = "q: Quit | <-/->: Navigate | Tab: Toggle | Enter: Continue";
        let help = Paragraph::new(help_text)
            .style(self.theme.muted_style())
            .block(Block::default().borders(Borders::TOP).border_style(self.theme.border_style()));
        frame.render_widget(help, area);
    }
}

/// Run the TUI wizard.
pub async fn run(config_dir: &str, portal_url: &str) -> Result<()> {
    // Setup terminal
    enable_raw_mode()?;
    let mut stdout = io::stdout();
    // Clear screen first (for Linux TTY where alternate screen may not work)
    execute!(stdout, Clear(ClearType::All), MoveTo(0, 0))?;
    execute!(stdout, EnterAlternateScreen, EnableMouseCapture)?;
    let backend = CrosstermBackend::new(stdout);
    let mut terminal = Terminal::new(backend)?;

    // Create app
    let mut app = App::new(portal_url);

    // Main loop
    loop {
        terminal.draw(|f| app.render(f))?;

        if event::poll(std::time::Duration::from_millis(100))? {
            if let Event::Key(key) = event::read()? {
                if key.kind == KeyEventKind::Press {
                    app.handle_key(key.code);
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

    // Write configuration if we completed the wizard (not cancelled)
    if app.step == Step::Summary {
        // Update config with identity
        let mut config = app.config.clone();
        config.node_id = Some(app.identity.did.clone());

        crate::config::write_config(
            config_dir,
            &config,
            &app.identity,
            app.portal_token.as_ref(),
        )
        .await?;

        println!("\nMaculaOS setup complete!");
        println!("Configuration written to {}", config_dir);
    } else {
        println!("\nSetup cancelled.");
        std::process::exit(1);
    }

    Ok(())
}
