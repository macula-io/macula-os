# EXPLORATION: NixOS as Base for MaculaOS

> **Status:** 📋 Under Evaluation
> **Created:** 2026-01-27
> **Author:** Architecture Review

---

## Summary

Evaluate NixOS as an alternative base for MaculaOS instead of the current Alpine/k3OS fork approach. NixOS offers stronger declarative guarantees and reproducibility that align well with GitOps principles.

---

## 1. Current Approach: Alpine/k3OS Fork

### Architecture

```
┌─────────────────────────────────────────────────────────────┐
│                     MaculaOS (Current)                      │
├─────────────────────────────────────────────────────────────┤
│  YAML Config (/var/lib/maculaos/config.yaml)               │
│       ↓                                                     │
│  Go Binary (maculaos) applies config                       │
│       ↓                                                     │
│  Squashfs Root + Overlay (tmpfs or persistent)             │
│       ↓                                                     │
│  Alpine Linux + k3s                                        │
└─────────────────────────────────────────────────────────────┘
```

### Characteristics

| Aspect | Value |
|--------|-------|
| Base Size | ~300MB ISO |
| Config Format | YAML |
| Immutability | Squashfs + overlay |
| Rollback | A/B partition scheme |
| Package Manager | apk (Alpine) |
| k3s Integration | Native (purpose-built) |

### Limitations

1. **Overlay drift** - Changes in overlay can diverge from declared state
2. **Limited rollback** - Only A/B (2 states), no generation history
3. **Partial reproducibility** - Same YAML doesn't guarantee identical system
4. **Manual package management** - apk add outside config isn't tracked
5. **Custom Go tooling** - Must maintain applicator code for each config option

---

## 2. Proposed Approach: NixOS

### Architecture

```
┌─────────────────────────────────────────────────────────────┐
│                     MaculaOS (NixOS)                        │
├─────────────────────────────────────────────────────────────┤
│  Nix Flake (flake.nix + configuration.nix)                 │
│       ↓                                                     │
│  nix build / nixos-rebuild                                 │
│       ↓                                                     │
│  Immutable /nix/store (content-addressed)                  │
│       ↓                                                     │
│  NixOS + k3s module                                        │
└─────────────────────────────────────────────────────────────┘
```

### Characteristics

| Aspect | Value |
|--------|-------|
| Base Size | ~500MB-1GB (can optimize) |
| Config Format | Nix language |
| Immutability | Complete (/nix/store) |
| Rollback | Unlimited generations |
| Package Manager | Nix (declarative) |
| k3s Integration | nixpkgs module |

---

## 3. Detailed Comparison

### 3.1 Declarative Configuration

**Alpine/k3OS:**
```yaml
# /var/lib/maculaos/config.yaml
maculaos:
  hostname: edge-node-01
  k3s:
    args:
      - server
      - --disable=traefik
  network:
    interfaces:
      eth0:
        dhcp: true
```

**NixOS:**
```nix
# configuration.nix
{ config, pkgs, ... }:
{
  networking.hostName = "edge-node-01";

  services.k3s = {
    enable = true;
    role = "server";
    extraFlags = [ "--disable=traefik" ];
  };

  networking.interfaces.eth0.useDHCP = true;
}
```

**Verdict:** NixOS config is more verbose but guarantees the ENTIRE system state, not just what the applicator handles.

---

### 3.2 Reproducibility

**Alpine/k3OS:**
- Same YAML + same ISO version ≈ similar system
- Overlay changes not tracked
- Package versions depend on repository state at build time
- `apk add vim` in overlay breaks reproducibility

**NixOS:**
- Same flake.lock = **byte-for-byte identical system**
- All packages pinned to exact versions
- No way to make undeclared changes persist
- `nix-shell -p vim` for temporary tools (doesn't affect system)

**Verdict:** NixOS wins decisively for reproducibility.

---

### 3.3 Rollback Capabilities

**Alpine/k3OS:**
```
┌─────────┐     ┌─────────┐
│ Root A  │ ←→  │ Root B  │
│ Active  │     │ Standby │
└─────────┘     └─────────┘
     ↑
  Only 2 states available
```

**NixOS:**
```
┌─────────┐  ┌─────────┐  ┌─────────┐  ┌─────────┐
│ Gen 1   │→ │ Gen 2   │→ │ Gen 3   │→ │ Gen 4   │
│         │  │         │  │         │  │ Current │
└─────────┘  └─────────┘  └─────────┘  └─────────┘
     ↑            ↑            ↑
  Can boot any previous generation
```

**Commands:**
```bash
# List generations
nixos-rebuild list-generations

# Rollback to previous
nixos-rebuild switch --rollback

# Boot specific generation
nixos-rebuild switch --generation 42
```

**Verdict:** NixOS provides superior rollback flexibility.

---

### 3.4 Image Size

**Alpine/k3OS (current):**
- Minimal ISO: ~300MB
- With k3s + tools: ~500MB

**NixOS (estimated):**
- Standard minimal: ~800MB-1GB
- Optimized for embedded: ~500-600MB possible

**Optimization strategies for NixOS:**
```nix
{
  # Disable documentation
  documentation.enable = false;
  documentation.man.enable = false;
  documentation.doc.enable = false;

  # Minimal kernel
  boot.kernelPackages = pkgs.linuxPackages_latest;

  # Remove unused firmware
  hardware.enableRedistributableFirmware = false;

  # Aggressive garbage collection
  nix.gc.automatic = true;
  nix.gc.options = "--delete-older-than 7d";
}
```

**Verdict:** Alpine is smaller, but NixOS can be optimized to acceptable size (~500MB).

---

### 3.5 GitOps Alignment

**Alpine/k3OS:**
```
Git Repo (YAML configs)
    ↓
FluxCD detects change
    ↓
Deploys ConfigMap to k3s
    ↓
maculaos binary reads ConfigMap
    ↓
Applies changes (best effort)
```

**NixOS:**
```
Git Repo (Nix flake)
    ↓
CI builds new system closure
    ↓
Push to binary cache or node
    ↓
nixos-rebuild switch (atomic)
    ↓
System IS the declared state
```

**Verdict:** NixOS IS GitOps at the OS level - the flake IS the system.

---

### 3.6 k3s Integration

**Alpine/k3OS:**
- Purpose-built for k3s
- k3s binary included
- Airgap images supported
- Seamless integration

**NixOS:**
```nix
{
  services.k3s = {
    enable = true;
    role = "server"; # or "agent"
    token = "/var/lib/k3s/token";
    extraFlags = [
      "--disable=traefik"
      "--disable=servicelb"
      "--write-kubeconfig-mode=644"
    ];
  };

  # Airgap: pre-load images
  virtualisation.containerd.settings = {
    plugins."io.containerd.grpc.v1.cri".registry.mirrors = {
      "docker.io".endpoint = [ "https://registry.local" ];
    };
  };
}
```

**Verdict:** Both work well. k3OS is more turnkey; NixOS is more flexible.

---

## 4. Prototype Configuration

### 4.1 Minimal MaculaOS NixOS Flake

```nix
# flake.nix
{
  description = "MaculaOS - NixOS-based edge computing platform";

  inputs = {
    nixpkgs.url = "github:NixOS/nixpkgs/nixos-24.05";
    nixos-generators.url = "github:nix-community/nixos-generators";
  };

  outputs = { self, nixpkgs, nixos-generators, ... }: {

    nixosConfigurations.maculaos = nixpkgs.lib.nixosSystem {
      system = "x86_64-linux";
      modules = [
        ./modules/base.nix
        ./modules/k3s.nix
        ./modules/macula.nix
        ./modules/networking.nix
      ];
    };

    # Generate ISO
    packages.x86_64-linux.iso = nixos-generators.nixosGenerate {
      system = "x86_64-linux";
      modules = [ self.nixosConfigurations.maculaos.config ];
      format = "iso";
    };

    # Generate SD card image (Raspberry Pi)
    packages.aarch64-linux.sdcard = nixos-generators.nixosGenerate {
      system = "aarch64-linux";
      modules = [ self.nixosConfigurations.maculaos.config ];
      format = "sd-aarch64";
    };
  };
}
```

### 4.2 Base Module

```nix
# modules/base.nix
{ config, pkgs, lib, ... }:

{
  # Minimal system
  documentation.enable = false;

  # Boot
  boot.loader.systemd-boot.enable = true;
  boot.loader.efi.canTouchEfiVariables = true;
  boot.kernelPackages = pkgs.linuxPackages_latest;

  # Firmware (minimal)
  hardware.enableRedistributableFirmware = lib.mkDefault false;

  # Time
  time.timeZone = "UTC";

  # Users
  users.users.macula = {
    isNormalUser = true;
    extraGroups = [ "wheel" "docker" ];
    initialPassword = "macula";
  };

  # SSH
  services.openssh = {
    enable = true;
    settings.PermitRootLogin = "no";
  };

  # Essential packages
  environment.systemPackages = with pkgs; [
    vim
    htop
    curl
    jq
    git
  ];

  # Nix settings
  nix = {
    settings.experimental-features = [ "nix-command" "flakes" ];
    gc = {
      automatic = true;
      dates = "weekly";
      options = "--delete-older-than 7d";
    };
  };

  system.stateVersion = "24.05";
}
```

### 4.3 k3s Module

```nix
# modules/k3s.nix
{ config, pkgs, ... }:

{
  # k3s server
  services.k3s = {
    enable = true;
    role = "server";
    extraFlags = toString [
      "--disable=traefik"
      "--disable=servicelb"
      "--write-kubeconfig-mode=644"
      "--data-dir=/var/lib/rancher/k3s"
    ];
  };

  # Required for k3s
  boot.kernelModules = [ "br_netfilter" "overlay" ];
  boot.kernel.sysctl = {
    "net.bridge.bridge-nf-call-iptables" = 1;
    "net.bridge.bridge-nf-call-ip6tables" = 1;
    "net.ipv4.ip_forward" = 1;
  };

  # Firewall rules for k3s
  networking.firewall.allowedTCPPorts = [
    6443  # k3s API
    10250 # kubelet
  ];

  # kubectl alias
  environment.shellAliases = {
    k = "kubectl";
  };

  environment.systemPackages = with pkgs; [
    kubectl
    kubernetes-helm
  ];
}
```

### 4.4 Macula Module

```nix
# modules/macula.nix
{ config, pkgs, lib, ... }:

with lib;

let
  cfg = config.services.macula;
in {
  options.services.macula = {
    enable = mkEnableOption "Macula mesh services";

    role = mkOption {
      type = types.enum [ "peer" "bootstrap" "gateway" ];
      default = "peer";
      description = "Mesh role for this node";
    };

    realm = mkOption {
      type = types.str;
      default = "io.macula";
      description = "Macula realm identifier";
    };

    bootstrapPeers = mkOption {
      type = types.listOf types.str;
      default = [ "https://boot.macula.io:443" ];
      description = "Bootstrap peers to connect to";
    };
  };

  config = mkIf cfg.enable {
    # mDNS discovery
    services.avahi = {
      enable = true;
      nssmdns4 = true;
      publish = {
        enable = true;
        addresses = true;
        domain = true;
        userServices = true;
      };
    };

    # Macula Console (via k3s)
    # Deployed as k8s manifest, not system service

    # First-boot wizard service
    systemd.services.macula-firstboot = {
      description = "MaculaOS First Boot Wizard";
      wantedBy = [ "multi-user.target" ];
      after = [ "network-online.target" ];
      wants = [ "network-online.target" ];

      serviceConfig = {
        Type = "simple";
        ExecStart = "${pkgs.macula-firstboot}/bin/macula-firstboot";
        Restart = "on-failure";
      };

      # Only run if not yet configured
      unitConfig = {
        ConditionPathExists = "!/var/lib/maculaos/configured";
      };
    };

    # Generate macula config
    environment.etc."macula/config.yaml".text = ''
      maculaos:
        mesh:
          role: ${cfg.role}
          realm: ${cfg.realm}
          bootstrap_peers:
            ${concatMapStringsSep "\n            " (p: "- \"${p}\"") cfg.bootstrapPeers}
    '';
  };
}
```

### 4.5 Networking Module

```nix
# modules/networking.nix
{ config, pkgs, ... }:

{
  networking = {
    hostName = "maculaos";

    # Use networkd for predictable interface names
    useNetworkd = true;
    useDHCP = false;

    # Default: DHCP on all interfaces
    interfaces.eth0.useDHCP = true;

    # Firewall
    firewall = {
      enable = true;
      allowedTCPPorts = [
        22    # SSH
        80    # HTTP
        443   # HTTPS
        5353  # mDNS
      ];
      allowedUDPPorts = [
        5353  # mDNS
      ];
    };

    # WireGuard (optional, for mesh VPN)
    # wireguard.interfaces.wg-mesh = { ... };
  };

  # DNS
  services.resolved = {
    enable = true;
    dnssec = "false"; # Edge devices may have clock issues
    llmnr = "true";
  };
}
```

---

## 5. Build and Test

### 5.1 Build ISO

```bash
# Clone the flake
cd /home/rl/work/github.com/macula-io/macula-os-nix

# Build ISO
nix build .#iso

# Result: ./result/iso/maculaos-*.iso
ls -lh result/iso/
```

### 5.2 Test in QEMU

```bash
# Run in QEMU
nix run nixpkgs#qemu -- \
  -m 4G \
  -smp 2 \
  -enable-kvm \
  -cdrom result/iso/maculaos-*.iso
```

### 5.3 Deploy to Real Hardware

```bash
# Write to USB
sudo dd if=result/iso/maculaos-*.iso of=/dev/sdX bs=4M status=progress

# Or use nixos-anywhere for remote install
nix run github:numtide/nixos-anywhere -- \
  --flake .#maculaos \
  root@192.168.1.100
```

---

## 6. Migration Path

If we decide to adopt NixOS:

### Phase 1: Parallel Development
- Create `macula-os-nix/` repository
- Develop NixOS configuration alongside current k3OS fork
- Validate feature parity

### Phase 2: Testing
- Deploy NixOS variant to test hardware
- Compare performance, size, boot time
- Test all MaculaOS features (pairing, mesh, updates)

### Phase 3: Gradual Rollout
- Offer NixOS as "experimental" option
- Gather feedback from early adopters
- Fix issues, optimize size

### Phase 4: Primary Platform
- If successful, make NixOS the default
- Maintain k3OS variant for size-constrained devices
- Document migration path for existing deployments

---

## 7. Decision Criteria

| Criterion | Weight | k3OS | NixOS | Notes |
|-----------|--------|------|-------|-------|
| Reproducibility | High | 3 | 5 | NixOS guarantees identical systems |
| GitOps alignment | High | 3 | 5 | Flake IS the system declaration |
| Image size | Medium | 5 | 3 | k3OS smaller, but NixOS acceptable |
| Rollback | Medium | 3 | 5 | Unlimited generations vs A/B |
| Learning curve | Medium | 5 | 2 | Nix language is unfamiliar |
| k3s integration | Medium | 5 | 4 | Both work, k3OS more native |
| Community/support | Low | 3 | 5 | NixOS has larger, active community |
| Cross-compilation | Low | 2 | 5 | NixOS handles arm64 seamlessly |

**Scoring:** 1=Poor, 5=Excellent

**Weighted Total:**
- k3OS: ~3.5
- NixOS: ~4.2

---

## 8. Recommendation

**Explore NixOS as the future direction for MaculaOS.**

Rationale:
1. **GitOps philosophy** - NixOS embodies "infrastructure as code" at the OS level
2. **Fleet management** - Identical systems across all nodes guaranteed
3. **Rollback safety** - Unlimited generations beats A/B
4. **Long-term maintainability** - No custom Go applicator code needed
5. **Community** - Active NixOS community vs archived k3OS

**Concerns to address:**
1. Image size optimization (target: <600MB)
2. Team Nix language training
3. Build time optimization (binary cache)
4. First-boot wizard in Nix

**Next steps:**
1. Create `macula-os-nix/` prototype repository
2. Build minimal ISO, measure actual size
3. Test k3s + Macula Console deployment
4. Evaluate developer experience
5. Make go/no-go decision based on prototype

---

## 9. References

- [NixOS Manual](https://nixos.org/manual/nixos/stable/)
- [NixOS on ARM](https://nixos.wiki/wiki/NixOS_on_ARM)
- [nixos-generators](https://github.com/nix-community/nixos-generators)
- [nixos-anywhere](https://github.com/numtide/nixos-anywhere)
- [k3s NixOS module](https://search.nixos.org/options?query=services.k3s)
- [Minimal NixOS images](https://nixos.wiki/wiki/Creating_a_NixOS_live_CD)
- [Nix Flakes](https://nixos.wiki/wiki/Flakes)
