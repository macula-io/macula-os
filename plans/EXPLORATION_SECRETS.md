# Exploration: Secrets Management

**Status:** Exploration / RFC
**Created:** 2026-01-12
**Related:** EXPLORATION_BEAM_NATIVE_GITOPS.md, EXPLORATION_STORAGE.md

## Overview

In Pure BEAM MaculaOS (without Kubernetes), we need alternatives to Kubernetes Secrets for managing sensitive configuration (API keys, database passwords, certificates).

## The Problem

Kubernetes provides:
- Secrets (base64 encoded, not encrypted at rest by default)
- External secrets operators (Vault, AWS Secrets Manager)
- Secret rotation and versioning

Without k8s, applications need their own secrets strategy.

## Requirements

```
┌─────────────────────────────────────────────────────────────────┐
│                    Secrets Requirements                          │
├─────────────────────────────────────────────────────────────────┤
│                                                                  │
│  Security:                                                       │
│  ├── Encrypted at rest                                          │
│  ├── Encrypted in transit                                       │
│  ├── Access control (per-app)                                   │
│  └── Audit logging                                              │
│                                                                  │
│  Operations:                                                     │
│  ├── Rotation without restart                                   │
│  ├── Versioning                                                 │
│  ├── GitOps compatible (encrypted in git)                       │
│  └── Offline capable (edge devices)                             │
│                                                                  │
│  Developer Experience:                                           │
│  ├── Simple API                                                 │
│  ├── Environment variable injection                             │
│  └── Works with existing Phoenix/Ecto config                    │
│                                                                  │
└─────────────────────────────────────────────────────────────────┘
```

---

## Option A: SOPS-Style (Encrypted Files in Git)

Encrypt secrets with age/GPG, store encrypted files in GitOps repo.

```yaml
# gitops/secrets/console.enc.yaml (encrypted)
apiVersion: macula/v1
kind: Secret
metadata:
  name: console
data:
  database_url: ENC[AES256_GCM,data:xK9f2...]
  secret_key_base: ENC[AES256_GCM,data:mN3p...]
  stripe_key: ENC[AES256_GCM,data:qR7y...]
sops:
  age:
    - recipient: age1...
  encrypted_regex: ^data$
  version: 3.7.0
```

**Implementation:**

```elixir
defmodule MaculaSecrets.SOPS do
  @moduledoc """
  SOPS-compatible secrets decryption.

  Secrets are encrypted with age and stored in GitOps repo.
  Private key is on the node (not in git).
  """

  @secrets_dir "/var/lib/maculaos/gitops/secrets"
  @key_file "/var/lib/maculaos/secrets/age.key"

  def get(app_name, key) do
    encrypted_file = Path.join(@secrets_dir, "#{app_name}.enc.yaml")

    case decrypt_file(encrypted_file) do
      {:ok, data} ->
        Map.get(data["data"], to_string(key))
      {:error, reason} ->
        nil
    end
  end

  def decrypt_file(path) do
    # Use sops CLI or native age decryption
    case System.cmd("sops", ["-d", path], stderr_to_stdout: true) do
      {output, 0} ->
        {:ok, YamlElixir.read_from_string!(output)}
      {error, _} ->
        {:error, error}
    end
  end

  def encrypt_file(path, data) do
    yaml = Ymlr.document!(data)
    temp_file = Path.join("/tmp", "secret_#{:erlang.unique_integer()}.yaml")
    File.write!(temp_file, yaml)

    case System.cmd("sops", ["-e", "-i", temp_file]) do
      {_, 0} ->
        File.rename!(temp_file, path)
        :ok
      {error, _} ->
        File.rm(temp_file)
        {:error, error}
    end
  end

  # Native age decryption (no sops CLI needed)
  def decrypt_with_age(ciphertext) do
    key = File.read!(@key_file) |> String.trim()

    case :age_decrypt.decrypt(ciphertext, key) do
      {:ok, plaintext} -> {:ok, plaintext}
      error -> error
    end
  end
end
```

**GitOps Integration:**

```elixir
# In MaculaGitops.Reconciler
defp resolve_secrets(app_spec) do
  env = for {key, value} <- app_spec.env do
    resolved = case value do
      {:secret, secret_path} ->
        [app, secret_key] = String.split(secret_path, "/")
        MaculaSecrets.SOPS.get(app, secret_key)
      {:env, var_name} ->
        System.get_env(var_name)
      other ->
        other
    end
    {key, resolved}
  end
  %{app_spec | env: Map.new(env)}
end
```

**Pros:**
- Secrets in git (auditable, versioned)
- Works offline (key on device)
- Standard tooling (sops, age)

**Cons:**
- Key distribution challenge
- No rotation without new commit
- Need sops/age tooling

---

## Option B: BEAM-Native Secret Store

Store secrets encrypted in BEAM-native storage (ETS/Khepri).

```elixir
defmodule MaculaSecrets.Store do
  @moduledoc """
  BEAM-native encrypted secret store.

  Secrets are encrypted with a master key and stored in ETS/Khepri.
  """

  use GenServer

  @table :macula_secrets
  @master_key_file "/var/lib/maculaos/secrets/master.key"

  # Client API

  def start_link(opts \\ []) do
    GenServer.start_link(__MODULE__, opts, name: __MODULE__)
  end

  def put(app, key, value) do
    GenServer.call(__MODULE__, {:put, app, key, value})
  end

  def get(app, key) do
    GenServer.call(__MODULE__, {:get, app, key})
  end

  def delete(app, key) do
    GenServer.call(__MODULE__, {:delete, app, key})
  end

  def list(app) do
    GenServer.call(__MODULE__, {:list, app})
  end

  def rotate_master_key(new_key) do
    GenServer.call(__MODULE__, {:rotate_master_key, new_key})
  end

  # GenServer Callbacks

  @impl true
  def init(_opts) do
    :ets.new(@table, [:named_table, :set, :protected])

    master_key = load_or_generate_master_key()

    # Load secrets from persistent storage
    load_persisted_secrets(master_key)

    {:ok, %{master_key: master_key}}
  end

  @impl true
  def handle_call({:put, app, key, value}, _from, state) do
    encrypted = encrypt(value, state.master_key)
    :ets.insert(@table, {{app, key}, encrypted})

    # Persist to disk
    persist_secrets(state.master_key)

    {:reply, :ok, state}
  end

  def handle_call({:get, app, key}, _from, state) do
    result = case :ets.lookup(@table, {app, key}) do
      [{{^app, ^key}, encrypted}] ->
        decrypt(encrypted, state.master_key)
      [] ->
        nil
    end
    {:reply, result, state}
  end

  def handle_call({:delete, app, key}, _from, state) do
    :ets.delete(@table, {app, key})
    persist_secrets(state.master_key)
    {:reply, :ok, state}
  end

  def handle_call({:list, app}, _from, state) do
    keys = :ets.match(@table, {{app, :"$1"}, :_})
           |> List.flatten()
    {:reply, keys, state}
  end

  def handle_call({:rotate_master_key, new_key}, _from, state) do
    # Re-encrypt all secrets with new key
    secrets = :ets.tab2list(@table)
    |> Enum.map(fn {{app, key}, encrypted} ->
      plaintext = decrypt(encrypted, state.master_key)
      new_encrypted = encrypt(plaintext, new_key)
      {{app, key}, new_encrypted}
    end)

    :ets.delete_all_objects(@table)
    :ets.insert(@table, secrets)

    # Save new master key
    save_master_key(new_key)
    persist_secrets(new_key)

    {:reply, :ok, %{state | master_key: new_key}}
  end

  # Encryption

  defp encrypt(plaintext, key) do
    iv = :crypto.strong_rand_bytes(16)
    {ciphertext, tag} = :crypto.crypto_one_time_aead(
      :aes_256_gcm,
      key,
      iv,
      plaintext,
      "",
      true
    )
    iv <> tag <> ciphertext
  end

  defp decrypt(<<iv::binary-16, tag::binary-16, ciphertext::binary>>, key) do
    case :crypto.crypto_one_time_aead(
      :aes_256_gcm,
      key,
      iv,
      ciphertext,
      "",
      tag,
      false
    ) do
      plaintext when is_binary(plaintext) -> plaintext
      :error -> nil
    end
  end

  # Persistence

  defp load_or_generate_master_key do
    case File.read(@master_key_file) do
      {:ok, key} when byte_size(key) == 32 ->
        key
      _ ->
        key = :crypto.strong_rand_bytes(32)
        save_master_key(key)
        key
    end
  end

  defp save_master_key(key) do
    File.mkdir_p!(Path.dirname(@master_key_file))
    File.write!(@master_key_file, key)
    File.chmod!(@master_key_file, 0o600)
  end

  defp persist_secrets(master_key) do
    secrets_file = "/var/lib/maculaos/secrets/secrets.enc"

    data = :ets.tab2list(@table)
           |> :erlang.term_to_binary()

    encrypted = encrypt(data, master_key)
    File.write!(secrets_file, encrypted)
  end

  defp load_persisted_secrets(master_key) do
    secrets_file = "/var/lib/maculaos/secrets/secrets.enc"

    case File.read(secrets_file) do
      {:ok, encrypted} ->
        case decrypt(encrypted, master_key) do
          nil -> :ok
          data ->
            secrets = :erlang.binary_to_term(data)
            :ets.insert(@table, secrets)
        end
      {:error, :enoent} ->
        :ok
    end
  end
end
```

**Pros:**
- Pure BEAM, no external tools
- Runtime rotation
- Fast (ETS lookup)

**Cons:**
- Not in git (separate sync needed)
- Master key distribution
- Custom tooling

---

## Option C: HashiCorp Vault Integration

Connect to a Vault server for enterprise-grade secrets.

```elixir
defmodule MaculaSecrets.Vault do
  @moduledoc """
  HashiCorp Vault integration for secrets management.
  """

  @vault_addr System.get_env("VAULT_ADDR", "http://localhost:8200")

  def get(app, key) do
    path = "secret/data/macula/#{app}/#{key}"

    case http_get(path) do
      {:ok, %{"data" => %{"data" => data}}} ->
        Map.get(data, "value")
      _ ->
        nil
    end
  end

  def put(app, key, value) do
    path = "secret/data/macula/#{app}/#{key}"
    http_post(path, %{data: %{value: value}})
  end

  def list(app) do
    path = "secret/metadata/macula/#{app}"

    case http_list(path) do
      {:ok, %{"data" => %{"keys" => keys}}} -> keys
      _ -> []
    end
  end

  # HTTP Client (using Req or Finch)

  defp http_get(path) do
    url = "#{@vault_addr}/v1/#{path}"

    case Req.get(url, headers: auth_headers()) do
      {:ok, %{status: 200, body: body}} -> {:ok, body}
      {:ok, %{status: status}} -> {:error, {:http_error, status}}
      error -> error
    end
  end

  defp http_post(path, data) do
    url = "#{@vault_addr}/v1/#{path}"

    case Req.post(url, json: data, headers: auth_headers()) do
      {:ok, %{status: status}} when status in [200, 204] -> :ok
      {:ok, %{status: status}} -> {:error, {:http_error, status}}
      error -> error
    end
  end

  defp http_list(path) do
    url = "#{@vault_addr}/v1/#{path}"

    case Req.request(:list, url, headers: auth_headers()) do
      {:ok, %{status: 200, body: body}} -> {:ok, body}
      _ -> {:error, :not_found}
    end
  end

  defp auth_headers do
    token = get_vault_token()
    [{"X-Vault-Token", token}]
  end

  defp get_vault_token do
    # Read from file or use AppRole auth
    case File.read("/var/lib/maculaos/secrets/vault-token") do
      {:ok, token} -> String.trim(token)
      _ -> System.get_env("VAULT_TOKEN", "")
    end
  end
end
```

**Pros:**
- Enterprise-grade security
- Audit logging
- Dynamic secrets
- Rotation policies

**Cons:**
- Requires Vault server
- Network dependency
- Complexity

---

## Option D: Age-Encrypted Environment Files

Simple approach: age-encrypt .env files.

```bash
# Create encrypted env file
age -r age1... -o app.env.age app.env

# Decrypt at runtime
age -d -i /var/lib/maculaos/secrets/age.key app.env.age > /tmp/app.env
source /tmp/app.env
rm /tmp/app.env
```

**Implementation:**

```elixir
defmodule MaculaSecrets.EnvFile do
  @moduledoc """
  Age-encrypted environment file loader.
  """

  @key_file "/var/lib/maculaos/secrets/age.key"

  def load(app_name) do
    encrypted_file = "/var/lib/maculaos/gitops/secrets/#{app_name}.env.age"

    case decrypt_env_file(encrypted_file) do
      {:ok, env_content} ->
        parse_env(env_content)
        |> Enum.each(fn {key, value} ->
          System.put_env(key, value)
        end)
        :ok
      error ->
        error
    end
  end

  def get(app_name, key) do
    encrypted_file = "/var/lib/maculaos/gitops/secrets/#{app_name}.env.age"

    case decrypt_env_file(encrypted_file) do
      {:ok, env_content} ->
        parse_env(env_content)
        |> Map.get(key)
      _ ->
        nil
    end
  end

  defp decrypt_env_file(path) do
    case System.cmd("age", ["-d", "-i", @key_file, path], stderr_to_stdout: true) do
      {output, 0} -> {:ok, output}
      {error, _} -> {:error, error}
    end
  end

  defp parse_env(content) do
    content
    |> String.split("\n", trim: true)
    |> Enum.reject(&String.starts_with?(&1, "#"))
    |> Enum.map(fn line ->
      case String.split(line, "=", parts: 2) do
        [key, value] -> {String.trim(key), String.trim(value, "\"")}
        _ -> nil
      end
    end)
    |> Enum.reject(&is_nil/1)
    |> Map.new()
  end
end
```

**Pros:**
- Simple, familiar format
- Works with shell scripts
- Minimal tooling (just age)

**Cons:**
- All-or-nothing (load entire file)
- No individual secret access control
- No versioning per secret

---

## Recommendation: Hybrid Approach

```
┌─────────────────────────────────────────────────────────────────┐
│                    Recommended Architecture                      │
├─────────────────────────────────────────────────────────────────┤
│                                                                  │
│  Layer 1: SOPS-encrypted files in GitOps (source of truth)      │
│  └── Secrets version controlled                                 │
│  └── Encrypted with age keys                                    │
│  └── Per-app secret files                                       │
│                                                                  │
│  Layer 2: BEAM-native runtime store (fast access)               │
│  └── Secrets loaded from SOPS on startup                        │
│  └── ETS for fast lookups                                       │
│  └── Supports runtime updates                                   │
│                                                                  │
│  Layer 3: (Optional) Vault for enterprise                       │
│  └── Dynamic secrets                                            │
│  └── Database credentials                                       │
│  └── PKI certificates                                           │
│                                                                  │
└─────────────────────────────────────────────────────────────────┘
```

**Unified API:**

```elixir
defmodule MaculaSecrets do
  @moduledoc """
  Unified secrets API with pluggable backends.
  """

  @backend Application.compile_env(:macula, :secrets_backend, :sops)

  def get(app, key) do
    # Try runtime store first (fastest)
    case MaculaSecrets.Store.get(app, key) do
      nil ->
        # Fall back to configured backend
        value = case @backend do
          :sops -> MaculaSecrets.SOPS.get(app, key)
          :vault -> MaculaSecrets.Vault.get(app, key)
          :env_file -> MaculaSecrets.EnvFile.get(app, key)
        end

        # Cache in runtime store
        if value do
          MaculaSecrets.Store.put(app, key, value)
        end

        value

      value ->
        value
    end
  end

  def put(app, key, value) do
    case @backend do
      :sops ->
        MaculaSecrets.SOPS.put(app, key, value)
        MaculaSecrets.Store.put(app, key, value)
      :vault ->
        MaculaSecrets.Vault.put(app, key, value)
        MaculaSecrets.Store.put(app, key, value)
      :env_file ->
        {:error, :read_only}
    end
  end

  def load_all(app) do
    keys = case @backend do
      :sops -> MaculaSecrets.SOPS.list(app)
      :vault -> MaculaSecrets.Vault.list(app)
      :env_file -> []
    end

    for key <- keys do
      value = get(app, key)
      {key, value}
    end
    |> Map.new()
  end
end
```

---

## Phoenix/Ecto Integration

```elixir
# config/runtime.exs
import Config

# Load secrets for this app
secrets = MaculaSecrets.load_all(:my_app)

config :my_app, MyApp.Repo,
  url: secrets["database_url"],
  pool_size: 10

config :my_app, MyAppWeb.Endpoint,
  secret_key_base: secrets["secret_key_base"]

config :stripity_stripe,
  api_key: secrets["stripe_secret_key"]
```

**Or with a helper:**

```elixir
defmodule MaculaSecrets.Config do
  @moduledoc """
  Helpers for Phoenix config.
  """

  defmacro secret(app, key) do
    quote do
      MaculaSecrets.get(unquote(app), unquote(key))
    end
  end
end

# In config/runtime.exs
import MaculaSecrets.Config

config :my_app, MyApp.Repo,
  url: secret(:my_app, "database_url")
```

---

## Key Distribution

The master/age key must be distributed to nodes securely:

```
┌─────────────────────────────────────────────────────────────────┐
│                    Key Distribution Options                      │
├─────────────────────────────────────────────────────────────────┤
│                                                                  │
│  Option A: Manual (simplest for edge)                           │
│  └── Copy key file during initial setup                         │
│  └── TUI wizard prompts for key                                 │
│  └── Stored in /var/lib/maculaos/secrets/                       │
│                                                                  │
│  Option B: TPM/HSM (hardware security)                          │
│  └── Key stored in TPM chip                                     │
│  └── Never leaves hardware                                      │
│  └── Requires TPM-enabled device                                │
│                                                                  │
│  Option C: Mesh Key Exchange (distributed)                      │
│  └── Bootstrap node holds master key                            │
│  └── New nodes request key share via mesh                       │
│  └── Requires N-of-M shares to reconstruct                      │
│                                                                  │
│  Option D: Derivation from Identity (DID)                       │
│  └── Derive key from node's Ed25519 keypair                     │
│  └── Secrets encrypted per-node                                 │
│  └── Requires re-encryption for new nodes                       │
│                                                                  │
└─────────────────────────────────────────────────────────────────┘
```

**TPM Integration (if available):**

```elixir
defmodule MaculaSecrets.TPM do
  @moduledoc """
  TPM-based key storage (optional).
  """

  def seal_key(key) do
    # Seal key to TPM
    case System.cmd("tpm2_create", [
      "-C", "0x81000001",
      "-i", "-",
      "-o", "/var/lib/maculaos/secrets/sealed.key"
    ], input: key) do
      {_, 0} -> :ok
      {error, _} -> {:error, error}
    end
  end

  def unseal_key do
    case System.cmd("tpm2_unseal", [
      "-c", "0x81000001",
      "-i", "/var/lib/maculaos/secrets/sealed.key"
    ]) do
      {key, 0} -> {:ok, String.trim(key)}
      {error, _} -> {:error, error}
    end
  end
end
```

---

## Summary

```
┌─────────────────────────────────────────────────────────────────┐
│                  Secrets Management Summary                      │
├─────────────────────────────────────────────────────────────────┤
│                                                                  │
│  Recommended: SOPS + BEAM Store hybrid                          │
│                                                                  │
│  Source of Truth: SOPS-encrypted files in GitOps                │
│  ├── gitops/secrets/{app}.enc.yaml                              │
│  ├── Encrypted with age                                         │
│  └── Version controlled, auditable                              │
│                                                                  │
│  Runtime Access: BEAM-native ETS store                          │
│  ├── MaculaSecrets.Store GenServer                              │
│  ├── Loaded from SOPS on startup                                │
│  └── Fast ETS lookups                                           │
│                                                                  │
│  Unified API:                                                    │
│  ├── MaculaSecrets.get(app, key)                                │
│  ├── MaculaSecrets.put(app, key, value)                         │
│  └── MaculaSecrets.load_all(app)                                │
│                                                                  │
│  Key Distribution:                                               │
│  ├── Manual for edge devices                                    │
│  ├── TPM for hardware security (optional)                       │
│  └── Mesh key exchange for clusters                             │
│                                                                  │
│  Phoenix Integration:                                            │
│  └── Load secrets in config/runtime.exs                         │
│  └── secret(:app, "key") macro                                  │
│                                                                  │
└─────────────────────────────────────────────────────────────────┘
```

## Open Questions

1. **Secret Rotation:** How to rotate without app restart?
   - Option A: File watcher + reload
   - Option B: Periodic refresh from source
   - Option C: Push notification via mesh

2. **Audit Logging:** Where to log secret access?
   - Option A: Local file
   - Option B: Telemetry events
   - Option C: Mesh broadcast

3. **Multi-Tenant:** How to isolate secrets between tenants?
   - Option A: Prefix keys with tenant ID
   - Option B: Separate stores per tenant
   - Option C: Encryption with tenant keys

4. **Emergency Access:** How to recover if key is lost?
   - Option A: Key escrow (encrypted backup)
   - Option B: M-of-N Shamir's secret sharing
   - Option C: Accept data loss (re-bootstrap)

## Next Steps

1. Implement MaculaSecrets.Store (BEAM-native)
2. Implement MaculaSecrets.SOPS (age encryption)
3. Create TUI commands for secret management
4. Integrate with GitOps reconciler
5. Add Phoenix config helpers
