# Exploration: TUI Multiplexer (tmux-like Panes)

**Status:** Exploration / RFC
**Created:** 2026-01-12
**Related:** EXPLORATION_FRONTEND_WEB_TUI_APP_LAUNCHER.md

## Overview

Explore implementing a tmux-like terminal multiplexer in the MaculaOS TUI that allows users to:
- View multiple application outputs simultaneously
- Split terminal into panes (horizontal/vertical)
- Switch between applications
- Detach/reattach to running sessions

## Why This Matters

The TUI is the primary interface for MaculaOS (see Frontend exploration). A multiplexer enables:
- Monitoring multiple services at once
- Comparing logs side-by-side
- Running commands while watching app output
- SSH-friendly (works over remote connections)

## Architecture Options

### Option A: In-Process Panes (Ratatui)

Use ratatui's layout system to divide terminal into panes, each rendering output from a different source.

```
┌─────────────────────────────────────────────────────────────────┐
│  macula-tui                                                      │
├─────────────────────────────────────────────────────────────────┤
│                                                                  │
│  ┌─────────────────────────┬───────────────────────────────────┐│
│  │ [1] Console Logs        │ [2] Mesh Status                   ││
│  │                         │                                   ││
│  │ [info] Request /api     │ Peers: 5                          ││
│  │ [info] 200 OK 12ms      │ DHT entries: 1,234                ││
│  │ [debug] Cache hit       │ Uptime: 3h 42m                    ││
│  │                         │                                   ││
│  ├─────────────────────────┴───────────────────────────────────┤│
│  │ [3] GitOps Status                                           ││
│  │                                                             ││
│  │ ✓ console       v1.2.0  synced                              ││
│  │ ✓ my-app        v0.5.0  synced                              ││
│  │ ↻ data-pipe     v0.3.0  upgrading...                        ││
│  │                                                             ││
│  └─────────────────────────────────────────────────────────────┘│
│                                                                  │
│  [Ctrl+B] Prefix  [←↑→↓] Navigate  [|] Split V  [-] Split H     │
└─────────────────────────────────────────────────────────────────┘
```

**Implementation:**

```rust
// rust/crates/macula-tui/src/multiplexer.rs

use ratatui::{
    layout::{Constraint, Direction, Layout, Rect},
    widgets::{Block, Borders, Paragraph},
    Frame,
};

#[derive(Debug)]
pub struct Pane {
    pub id: usize,
    pub title: String,
    pub content: PaneContent,
    pub scroll_offset: usize,
}

#[derive(Debug)]
pub enum PaneContent {
    AppLogs { app_name: String, lines: Vec<String> },
    MeshStatus { stats: MeshStats },
    GitOpsStatus { apps: Vec<AppStatus> },
    Shell { pty: PtyHandle },
    Custom { widget_id: String },
}

#[derive(Debug)]
pub struct PaneLayout {
    pub root: LayoutNode,
    pub active_pane: usize,
}

#[derive(Debug)]
pub enum LayoutNode {
    Leaf(usize),  // Pane ID
    Split {
        direction: Direction,
        ratio: f32,  // 0.0 to 1.0
        first: Box<LayoutNode>,
        second: Box<LayoutNode>,
    },
}

impl PaneLayout {
    pub fn new() -> Self {
        Self {
            root: LayoutNode::Leaf(0),
            active_pane: 0,
        }
    }

    pub fn split_horizontal(&mut self, pane_id: usize, new_pane_id: usize) {
        self.split(pane_id, new_pane_id, Direction::Horizontal);
    }

    pub fn split_vertical(&mut self, pane_id: usize, new_pane_id: usize) {
        self.split(pane_id, new_pane_id, Direction::Vertical);
    }

    fn split(&mut self, target: usize, new_id: usize, direction: Direction) {
        self.root = self.split_node(&self.root, target, new_id, direction);
    }

    fn split_node(
        &self,
        node: &LayoutNode,
        target: usize,
        new_id: usize,
        direction: Direction,
    ) -> LayoutNode {
        match node {
            LayoutNode::Leaf(id) if *id == target => {
                LayoutNode::Split {
                    direction,
                    ratio: 0.5,
                    first: Box::new(LayoutNode::Leaf(*id)),
                    second: Box::new(LayoutNode::Leaf(new_id)),
                }
            }
            LayoutNode::Split { direction: d, ratio, first, second } => {
                LayoutNode::Split {
                    direction: *d,
                    ratio: *ratio,
                    first: Box::new(self.split_node(first, target, new_id, direction)),
                    second: Box::new(self.split_node(second, target, new_id, direction)),
                }
            }
            other => other.clone(),
        }
    }

    pub fn render(&self, frame: &mut Frame, area: Rect, panes: &HashMap<usize, Pane>) {
        self.render_node(frame, area, &self.root, panes);
    }

    fn render_node(
        &self,
        frame: &mut Frame,
        area: Rect,
        node: &LayoutNode,
        panes: &HashMap<usize, Pane>,
    ) {
        match node {
            LayoutNode::Leaf(id) => {
                if let Some(pane) = panes.get(id) {
                    let is_active = *id == self.active_pane;
                    render_pane(frame, area, pane, is_active);
                }
            }
            LayoutNode::Split { direction, ratio, first, second } => {
                let constraints = [
                    Constraint::Percentage((ratio * 100.0) as u16),
                    Constraint::Percentage(((1.0 - ratio) * 100.0) as u16),
                ];
                let chunks = Layout::default()
                    .direction(*direction)
                    .constraints(constraints)
                    .split(area);

                self.render_node(frame, chunks[0], first, panes);
                self.render_node(frame, chunks[1], second, panes);
            }
        }
    }
}

fn render_pane(frame: &mut Frame, area: Rect, pane: &Pane, is_active: bool) {
    let border_style = if is_active {
        Style::default().fg(Color::Cyan)
    } else {
        Style::default().fg(Color::Gray)
    };

    let block = Block::default()
        .title(format!("[{}] {}", pane.id, pane.title))
        .borders(Borders::ALL)
        .border_style(border_style);

    match &pane.content {
        PaneContent::AppLogs { lines, .. } => {
            let text: Vec<Line> = lines
                .iter()
                .skip(pane.scroll_offset)
                .map(|l| Line::from(l.as_str()))
                .collect();
            let paragraph = Paragraph::new(text).block(block);
            frame.render_widget(paragraph, area);
        }
        PaneContent::MeshStatus { stats } => {
            let text = format!(
                "Peers: {}\nDHT entries: {}\nUptime: {}",
                stats.peer_count, stats.dht_entries, stats.uptime
            );
            let paragraph = Paragraph::new(text).block(block);
            frame.render_widget(paragraph, area);
        }
        // ... other content types
    }
}
```

**Pros:**
- Single process, simple architecture
- Fast rendering (no IPC)
- Full control over layout

**Cons:**
- Must implement all pane types ourselves
- No true shell in panes (unless we embed PTY)

---

### Option B: PTY Multiplexer (Like tmux)

Each pane runs a real PTY (pseudo-terminal), allowing actual shell commands.

```rust
// rust/crates/macula-tui/src/pty_mux.rs

use portable_pty::{native_pty_system, CommandBuilder, PtySize};
use std::sync::Arc;
use tokio::sync::mpsc;

pub struct PtyPane {
    pub id: usize,
    pub master: Box<dyn portable_pty::MasterPty + Send>,
    pub child: Box<dyn portable_pty::Child + Send>,
    pub reader: mpsc::Receiver<Vec<u8>>,
    pub output_buffer: Vec<u8>,
}

impl PtyPane {
    pub fn spawn(id: usize, command: &str, args: &[&str]) -> anyhow::Result<Self> {
        let pty_system = native_pty_system();

        let pair = pty_system.openpty(PtySize {
            rows: 24,
            cols: 80,
            pixel_width: 0,
            pixel_height: 0,
        })?;

        let mut cmd = CommandBuilder::new(command);
        cmd.args(args);

        let child = pair.slave.spawn_command(cmd)?;

        // Async reader for PTY output
        let mut reader = pair.master.try_clone_reader()?;
        let (tx, rx) = mpsc::channel(1024);

        std::thread::spawn(move || {
            let mut buf = [0u8; 1024];
            loop {
                match reader.read(&mut buf) {
                    Ok(0) => break,
                    Ok(n) => {
                        if tx.blocking_send(buf[..n].to_vec()).is_err() {
                            break;
                        }
                    }
                    Err(_) => break,
                }
            }
        });

        Ok(Self {
            id,
            master: pair.master,
            child,
            reader: rx,
            output_buffer: Vec::new(),
        })
    }

    pub fn write(&mut self, data: &[u8]) -> anyhow::Result<()> {
        self.master.write_all(data)?;
        Ok(())
    }

    pub fn resize(&mut self, rows: u16, cols: u16) -> anyhow::Result<()> {
        self.master.resize(PtySize {
            rows,
            cols,
            pixel_width: 0,
            pixel_height: 0,
        })?;
        Ok(())
    }

    pub fn poll_output(&mut self) -> Option<Vec<u8>> {
        match self.reader.try_recv() {
            Ok(data) => {
                self.output_buffer.extend(&data);
                Some(data)
            }
            Err(_) => None,
        }
    }
}
```

**Terminal Emulation:**

To properly render PTY output, we need a terminal emulator:

```rust
use vte::{Parser, Perform};
use alacritty_terminal::Term;  // Or custom implementation

pub struct TerminalEmulator {
    term: Term<()>,
    parser: Parser,
}

impl TerminalEmulator {
    pub fn new(rows: u16, cols: u16) -> Self {
        let size = SizeInfo::new(cols as f32, rows as f32, 1.0, 1.0, 0.0, 0.0, false);
        let term = Term::new(Config::default(), size, ());
        Self {
            term,
            parser: Parser::new(),
        }
    }

    pub fn process(&mut self, data: &[u8]) {
        for byte in data {
            self.parser.advance(&mut self.term, *byte);
        }
    }

    pub fn render(&self) -> Vec<Vec<Cell>> {
        // Convert terminal grid to renderable cells
        self.term.grid().display_iter().collect()
    }
}
```

**Pros:**
- True terminal in each pane
- Can run any CLI tool
- Familiar tmux-like experience

**Cons:**
- More complex (PTY management, terminal emulation)
- Heavier dependencies
- Need to handle resize, signals properly

---

### Option C: Hybrid Approach (Recommended)

Combine both: Some panes are "virtual" (logs, status), others are real PTYs.

```rust
pub enum PaneContent {
    // Virtual panes - rendered by TUI directly
    Logs(LogViewer),
    Status(StatusWidget),
    GitOps(GitOpsWidget),

    // PTY panes - real terminal
    Shell(PtyPane),
    AppAttach(PtyPane),  // Attached to running app's stdout
}

pub struct Multiplexer {
    layout: PaneLayout,
    panes: HashMap<usize, Pane>,
    next_id: usize,
}

impl Multiplexer {
    pub fn new_log_pane(&mut self, title: &str, source: LogSource) -> usize {
        let id = self.next_id;
        self.next_id += 1;

        self.panes.insert(id, Pane {
            id,
            title: title.to_string(),
            content: PaneContent::Logs(LogViewer::new(source)),
        });

        id
    }

    pub fn new_shell_pane(&mut self) -> anyhow::Result<usize> {
        let id = self.next_id;
        self.next_id += 1;

        let pty = PtyPane::spawn(id, "/bin/sh", &[])?;

        self.panes.insert(id, Pane {
            id,
            title: "Shell".to_string(),
            content: PaneContent::Shell(pty),
        });

        Ok(id)
    }

    pub fn attach_app(&mut self, app_name: &str) -> anyhow::Result<usize> {
        let id = self.next_id;
        self.next_id += 1;

        // Attach to app's stdout via TUI socket
        let pty = PtyPane::spawn(id, "macula", &["attach", app_name])?;

        self.panes.insert(id, Pane {
            id,
            title: format!("App: {}", app_name),
            content: PaneContent::AppAttach(pty),
        });

        Ok(id)
    }
}
```

---

## Key Bindings

Tmux-compatible bindings with Macula-specific additions:

```
┌─────────────────────────────────────────────────────────────────┐
│                     Key Bindings                                 │
├─────────────────────────────────────────────────────────────────┤
│                                                                  │
│  Prefix: Ctrl+B (configurable)                                  │
│                                                                  │
│  Pane Management:                                                │
│  ├── |       Split vertical                                     │
│  ├── -       Split horizontal                                   │
│  ├── x       Close pane                                         │
│  ├── z       Zoom pane (toggle fullscreen)                      │
│  ├── ←↑→↓   Navigate panes                                      │
│  └── q       Show pane numbers, then press number to jump       │
│                                                                  │
│  Pane Resize:                                                    │
│  ├── Alt+←↑→↓   Resize active pane                              │
│  └── =          Reset all panes to equal size                   │
│                                                                  │
│  Window Management:                                              │
│  ├── c       Create new window (tab)                            │
│  ├── n/p     Next/previous window                               │
│  ├── 0-9     Jump to window by number                           │
│  └── ,       Rename window                                      │
│                                                                  │
│  Macula-Specific:                                                │
│  ├── a       Open app selector (fuzzy find)                     │
│  ├── l       Open log viewer for active app                     │
│  ├── g       Open GitOps status pane                            │
│  ├── m       Open mesh status pane                              │
│  └── s       Open shell pane                                    │
│                                                                  │
│  Scrollback:                                                     │
│  ├── [       Enter copy mode (vim-like navigation)              │
│  ├── /       Search in pane                                     │
│  └── PgUp/Dn Scroll history                                     │
│                                                                  │
└─────────────────────────────────────────────────────────────────┘
```

---

## Session Persistence

Save and restore multiplexer sessions:

```rust
#[derive(Serialize, Deserialize)]
pub struct SessionConfig {
    pub name: String,
    pub layout: LayoutNode,
    pub panes: Vec<PaneConfig>,
}

#[derive(Serialize, Deserialize)]
pub struct PaneConfig {
    pub id: usize,
    pub title: String,
    pub pane_type: PaneType,
    pub command: Option<String>,  // For shell panes
}

#[derive(Serialize, Deserialize)]
pub enum PaneType {
    Logs { source: String },
    MeshStatus,
    GitOps,
    Shell,
    AppAttach { app: String },
}

impl SessionConfig {
    pub fn save(&self, path: &Path) -> anyhow::Result<()> {
        let json = serde_json::to_string_pretty(self)?;
        std::fs::write(path, json)?;
        Ok(())
    }

    pub fn load(path: &Path) -> anyhow::Result<Self> {
        let json = std::fs::read_to_string(path)?;
        Ok(serde_json::from_str(&json)?)
    }
}
```

---

## Integration with BEAM Backend

The multiplexer connects to the BEAM backend via WebSocket:

```rust
impl Multiplexer {
    pub async fn connect_backend(&mut self, url: &str) -> anyhow::Result<()> {
        let (client, mut events) = MaculaClient::connect(url).await?;

        // Subscribe to topics for virtual panes
        client.subscribe(vec![
            "mesh:stats".to_string(),
            "mesh:peers".to_string(),
            "gitops:status".to_string(),
            "apps:logs:*".to_string(),
        ]).await?;

        // Handle incoming events
        tokio::spawn(async move {
            while let Some(event) = events.recv().await {
                match event.topic.as_str() {
                    "mesh:stats" => {
                        // Update mesh status pane
                    }
                    "gitops:status" => {
                        // Update gitops pane
                    }
                    topic if topic.starts_with("apps:logs:") => {
                        // Append to app log pane
                    }
                    _ => {}
                }
            }
        });

        self.client = Some(client);
        Ok(())
    }
}
```

---

## Summary

```
┌─────────────────────────────────────────────────────────────────┐
│              TUI Multiplexer Summary                             │
├─────────────────────────────────────────────────────────────────┤
│                                                                  │
│  Approach: Hybrid (virtual panes + PTY panes)                   │
│                                                                  │
│  Virtual Panes (rendered directly):                             │
│  ├── Log viewers (streaming from BEAM)                          │
│  ├── Status widgets (mesh, gitops)                              │
│  └── Custom dashboards                                          │
│                                                                  │
│  PTY Panes (real terminals):                                    │
│  ├── Interactive shell                                          │
│  ├── Attached app stdout                                        │
│  └── Any CLI tool                                               │
│                                                                  │
│  Key Features:                                                   │
│  ├── tmux-compatible keybindings                                │
│  ├── Session persistence                                        │
│  ├── Split vertical/horizontal                                  │
│  ├── Zoom pane to fullscreen                                    │
│  └── Macula-specific shortcuts (apps, logs, gitops)             │
│                                                                  │
│  Dependencies:                                                   │
│  ├── ratatui (TUI framework)                                    │
│  ├── portable-pty (PTY handling)                                │
│  ├── vte / alacritty-terminal (terminal emulation)              │
│  └── tokio (async runtime)                                      │
│                                                                  │
│  Integration:                                                    │
│  ├── WebSocket to BEAM backend                                  │
│  ├── Streams logs from Phoenix.PubSub                           │
│  └── Commands via request/response                              │
│                                                                  │
└─────────────────────────────────────────────────────────────────┘
```

## Open Questions

1. **Copy/Paste:** How to handle clipboard across SSH?
   - Option A: OSC 52 escape sequences
   - Option B: tmux-style buffer

2. **Mouse Support:** Enable or keep keyboard-only?
   - tmux supports mouse, but keyboard purists prefer without

3. **Theming:** Should multiplexer support custom color schemes?
   - Could inherit from system/terminal theme

4. **Remote Access:** Should multiplexer support attaching from another machine?
   - Like `tmux attach` but over mesh

## Next Steps

1. Implement basic pane layout with ratatui
2. Add virtual panes (logs, status)
3. Add PTY pane support
4. Implement key bindings
5. Add session persistence
6. Connect to BEAM backend
