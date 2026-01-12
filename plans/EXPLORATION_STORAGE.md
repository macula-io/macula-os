# Exploration: Storage Without Kubernetes PVCs

**Status:** Exploration / RFC
**Created:** 2026-01-12
**Related:** EXPLORATION_BEAM_NATIVE_GITOPS.md

## Overview

In a Pure BEAM MaculaOS (without Kubernetes), we need alternatives to Kubernetes PersistentVolumeClaims (PVCs) for application storage.

## The Problem

Kubernetes provides:
- PersistentVolumes (PV) - storage resources
- PersistentVolumeClaims (PVC) - requests for storage
- StorageClasses - dynamic provisioning
- Volume snapshots - backup/restore

Without k8s, applications need their own storage strategy.

## Storage Categories

```
┌─────────────────────────────────────────────────────────────────┐
│                    Storage Categories                            │
├─────────────────────────────────────────────────────────────────┤
│                                                                  │
│  1. Application State (hot)                                      │
│     ├── In-memory: ETS, DETS, persistent_term                   │
│     ├── Local: SQLite, Mnesia, CubDB                            │
│     └── Distributed: Khepri/Ra, Riak Core                       │
│                                                                  │
│  2. Application Data (warm)                                      │
│     ├── User uploads, documents                                  │
│     ├── Media files                                              │
│     └── Generated content                                        │
│                                                                  │
│  3. System Data (cold)                                           │
│     ├── Logs                                                     │
│     ├── Metrics                                                  │
│     └── Backups                                                  │
│                                                                  │
└─────────────────────────────────────────────────────────────────┘
```

---

## Option A: Local Filesystem with Convention

The simplest approach: Apps store data in well-known locations.

```
/var/lib/maculaos/
├── apps/
│   ├── console/
│   │   ├── data/           # App data
│   │   ├── uploads/        # User uploads
│   │   └── cache/          # Temporary data
│   ├── my-app/
│   │   ├── data/
│   │   └── sqlite.db
│   └── ...
├── system/
│   ├── gitops/             # GitOps repo clone
│   ├── releases/           # OTP releases
│   └── secrets/            # Encrypted secrets
└── shared/
    ├── logs/               # Centralized logs
    └── metrics/            # Metrics data
```

**Implementation:**

```elixir
defmodule MaculaStorage do
  @base_path "/var/lib/maculaos"

  def app_data_path(app_name) do
    Path.join([@base_path, "apps", to_string(app_name), "data"])
  end

  def app_uploads_path(app_name) do
    Path.join([@base_path, "apps", to_string(app_name), "uploads"])
  end

  def ensure_app_dirs(app_name) do
    for subdir <- ["data", "uploads", "cache"] do
      path = Path.join([@base_path, "apps", to_string(app_name), subdir])
      File.mkdir_p!(path)
    end
    :ok
  end

  def app_quota(app_name) do
    # Get disk usage for app
    path = Path.join([@base_path, "apps", to_string(app_name)])
    {output, 0} = System.cmd("du", ["-sb", path])
    [size | _] = String.split(output)
    String.to_integer(size)
  end
end
```

**Pros:**
- Simple, no dependencies
- Easy to backup (rsync)
- Works offline

**Cons:**
- No replication
- Manual quota management
- Single point of failure

---

## Option B: SQLite per Application

Each app gets its own SQLite database for structured data.

```elixir
defmodule MaculaStorage.SQLite do
  @moduledoc """
  SQLite-based storage for applications.

  Each app gets a dedicated SQLite database.
  """

  def db_path(app_name) do
    Path.join([MaculaStorage.app_data_path(app_name), "app.db"])
  end

  def open(app_name) do
    path = db_path(app_name)
    File.mkdir_p!(Path.dirname(path))

    {:ok, conn} = Exqlite.Sqlite3.open(path)

    # Enable WAL mode for better concurrency
    :ok = Exqlite.Sqlite3.execute(conn, "PRAGMA journal_mode=WAL")

    # Enable foreign keys
    :ok = Exqlite.Sqlite3.execute(conn, "PRAGMA foreign_keys=ON")

    {:ok, conn}
  end

  def backup(app_name, dest_path) do
    {:ok, conn} = open(app_name)
    :ok = Exqlite.Sqlite3.execute(conn, "VACUUM INTO '#{dest_path}'")
    Exqlite.Sqlite3.close(conn)
  end
end
```

**Phoenix Integration:**

```elixir
# In app's config
config :my_app, MyApp.Repo,
  database: "/var/lib/maculaos/apps/my_app/data/app.db",
  pool_size: 5,
  journal_mode: :wal
```

**Pros:**
- Lightweight (single file)
- ACID compliant
- Easy backup (copy file)
- Good for edge devices

**Cons:**
- Write contention under load
- Not distributed
- Limited concurrent writers

---

## Option C: Mnesia for Distributed State

Mnesia provides distributed, replicated storage native to BEAM.

```elixir
defmodule MaculaStorage.Mnesia do
  @moduledoc """
  Mnesia-based distributed storage.
  """

  def setup(nodes) do
    # Stop Mnesia if running
    :mnesia.stop()

    # Create schema on all nodes
    :mnesia.create_schema(nodes)

    # Start Mnesia on all nodes
    for node <- nodes do
      :rpc.call(node, :mnesia, :start, [])
    end

    :ok
  end

  def create_app_table(app_name, opts \\ []) do
    table_name = :"#{app_name}_data"

    attributes = Keyword.get(opts, :attributes, [:key, :value, :updated_at])
    replicas = Keyword.get(opts, :replicas, [node()])

    :mnesia.create_table(table_name, [
      {:attributes, attributes},
      {:disc_copies, replicas},  # Persisted to disk
      {:type, :set}
    ])
  end

  def put(app_name, key, value) do
    table = :"#{app_name}_data"
    record = {table, key, value, DateTime.utc_now()}

    :mnesia.transaction(fn ->
      :mnesia.write(record)
    end)
  end

  def get(app_name, key) do
    table = :"#{app_name}_data"

    :mnesia.transaction(fn ->
      case :mnesia.read(table, key) do
        [{^table, ^key, value, _updated}] -> {:ok, value}
        [] -> {:error, :not_found}
      end
    end)
  end

  def backup(app_name, path) do
    table = :"#{app_name}_data"
    :mnesia.backup_checkpoint({:app_backup, [table]}, path)
  end
end
```

**Pros:**
- Native to BEAM
- Automatic replication
- Transactions
- Schema evolution

**Cons:**
- Network partition handling is complex
- Not great for large binary data
- Learning curve

---

## Option D: Khepri (Ra-based, Modern)

Khepri is a modern alternative to Mnesia, built on Ra (Raft consensus).

```elixir
defmodule MaculaStorage.Khepri do
  @moduledoc """
  Khepri-based storage using Raft consensus.

  Better than Mnesia for:
  - Partition tolerance
  - Consistent writes
  - Modern API
  """

  @store_id :macula_storage

  def start(nodes) do
    :khepri.start(@store_id, %{
      members: nodes,
      data_dir: "/var/lib/maculaos/khepri"
    })
  end

  def put(path, value) when is_list(path) do
    :khepri.put(@store_id, path, value)
  end

  def get(path) when is_list(path) do
    case :khepri.get(@store_id, path) do
      {:ok, value} -> {:ok, value}
      {:error, :node_not_found} -> {:error, :not_found}
      error -> error
    end
  end

  def delete(path) when is_list(path) do
    :khepri.delete(@store_id, path)
  end

  # App-specific helpers
  def app_put(app_name, key, value) do
    put([:apps, app_name, key], value)
  end

  def app_get(app_name, key) do
    get([:apps, app_name, key])
  end

  def app_list_keys(app_name) do
    case :khepri.list(@store_id, [:apps, app_name]) do
      {:ok, keys} -> {:ok, keys}
      _ -> {:ok, []}
    end
  end
end
```

**Pros:**
- Strong consistency (Raft)
- Better partition handling than Mnesia
- Tree-structured data
- Modern, maintained

**Cons:**
- Write latency (consensus)
- Quorum required for writes
- Relatively new

---

## Option E: CubDB (Embedded, Append-Only)

CubDB is a pure-Elixir embedded database, great for single-node apps.

```elixir
defmodule MaculaStorage.CubDB do
  @moduledoc """
  CubDB-based storage for single-node applications.
  """

  def start_link(app_name) do
    data_dir = Path.join([MaculaStorage.app_data_path(app_name), "cubdb"])
    CubDB.start_link(data_dir: data_dir, name: db_name(app_name))
  end

  def put(app_name, key, value) do
    CubDB.put(db_name(app_name), key, value)
  end

  def get(app_name, key) do
    CubDB.get(db_name(app_name), key)
  end

  def get_and_update(app_name, key, fun) do
    CubDB.get_and_update(db_name(app_name), key, fun)
  end

  def select(app_name, opts \\ []) do
    CubDB.select(db_name(app_name), opts)
  end

  defp db_name(app_name), do: :"cubdb_#{app_name}"
end
```

**Pros:**
- Pure Elixir, no NIFs
- ACID compliant
- Automatic compaction
- Simple API

**Cons:**
- Single-node only
- No replication
- Less mature than SQLite

---

## Option F: Mesh-Replicated Filesystem

For multi-node deployments, replicate files across the mesh.

```elixir
defmodule MaculaStorage.MeshFS do
  @moduledoc """
  Mesh-replicated filesystem.

  Files are stored locally and replicated to peer nodes.
  """

  @replication_factor 2

  def write(app_name, filename, content) do
    # Write locally
    local_path = local_file_path(app_name, filename)
    File.mkdir_p!(Path.dirname(local_path))
    File.write!(local_path, content)

    # Replicate to peers
    replicate_to_peers(app_name, filename, content)
  end

  def read(app_name, filename) do
    local_path = local_file_path(app_name, filename)

    case File.read(local_path) do
      {:ok, content} -> {:ok, content}
      {:error, :enoent} -> fetch_from_peers(app_name, filename)
      error -> error
    end
  end

  defp replicate_to_peers(app_name, filename, content) do
    peers = select_replication_peers(@replication_factor)

    for peer <- peers do
      # Use mesh RPC to write to peer
      :macula.call(
        get_client(),
        "io.macula.storage.write",
        %{app: app_name, file: filename, content: Base.encode64(content)},
        node: peer
      )
    end
  end

  defp fetch_from_peers(app_name, filename) do
    peers = MaculaMesh.get_peers()

    Enum.find_value(peers, {:error, :not_found}, fn peer ->
      case :macula.call(
        get_client(),
        "io.macula.storage.read",
        %{app: app_name, file: filename},
        node: peer
      ) do
        {:ok, %{"content" => encoded}} ->
          {:ok, Base.decode64!(encoded)}
        _ ->
          nil
      end
    end)
  end

  defp local_file_path(app_name, filename) do
    Path.join([MaculaStorage.app_data_path(app_name), "files", filename])
  end

  defp select_replication_peers(count) do
    MaculaMesh.get_peers()
    |> Enum.shuffle()
    |> Enum.take(count)
  end
end
```

**Pros:**
- Distributed across mesh
- Survives node failure
- Works with existing filesystem tools

**Cons:**
- Eventual consistency
- Network overhead
- Conflict resolution needed

---

## Recommendation: Tiered Storage

Use different storage backends based on data characteristics:

```
┌─────────────────────────────────────────────────────────────────┐
│                    Tiered Storage Strategy                       │
├─────────────────────────────────────────────────────────────────┤
│                                                                  │
│  Tier 1: Hot State (frequent access, consistency needed)        │
│  └── Khepri/Mnesia for distributed state                        │
│  └── ETS/persistent_term for local cache                        │
│                                                                  │
│  Tier 2: Warm Data (app data, queries)                          │
│  └── SQLite for structured data (per-app)                       │
│  └── CubDB for key-value data (per-app)                         │
│                                                                  │
│  Tier 3: Cold Data (files, backups)                             │
│  └── Local filesystem with convention                           │
│  └── MeshFS for replication                                     │
│                                                                  │
│  Tier 4: Archive (long-term, rarely accessed)                   │
│  └── Compressed tarball to /bulk drives                         │
│  └── Optional: S3-compatible object storage                     │
│                                                                  │
└─────────────────────────────────────────────────────────────────┘
```

**Implementation:**

```elixir
defmodule MaculaStorage do
  @moduledoc """
  Unified storage API with tiered backends.
  """

  # Tier 1: Distributed state
  defdelegate state_put(key, value), to: MaculaStorage.Khepri, as: :put
  defdelegate state_get(key), to: MaculaStorage.Khepri, as: :get

  # Tier 2: App data
  def data_put(app, key, value) do
    MaculaStorage.CubDB.put(app, key, value)
  end

  def data_get(app, key) do
    MaculaStorage.CubDB.get(app, key)
  end

  # Tier 3: Files
  def file_write(app, filename, content, opts \\ []) do
    replicate = Keyword.get(opts, :replicate, false)

    if replicate do
      MaculaStorage.MeshFS.write(app, filename, content)
    else
      path = Path.join([app_data_path(app), "files", filename])
      File.mkdir_p!(Path.dirname(path))
      File.write!(path, content)
    end
  end

  def file_read(app, filename, opts \\ []) do
    replicate = Keyword.get(opts, :replicate, false)

    if replicate do
      MaculaStorage.MeshFS.read(app, filename)
    else
      path = Path.join([app_data_path(app), "files", filename])
      File.read(path)
    end
  end

  # Tier 4: Archive
  def archive(app, dest_path) do
    app_dir = Path.join(["/var/lib/maculaos/apps", to_string(app)])
    System.cmd("tar", ["-czf", dest_path, "-C", app_dir, "."])
  end

  def restore(app, archive_path) do
    app_dir = Path.join(["/var/lib/maculaos/apps", to_string(app)])
    File.mkdir_p!(app_dir)
    System.cmd("tar", ["-xzf", archive_path, "-C", app_dir])
  end
end
```

---

## Backup Strategy

```elixir
defmodule MaculaBackup do
  @moduledoc """
  Backup and restore for MaculaOS storage.
  """

  @backup_dir "/var/lib/maculaos/backups"

  def backup_app(app_name) do
    timestamp = DateTime.utc_now() |> DateTime.to_iso8601(:basic)
    backup_name = "#{app_name}_#{timestamp}"
    backup_path = Path.join(@backup_dir, backup_name)
    File.mkdir_p!(backup_path)

    # Backup SQLite
    sqlite_path = MaculaStorage.SQLite.db_path(app_name)
    if File.exists?(sqlite_path) do
      MaculaStorage.SQLite.backup(app_name, Path.join(backup_path, "app.db"))
    end

    # Backup CubDB
    cubdb_dir = Path.join([MaculaStorage.app_data_path(app_name), "cubdb"])
    if File.exists?(cubdb_dir) do
      File.cp_r!(cubdb_dir, Path.join(backup_path, "cubdb"))
    end

    # Backup files
    files_dir = Path.join([MaculaStorage.app_data_path(app_name), "files"])
    if File.exists?(files_dir) do
      File.cp_r!(files_dir, Path.join(backup_path, "files"))
    end

    # Create tarball
    tarball = "#{backup_path}.tar.gz"
    {_, 0} = System.cmd("tar", ["-czf", tarball, "-C", @backup_dir, backup_name])
    File.rm_rf!(backup_path)

    {:ok, tarball}
  end

  def restore_app(app_name, backup_tarball) do
    # Extract tarball
    temp_dir = Path.join("/tmp", "restore_#{:erlang.unique_integer()}")
    {_, 0} = System.cmd("tar", ["-xzf", backup_tarball, "-C", temp_dir])

    backup_name = Path.basename(backup_tarball, ".tar.gz")
    backup_path = Path.join(temp_dir, backup_name)

    # Restore SQLite
    sqlite_backup = Path.join(backup_path, "app.db")
    if File.exists?(sqlite_backup) do
      sqlite_dest = MaculaStorage.SQLite.db_path(app_name)
      File.mkdir_p!(Path.dirname(sqlite_dest))
      File.cp!(sqlite_backup, sqlite_dest)
    end

    # Restore CubDB
    cubdb_backup = Path.join(backup_path, "cubdb")
    if File.exists?(cubdb_backup) do
      cubdb_dest = Path.join([MaculaStorage.app_data_path(app_name), "cubdb"])
      File.rm_rf!(cubdb_dest)
      File.cp_r!(cubdb_backup, cubdb_dest)
    end

    # Restore files
    files_backup = Path.join(backup_path, "files")
    if File.exists?(files_backup) do
      files_dest = Path.join([MaculaStorage.app_data_path(app_name), "files"])
      File.cp_r!(files_backup, files_dest)
    end

    File.rm_rf!(temp_dir)
    :ok
  end

  def list_backups(app_name) do
    @backup_dir
    |> File.ls!()
    |> Enum.filter(&String.starts_with?(&1, "#{app_name}_"))
    |> Enum.sort(:desc)
  end
end
```

---

## Summary

```
┌─────────────────────────────────────────────────────────────────┐
│                Storage Without k8s PVCs Summary                  │
├─────────────────────────────────────────────────────────────────┤
│                                                                  │
│  Recommended Strategy: Tiered Storage                           │
│                                                                  │
│  Tier 1 (Hot State):                                            │
│  └── Khepri for distributed consensus state                     │
│  └── ETS/persistent_term for local cache                        │
│                                                                  │
│  Tier 2 (App Data):                                             │
│  └── SQLite for relational data                                 │
│  └── CubDB for key-value data                                   │
│                                                                  │
│  Tier 3 (Files):                                                │
│  └── Local filesystem with conventions                          │
│  └── MeshFS for cross-node replication                          │
│                                                                  │
│  Tier 4 (Archive):                                              │
│  └── Compressed tarballs                                        │
│  └── /bulk drives on beam-cluster                               │
│                                                                  │
│  Key Conventions:                                                │
│  └── /var/lib/maculaos/apps/{app}/data/                         │
│  └── /var/lib/maculaos/apps/{app}/uploads/                      │
│  └── /var/lib/maculaos/backups/                                 │
│                                                                  │
│  Backup Strategy:                                                │
│  └── Per-app backup to tarball                                  │
│  └── Point-in-time restore                                      │
│  └── List and manage backups                                    │
│                                                                  │
└─────────────────────────────────────────────────────────────────┘
```

## Open Questions

1. **Quota Enforcement:** How to limit storage per app?
   - Option A: Filesystem quotas (requires root)
   - Option B: Soft limits with alerts
   - Option C: Trust apps to behave

2. **Encryption at Rest:** Should app data be encrypted?
   - Option A: LUKS full-disk encryption
   - Option B: Per-app encrypted directories
   - Option C: App-level encryption (app responsibility)

3. **Garbage Collection:** How to clean up orphaned data?
   - Option A: Reference counting
   - Option B: Periodic cleanup job
   - Option C: Manual management

4. **Large Files:** How to handle multi-GB files?
   - Option A: Stream to disk directly
   - Option B: Chunk and reassemble
   - Option C: External storage (S3, NFS)

## Next Steps

1. Implement MaculaStorage module with tiered backends
2. Add backup/restore functionality
3. Integrate with GitOps (storage config in manifests)
4. Add TUI commands for storage management
5. Implement MeshFS for distributed files
