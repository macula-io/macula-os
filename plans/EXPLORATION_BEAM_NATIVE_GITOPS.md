# EXPLORATION: BEAM-Native GitOps Reconciler

> **Status:** ❌ REJECTED (2026-01-27)
>
> **Decision:** Use **k3s + FluxCD** instead of BEAM-native GitOps.
>
> **Rationale:**
> - k3s provides battle-tested container orchestration with minimal overhead
> - FluxCD is mature, well-documented, and has strong community support
> - The BEAM-native approach would require significant development effort for uncertain gains
> - k3s + Flux gives us GitOps out of the box with proven reliability
> - Focus engineering effort on Macula mesh networking and applications, not reinventing deployment infrastructure
>
> **This document is preserved for historical reference only.**

---

## Original Exploration

A core component of Pure BEAM MaculaOS: replacing Flux/ArgoCD with native Erlang/Elixir.

### What GitOps Reconcilers Do

```
┌─────────────────────────────────────────────────────────────────┐
│                    GitOps Reconciliation Loop                    │
│                                                                  │
│   ┌─────────┐     ┌──────────┐     ┌──────────┐     ┌────────┐ │
│   │  Git    │────▶│  Parse   │────▶│  Compare │────▶│ Apply  │ │
│   │  Fetch  │     │  Desired │     │  States  │     │ Changes│ │
│   └─────────┘     └──────────┘     └──────────┘     └────────┘ │
│        │                                                  │      │
│        │              Reconcile Loop (every N seconds)    │      │
│        └──────────────────────────────────────────────────┘      │
│                                                                  │
│   Desired State (Git)          Current State (Runtime)          │
│   ┌─────────────────┐          ┌─────────────────┐              │
│   │ app: console    │          │ console: v1.0.0 │              │
│   │ version: 1.1.0  │    !=    │ running         │              │
│   │ replicas: 1     │          │                 │              │
│   └─────────────────┘          └─────────────────┘              │
│           │                                                      │
│           └──────▶ Action: upgrade console 1.0.0 → 1.1.0        │
│                                                                  │
└─────────────────────────────────────────────────────────────────┘
```

### Flux vs BEAM-Native Comparison

| Aspect   | Flux (k8s)      | BEAM GitOps                   |
| -------- | --------------- | ----------------------------- |
| Target   | Kubernetes API  | OTP Supervisor                |
| Config   | YAML manifests  | Erlang terms / Elixir DSL     |
| Deploy   | kubectl apply   | release_handler / code:load   |
| Rollback | kubectl rollout | release_handler:revert        |
| Health   | k8s probes      | Supervisor / health GenServer |
| Scaling  | Replica sets    | pg groups / :global           |
| Secrets  | k8s Secrets     | ETS / encrypted file          |

### Architecture

```
┌─────────────────────────────────────────────────────────────────┐
│                    macula_gitops Application                     │
├─────────────────────────────────────────────────────────────────┤
│                                                                  │
│  ┌─────────────────────────────────────────────────────────┐    │
│  │                 macula_gitops_sup                        │    │
│  │                   (supervisor)                           │    │
│  └─────────────────────────┬───────────────────────────────┘    │
│                            │                                     │
│        ┌───────────────────┼───────────────────┐                │
│        │                   │                   │                 │
│        ▼                   ▼                   ▼                 │
│  ┌───────────┐      ┌───────────┐      ┌───────────┐           │
│  │ GitFetcher│      │Reconciler │      │ Reporter  │           │
│  │           │      │           │      │           │           │
│  │ - clone   │      │ - parse   │      │ - status  │           │
│  │ - pull    │─────▶│ - diff    │─────▶│ - events  │           │
│  │ - watch   │      │ - apply   │      │ - git     │           │
│  └───────────┘      └───────────┘      └───────────┘           │
│        │                   │                                     │
│        │                   ▼                                     │
│        │            ┌───────────┐                               │
│        │            │ Runtime   │                               │
│        │            │           │                               │
│        │            │ - start   │                               │
│        │            │ - stop    │                               │
│        │            │ - upgrade │                               │
│        │            │ - health  │                               │
│        │            └───────────┘                               │
│        │                                                        │
│        ▼                                                        │
│  ┌─────────────────────────────────────────────────────────┐   │
│  │                  Local Git Clone                         │   │
│  │              /var/lib/maculaos/gitops/                   │   │
│  │                                                          │   │
│  │  apps/                                                   │   │
│  │  ├── console.app.exs                                    │   │
│  │  ├── my-phoenix-app.app.exs                             │   │
│  │  └── data-pipeline.app.exs                              │   │
│  │                                                          │   │
│  │  infrastructure/                                         │   │
│  │  ├── nats.service.exs                                   │   │
│  │  └── postgres.service.exs                               │   │
│  │                                                          │   │
│  └─────────────────────────────────────────────────────────┘   │
│                                                                  │
└─────────────────────────────────────────────────────────────────┘
```

### Configuration Format Options

#### Option A: Erlang Terms (Native, Zero Dependencies)

```erlang
%% apps/console.app.config
#{
    name => macula_console,
    version => <<"1.2.0">>,
    source => #{
        type => release,
        url => <<"https://releases.macula.io/console/1.2.0.tar.gz">>,
        sha256 => <<"abc123...">>
    },
    env => #{
        port => 4000,
        secret_key_base => {env, "SECRET_KEY_BASE"},  %% Read from OS env
        database_url => {secret, "console/database_url"}  %% Read from secrets
    },
    health => #{
        type => http,
        path => <<"/health">>,
        port => 4000,
        interval => 10000,
        timeout => 5000
    },
    depends_on => [postgres, nats]
}.
```

**Pros:** Native parsing with `file:consult/1`, no dependencies
**Cons:** Unfamiliar syntax for non-Erlang users

#### Option B: Elixir DSL (Expressive, Type-Safe)

```elixir
# apps/console.app.exs
app :macula_console do
  version "1.2.0"

  source :release do
    url "https://releases.macula.io/console/1.2.0.tar.gz"
    sha256 "abc123..."
  end

  env do
    port 4000
    secret_key_base env("SECRET_KEY_BASE")
    database_url secret("console/database_url")
  end

  health :http do
    path "/health"
    port 4000
    interval :timer.seconds(10)
    timeout :timer.seconds(5)
  end

  depends_on [:postgres, :nats]
end
```

**Pros:** Familiar Mix-like syntax, compile-time validation possible
**Cons:** Requires Elixir parser

#### Option C: YAML (Familiar to DevOps)

```yaml
# apps/console.yaml
name: macula_console
version: "1.2.0"

source:
  type: release
  url: https://releases.macula.io/console/1.2.0.tar.gz
  sha256: abc123...

env:
  port: 4000
  secret_key_base: ${SECRET_KEY_BASE}
  database_url: !secret console/database_url

health:
  type: http
  path: /health
  port: 4000
  interval: 10s
  timeout: 5s

depends_on:
  - postgres
  - nats
```

**Pros:** Familiar to k8s users, many editors support it
**Cons:** Requires YAML parser (yamerl), less expressive

#### Recommendation: Elixir DSL with YAML Fallback

```elixir
# macula_gitops_parser.ex
defmodule MaculaGitops.Parser do
  def parse_file(path) do
    case Path.extname(path) do
      ".exs" -> parse_elixir(path)
      ".yaml" -> parse_yaml(path)
      ".yml" -> parse_yaml(path)
      ".config" -> parse_erlang(path)
      _ -> {:error, :unknown_format}
    end
  end

  defp parse_elixir(path) do
    {result, _} = Code.eval_file(path)
    {:ok, normalize(result)}
  end

  defp parse_yaml(path) do
    {:ok, content} = File.read(path)
    {:ok, parsed} = YamlElixir.read_from_string(content)
    {:ok, normalize(parsed)}
  end

  defp parse_erlang(path) do
    {:ok, [term]} = :file.consult(path)
    {:ok, normalize(term)}
  end
end
```

### Git Integration

#### Option A: Shell Out to Git (Simple, Universal)

```elixir
defmodule MaculaGitops.Git do
  @gitops_dir "/var/lib/maculaos/gitops"

  def clone(repo_url) do
    case System.cmd("git", ["clone", "--depth", "1", repo_url, @gitops_dir]) do
      {_, 0} -> :ok
      {error, _} -> {:error, error}
    end
  end

  def pull do
    case System.cmd("git", ["-C", @gitops_dir, "pull", "--ff-only"]) do
      {_, 0} -> :ok
      {error, code} -> {:error, {code, error}}
    end
  end

  def current_commit do
    case System.cmd("git", ["-C", @gitops_dir, "rev-parse", "HEAD"]) do
      {sha, 0} -> {:ok, String.trim(sha)}
      {error, _} -> {:error, error}
    end
  end

  def changed_files(from_sha, to_sha) do
    case System.cmd("git", ["-C", @gitops_dir, "diff", "--name-only", from_sha, to_sha]) do
      {output, 0} -> {:ok, String.split(output, "\n", trim: true)}
      {error, _} -> {:error, error}
    end
  end
end
```

**Pros:** Simple, works everywhere git is installed
**Cons:** Requires git binary, slower than native

#### Option B: Pure Erlang Git (No External Dependencies)

There's no mature pure-Erlang git implementation, but we could use:

- **gitex** (Elixir) - Limited functionality
- **libgit2** via NIF - Full featured but adds complexity

**Recommendation:** Shell out to git. It's simple, reliable, and git is always available.

### The Reconciler

```elixir
defmodule MaculaGitops.Reconciler do
  use GenServer
  require Logger

  @reconcile_interval :timer.seconds(60)

  defstruct [
    :repo_url,
    :local_path,
    :last_commit,
    :desired_state,
    :current_state,
    :status
  ]

  # Client API

  def start_link(opts) do
    GenServer.start_link(__MODULE__, opts, name: __MODULE__)
  end

  def sync_now do
    GenServer.call(__MODULE__, :sync_now, :timer.minutes(5))
  end

  def status do
    GenServer.call(__MODULE__, :status)
  end

  def get_apps do
    GenServer.call(__MODULE__, :get_apps)
  end

  # GenServer Callbacks

  @impl true
  def init(opts) do
    repo_url = Keyword.fetch!(opts, :repo_url)
    local_path = Keyword.get(opts, :local_path, "/var/lib/maculaos/gitops")

    state = %__MODULE__{
      repo_url: repo_url,
      local_path: local_path,
      last_commit: nil,
      desired_state: %{},
      current_state: %{},
      status: :initializing
    }

    # Initial clone/pull
    send(self(), :init_repo)

    {:ok, state}
  end

  @impl true
  def handle_info(:init_repo, state) do
    state = init_repository(state)
    schedule_reconcile()
    {:noreply, state}
  end

  def handle_info(:reconcile, state) do
    state = reconcile(state)
    schedule_reconcile()
    {:noreply, state}
  end

  @impl true
  def handle_call(:sync_now, _from, state) do
    state = reconcile(state)
    {:reply, {:ok, state.status}, state}
  end

  def handle_call(:status, _from, state) do
    status = %{
      status: state.status,
      last_commit: state.last_commit,
      apps: map_app_status(state)
    }
    {:reply, status, state}
  end

  def handle_call(:get_apps, _from, state) do
    {:reply, state.desired_state, state}
  end

  # Private Functions

  defp init_repository(state) do
    case File.exists?(Path.join(state.local_path, ".git")) do
      true ->
        Logger.info("[GitOps] Repository exists, pulling...")
        MaculaGitops.Git.pull(state.local_path)

      false ->
        Logger.info("[GitOps] Cloning repository...")
        MaculaGitops.Git.clone(state.repo_url, state.local_path)
    end

    {:ok, commit} = MaculaGitops.Git.current_commit(state.local_path)
    desired = parse_desired_state(state.local_path)
    current = get_current_state()

    %{state |
      last_commit: commit,
      desired_state: desired,
      current_state: current,
      status: :ready
    }
  end

  defp reconcile(state) do
    Logger.debug("[GitOps] Starting reconciliation...")

    # 1. Pull latest changes
    :ok = MaculaGitops.Git.pull(state.local_path)

    # 2. Check if commit changed
    {:ok, new_commit} = MaculaGitops.Git.current_commit(state.local_path)

    state =
      if new_commit != state.last_commit do
        Logger.info("[GitOps] New commit detected: #{new_commit}")
        %{state |
          last_commit: new_commit,
          desired_state: parse_desired_state(state.local_path)
        }
      else
        state
      end

    # 3. Get current runtime state
    current = get_current_state()
    state = %{state | current_state: current}

    # 4. Compute diff
    actions = compute_actions(state.desired_state, current)

    # 5. Apply actions
    results = Enum.map(actions, &apply_action/1)

    # 6. Report status
    report_status(state, actions, results)

    # 7. Update state
    %{state |
      current_state: get_current_state(),
      status: if(Enum.all?(results, &match?({:ok, _}, &1)), do: :synced, else: :degraded)
    }
  end

  defp parse_desired_state(local_path) do
    apps_dir = Path.join(local_path, "apps")

    apps_dir
    |> File.ls!()
    |> Enum.filter(&(Path.extname(&1) in [".exs", ".yaml", ".yml", ".config"]))
    |> Enum.map(fn file ->
      path = Path.join(apps_dir, file)
      {:ok, spec} = MaculaGitops.Parser.parse_file(path)
      {spec.name, spec}
    end)
    |> Map.new()
  end

  defp get_current_state do
    MaculaRuntime.list_apps()
    |> Enum.map(fn app -> {app.name, app} end)
    |> Map.new()
  end

  defp compute_actions(desired, current) do
    desired_names = Map.keys(desired) |> MapSet.new()
    current_names = Map.keys(current) |> MapSet.new()

    # Apps to deploy (in desired, not in current)
    to_deploy =
      MapSet.difference(desired_names, current_names)
      |> Enum.map(fn name -> {:deploy, desired[name]} end)

    # Apps to remove (in current, not in desired)
    to_remove =
      MapSet.difference(current_names, desired_names)
      |> Enum.map(fn name -> {:remove, current[name]} end)

    # Apps to potentially upgrade (in both)
    to_check =
      MapSet.intersection(desired_names, current_names)
      |> Enum.flat_map(fn name ->
        desired_spec = desired[name]
        current_app = current[name]

        cond do
          desired_spec.version != current_app.version ->
            [{:upgrade, desired_spec, current_app.version}]

          config_changed?(desired_spec, current_app) ->
            [{:reconfigure, desired_spec}]

          true ->
            []
        end
      end)

    to_deploy ++ to_remove ++ to_check
  end

  defp apply_action({:deploy, spec}) do
    Logger.info("[GitOps] Deploying #{spec.name} v#{spec.version}")

    with {:ok, release_path} <- download_release(spec),
         :ok <- MaculaRuntime.deploy(spec.name, release_path, spec.env) do
      {:ok, :deployed}
    else
      {:error, reason} ->
        Logger.error("[GitOps] Deploy failed: #{inspect(reason)}")
        {:error, reason}
    end
  end

  defp apply_action({:remove, app}) do
    Logger.info("[GitOps] Removing #{app.name}")
    MaculaRuntime.stop(app.name)
    {:ok, :removed}
  end

  defp apply_action({:upgrade, spec, from_version}) do
    Logger.info("[GitOps] Upgrading #{spec.name} #{from_version} -> #{spec.version}")

    with {:ok, release_path} <- download_release(spec),
         :ok <- MaculaRuntime.upgrade(spec.name, release_path) do
      {:ok, :upgraded}
    else
      {:error, reason} ->
        Logger.error("[GitOps] Upgrade failed: #{inspect(reason)}")
        {:error, reason}
    end
  end

  defp apply_action({:reconfigure, spec}) do
    Logger.info("[GitOps] Reconfiguring #{spec.name}")
    MaculaRuntime.reconfigure(spec.name, spec.env)
    {:ok, :reconfigured}
  end

  defp download_release(spec) do
    releases_dir = "/var/lib/maculaos/releases"
    File.mkdir_p!(releases_dir)

    release_path = Path.join(releases_dir, "#{spec.name}-#{spec.version}.tar.gz")

    if File.exists?(release_path) do
      {:ok, release_path}
    else
      Logger.info("[GitOps] Downloading release from #{spec.source.url}")

      case download_file(spec.source.url, release_path) do
        :ok ->
          if verify_checksum(release_path, spec.source.sha256) do
            {:ok, release_path}
          else
            File.rm(release_path)
            {:error, :checksum_mismatch}
          end

        {:error, reason} ->
          {:error, reason}
      end
    end
  end

  defp download_file(url, dest) do
    case System.cmd("curl", ["-fsSL", "-o", dest, url]) do
      {_, 0} -> :ok
      {error, _} -> {:error, error}
    end
  end

  defp verify_checksum(path, expected_sha256) do
    {:ok, content} = File.read(path)
    actual = :crypto.hash(:sha256, content) |> Base.encode16(case: :lower)
    actual == expected_sha256
  end

  defp config_changed?(desired_spec, current_app) do
    # Compare env configurations
    Map.get(desired_spec, :env, %{}) != Map.get(current_app, :env, %{})
  end

  defp schedule_reconcile do
    Process.send_after(self(), :reconcile, @reconcile_interval)
  end

  defp report_status(state, actions, results) do
    # Broadcast to PubSub for TUI/Console
    Phoenix.PubSub.broadcast(
      MaculaCluster.PubSub,
      "gitops:status",
      {:gitops_reconciled, %{
        commit: state.last_commit,
        actions: length(actions),
        success: Enum.count(results, &match?({:ok, _}, &1)),
        failed: Enum.count(results, &match?({:error, _}, &1))
      }}
    )

    # Could also: write status to git repo, emit telemetry, etc.
  end

  defp map_app_status(state) do
    for {name, spec} <- state.desired_state do
      current = Map.get(state.current_state, name)

      status =
        cond do
          current == nil -> :pending
          current.version != spec.version -> :outdated
          true -> :synced
        end

      %{
        name: name,
        desired_version: spec.version,
        current_version: current && current.version,
        status: status
      }
    end
  end
end
```

### Runtime Manager

```elixir
defmodule MaculaRuntime do
  @moduledoc """
  Manages OTP application lifecycle.

  This module is responsible for:
  - Starting/stopping applications
  - Hot code upgrades
  - Health checking
  - Configuration management
  """

  use GenServer
  require Logger

  @releases_dir "/var/lib/maculaos/releases"
  @apps_dir "/var/lib/maculaos/apps"

  defstruct [:apps, :health_checks]

  # Client API

  def start_link(opts \\ []) do
    GenServer.start_link(__MODULE__, opts, name: __MODULE__)
  end

  def list_apps do
    GenServer.call(__MODULE__, :list_apps)
  end

  def deploy(name, release_path, env) do
    GenServer.call(__MODULE__, {:deploy, name, release_path, env}, :timer.minutes(5))
  end

  def stop(name) do
    GenServer.call(__MODULE__, {:stop, name})
  end

  def upgrade(name, release_path) do
    GenServer.call(__MODULE__, {:upgrade, name, release_path}, :timer.minutes(5))
  end

  def reconfigure(name, env) do
    GenServer.call(__MODULE__, {:reconfigure, name, env})
  end

  def health(name) do
    GenServer.call(__MODULE__, {:health, name})
  end

  # GenServer Callbacks

  @impl true
  def init(_opts) do
    File.mkdir_p!(@releases_dir)
    File.mkdir_p!(@apps_dir)

    state = %__MODULE__{
      apps: %{},
      health_checks: %{}
    }

    # Discover already-running apps
    state = discover_running_apps(state)

    {:ok, state}
  end

  @impl true
  def handle_call(:list_apps, _from, state) do
    apps = Map.values(state.apps)
    {:reply, apps, state}
  end

  def handle_call({:deploy, name, release_path, env}, _from, state) do
    case do_deploy(name, release_path, env) do
      {:ok, app_info} ->
        state = put_in(state.apps[name], app_info)
        state = start_health_check(state, name, app_info)
        {:reply, :ok, state}

      {:error, reason} ->
        {:reply, {:error, reason}, state}
    end
  end

  def handle_call({:stop, name}, _from, state) do
    case do_stop(name) do
      :ok ->
        state = %{state | apps: Map.delete(state.apps, name)}
        state = stop_health_check(state, name)
        {:reply, :ok, state}

      {:error, reason} ->
        {:reply, {:error, reason}, state}
    end
  end

  def handle_call({:upgrade, name, release_path}, _from, state) do
    case do_upgrade(name, release_path, state.apps[name]) do
      {:ok, new_app_info} ->
        state = put_in(state.apps[name], new_app_info)
        {:reply, :ok, state}

      {:error, reason} ->
        {:reply, {:error, reason}, state}
    end
  end

  def handle_call({:reconfigure, name, env}, _from, state) do
    case do_reconfigure(name, env) do
      :ok ->
        state = put_in(state.apps[name].env, env)
        {:reply, :ok, state}

      {:error, reason} ->
        {:reply, {:error, reason}, state}
    end
  end

  def handle_call({:health, name}, _from, state) do
    status = Map.get(state.health_checks, name, :unknown)
    {:reply, status, state}
  end

  @impl true
  def handle_info({:health_result, name, result}, state) do
    state = put_in(state.health_checks[name], result)

    # Broadcast health status
    Phoenix.PubSub.broadcast(
      MaculaCluster.PubSub,
      "apps:health",
      {:app_health, name, result}
    )

    {:noreply, state}
  end

  # Private Functions

  defp do_deploy(name, release_path, env) do
    Logger.info("[Runtime] Deploying #{name} from #{release_path}")

    app_dir = Path.join(@apps_dir, to_string(name))

    with :ok <- extract_release(release_path, app_dir),
         :ok <- write_env_file(app_dir, env),
         :ok <- start_app(name, app_dir) do
      {:ok, %{
        name: name,
        version: get_app_version(app_dir),
        path: app_dir,
        env: env,
        started_at: DateTime.utc_now(),
        status: :running
      }}
    end
  end

  defp do_stop(name) do
    Logger.info("[Runtime] Stopping #{name}")

    # For OTP releases, we use the release script
    app_dir = Path.join(@apps_dir, to_string(name))
    script = Path.join([app_dir, "bin", to_string(name)])

    case System.cmd(script, ["stop"]) do
      {_, 0} -> :ok
      {error, _} -> {:error, error}
    end
  end

  defp do_upgrade(name, release_path, current_app) do
    Logger.info("[Runtime] Upgrading #{name}")

    app_dir = current_app.path

    # OTP hot upgrade process:
    # 1. Extract new release to releases/{version}/
    # 2. Run upgrade script
    # 3. Verify health

    with {:ok, new_version} <- extract_upgrade(release_path, app_dir),
         :ok <- run_upgrade(name, app_dir, current_app.version, new_version) do
      {:ok, %{current_app |
        version: new_version,
        upgraded_at: DateTime.utc_now()
      }}
    end
  end

  defp do_reconfigure(name, env) do
    Logger.info("[Runtime] Reconfiguring #{name}")

    # Update application env at runtime
    for {key, value} <- env do
      Application.put_env(name, key, resolve_value(value))
    end

    :ok
  end

  defp extract_release(release_path, app_dir) do
    File.mkdir_p!(app_dir)

    case System.cmd("tar", ["-xzf", release_path, "-C", app_dir]) do
      {_, 0} -> :ok
      {error, _} -> {:error, {:extract_failed, error}}
    end
  end

  defp write_env_file(app_dir, env) do
    env_file = Path.join(app_dir, "releases/0.1.0/env.sh")

    content =
      env
      |> Enum.map(fn {key, value} ->
        resolved = resolve_value(value)
        "export #{String.upcase(to_string(key))}=\"#{resolved}\""
      end)
      |> Enum.join("\n")

    File.write(env_file, content)
  end

  defp start_app(name, app_dir) do
    script = Path.join([app_dir, "bin", to_string(name)])

    case System.cmd(script, ["daemon"]) do
      {_, 0} -> :ok
      {error, _} -> {:error, {:start_failed, error}}
    end
  end

  defp get_app_version(app_dir) do
    # Read from release metadata
    case File.read(Path.join(app_dir, "releases/start_erl.data")) do
      {:ok, content} ->
        [_erts, version] = String.split(content)
        String.trim(version)

      _ ->
        "unknown"
    end
  end

  defp extract_upgrade(release_path, app_dir) do
    # Extract to find version
    temp_dir = Path.join("/tmp", "upgrade_#{:erlang.unique_integer()}")
    File.mkdir_p!(temp_dir)

    case System.cmd("tar", ["-xzf", release_path, "-C", temp_dir]) do
      {_, 0} ->
        version = get_app_version(temp_dir)
        releases_dir = Path.join(app_dir, "releases/#{version}")
        File.mkdir_p!(releases_dir)
        File.cp_r!(Path.join(temp_dir, "releases/#{version}"), releases_dir)
        File.rm_rf!(temp_dir)
        {:ok, version}

      {error, _} ->
        File.rm_rf!(temp_dir)
        {:error, {:extract_failed, error}}
    end
  end

  defp run_upgrade(name, app_dir, from_version, to_version) do
    script = Path.join([app_dir, "bin", to_string(name)])

    # OTP release upgrade
    case System.cmd(script, ["upgrade", to_version]) do
      {_, 0} ->
        Logger.info("[Runtime] Upgraded #{name} from #{from_version} to #{to_version}")
        :ok

      {error, _} ->
        Logger.error("[Runtime] Upgrade failed: #{error}")
        {:error, {:upgrade_failed, error}}
    end
  end

  defp resolve_value({:env, var_name}), do: System.get_env(var_name)
  defp resolve_value({:secret, secret_path}), do: MaculaSecrets.get(secret_path)
  defp resolve_value(value), do: value

  defp discover_running_apps(state) do
    # Check @apps_dir for existing apps
    case File.ls(@apps_dir) do
      {:ok, dirs} ->
        apps =
          for dir <- dirs, File.dir?(Path.join(@apps_dir, dir)) do
            app_dir = Path.join(@apps_dir, dir)
            name = String.to_atom(dir)

            if app_running?(name, app_dir) do
              {name, %{
                name: name,
                version: get_app_version(app_dir),
                path: app_dir,
                env: %{},
                started_at: nil,
                status: :running
              }}
            end
          end
          |> Enum.filter(& &1)
          |> Map.new()

        %{state | apps: apps}

      {:error, _} ->
        state
    end
  end

  defp app_running?(name, app_dir) do
    script = Path.join([app_dir, "bin", to_string(name)])

    case System.cmd(script, ["pid"], stderr_to_stdout: true) do
      {_, 0} -> true
      _ -> false
    end
  end

  defp start_health_check(state, name, app_info) do
    # Start periodic health checking
    if health_spec = app_info[:health] do
      Task.start(fn -> health_check_loop(name, health_spec) end)
    end
    state
  end

  defp stop_health_check(state, _name) do
    # Health check will stop when app is gone
    state
  end

  defp health_check_loop(name, spec) do
    result = perform_health_check(spec)
    send(MaculaRuntime, {:health_result, name, result})
    Process.sleep(spec.interval || 10_000)
    health_check_loop(name, spec)
  end

  defp perform_health_check(%{type: :http, path: path, port: port, timeout: timeout}) do
    url = "http://localhost:#{port}#{path}"

    case :httpc.request(:get, {to_charlist(url), []}, [{:timeout, timeout}], []) do
      {:ok, {{_, 200, _}, _, _}} -> :healthy
      {:ok, {{_, status, _}, _, _}} -> {:unhealthy, {:http_status, status}}
      {:error, reason} -> {:unhealthy, reason}
    end
  end

  defp perform_health_check(%{type: :tcp, port: port, timeout: timeout}) do
    case :gen_tcp.connect(~c"localhost", port, [], timeout) do
      {:ok, socket} ->
        :gen_tcp.close(socket)
        :healthy

      {:error, reason} ->
        {:unhealthy, reason}
    end
  end
end
```

### Webhook Support (Optional)

For faster reconciliation on git push:

```elixir
defmodule MaculaGitopsWeb.WebhookController do
  use MaculaConsoleWeb, :controller

  def github(conn, %{"ref" => "refs/heads/main"} = params) do
    # Verify webhook signature
    case verify_signature(conn, params) do
      :ok ->
        # Trigger immediate reconciliation
        MaculaGitops.Reconciler.sync_now()
        json(conn, %{status: "ok"})

      {:error, reason} ->
        conn |> put_status(401) |> json(%{error: reason})
    end
  end

  def github(conn, _params) do
    # Ignore non-main branch pushes
    json(conn, %{status: "ignored"})
  end

  defp verify_signature(conn, _params) do
    secret = Application.get_env(:macula_gitops, :webhook_secret)
    signature = get_req_header(conn, "x-hub-signature-256") |> List.first()
    body = conn.assigns[:raw_body]

    expected = "sha256=" <> (:crypto.mac(:hmac, :sha256, secret, body) |> Base.encode16(case: :lower))

    if Plug.Crypto.secure_compare(signature, expected) do
      :ok
    else
      {:error, :invalid_signature}
    end
  end
end
```

### Local Git Server (Soft-Serve)

For offline/air-gapped deployments, embed a local git server:

```elixir
defmodule MaculaGitops.LocalServer do
  @moduledoc """
  Embedded git server using soft-serve for local GitOps.

  Allows pushing manifests directly to the node without external git hosting.
  """

  use GenServer

  @soft_serve_port 23231
  @repos_dir "/var/lib/maculaos/git-repos"

  def start_link(opts) do
    GenServer.start_link(__MODULE__, opts, name: __MODULE__)
  end

  def init(_opts) do
    File.mkdir_p!(@repos_dir)

    # Start soft-serve (if embedded)
    case System.find_executable("soft") do
      nil ->
        Logger.warning("[GitOps] soft-serve not found, local server disabled")
        {:ok, %{enabled: false}}

      path ->
        port = start_soft_serve(path)
        {:ok, %{enabled: true, port: port}}
    end
  end

  defp start_soft_serve(path) do
    Port.open({:spawn_executable, path}, [
      :binary,
      :exit_status,
      args: ["serve", "--port", to_string(@soft_serve_port), "--data-path", @repos_dir]
    ])
  end
end
```

### Summary: BEAM-Native GitOps

```
┌─────────────────────────────────────────────────────────────────┐
│                 BEAM-Native GitOps Stack                         │
├─────────────────────────────────────────────────────────────────┤
│                                                                  │
│  Config Format:    Elixir DSL (.exs) + YAML fallback            │
│  Git Integration:  Shell out to git (simple, reliable)          │
│  Reconcile Loop:   GenServer with timer (default: 60s)          │
│  Webhook Support:  Optional, for instant reconciliation         │
│  Runtime:          OTP releases with release_handler            │
│  Health Checks:    HTTP, TCP, or custom                         │
│  Hot Upgrades:     Native OTP upgrade mechanism                 │
│  Local Server:     Optional soft-serve for offline              │
│                                                                  │
│  Advantages over Flux:                                          │
│  ✓ No k8s dependency                                            │
│  ✓ Native hot code upgrades                                     │
│  ✓ Single binary (no CRDs, controllers)                         │
│  ✓ ~50KB vs ~50MB                                               │
│  ✓ Erlang terms as config (no YAML parsing issues)              │
│  ✓ Integrated with BEAM supervision                             │
│                                                                  │
└─────────────────────────────────────────────────────────────────┘
```

## OTP Release Management

A core capability for Pure BEAM MaculaOS: leveraging OTP's battle-tested release handling for zero-downtime deployments.

### What OTP Releases Provide

OTP releases are the standard way to package and deploy Erlang/Elixir applications for production. They include:

```
┌─────────────────────────────────────────────────────────────────┐
│                    OTP Release Structure                         │
├─────────────────────────────────────────────────────────────────┤
│                                                                  │
│  my_app/                                                         │
│  ├── bin/                                                        │
│  │   └── my_app          # Control script (start/stop/upgrade)  │
│  ├── lib/                                                        │
│  │   ├── my_app-1.0.0/   # Application BEAM files               │
│  │   ├── stdlib-4.0/     # Erlang stdlib                        │
│  │   └── ...             # All dependencies                      │
│  ├── releases/                                                   │
│  │   ├── 1.0.0/                                                 │
│  │   │   ├── my_app.rel  # Release resource file                │
│  │   │   ├── sys.config  # System configuration                 │
│  │   │   ├── vm.args     # VM arguments                         │
│  │   │   └── my_app.script  # Boot script                       │
│  │   └── RELEASES        # Release manifest                      │
│  └── erts-13.0/          # Embedded Erlang runtime (optional)   │
│                                                                  │
└─────────────────────────────────────────────────────────────────┘
```

### Hot Code Upgrade Mechanism

OTP's hot code upgrade is what makes BEAM unique - you can upgrade running code without stopping the system:

```
┌─────────────────────────────────────────────────────────────────┐
│                    Hot Code Upgrade Flow                         │
├─────────────────────────────────────────────────────────────────┤
│                                                                  │
│  1. Load new code (code_change callback)                        │
│     ┌─────────┐     ┌─────────┐                                 │
│     │ v1.0.0  │────▶│ v1.1.0  │  (both versions in memory)      │
│     │ current │     │ new     │                                 │
│     └─────────┘     └─────────┘                                 │
│                                                                  │
│  2. Suspend processes                                            │
│     ┌─────────────────────────────────────────────┐             │
│     │ GenServer A, GenServer B, ... (suspended)   │             │
│     └─────────────────────────────────────────────┘             │
│                                                                  │
│  3. Transform state via code_change/3                            │
│     ┌─────────┐     ┌─────────┐                                 │
│     │old_state│────▶│new_state│  (state migration)              │
│     └─────────┘     └─────────┘                                 │
│                                                                  │
│  4. Resume processes with new code                               │
│     ┌─────────────────────────────────────────────┐             │
│     │ GenServer A, GenServer B, ... (running v1.1.0)│           │
│     └─────────────────────────────────────────────┘             │
│                                                                  │
│  5. Purge old code                                               │
│     ┌─────────┐                                                 │
│     │ v1.0.0  │ (removed from memory)                           │
│     └─────────┘                                                 │
│                                                                  │
└─────────────────────────────────────────────────────────────────┘
```

### Appup Files: The Upgrade Recipe

The `.appup` file tells OTP how to upgrade between versions:

```erlang
%% my_app.appup
{"1.1.0",
 %% Upgrade instructions from older versions
 [{"1.0.0",
   [{load_module, my_app_server},
    {update, my_app_worker, {advanced, []}, [my_app_server]},
    {add_module, my_app_new_module}]}],
 %% Downgrade instructions to older versions
 [{"1.0.0",
   [{delete_module, my_app_new_module},
    {update, my_app_worker, {advanced, []}, []},
    {load_module, my_app_server}]}]}.
```

**Appup Instructions:**

| Instruction           | Purpose                                        |
| --------------------- | ---------------------------------------------- |
| `load_module`         | Simple module replacement (stateless)          |
| `update`              | Replace module + call code_change/3 (stateful) |
| `add_module`          | Add new module                                 |
| `delete_module`       | Remove module                                  |
| `apply`               | Run arbitrary function                         |
| `restart_application` | Full restart (fallback)                        |

### Implementing code_change/3

For stateful GenServers, implement `code_change/3` to migrate state:

```elixir
defmodule MyApp.Worker do
  use GenServer

  # State structure changed from v1.0.0 to v1.1.0:
  # v1.0.0: %{name: string, count: integer}
  # v1.1.0: %{name: string, count: integer, last_updated: DateTime}

  @impl true
  def code_change("1.0.0", old_state, _extra) do
    # Migrate state from v1.0.0 to v1.1.0
    new_state = Map.put(old_state, :last_updated, DateTime.utc_now())
    {:ok, new_state}
  end

  def code_change(_old_vsn, state, _extra) do
    # Default: keep state as-is
    {:ok, state}
  end
end
```

### Relup Files: The Release Upgrade Script

While `.appup` handles individual applications, `.relup` handles the entire release:

```erlang
%% relup
{"1.1.0",
 %% Upgrade from 1.0.0
 [{"1.0.0", [],
   [{load_object_code, {my_app, "1.1.0",
      [my_app_server, my_app_worker, my_app_new_module]}},
    point_of_no_return,
    {suspend, [my_app_worker]},
    {load, {my_app_worker, brutal_purge, brutal_purge}},
    {load, {my_app_server, brutal_purge, brutal_purge}},
    {load, {my_app_new_module, brutal_purge, brutal_purge}},
    {code_change, up, [{my_app_worker, []}]},
    {resume, [my_app_worker]},
    {purge, [my_app_server, my_app_worker]}]}],
 %% Downgrade to 1.0.0
 [{"1.0.0", [],
   [{load_object_code, {my_app, "1.0.0",
      [my_app_server, my_app_worker]}},
    point_of_no_return,
    {suspend, [my_app_worker]},
    {code_change, down, [{my_app_worker, []}]},
    {load, {my_app_worker, brutal_purge, brutal_purge}},
    {load, {my_app_server, brutal_purge, brutal_purge}},
    {delete_module, my_app_new_module},
    {resume, [my_app_worker]},
    {purge, [my_app_server, my_app_worker]}]}]}.
```

### Elixir/Mix Release Integration

Mix releases (since Elixir 1.9) support hot upgrades:

```elixir
# mix.exs
def project do
  [
    app: :my_app,
    version: "1.1.0",
    releases: [
      my_app: [
        include_executables_for: [:unix],
        steps: [:assemble, :tar],
        # Enable upgrade artifacts
        applications: [my_app: :permanent],
        # Generate relup from previous release
        rel_templates_path: "rel"
      ]
    ]
  ]
end
```

**Generate Upgrade Tarball:**

```bash
# Build release with upgrade capability
MIX_ENV=prod mix release --upgrade

# Creates:
# _build/prod/my_app-1.1.0.tar.gz
```

### MaculaRuntime Upgrade Implementation

The `MaculaRuntime` module from the GitOps section needs to leverage these OTP primitives:

```elixir
defmodule MaculaRuntime.Upgrader do
  @moduledoc """
  Handles OTP release upgrades using :release_handler.
  """

  require Logger

  @doc """
  Perform a hot upgrade from current version to new version.

  This uses OTP's release_handler for proper hot code upgrade.
  """
  def upgrade(app_dir, new_version) do
    # 1. Unpack the release
    with :ok <- unpack_release(app_dir, new_version),
         # 2. Install the release (runs relup)
         {:ok, _} <- install_release(new_version),
         # 3. Make it permanent (survives restart)
         :ok <- make_permanent(new_version) do
      Logger.info("[Upgrader] Successfully upgraded to #{new_version}")
      :ok
    else
      {:error, reason} = error ->
        Logger.error("[Upgrader] Upgrade failed: #{inspect(reason)}")
        error
    end
  end

  @doc """
  Rollback to a previous version.
  """
  def rollback(previous_version) do
    case :release_handler.install_release(to_charlist(previous_version)) do
      {:ok, _, _} ->
        :release_handler.make_permanent(to_charlist(previous_version))
        Logger.info("[Upgrader] Rolled back to #{previous_version}")
        :ok

      {:error, reason} ->
        Logger.error("[Upgrader] Rollback failed: #{inspect(reason)}")
        {:error, reason}
    end
  end

  @doc """
  List available releases (installed versions).
  """
  def list_releases do
    :release_handler.which_releases()
    |> Enum.map(fn {name, vsn, _apps, status} ->
      %{
        name: to_string(name),
        version: to_string(vsn),
        status: status  # :permanent, :current, :old
      }
    end)
  end

  # Private functions

  defp unpack_release(app_dir, version) do
    release_tar = Path.join([app_dir, "releases", "#{version}.tar.gz"])

    case :release_handler.unpack_release(to_charlist(version)) do
      {:ok, _} -> :ok
      {:error, {:already_installed, _}} -> :ok
      {:error, reason} -> {:error, {:unpack_failed, reason}}
    end
  end

  defp install_release(version) do
    # This executes the relup script
    case :release_handler.install_release(to_charlist(version)) do
      {:ok, old_vsn, _} ->
        Logger.info("[Upgrader] Upgraded from #{old_vsn} to #{version}")
        {:ok, old_vsn}

      {:error, {:no_such_release, _}} ->
        {:error, :release_not_found}

      {:error, {:code_change_failed, pid, mod, reason}} ->
        Logger.error("[Upgrader] code_change failed in #{mod}: #{inspect(reason)}")
        {:error, {:code_change_failed, mod, reason}}

      {:error, reason} ->
        {:error, reason}
    end
  end

  defp make_permanent(version) do
    case :release_handler.make_permanent(to_charlist(version)) do
      :ok -> :ok
      {:error, reason} -> {:error, {:make_permanent_failed, reason}}
    end
  end
end
```

### Automatic Appup Generation

Writing appups manually is error-prone. We can automate it:

```elixir
defmodule MaculaRuntime.AppupGenerator do
  @moduledoc """
  Automatically generates .appup files by diffing module bytecode.
  """

  @doc """
  Generate appup from two release directories.
  """
  def generate(old_release_dir, new_release_dir, app_name) do
    old_modules = get_modules(old_release_dir, app_name)
    new_modules = get_modules(new_release_dir, app_name)

    old_set = MapSet.new(Map.keys(old_modules))
    new_set = MapSet.new(Map.keys(new_modules))

    added = MapSet.difference(new_set, old_set)
    removed = MapSet.difference(old_set, new_set)
    common = MapSet.intersection(old_set, new_set)

    changed =
      common
      |> Enum.filter(fn mod ->
        old_modules[mod].hash != new_modules[mod].hash
      end)

    up_instructions =
      Enum.map(added, &{:add_module, &1}) ++
      Enum.map(changed, &upgrade_instruction(&1, new_modules[&1]))

    down_instructions =
      Enum.map(added, &{:delete_module, &1}) ++
      Enum.map(changed, &downgrade_instruction(&1, old_modules[&1]))

    {up_instructions, down_instructions}
  end

  defp get_modules(release_dir, app_name) do
    lib_dir = Path.join(release_dir, "lib")

    app_dir =
      lib_dir
      |> File.ls!()
      |> Enum.find(&String.starts_with?(&1, "#{app_name}-"))

    beam_dir = Path.join([lib_dir, app_dir, "ebin"])

    beam_dir
    |> File.ls!()
    |> Enum.filter(&String.ends_with?(&1, ".beam"))
    |> Enum.map(fn file ->
      path = Path.join(beam_dir, file)
      {:ok, {mod, chunks}} = :beam_lib.chunks(to_charlist(path), [:abstract_code, :attributes])

      attrs = Keyword.get(chunks, :attributes, [])
      has_state = has_gen_server_state?(attrs)

      {mod, %{
        path: path,
        hash: md5_file(path),
        stateful: has_state
      }}
    end)
    |> Map.new()
  end

  defp has_gen_server_state?(attrs) do
    behaviours = Keyword.get(attrs, :behaviour, [])
    :gen_server in behaviours or GenServer in behaviours
  end

  defp md5_file(path) do
    path |> File.read!() |> :erlang.md5() |> Base.encode16()
  end

  defp upgrade_instruction(mod, %{stateful: true}) do
    # Stateful module needs code_change
    {:update, mod, {:advanced, []}}
  end

  defp upgrade_instruction(mod, %{stateful: false}) do
    # Stateless module - simple reload
    {:load_module, mod}
  end

  defp downgrade_instruction(mod, info) do
    # Same logic for downgrade
    upgrade_instruction(mod, info)
  end
end
```

### Graceful Upgrade Strategy

For Phoenix/LiveView apps, we need to handle WebSocket connections gracefully:

```elixir
defmodule MaculaRuntime.GracefulUpgrade do
  @moduledoc """
  Coordinates graceful upgrades for web applications.
  """

  require Logger

  @drain_timeout :timer.seconds(30)

  def upgrade_with_drain(app_name, new_version) do
    # 1. Stop accepting new connections
    Logger.info("[GracefulUpgrade] Draining connections for #{app_name}")
    drain_connections(app_name)

    # 2. Wait for existing connections to finish (with timeout)
    wait_for_drain(app_name, @drain_timeout)

    # 3. Perform the upgrade
    result = MaculaRuntime.Upgrader.upgrade(app_name, new_version)

    # 4. Resume accepting connections
    resume_connections(app_name)

    result
  end

  defp drain_connections(app_name) do
    # Set health check to return 503 (for load balancers)
    Phoenix.PubSub.broadcast(
      MaculaCluster.PubSub,
      "app:#{app_name}",
      {:health_status, :draining}
    )

    # Signal Phoenix to reject new WebSocket connections
    # This depends on your endpoint configuration
  end

  defp wait_for_drain(app_name, timeout) do
    deadline = System.monotonic_time(:millisecond) + timeout

    wait_loop(app_name, deadline)
  end

  defp wait_loop(app_name, deadline) do
    active_connections = count_active_connections(app_name)

    cond do
      active_connections == 0 ->
        Logger.info("[GracefulUpgrade] All connections drained")
        :ok

      System.monotonic_time(:millisecond) > deadline ->
        Logger.warning("[GracefulUpgrade] Drain timeout, #{active_connections} connections remaining")
        :timeout

      true ->
        Process.sleep(100)
        wait_loop(app_name, deadline)
    end
  end

  defp count_active_connections(_app_name) do
    # Count Phoenix.Socket connections
    # This is framework-specific
    0
  end

  defp resume_connections(app_name) do
    Phoenix.PubSub.broadcast(
      MaculaCluster.PubSub,
      "app:#{app_name}",
      {:health_status, :healthy}
    )
  end
end
```

### Rollback on Failure

Implement automatic rollback if upgrade fails health checks:

```elixir
defmodule MaculaRuntime.SafeUpgrade do
  @moduledoc """
  Upgrade with automatic rollback on failure.
  """

  require Logger

  @health_check_delay :timer.seconds(5)
  @health_check_timeout :timer.seconds(30)

  def upgrade_with_rollback(app_name, new_version) do
    # Record current version for potential rollback
    current_version = get_current_version(app_name)

    Logger.info("[SafeUpgrade] Upgrading #{app_name} #{current_version} -> #{new_version}")

    case MaculaRuntime.Upgrader.upgrade(app_name, new_version) do
      :ok ->
        # Wait and verify health
        Process.sleep(@health_check_delay)

        case verify_health(app_name, @health_check_timeout) do
          :healthy ->
            Logger.info("[SafeUpgrade] Upgrade successful, app is healthy")
            {:ok, new_version}

          {:unhealthy, reason} ->
            Logger.error("[SafeUpgrade] App unhealthy after upgrade: #{inspect(reason)}")
            Logger.info("[SafeUpgrade] Rolling back to #{current_version}")

            case MaculaRuntime.Upgrader.rollback(current_version) do
              :ok ->
                {:error, {:rolled_back, reason}}

              {:error, rollback_error} ->
                Logger.error("[SafeUpgrade] Rollback also failed: #{inspect(rollback_error)}")
                {:error, {:upgrade_and_rollback_failed, reason, rollback_error}}
            end
        end

      {:error, reason} ->
        Logger.error("[SafeUpgrade] Upgrade failed: #{inspect(reason)}")
        {:error, reason}
    end
  end

  defp get_current_version(app_name) do
    :release_handler.which_releases()
    |> Enum.find(fn {name, _, _, status} ->
      to_string(name) == to_string(app_name) and status == :current
    end)
    |> case do
      {_, vsn, _, _} -> to_string(vsn)
      nil -> "unknown"
    end
  end

  defp verify_health(app_name, timeout) do
    deadline = System.monotonic_time(:millisecond) + timeout

    health_loop(app_name, deadline)
  end

  defp health_loop(app_name, deadline) do
    case MaculaRuntime.health(app_name) do
      :healthy ->
        :healthy

      {:unhealthy, reason} when System.monotonic_time(:millisecond) > deadline ->
        {:unhealthy, reason}

      _ ->
        Process.sleep(500)
        health_loop(app_name, deadline)
    end
  end
end
```

### Blue-Green Deployment Alternative

For applications that can't do hot upgrades (e.g., database schema changes), use blue-green:

```elixir
defmodule MaculaRuntime.BlueGreen do
  @moduledoc """
  Blue-Green deployment for incompatible upgrades.

  Runs old and new versions simultaneously, then switches traffic.
  """

  require Logger

  def deploy_blue_green(app_name, new_version) do
    # 1. Deploy new version alongside old (different port)
    green_port = get_green_port(app_name)

    {:ok, green_app} = deploy_green(app_name, new_version, green_port)

    # 2. Wait for green to be healthy
    case wait_for_healthy(green_app, :timer.seconds(60)) do
      :ok ->
        # 3. Switch traffic (update proxy config)
        :ok = switch_traffic(app_name, green_port)

        # 4. Drain and stop old (blue) version
        Process.sleep(:timer.seconds(10))
        :ok = stop_blue(app_name)

        # 5. Rebind green to standard port
        :ok = rebind_port(app_name, green_port)

        {:ok, new_version}

      {:error, reason} ->
        # Green failed - clean up and abort
        stop_green(green_app)
        {:error, {:green_unhealthy, reason}}
    end
  end

  # Implementation details...
  defp get_green_port(_app_name), do: 4100  # Offset port
  defp deploy_green(_app_name, _version, _port), do: {:ok, %{}}
  defp wait_for_healthy(_app, _timeout), do: :ok
  defp switch_traffic(_app_name, _port), do: :ok
  defp stop_blue(_app_name), do: :ok
  defp stop_green(_app), do: :ok
  defp rebind_port(_app_name, _port), do: :ok
end
```

### Summary: OTP Release Management

```
┌─────────────────────────────────────────────────────────────────┐
│              OTP Release Management Summary                      │
├─────────────────────────────────────────────────────────────────┤
│                                                                  │
│  Key Files:                                                      │
│  ├── .appup    - Per-application upgrade instructions           │
│  ├── .relup    - Release-wide upgrade script                    │
│  └── RELEASES  - Installed release manifest                     │
│                                                                  │
│  Key Functions:                                                  │
│  ├── :release_handler.unpack_release/1                          │
│  ├── :release_handler.install_release/1   <- Hot upgrade!       │
│  ├── :release_handler.make_permanent/1                          │
│  └── :release_handler.which_releases/0                          │
│                                                                  │
│  code_change/3 Callback:                                         │
│  ├── Called during upgrade for stateful processes               │
│  ├── Migrates state from old version to new                     │
│  └── Return {:ok, new_state} or {:error, reason}                │
│                                                                  │
│  Upgrade Strategies:                                             │
│  ├── Hot Upgrade  - Zero downtime, state preserved              │
│  ├── Blue-Green   - For incompatible changes (schema, etc.)     │
│  └── Rolling      - For clustered deployments                   │
│                                                                  │
│  MaculaOS Integration:                                           │
│  ├── MaculaRuntime.Upgrader    - OTP release_handler wrapper    │
│  ├── MaculaRuntime.SafeUpgrade - Auto-rollback on failure       │
│  ├── AppupGenerator            - Auto-generate .appup files     │
│  └── GracefulUpgrade           - Drain connections first        │
│                                                                  │
│  Why This Matters for MaculaOS:                                  │
│  ✓ Zero-downtime updates (no container restarts)                │
│  ✓ State preserved across upgrades                              │
│  ✓ Automatic rollback capability                                │
│  ✓ Native to BEAM (no k8s rolling update needed)                │
│  ✓ Works for edge devices with limited connectivity             │
│                                                                  │
└─────────────────────────────────────────────────────────────────┘
```

## Topics to Explore Next

1. **TUI-First Console** - Redesign macula-console as TUI-primary, web-secondary
2. **Micro-Frontend for Phoenix** - How to load app LiveViews in Console shell
3. **tmux-like Multiplexer** - Multiple app outputs in TUI panes
4. **Storage Without k8s PVCs** - Local filesystem, SQLite, distributed options
5. **Secrets Management** - SOPS, Vault, or BEAM-native

## Open Questions

1. **App Isolation:** Without containers, how do we isolate misbehaving apps?
   - Option A: Separate BEAM nodes per app (defeats purpose)
   - Option B: Resource limits via BEAM schedulers
   - Option C: Accept shared-fate model (BEAM is good at this)

2. **Storage:** k8s has PVCs, what do we use?
   - Option A: Local filesystem + replication via mesh
   - Option B: SQLite/Mnesia per app
   - Option C: Distributed storage (e.g., Riak Core)

3. **Secrets:** k8s has Secrets, what do we use?
   - Option A: Encrypted files in git (SOPS-style)
   - Option B: Vault integration
   - Option C: BEAM-native secret store

4. **Multi-tenancy:** How do we support multiple users/orgs?
   - Option A: Separate BEAM nodes
   - Option B: Namespace isolation in the app layer
   - Option C: This is for single-user/small-team anyway

## Conclusion

A pure BEAM MaculaOS is technically feasible and would result in:

- **Smaller image:** ~80MB vs ~1.5GB
- **Faster boot:** ~5s vs ~30s
- **Native hot upgrades:** No downtime deployments
- **Simpler networking:** No CNI/iptables complexity
- **Universal TUI:** Any language can build clients via NATS

The main trade-offs:

- Less isolation between apps
- Custom tooling instead of k8s ecosystem
- Need to build GitOps reconciler from scratch

This aligns well with the Macula philosophy: BEAM-native, decentralized, edge-first.
