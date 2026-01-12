# Exploration: Front End Web/TUI App Launcher

**Status:** Exploration / RFC
**Created:** 2026-01-12

## Premise

What if we abandoned Kubernetes entirely and built MaculaOS on a pure BEAM foundation?

The BEAM VM already provides many primitives that Kubernetes was designed to solve:

| Kubernetes Concept | BEAM Equivalent                   |
| ------------------ | --------------------------------- |
| Container restart  | Supervisor restart strategies     |
| Pod networking     | Distributed Erlang / Partisan     |
| Rolling deployment | Hot code upgrades                 |
| Container image    | OTP release                       |
| Service discovery  | pg / syn / gproc                  |
| ConfigMaps         | Application env / persistent_term |
| Health checks      | Supervisor / heart                |
| Horizontal scaling | Add nodes to cluster              |

## The Three Pillars

### 1. Pure BEAM Runtime (No k3s)

Replace k3s with a native BEAM application supervisor:

```
┌─────────────────────────────────────────────────────────────┐
│                    MaculaOS Runtime                         │
├─────────────────────────────────────────────────────────────┤
│                                                             │
│  ┌─────────────────────────────────────────────────────┐   │
│  │              macula_runtime (supervisor)             │   │
│  ├─────────────────────────────────────────────────────┤   │
│  │                                                     │   │
│  │  ┌──────────┐ ┌──────────┐ ┌──────────┐            │   │
│  │  │ App 1    │ │ App 2    │ │ App N    │            │   │
│  │  │(release) │ │(release) │ │(release) │            │   │
│  │  └──────────┘ └──────────┘ └──────────┘            │   │
│  │                                                     │   │
│  │  ┌──────────────────────────────────────────────┐  │   │
│  │  │           macula_mesh (networking)            │  │   │
│  │  │  - Partisan clustering                       │  │   │
│  │  │  - QUIC transport                            │  │   │
│  │  │  - DHT discovery                             │  │   │
│  │  └──────────────────────────────────────────────┘  │   │
│  │                                                     │   │
│  │  ┌──────────────────────────────────────────────┐  │   │
│  │  │           macula_gitops (reconciler)          │  │   │
│  │  │  - Watches git repo                          │  │   │
│  │  │  - Parses desired state                      │  │   │
│  │  │  - Deploys/upgrades releases                 │  │   │
│  │  └──────────────────────────────────────────────┘  │   │
│  │                                                     │   │
│  └─────────────────────────────────────────────────────┘   │
│                                                             │
│  ┌─────────────────────────────────────────────────────┐   │
│  │                 NATS Server (embedded)               │   │
│  │  - Local pub/sub bus                                │   │
│  │  - Bridge to external clients (TUI, CLI)            │   │
│  │  - Cluster-wide via NATS clustering or mesh         │   │
│  └─────────────────────────────────────────────────────┘   │
│                                                             │
└─────────────────────────────────────────────────────────────┘
```

**Benefits:**

- Single runtime (BEAM) instead of BEAM + k3s + containerd
- Native hot code upgrades (no pod restarts)
- Smaller footprint (~50MB vs ~200MB for k3s)
- Simpler networking (no CNI, no iptables chaos)
- True fault tolerance (let-it-crash, not restart-and-pray)

**Challenges:**

- No container isolation (all apps share BEAM VM)
- Need to build release management tooling
- Less ecosystem (no Helm charts, operators)

### 2. BEAM-Native GitOps

Instead of Flux watching git and applying YAML to k8s API, we build a native reconciler:

```erlang
%% macula_gitops_reconciler.erl

-module(macula_gitops_reconciler).
-behaviour(gen_server).

%% Reconciliation loop
handle_info(reconcile, State) ->
    {ok, DesiredState} = fetch_git_repo(State#state.repo_url),
    CurrentState = get_running_apps(),
    Actions = diff_states(DesiredState, CurrentState),
    lists:foreach(fun execute_action/1, Actions),
    erlang:send_after(?RECONCILE_INTERVAL, self(), reconcile),
    {noreply, State}.

%% Actions
execute_action({deploy, AppSpec}) ->
    {ok, Release} = download_release(AppSpec),
    macula_runtime:start_app(Release);

execute_action({upgrade, AppName, NewVersion}) ->
    macula_runtime:hot_upgrade(AppName, NewVersion);

execute_action({stop, AppName}) ->
    macula_runtime:stop_app(AppName).
```

**GitOps Config Format:**

Option A: Erlang terms (native, no parsing)

```erlang
%% gitops/apps/console.app.config
#{
    name => macula_console,
    version => "1.0.0",
    release_url => "https://releases.macula.io/console/1.0.0.tar.gz",
    env => #{
        port => 4000,
        host => "console.macula.io"
    },
    replicas => 1,  %% For distributed deployment
    health_check => {http, "/health", 4000}
}.
```

Option B: YAML/TOML (familiar to k8s users)

```yaml
# gitops/apps/console.yaml
name: macula_console
version: "1.0.0"
release_url: https://releases.macula.io/console/1.0.0.tar.gz
env:
  port: 4000
  host: console.macula.io
replicas: 1
health_check:
  type: http
  path: /health
  port: 4000
```

Option C: Elixir DSL (expressive, type-safe)

```elixir
# gitops/apps/console.exs
app :macula_console do
  version "1.0.0"
  release "https://releases.macula.io/console/1.0.0.tar.gz"

  env do
    port 4000
    host "console.macula.io"
  end

  health_check :http, path: "/health", port: 4000
end
```

**Reconciliation Features:**

- Poll git repo (configurable interval, default 1 min)
- Webhook support for instant reconciliation
- Drift detection and auto-correction
- Rollback on failed health checks
- Status reporting (to git, to mesh, to TUI)

### 3. Multi-Client Architecture via NATS

The key insight: **Phoenix LiveView and TUIs are both just views into backend state.**

```
┌─────────────────────────────────────────────────────────────────┐
│                        Backend (BEAM)                           │
├─────────────────────────────────────────────────────────────────┤
│                                                                 │
│  ┌─────────────────────────────────────────────────────────┐   │
│  │                   Application State                      │   │
│  │  - Running apps, versions, health                       │   │
│  │  - Mesh topology, peers, latency                        │   │
│  │  - GitOps status, last sync, drift                      │   │
│  └────────────────────────┬────────────────────────────────┘   │
│                           │                                     │
│                           ▼                                     │
│  ┌─────────────────────────────────────────────────────────┐   │
│  │               Event Bus (Phoenix.PubSub)                 │   │
│  │                                                         │   │
│  │  Topics:                                                │   │
│  │    macula.apps.state_changed                            │   │
│  │    macula.mesh.peer_joined                              │   │
│  │    macula.mesh.peer_left                                │   │
│  │    macula.gitops.sync_completed                         │   │
│  │    macula.gitops.drift_detected                         │   │
│  │    macula.system.metrics                                │   │
│  └────────────────────────┬────────────────────────────────┘   │
│                           │                                     │
│              ┌────────────┴────────────┐                       │
│              ▼                         ▼                        │
│  ┌──────────────────────┐  ┌──────────────────────┐            │
│  │  Phoenix.Endpoint    │  │  NATS Bridge         │            │
│  │  (WebSocket)         │  │  (macula_nats_bridge)│            │
│  └──────────┬───────────┘  └──────────┬───────────┘            │
│             │                         │                         │
└─────────────┼─────────────────────────┼─────────────────────────┘
              │                         │
              ▼                         ▼
    ┌─────────────────┐      ┌─────────────────────────────┐
    │    Browser      │      │      NATS Protocol          │
    │  (LiveView)     │      │                             │
    └─────────────────┘      │  ┌───────┐ ┌───────┐       │
                             │  │Rust   │ │Go     │       │
                             │  │TUI    │ │TUI    │       │
                             │  └───────┘ └───────┘       │
                             │                             │
                             │  ┌───────┐ ┌───────┐       │
                             │  │Python │ │CLI    │       │
                             │  │TUI    │ │tools  │       │
                             │  └───────┘ └───────┘       │
                             └─────────────────────────────┘
```

**NATS Bridge (Elixir):**

```elixir
defmodule Macula.NatsBridge do
  use GenServer

  @nats_url "nats://localhost:4222"

  def start_link(opts) do
    GenServer.start_link(__MODULE__, opts, name: __MODULE__)
  end

  def init(_opts) do
    {:ok, conn} = Gnat.start_link(%{host: "localhost", port: 4222})

    # Subscribe to Phoenix PubSub and forward to NATS
    Phoenix.PubSub.subscribe(Macula.PubSub, "macula.*")

    # Subscribe to NATS and forward to Phoenix PubSub
    Gnat.sub(conn, self(), "macula.>")

    {:ok, %{conn: conn}}
  end

  # Phoenix → NATS
  def handle_info(%Phoenix.Socket.Broadcast{topic: topic, event: event, payload: payload}, state) do
    nats_subject = phoenix_topic_to_nats(topic, event)
    Gnat.pub(state.conn, nats_subject, Jason.encode!(payload))
    {:noreply, state}
  end

  # NATS → Phoenix
  def handle_info({:msg, %{topic: subject, body: body}}, state) do
    {topic, event} = nats_subject_to_phoenix(subject)
    payload = Jason.decode!(body)
    Phoenix.PubSub.broadcast(Macula.PubSub, topic, {event, payload})
    {:noreply, state}
  end
end
```

**TUI Protocol:**

```
NATS Subject                    | Direction | Payload
--------------------------------|-----------|----------------------------------
macula.apps.list                | TUI→BEAM  | {} (request)
macula.apps.list.response       | BEAM→TUI  | [{name, version, status}, ...]
macula.apps.start               | TUI→BEAM  | {app: "console"}
macula.apps.started             | BEAM→TUI  | {app: "console", pid: "..."}
macula.mesh.peers               | TUI→BEAM  | {} (request)
macula.mesh.peers.response      | BEAM→TUI  | [{node, ip, latency}, ...]
macula.mesh.peer_joined         | BEAM→TUI  | {node: "...", ip: "..."}
macula.gitops.status            | TUI→BEAM  | {} (request)
macula.gitops.status.response   | BEAM→TUI  | {last_sync, drift, apps}
macula.gitops.sync              | TUI→BEAM  | {} (trigger manual sync)
macula.system.metrics           | BEAM→TUI  | {cpu, mem, disk, ...} (periodic)
```

**Rust TUI Client:**

```rust
// macula-tui/src/nats_client.rs

use async_nats::Client;
use tokio::sync::mpsc;

pub struct MaculaClient {
    nats: Client,
    rx: mpsc::Receiver<MaculaEvent>,
}

impl MaculaClient {
    pub async fn connect(url: &str) -> Result<Self> {
        let nats = async_nats::connect(url).await?;
        let (tx, rx) = mpsc::channel(100);

        // Subscribe to all macula events
        let mut sub = nats.subscribe("macula.>").await?;

        tokio::spawn(async move {
            while let Some(msg) = sub.next().await {
                let event = parse_event(&msg);
                tx.send(event).await.ok();
            }
        });

        Ok(Self { nats, rx })
    }

    pub async fn list_apps(&self) -> Result<Vec<AppInfo>> {
        let response = self.nats
            .request("macula.apps.list", "{}".into())
            .await?;
        Ok(serde_json::from_slice(&response.payload)?)
    }

    pub async fn start_app(&self, name: &str) -> Result<()> {
        self.nats
            .publish("macula.apps.start", format!(r#"{{"app":"{}"}}"#, name).into())
            .await?;
        Ok(())
    }

    pub fn recv_event(&mut self) -> Option<MaculaEvent> {
        self.rx.try_recv().ok()
    }
}
```

## TUI Framework Options

| Language   | Framework       | Pros                     | Cons                  |
| ---------- | --------------- | ------------------------ | --------------------- |
| **Rust**   | ratatui         | Fast, safe, we have it   | Compile times         |
| **Go**     | bubbletea       | Simple, fast compile     | Less expressive       |
| **Go**     | tview           | Rich widgets             | Heavier               |
| **Python** | textual         | Rapid dev, CSS styling   | Slower, needs runtime |
| **Zig**    | (custom)        | Tiny binary, embedded    | Immature ecosystem    |
| **Elixir** | owl/ratatouille | Same language as backend | Less mature           |

**Recommendation:** Keep Rust (ratatui) as primary, but design protocol so any language can build a client.

## System Architecture

```
┌─────────────────────────────────────────────────────────────────┐
│                     MaculaOS (Pure BEAM)                        │
│                        (~80MB total)                            │
├─────────────────────────────────────────────────────────────────┤
│                                                                 │
│  Linux Kernel (minimal)                              ~15MB      │
│  ├── Network drivers                                            │
│  ├── Storage drivers                                            │
│  └── Essential modules                                          │
│                                                                 │
│  Alpine Base (minimal)                               ~20MB      │
│  ├── busybox                                                    │
│  ├── openrc                                                     │
│  └── networking                                                 │
│                                                                 │
│  BEAM Runtime                                        ~15MB      │
│  ├── ERTS (Erlang Runtime System)                              │
│  └── Essential OTP apps                                         │
│                                                                 │
│  Macula Runtime (single release)                     ~20MB      │
│  ├── macula_runtime     (app supervisor)                       │
│  ├── macula_gitops      (reconciler)                           │
│  ├── macula_mesh        (networking)                           │
│  ├── macula_nats_bridge (TUI connectivity)                     │
│  └── macula_console_web (Phoenix LiveView)                     │
│                                                                 │
│  NATS Server (embedded or standalone)                ~10MB      │
│                                                                 │
│  Rust TUI (optional, for local management)           ~5MB       │
│                                                                 │
└─────────────────────────────────────────────────────────────────┘

Total: ~80MB (vs ~1.5GB current, vs ~150MB with k3s lite)
```

## Comparison

| Aspect           | k8s/Flux               | Pure BEAM          |
| ---------------- | ---------------------- | ------------------ |
| Runtime overhead | ~200MB (k3s)           | ~15MB (ERTS)       |
| Startup time     | ~30s                   | ~5s                |
| Hot upgrades     | No (pod restart)       | Yes (native)       |
| Networking       | CNI + iptables         | Distributed Erlang |
| Config format    | YAML manifests         | Erlang terms / DSL |
| Ecosystem        | Huge (Helm, operators) | Small (custom)     |
| Learning curve   | Steep                  | Steep (different)  |
| Isolation        | Strong (containers)    | Weak (shared VM)   |
| Multi-tenant     | Yes                    | Needs work         |

## Migration Path

### Phase 1: Add NATS Bridge

- Keep current architecture (k3s)
- Add macula_nats_bridge to Phoenix app
- Update Rust TUI to use NATS instead of direct API calls
- Validate TUI works via NATS

### Phase 2: Build macula_gitops

- Create Erlang/Elixir GitOps reconciler
- Run alongside Flux initially
- Validate it can deploy OTP releases

### Phase 3: Build macula_runtime

- Create app supervisor that manages OTP releases
- Implement health checks, restart policies
- Test with simple apps

### Phase 4: MaculaOS Pure BEAM

- New build pipeline: Linux + BEAM + Macula Runtime
- Remove k3s dependency
- Single ~80MB image

## Deep Dive: Existing NATS Bridge Architecture

The macula-console codebase already has a working NATS bridge implementation that we can build upon.

### Current Layer Architecture

```
┌──────────────────────────────────────────────────────────────────────────┐
│                          Layer 5: Views                                   │
│  ┌─────────────────────────────┐  ┌─────────────────────────────────┐    │
│  │    Phoenix LiveViews        │  │         TUI Clients             │    │
│  │  (subscribe to PubSub)      │  │   (subscribe to NATS)           │    │
│  └──────────────┬──────────────┘  └────────────────┬────────────────┘    │
│                 │                                   │                     │
├─────────────────┼───────────────────────────────────┼─────────────────────┤
│                 ▼                                   ▼                     │
│  Layer 4: Event Distribution                                              │
│  ┌─────────────────────────────┐  ┌─────────────────────────────────┐    │
│  │    Phoenix.PubSub           │  │       NATS Server               │    │
│  │  Topics: mesh:stats,        │  │  Subjects: macula.>             │    │
│  │  mesh:peers, mesh:dht, etc  │  │                                 │    │
│  └──────────────┬──────────────┘  └────────────────┬────────────────┘    │
│                 │                                   │                     │
│                 └─────────────┬─────────────────────┘                     │
│                               ▼                                           │
├───────────────────────────────────────────────────────────────────────────┤
│  Layer 3: Bridges                                                         │
│  ┌─────────────────────────────┐  ┌─────────────────────────────────┐    │
│  │  MaculaCluster.Mesh.Monitor │  │  MaculaGateway.NatsBridge       │    │
│  │  - Polls Erlang every 5s    │  │  - Subscribes to NATS (>)       │    │
│  │  - Broadcasts to PubSub     │  │  - Routes to/from mesh          │    │
│  │  - mesh:stats, mesh:peers   │  │  - 1:1 topic mapping            │    │
│  └──────────────┬──────────────┘  └────────────────┬────────────────┘    │
│                 │                                   │                     │
├─────────────────┼───────────────────────────────────┼─────────────────────┤
│                 ▼                                   ▼                     │
│  Layer 2: Gateway Bridge                                                  │
│  ┌──────────────────────────────────────────────────────────────────┐    │
│  │                MaculaGateway.MeshBridge                           │    │
│  │  - call_rpc(realm, procedure, args) → mesh RPC                   │    │
│  │  - publish(realm, topic, data) → mesh pub/sub                    │    │
│  │  - subscribe(realm, pattern, handler) → mesh subscription        │    │
│  │  - register_handler(realm, procedure, handler) → mesh advertise  │    │
│  └──────────────────────────────────┬───────────────────────────────┘    │
│                                     │                                     │
├─────────────────────────────────────┼─────────────────────────────────────┤
│                                     ▼                                     │
│  Layer 1: Mesh Interface                                                  │
│  ┌──────────────────────────────────────────────────────────────────┐    │
│  │                   MaculaCluster.Mesh (GenServer)                  │    │
│  │  - Wraps :macula Erlang library                                  │    │
│  │  - Manages client lifecycle, reconnection                        │    │
│  │  - Tracks subscriptions and registrations                        │    │
│  └──────────────────────────────────┬───────────────────────────────┘    │
│                                     │                                     │
├─────────────────────────────────────┼─────────────────────────────────────┤
│                                     ▼                                     │
│  Layer 0: Erlang Mesh Library                                             │
│  ┌──────────────────────────────────────────────────────────────────┐    │
│  │                        :macula (Erlang)                           │    │
│  │  - connect_local/1: Get local client                             │    │
│  │  - call/4: RPC call to mesh                                      │    │
│  │  - publish/3: Publish event to mesh                              │    │
│  │  - subscribe/3: Subscribe to mesh events                         │    │
│  │  - advertise/3: Register as RPC handler                          │    │
│  │  - discover_subscribers/2: Find services                         │    │
│  └──────────────────────────────────────────────────────────────────┘    │
└──────────────────────────────────────────────────────────────────────────┘
```

### Existing NatsBridge Implementation

**Location:** `macula-console/system/apps/macula_gateway/lib/macula_gateway/nats_bridge.ex`

**Key Features:**

1. **Egress (NATS → Mesh):**

   ```elixir
   # Subscribes to ALL NATS subjects with queue group
   Gnat.sub(conn, self(), ">", queue_group: "macula-gateway")

   # Routes based on reply_to field:
   # - With reply_to → RPC (nc.Request)
   # - Without reply_to → Pub/Sub (nc.Publish)
   ```

2. **Ingress (Mesh → NATS):**

   ```elixir
   # Services register handlers
   NatsBridge.register_handler("io.macula.orders.created", handler_pid)

   # When mesh events arrive, published to NATS
   NatsBridge.publish_to_local(topic, data)
   ```

3. **Topic Mapping:** 1:1 - No translation

   ```
   NATS Subject: io.macula.orders.created
   Mesh Topic:   io.macula.orders.created
   ```

4. **Realm Extraction:**
   ```elixir
   # "io.macula.orders.created" → "io.macula"
   extract_realm("io.macula.orders.created")
   ```

### Existing Monitor Implementation

**Location:** `macula-console/system/apps/macula_cluster/lib/macula_cluster/mesh/monitor.ex`

**Key Features:**

1. **Polling:** Every 5 seconds
2. **State Fetched:**
   - Gateway stats (clients, registrations, uptime)
   - NAT status (type, relay sessions)
   - Peers (connection pool)
   - DHT state (routing table, entries)
   - PubSub state (subscriptions, message rates)

3. **Phoenix.PubSub Topics:**
   ```elixir
   "mesh:stats"    → {:mesh_stats, stats_map}
   "mesh:nat"      → {:mesh_nat, nat_status}
   "mesh:peers"    → {:mesh_peers, peers_list}
   "mesh:dht"      → {:mesh_dht, dht_state}
   "mesh:pubsub"   → {:mesh_pubsub, pubsub_state}
   "mesh:topology" → {:mesh_topology, topology_map}
   "mesh:status"   → {:mesh_status, boolean}
   ```

### Gap Analysis: What's Missing for TUI

The current architecture has:

- ✅ NatsBridge for external services (NATS ↔ Mesh for business events)
- ✅ Monitor for internal state (Mesh → Phoenix.PubSub for LiveViews)

What's missing for TUI clients:

- ❌ Monitor doesn't publish to NATS (only Phoenix.PubSub)
- ❌ No NATS subjects for system state (apps, gitops, etc.)
- ❌ No request/response pattern for TUI commands

### Proposed Enhancement: State Bridge

Add a new module that bridges Monitor state to NATS:

```elixir
defmodule MaculaGateway.StateBridge do
  @moduledoc """
  Bridges internal state (Phoenix.PubSub) to NATS for TUI clients.

  Subscribes to Monitor's PubSub topics and republishes to NATS.
  Also handles TUI request/response commands.
  """
  use GenServer

  @pubsub MaculaCluster.PubSub

  # Phoenix.PubSub topic → NATS subject mapping
  @topic_mapping %{
    "mesh:stats" => "macula.mesh.stats",
    "mesh:nat" => "macula.mesh.nat",
    "mesh:peers" => "macula.mesh.peers",
    "mesh:dht" => "macula.mesh.dht",
    "mesh:pubsub" => "macula.mesh.pubsub",
    "mesh:topology" => "macula.mesh.topology",
    "mesh:status" => "macula.mesh.status"
  }

  def init(_opts) do
    # Subscribe to all Monitor topics
    for {topic, _} <- @topic_mapping do
      Phoenix.PubSub.subscribe(@pubsub, topic)
    end

    {:ok, conn} = Gnat.start_link(%{host: "localhost", port: 4222})

    # Subscribe to TUI commands (requests with reply-to)
    Gnat.sub(conn, self(), "macula.cmd.>")

    {:ok, %{conn: conn}}
  end

  # Forward Phoenix.PubSub → NATS
  def handle_info({:mesh_stats, stats}, state) do
    Gnat.pub(state.conn, "macula.mesh.stats", Jason.encode!(stats))
    {:noreply, state}
  end

  def handle_info({:mesh_peers, peers}, state) do
    Gnat.pub(state.conn, "macula.mesh.peers", Jason.encode!(peers))
    {:noreply, state}
  end

  # ... similar for other topics

  # Handle TUI commands (NATS → Action → NATS reply)
  def handle_info({:msg, %{topic: "macula.cmd.apps.list", reply_to: inbox}}, state) do
    apps = MaculaRuntime.list_apps()
    Gnat.pub(state.conn, inbox, Jason.encode!(apps))
    {:noreply, state}
  end

  def handle_info({:msg, %{topic: "macula.cmd.apps.start", body: body, reply_to: inbox}}, state) do
    %{"app" => name} = Jason.decode!(body)
    result = MaculaRuntime.start_app(name)
    Gnat.pub(state.conn, inbox, Jason.encode!(result))
    {:noreply, state}
  end
end
```

### TUI Protocol Definition

**State Updates (Broadcast):**

```
NATS Subject                | Event Payload
----------------------------|--------------------------------------------------
macula.mesh.stats          | {clients, registrations, subscriptions, uptime}
macula.mesh.nat            | {type, public_ip, relay_enabled, relay_sessions}
macula.mesh.peers          | [{node_id, ip, latency, status}, ...]
macula.mesh.dht            | {node_id, routing_table_size, entries}
macula.mesh.pubsub         | {subscriptions, topics, messages_in, messages_out}
macula.mesh.topology       | {nodes: [...], links: [...]}
macula.mesh.status         | true | false
macula.apps.state          | [{name, version, status, health}, ...]
macula.gitops.status       | {last_sync, drift_detected, pending_changes}
macula.system.metrics      | {cpu, memory, disk, network}
```

**Commands (Request/Response via nc.Request):**

```
NATS Subject                | Request Payload      | Response Payload
----------------------------|---------------------|---------------------------
macula.cmd.apps.list       | {}                  | [{name, version, status}, ...]
macula.cmd.apps.start      | {app: "name"}       | {ok: true} | {error: "reason"}
macula.cmd.apps.stop       | {app: "name"}       | {ok: true} | {error: "reason"}
macula.cmd.apps.restart    | {app: "name"}       | {ok: true} | {error: "reason"}
macula.cmd.gitops.sync     | {}                  | {ok: true, changes: [...]}
macula.cmd.gitops.status   | {}                  | {last_sync, drift, apps}
macula.cmd.mesh.peers      | {}                  | [{node_id, ip, latency}, ...]
macula.cmd.mesh.discover   | {pattern: "io.*"}   | [{procedure, node_id}, ...]
```

### Rust TUI Integration

The existing `macula-wizard` and `macula-tui` can be updated to use NATS:

```rust
// rust/crates/macula-tui/src/nats_client.rs

use async_nats::Client;
use serde::{Deserialize, Serialize};

#[derive(Debug, Clone)]
pub struct MaculaClient {
    nats: Client,
}

#[derive(Debug, Deserialize)]
pub struct MeshStats {
    pub clients: u64,
    pub registrations: u64,
    pub subscriptions: u64,
    pub uptime: u64,
    pub status: String,
}

#[derive(Debug, Deserialize)]
pub struct AppInfo {
    pub name: String,
    pub version: String,
    pub status: String,
    pub health: String,
}

impl MaculaClient {
    pub async fn connect(url: &str) -> anyhow::Result<Self> {
        let nats = async_nats::connect(url).await?;
        Ok(Self { nats })
    }

    // Subscribe to state updates
    pub async fn subscribe_mesh_stats(&self) -> anyhow::Result<impl futures::Stream<Item = MeshStats>> {
        let sub = self.nats.subscribe("macula.mesh.stats").await?;
        Ok(sub.map(|msg| serde_json::from_slice(&msg.payload).unwrap()))
    }

    // Request/response commands
    pub async fn list_apps(&self) -> anyhow::Result<Vec<AppInfo>> {
        let response = self.nats
            .request("macula.cmd.apps.list", "{}".into())
            .await?;
        Ok(serde_json::from_slice(&response.payload)?)
    }

    pub async fn start_app(&self, name: &str) -> anyhow::Result<()> {
        let payload = serde_json::json!({"app": name}).to_string();
        let response = self.nats
            .request("macula.cmd.apps.start", payload.into())
            .await?;
        let result: serde_json::Value = serde_json::from_slice(&response.payload)?;
        if result.get("error").is_some() {
            anyhow::bail!("{}", result["error"]);
        }
        Ok(())
    }
}
```

### Alternative: Raw WebSocket (Recommended)

After analysis, **Raw WebSocket** is preferred over NATS for TUI communication:

| Aspect           | NATS                | Raw WebSocket        |
| ---------------- | ------------------- | -------------------- |
| Rust client      | async-nats          | tungstenite (mature) |
| Extra components | NATS server (~15MB) | None                 |
| Protocol         | NATS protocol       | Custom (simple JSON) |
| Request/response | Native              | Simple to implement  |
| Size overhead    | ~15MB               | 0                    |

**Why Raw WebSocket wins for MaculaOS:**

1. TUI is local (same machine as BEAM)
2. Only Rust TUI initially
3. Simpler deployment (no NATS server)
4. Direct Phoenix.PubSub integration
5. Can add NATS later if multi-language TUIs needed

### Raw WebSocket Protocol Design

**Phoenix Socket Handler:**

```elixir
# lib/macula_cluster_web/channels/tui_socket.ex
defmodule MaculaClusterWeb.TuiSocket do
  @behaviour Phoenix.Socket.Transport

  @pubsub MaculaCluster.PubSub

  def child_spec(_opts), do: :ignore

  def connect(%{params: params} = state) do
    # Optional: verify auth token
    {:ok, %{subscriptions: MapSet.new()}}
  end

  def init(state) do
    {:ok, state}
  end

  def terminate(_reason, _state), do: :ok

  # Handle incoming messages from TUI
  def handle_in({:text, msg}, state) do
    case Jason.decode(msg) do
      {:ok, %{"type" => "subscribe", "topics" => topics}} ->
        new_subs = subscribe_topics(topics, state.subscriptions)
        {:ok, %{state | subscriptions: new_subs}}

      {:ok, %{"type" => "unsubscribe", "topics" => topics}} ->
        new_subs = unsubscribe_topics(topics, state.subscriptions)
        {:ok, %{state | subscriptions: new_subs}}

      {:ok, %{"type" => "cmd", "id" => id, "action" => action, "args" => args}} ->
        result = handle_command(action, args)
        response = Jason.encode!(%{type: "response", id: id, result: result})
        {:reply, :ok, {:text, response}, state}

      {:ok, %{"type" => "cmd", "id" => id, "action" => action}} ->
        result = handle_command(action, %{})
        response = Jason.encode!(%{type: "response", id: id, result: result})
        {:reply, :ok, {:text, response}, state}

      _ ->
        {:ok, state}
    end
  end

  def handle_in({:binary, _}, state), do: {:ok, state}

  # Forward PubSub events to TUI
  def handle_info({:mesh_stats, stats}, state) do
    msg = Jason.encode!(%{type: "event", topic: "mesh:stats", payload: stats})
    {:push, {:text, msg}, state}
  end

  def handle_info({:mesh_peers, peers}, state) do
    msg = Jason.encode!(%{type: "event", topic: "mesh:peers", payload: peers})
    {:push, {:text, msg}, state}
  end

  def handle_info({:mesh_topology, topology}, state) do
    msg = Jason.encode!(%{type: "event", topic: "mesh:topology", payload: topology})
    {:push, {:text, msg}, state}
  end

  def handle_info({:apps_state, apps}, state) do
    msg = Jason.encode!(%{type: "event", topic: "apps:state", payload: apps})
    {:push, {:text, msg}, state}
  end

  def handle_info(_, state), do: {:ok, state}

  # Subscribe to PubSub topics
  defp subscribe_topics(topics, current) do
    Enum.reduce(topics, current, fn topic, acc ->
      unless MapSet.member?(acc, topic) do
        Phoenix.PubSub.subscribe(@pubsub, topic)
      end
      MapSet.put(acc, topic)
    end)
  end

  defp unsubscribe_topics(topics, current) do
    Enum.reduce(topics, current, fn topic, acc ->
      if MapSet.member?(acc, topic) do
        Phoenix.PubSub.unsubscribe(@pubsub, topic)
      end
      MapSet.delete(acc, topic)
    end)
  end

  # Command handlers
  defp handle_command("apps.list", _args) do
    MaculaRuntime.list_apps()
  end

  defp handle_command("apps.start", %{"name" => name}) do
    MaculaRuntime.start_app(name)
  end

  defp handle_command("apps.stop", %{"name" => name}) do
    MaculaRuntime.stop_app(name)
  end

  defp handle_command("apps.launch", %{"name" => name}) do
    # Launch app in foreground (TUI can display it)
    MaculaRuntime.launch_app(name)
  end

  defp handle_command("gitops.sync", _args) do
    MaculaGitops.sync()
  end

  defp handle_command("gitops.status", _args) do
    MaculaGitops.status()
  end

  defp handle_command("mesh.peers", _args) do
    MaculaCluster.Mesh.Monitor.get_peers()
  end

  defp handle_command("system.metrics", _args) do
    MaculaSystem.metrics()
  end

  defp handle_command(action, _args) do
    %{error: "unknown_command", action: action}
  end
end
```

**Rust TUI Client:**

```rust
// rust/crates/macula-tui/src/ws_client.rs

use futures_util::{SinkExt, StreamExt};
use serde::{Deserialize, Serialize};
use std::collections::HashMap;
use std::sync::atomic::{AtomicU64, Ordering};
use tokio::sync::{mpsc, oneshot};
use tokio_tungstenite::{connect_async, tungstenite::Message};

static REQUEST_ID: AtomicU64 = AtomicU64::new(1);

#[derive(Debug, Serialize)]
#[serde(tag = "type")]
enum OutgoingMessage {
    #[serde(rename = "subscribe")]
    Subscribe { topics: Vec<String> },
    #[serde(rename = "cmd")]
    Command { id: String, action: String, args: serde_json::Value },
}

#[derive(Debug, Deserialize)]
#[serde(tag = "type")]
enum IncomingMessage {
    #[serde(rename = "event")]
    Event { topic: String, payload: serde_json::Value },
    #[serde(rename = "response")]
    Response { id: String, result: serde_json::Value },
}

#[derive(Debug, Clone)]
pub struct MaculaClient {
    tx: mpsc::Sender<OutgoingMessage>,
    pending: std::sync::Arc<tokio::sync::Mutex<HashMap<String, oneshot::Sender<serde_json::Value>>>>,
}

impl MaculaClient {
    pub async fn connect(url: &str) -> anyhow::Result<(Self, mpsc::Receiver<Event>)> {
        let (ws_stream, _) = connect_async(url).await?;
        let (mut write, mut read) = ws_stream.split();

        let (tx, mut rx) = mpsc::channel::<OutgoingMessage>(100);
        let (event_tx, event_rx) = mpsc::channel::<Event>(100);
        let pending = std::sync::Arc::new(tokio::sync::Mutex::new(HashMap::new()));
        let pending_clone = pending.clone();

        // Writer task
        tokio::spawn(async move {
            while let Some(msg) = rx.recv().await {
                let json = serde_json::to_string(&msg).unwrap();
                write.send(Message::Text(json)).await.ok();
            }
        });

        // Reader task
        tokio::spawn(async move {
            while let Some(Ok(Message::Text(text))) = read.next().await {
                if let Ok(msg) = serde_json::from_str::<IncomingMessage>(&text) {
                    match msg {
                        IncomingMessage::Event { topic, payload } => {
                            event_tx.send(Event { topic, payload }).await.ok();
                        }
                        IncomingMessage::Response { id, result } => {
                            let mut pending = pending_clone.lock().await;
                            if let Some(tx) = pending.remove(&id) {
                                tx.send(result).ok();
                            }
                        }
                    }
                }
            }
        });

        Ok((Self { tx, pending }, event_rx))
    }

    pub async fn subscribe(&self, topics: Vec<String>) -> anyhow::Result<()> {
        self.tx.send(OutgoingMessage::Subscribe { topics }).await?;
        Ok(())
    }

    pub async fn command(&self, action: &str, args: serde_json::Value) -> anyhow::Result<serde_json::Value> {
        let id = REQUEST_ID.fetch_add(1, Ordering::SeqCst).to_string();
        let (tx, rx) = oneshot::channel();

        {
            let mut pending = self.pending.lock().await;
            pending.insert(id.clone(), tx);
        }

        self.tx.send(OutgoingMessage::Command {
            id,
            action: action.to_string(),
            args,
        }).await?;

        Ok(rx.await?)
    }

    // Convenience methods
    pub async fn list_apps(&self) -> anyhow::Result<Vec<AppInfo>> {
        let result = self.command("apps.list", serde_json::json!({})).await?;
        Ok(serde_json::from_value(result)?)
    }

    pub async fn start_app(&self, name: &str) -> anyhow::Result<()> {
        self.command("apps.start", serde_json::json!({"name": name})).await?;
        Ok(())
    }

    pub async fn launch_app(&self, name: &str) -> anyhow::Result<()> {
        self.command("apps.launch", serde_json::json!({"name": name})).await?;
        Ok(())
    }
}

#[derive(Debug)]
pub struct Event {
    pub topic: String,
    pub payload: serde_json::Value,
}

#[derive(Debug, Deserialize)]
pub struct AppInfo {
    pub name: String,
    pub version: String,
    pub status: String,
}
```

### WebSocket Protocol Summary

**TUI → Backend:**

```json
// Subscribe to state updates
{"type": "subscribe", "topics": ["mesh:stats", "mesh:peers", "apps:state"]}

// Unsubscribe
{"type": "unsubscribe", "topics": ["mesh:peers"]}

// Command with response
{"type": "cmd", "id": "1", "action": "apps.list", "args": {}}
{"type": "cmd", "id": "2", "action": "apps.launch", "args": {"name": "my-app"}}
```

**Backend → TUI:**

```json
// State update (broadcast)
{"type": "event", "topic": "mesh:stats", "payload": {"clients": 5, "uptime": 3600}}

// Command response
{"type": "response", "id": "1", "result": [{"name": "console", "status": "running"}]}
```

## Key Insight: TUI as Application Launcher

**Observation:** A TUI-based macula-console is fundamentally better suited as an "application launcher" than a Phoenix/LiveView web UI.

### The Problem with Web-Based Launchers

Phoenix LiveView runs in a browser, which creates significant friction for launching native applications:

```
Browser Limitations:
┌─────────────────────────────────────────────────────────────────┐
│  Browser Sandbox                                                 │
│  ┌─────────────────────────────────────────────────────────┐    │
│  │  LiveView can:                                           │    │
│  │  ✓ Display app list                                     │    │
│  │  ✓ Show app status                                      │    │
│  │  ✓ Send "launch" command to backend                     │    │
│  │                                                          │    │
│  │  LiveView cannot:                                        │    │
│  │  ✗ Open a terminal window                               │    │
│  │  ✗ Launch a native GUI app                              │    │
│  │  ✗ Attach to app's stdin/stdout                         │    │
│  │  ✗ Display TUI apps inline                              │    │
│  └─────────────────────────────────────────────────────────┘    │
│                                                                  │
│  Workarounds (all clunky):                                       │
│  - Custom URL schemes (macula://launch/app-name)                │
│  - Browser extensions                                            │
│  - Electron wrapper (defeats the purpose)                        │
│  - xdg-open from backend (user doesn't see output)              │
└─────────────────────────────────────────────────────────────────┘
```

### TUI Advantages for Application Launching

A Rust TUI running locally has **direct OS access**:

```
TUI Capabilities:
┌─────────────────────────────────────────────────────────────────┐
│  Rust TUI (native process)                                       │
│                                                                  │
│  ✓ Launch child processes (std::process::Command)               │
│  ✓ Capture stdout/stderr in real-time                           │
│  ✓ Display output inline in TUI                                 │
│  ✓ Forward stdin to child process                               │
│  ✓ Manage process lifecycle (signals, kill)                     │
│  ✓ Switch between app outputs (like tmux)                       │
│  ✓ Launch GUI apps via fork/exec                                │
│  ✓ Access filesystem directly                                   │
│  ✓ Modify /etc/hosts, iptables (with sudo)                      │
└─────────────────────────────────────────────────────────────────┘
```

### Application Launcher Architecture

```
┌─────────────────────────────────────────────────────────────────┐
│                     macula-tui (Rust)                            │
├─────────────────────────────────────────────────────────────────┤
│                                                                  │
│  ┌─────────────────────────────────────────────────────────┐    │
│  │  App Launcher View                                       │    │
│  │                                                          │    │
│  │  [1] macula-console   ● running   [Enter] Attach        │    │
│  │  [2] my-phoenix-app   ○ stopped   [Enter] Start         │    │
│  │  [3] data-pipeline    ● running   [Enter] Attach        │    │
│  │  [4] ml-training      ○ stopped   [Enter] Start         │    │
│  │                                                          │    │
│  │  [Tab] Switch pane  [q] Quit  [l] Logs  [r] Restart     │    │
│  └─────────────────────────────────────────────────────────┘    │
│                                                                  │
│  ┌─────────────────────────────────────────────────────────┐    │
│  │  App Output Pane (attached to macula-console)            │    │
│  │                                                          │    │
│  │  [info] Starting Phoenix endpoint...                    │    │
│  │  [info] Running on http://localhost:4000                │    │
│  │  [debug] LiveView connected: dashboard                  │    │
│  │  █                                                       │    │
│  └─────────────────────────────────────────────────────────┘    │
│                                                                  │
└─────────────────────────────────────────────────────────────────┘
```

### Implementation Sketch

```rust
// rust/crates/macula-tui/src/launcher.rs

use std::process::{Child, Command, Stdio};
use std::collections::HashMap;

pub struct AppLauncher {
    running: HashMap<String, Child>,
}

impl AppLauncher {
    pub fn launch(&mut self, app: &AppSpec) -> anyhow::Result<()> {
        let child = Command::new(&app.command)
            .args(&app.args)
            .envs(&app.env)
            .current_dir(&app.working_dir)
            .stdout(Stdio::piped())
            .stderr(Stdio::piped())
            .spawn()?;

        self.running.insert(app.name.clone(), child);
        Ok(())
    }

    pub fn attach(&mut self, name: &str) -> Option<&mut Child> {
        self.running.get_mut(name)
    }

    pub fn stop(&mut self, name: &str) -> anyhow::Result<()> {
        if let Some(mut child) = self.running.remove(name) {
            child.kill()?;
        }
        Ok(())
    }

    pub fn read_output(&mut self, name: &str) -> Option<String> {
        // Non-blocking read from child's stdout
        if let Some(child) = self.running.get_mut(name) {
            // Read available bytes from stdout pipe
            // ...
        }
        None
    }
}

// Integration with TUI
pub struct AppView {
    launcher: AppLauncher,
    attached_app: Option<String>,
    output_buffer: Vec<String>,
}

impl AppView {
    pub fn handle_key(&mut self, key: KeyCode) {
        match key {
            KeyCode::Enter => {
                if let Some(app) = self.selected_app() {
                    if self.launcher.is_running(&app.name) {
                        self.attached_app = Some(app.name.clone());
                    } else {
                        self.launcher.launch(&app).ok();
                    }
                }
            }
            KeyCode::Char('q') => {
                if let Some(name) = &self.attached_app {
                    self.attached_app = None; // Detach, don't stop
                }
            }
            KeyCode::Char('k') => {
                if let Some(name) = &self.attached_app {
                    self.launcher.stop(name).ok();
                    self.attached_app = None;
                }
            }
            _ => {}
        }
    }

    pub fn tick(&mut self) {
        // Read output from attached app
        if let Some(name) = &self.attached_app {
            if let Some(output) = self.launcher.read_output(name) {
                self.output_buffer.push(output);
            }
        }
    }
}
```

### Comparison: Web UI vs TUI for Application Management

| Capability              | Phoenix LiveView       | Rust TUI        |
| ----------------------- | ---------------------- | --------------- |
| List apps               | ✅                     | ✅              |
| Show status             | ✅                     | ✅              |
| Start/stop backend apps | ✅                     | ✅              |
| View logs (via API)     | ✅                     | ✅              |
| **Launch native apps**  | ❌ Browser sandbox     | ✅ Direct       |
| **Attach to stdout**    | ❌ Need terminal       | ✅ Inline       |
| **Interactive stdin**   | ❌ Not possible        | ✅ Forward keys |
| **tmux-like panes**     | ❌ Would need xterm.js | ✅ Native       |
| **Works headless**      | ❌ Needs browser       | ✅ SSH works    |
| **Works over SSH**      | 🔶 Port forward        | ✅ Native       |

### Conclusion

**For MaculaOS, the TUI should be the primary interface**, not a secondary one:

1. **Application launcher** - Launch, attach, manage native processes
2. **tmux-like multiplexer** - Multiple app outputs in panes
3. **System management** - GitOps, mesh status, metrics
4. **Works everywhere** - Local console, SSH, serial port

Phoenix LiveView remains valuable for:

- Remote web access (when TUI isn't available)
- Rich visualizations (topology graphs, dashboards)
- Mobile access
- Sharing dashboards with non-technical users

But the **TUI is the power-user interface** and should be first-class.

## Phoenix Console as Web Application Launcher

While the TUI excels at launching native/CLI applications, Phoenix Console Web could still serve as an effective launcher for **other Phoenix/web applications**. Let's explore the options.

### The Challenge

```
Phoenix Console Web (Browser)
         │
         │  User clicks "Launch my-phoenix-app"
         │
         ▼
┌─────────────────────────────────────────────────────────────────┐
│  What happens next?                                              │
│                                                                  │
│  Backend can:                                                    │
│  ✓ Start the Phoenix app (MaculaRuntime.start_app)              │
│  ✓ Know its port (e.g., 4001)                                   │
│  ✓ Check its health                                             │
│                                                                  │
│  But how does the user ACCESS it?                               │
│  - Direct port? http://localhost:4001 (user must know port)     │
│  - New tab? (loses context, separate session)                   │
│  - Iframe? (works but has limitations)                          │
│  - Proxy? (Console routes traffic)                              │
│  - Embedded? (load app's LiveView in Console's shell)           │
└─────────────────────────────────────────────────────────────────┘
```

### Option 1: Multi-Tab with Service Discovery

**How it works:** Console launches app, opens new browser tab to app's URL.

```
┌─────────────────────────────────────────────────────────────────┐
│  Browser Tab 1: Console                                          │
│  ┌─────────────────────────────────────────────────────────┐    │
│  │  Apps                                                    │    │
│  │  [my-phoenix-app]  ● running  [Open ↗]                  │    │
│  │  [data-dashboard]  ○ stopped  [Start]                   │    │
│  └─────────────────────────────────────────────────────────┘    │
└─────────────────────────────────────────────────────────────────┘
          │
          │ Click "Open ↗"
          ▼
┌─────────────────────────────────────────────────────────────────┐
│  Browser Tab 2: my-phoenix-app                                   │
│  URL: http://my-phoenix-app.macula.local:4001                   │
│  ┌─────────────────────────────────────────────────────────┐    │
│  │  My Phoenix App UI                                       │    │
│  └─────────────────────────────────────────────────────────┘    │
└─────────────────────────────────────────────────────────────────┘
```

**Implementation:**

```elixir
# Console LiveView
def handle_event("open_app", %{"name" => name}, socket) do
  case MaculaRuntime.get_app_url(name) do
    {:ok, url} ->
      {:noreply, push_event(socket, "open_url", %{url: url})}
    {:error, :not_running} ->
      {:noreply, put_flash(socket, :error, "App not running")}
  end
end
```

```javascript
// Console JS hook
Hooks.AppLauncher = {
  mounted() {
    this.handleEvent("open_url", ({ url }) => {
      window.open(url, "_blank");
    });
  },
};
```

**Pros:**

- Simple to implement
- Apps run independently
- No proxy overhead
- Apps have full browser capabilities

**Cons:**

- User manages multiple tabs
- Need DNS/hosts setup for each app
- No unified navigation
- Separate authentication per app

---

### Option 2: Reverse Proxy (Console as Gateway)

**How it works:** Console acts as reverse proxy, routing `/apps/{name}/*` to the app.

```
┌─────────────────────────────────────────────────────────────────┐
│  Browser: http://console.macula.local/apps/my-phoenix-app       │
│                                                                  │
│  ┌─────────────────────────────────────────────────────────┐    │
│  │  Console Shell (header, nav)                             │    │
│  ├─────────────────────────────────────────────────────────┤    │
│  │                                                          │    │
│  │  my-phoenix-app content (proxied from :4001)            │    │
│  │                                                          │    │
│  └─────────────────────────────────────────────────────────┘    │
└─────────────────────────────────────────────────────────────────┘
         │
         │  Console proxies request
         ▼
┌─────────────────────────────────────────────────────────────────┐
│  my-phoenix-app (localhost:4001)                                 │
└─────────────────────────────────────────────────────────────────┘
```

**Implementation:**

```elixir
# router.ex
scope "/apps/:app_name", MaculaConsoleWeb do
  pipe_through [:browser, :proxy_auth]

  match :*, "/*path", ProxyController, :proxy
end

# proxy_controller.ex
defmodule MaculaConsoleWeb.ProxyController do
  use MaculaConsoleWeb, :controller

  def proxy(conn, %{"app_name" => app_name, "path" => path}) do
    case MaculaRuntime.get_app_port(app_name) do
      {:ok, port} ->
        target_url = "http://localhost:#{port}/#{Enum.join(path, "/")}"
        proxy_request(conn, target_url)

      {:error, :not_found} ->
        conn |> put_status(404) |> text("App not found")
    end
  end

  defp proxy_request(conn, target_url) do
    # Use Finch or Mint to proxy the request
    # Handle WebSocket upgrade for LiveView
    # Rewrite URLs in response
  end
end
```

**Challenge: LiveView WebSocket Proxying**

LiveView uses WebSocket, which complicates proxying:

```elixir
# Need to handle WebSocket upgrade
defmodule MaculaConsoleWeb.ProxySocket do
  @behaviour Phoenix.Socket.Transport

  def connect(%{params: %{"app" => app_name}} = state) do
    case MaculaRuntime.get_app_port(app_name) do
      {:ok, port} ->
        # Establish WebSocket to backend app
        {:ok, ws} = WebSocket.connect("ws://localhost:#{port}/live/websocket")
        {:ok, %{upstream: ws, app: app_name}}
      _ ->
        :error
    end
  end

  # Bidirectional message forwarding
  def handle_in({:text, msg}, state) do
    WebSocket.send(state.upstream, msg)
    {:ok, state}
  end

  def handle_info({:websocket, msg}, state) do
    {:push, {:text, msg}, state}
  end
end
```

**Pros:**

- Single URL/domain for all apps
- Console can inject shell (header, nav)
- Unified authentication possible
- No DNS setup per app

**Cons:**

- Complex WebSocket proxying for LiveView
- URL rewriting is fragile
- Performance overhead
- Asset path issues (CSS, JS, images)

---

### Option 3: Iframe Embedding

**How it works:** Console embeds apps in iframes, maintaining its shell.

```
┌─────────────────────────────────────────────────────────────────┐
│  Console: http://console.macula.local                            │
│  ┌────────────────────────────────────────────────────────────┐ │
│  │ [Console] [my-app ×] [dashboard ×]              [Settings] │ │
│  ├────────────────────────────────────────────────────────────┤ │
│  │                                                             │ │
│  │  ┌───────────────────────────────────────────────────────┐ │ │
│  │  │ <iframe src="http://localhost:4001">                  │ │ │
│  │  │                                                       │ │ │
│  │  │   my-phoenix-app UI                                  │ │ │
│  │  │                                                       │ │ │
│  │  └───────────────────────────────────────────────────────┘ │ │
│  │                                                             │ │
│  └────────────────────────────────────────────────────────────┘ │
└─────────────────────────────────────────────────────────────────┘
```

**Implementation:**

```elixir
# LiveView
defmodule MaculaConsoleWeb.AppShellLive do
  use MaculaConsoleWeb, :live_view

  def mount(_params, _session, socket) do
    {:ok, assign(socket, tabs: [], active_tab: nil)}
  end

  def handle_event("open_app", %{"name" => name}, socket) do
    case MaculaRuntime.get_app_url(name) do
      {:ok, url} ->
        tabs = socket.assigns.tabs ++ [%{name: name, url: url}]
        {:noreply, assign(socket, tabs: tabs, active_tab: name)}
      _ ->
        {:noreply, socket}
    end
  end

  def render(assigns) do
    ~H"""
    <div class="app-shell">
      <nav class="tabs">
        <button phx-click="show_console">Console</button>
        <%= for tab <- @tabs do %>
          <button phx-click="switch_tab" phx-value-name={tab.name}>
            <%= tab.name %> <span phx-click="close_tab" phx-value-name={tab.name}>×</span>
          </button>
        <% end %>
      </nav>

      <div class="content">
        <%= if @active_tab do %>
          <%= for tab <- @tabs do %>
            <iframe
              src={tab.url}
              class={if tab.name == @active_tab, do: "visible", else: "hidden"}
              sandbox="allow-same-origin allow-scripts allow-forms allow-popups"
            />
          <% end %>
        <% else %>
          <.live_component module={ConsoleDashboard} id="dashboard" />
        <% end %>
      </div>
    </div>
    """
  end
end
```

**Pros:**

- Console shell always visible (tabs, nav)
- Apps run independently
- Simple implementation
- No URL rewriting needed

**Cons:**

- Cross-origin restrictions (if different domains)
- Can't easily share auth between Console and apps
- iframe has limited browser API access
- Apps can't go fullscreen easily
- Mobile UX is poor

---

### Option 4: Micro-Frontend Architecture (Federated Modules)

**How it works:** Apps expose LiveView components that Console loads dynamically.

```
┌─────────────────────────────────────────────────────────────────┐
│  Console (Host)                                                  │
│                                                                  │
│  ┌─────────────────────────────────────────────────────────┐    │
│  │  Console Shell                                           │    │
│  │  ┌─────────────────────────────────────────────────────┐│    │
│  │  │                                                     ││    │
│  │  │  <.live_component                                   ││    │
│  │  │     module={MyApp.DashboardComponent}               ││    │
│  │  │     id="my-app-dashboard" />                        ││    │
│  │  │                                                     ││    │
│  │  │  (Component loaded from my-phoenix-app's beam)      ││    │
│  │  │                                                     ││    │
│  │  └─────────────────────────────────────────────────────┘│    │
│  └─────────────────────────────────────────────────────────┘    │
└─────────────────────────────────────────────────────────────────┘
```

**This requires apps to run in the same BEAM VM as Console.**

```elixir
# Console dynamically loads app modules
defmodule MaculaConsoleWeb.AppHostLive do
  use MaculaConsoleWeb, :live_view

  def mount(%{"app" => app_name}, _session, socket) do
    case MaculaRuntime.get_app_module(app_name, :live_view) do
      {:ok, module} ->
        {:ok, assign(socket, app_module: module, app_name: app_name)}
      {:error, reason} ->
        {:ok, assign(socket, error: reason)}
    end
  end

  def render(assigns) do
    ~H"""
    <div class="app-host">
      <.console_header />

      <%= if @app_module do %>
        <%= live_render(@socket, @app_module, id: @app_name) %>
      <% else %>
        <p>Error: <%= @error %></p>
      <% end %>

      <.console_footer />
    </div>
    """
  end
end
```

**Pros:**

- True integration (shared BEAM, shared state)
- No network overhead
- Shared authentication
- Apps can use Console's PubSub
- Hot code upgrades work

**Cons:**

- Apps MUST run in same BEAM VM
- Tight coupling
- App isolation issues
- Not suitable for third-party apps

---

### Option 5: Portal Pattern (Shared Authentication + Links)

**How it works:** Console handles auth, issues tokens, apps trust Console's tokens.

```
┌──────────────────────────────────────────────────────────────────────┐
│                         User Flow                                     │
│                                                                       │
│  1. User logs into Console                                           │
│     ┌─────────────────┐                                              │
│     │ Console Login   │                                              │
│     │ ────────────    │                                              │
│     │ [Login]         │                                              │
│     └────────┬────────┘                                              │
│              │                                                        │
│  2. Console issues JWT/session token                                 │
│              │                                                        │
│              ▼                                                        │
│     ┌─────────────────────────────────────────────────────────────┐  │
│     │ Console Dashboard                                            │  │
│     │                                                              │  │
│     │  Apps:                                                       │  │
│     │  [my-app] ● running  [Open →]                               │  │
│     │                         │                                    │  │
│     └─────────────────────────┼────────────────────────────────────┘  │
│                               │                                       │
│  3. Click "Open" - redirect with token                               │
│              │                                                        │
│              ▼                                                        │
│     http://my-app.macula.local/?token=eyJ...                         │
│              │                                                        │
│  4. App validates token (shared secret with Console)                 │
│              │                                                        │
│              ▼                                                        │
│     ┌─────────────────────────────────────────────────────────────┐  │
│     │ my-app (authenticated as same user)                          │  │
│     │                                                              │  │
│     │  Welcome, user@example.com                                   │  │
│     │  [← Back to Console]                                         │  │
│     │                                                              │  │
│     └─────────────────────────────────────────────────────────────┘  │
└──────────────────────────────────────────────────────────────────────┘
```

**Implementation:**

```elixir
# Console: Generate app launch URL with token
defmodule MaculaConsole.AppLauncher do
  def launch_url(app_name, user, conn) do
    app_url = MaculaRuntime.get_app_url(app_name)
    token = generate_launch_token(user, app_name)

    "#{app_url}?macula_token=#{token}&return_url=#{current_url(conn)}"
  end

  defp generate_launch_token(user, app_name) do
    Phoenix.Token.sign(
      MaculaConsoleWeb.Endpoint,
      "app_launch",
      %{user_id: user.id, app: app_name, exp: :os.system_time(:second) + 60}
    )
  end
end

# App: Verify token from Console
defmodule MyApp.MaculaAuthPlug do
  def call(conn, _opts) do
    case conn.params["macula_token"] do
      nil -> conn
      token -> verify_and_auth(conn, token)
    end
  end

  defp verify_and_auth(conn, token) do
    # Apps share secret with Console (from config or mesh)
    case Phoenix.Token.verify(MaculaConsoleWeb.Endpoint, "app_launch", token, max_age: 60) do
      {:ok, %{user_id: user_id}} ->
        user = load_or_create_user(user_id)
        conn |> assign(:current_user, user) |> put_session(:user_id, user_id)
      {:error, _} ->
        conn |> put_status(401) |> halt()
    end
  end
end
```

**Pros:**

- Clean separation (apps are independent)
- Shared authentication (SSO-like)
- Apps can link back to Console
- Works with any web framework (not just Phoenix)

**Cons:**

- User navigates away from Console
- Multiple tabs/windows
- Need shared secret distribution

---

### Comparison Matrix

| Approach       | Complexity | UX               | Auth            | LiveView Apps | Non-Phoenix Apps |
| -------------- | ---------- | ---------------- | --------------- | ------------- | ---------------- |
| Multi-Tab      | Low        | 🔶 Multiple tabs | ❌ Separate     | ✅            | ✅               |
| Reverse Proxy  | High       | ✅ Single page   | ✅ Unified      | 🔶 Complex    | ✅               |
| Iframe         | Medium     | ✅ Tabbed        | 🔶 Cross-origin | ✅            | ✅               |
| Micro-Frontend | High       | ✅ Seamless      | ✅ Shared       | ✅            | ❌ BEAM only     |
| Portal/SSO     | Medium     | 🔶 Multiple tabs | ✅ SSO          | ✅            | ✅               |

### Recommendation for MaculaOS

**Hybrid approach:**

1. **For Phoenix apps in same BEAM:** Use **Micro-Frontend** (Option 4)
   - Apps run as OTP applications in MaculaRuntime
   - Console loads their LiveView components directly
   - Best UX, best performance, shared state

2. **For external web apps:** Use **Portal/SSO** (Option 5) + **Iframe** (Option 3)
   - Console handles auth, issues tokens
   - Quick preview via iframe
   - "Open in new tab" for full experience

3. **For native/CLI apps:** Use **TUI** (documented above)
   - TUI is the launcher for non-web apps

```
┌─────────────────────────────────────────────────────────────────┐
│                    MaculaOS App Launching                        │
├─────────────────────────────────────────────────────────────────┤
│                                                                  │
│  App Type          │ Primary Launcher    │ Alternative          │
│  ──────────────────┼────────────────────┼─────────────────────  │
│  Phoenix (BEAM)    │ Console Web        │ TUI (start/stop)     │
│                    │ (micro-frontend)   │                       │
│  ──────────────────┼────────────────────┼─────────────────────  │
│  External Web      │ Console Web        │ TUI (open URL)       │
│                    │ (iframe + SSO)     │                       │
│  ──────────────────┼────────────────────┼─────────────────────  │
│  CLI/Native        │ TUI                │ Console (logs only)  │
│                    │ (direct launch)    │                       │
│  ──────────────────┼────────────────────┼─────────────────────  │
│  OTP Release       │ TUI                │ Console (status)     │
│                    │ (systemctl-like)   │                       │
│                                                                  │
└─────────────────────────────────────────────────────────────────┘
```

### Implementation Priority

For Pure BEAM MaculaOS:

1. **Phase 1:** TUI as primary launcher (native apps, OTP releases)
2. **Phase 2:** Console Web with micro-frontend (BEAM apps share VM)
3. **Phase 3:** Console Web with iframe + SSO (external web apps)

This leverages the BEAM's strengths (shared VM, hot code loading) while still supporting external apps.
