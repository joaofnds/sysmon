{
  description = "Local Mac system monitoring: mactop, per-app usage and Claude plan limits scraped by VictoriaMetrics, and Claude Code telemetry in ClickHouse";

  inputs.nixpkgs.url = "github:NixOS/nixpkgs/nixpkgs-unstable";

  outputs = { nixpkgs, ... }:
    let
      pkgs = nixpkgs.legacyPackages.aarch64-darwin;

      mactop = pkgs.mactop.overrideAttrs (old: {
        postPatch = (old.postPatch or "") + ''
          substituteInPlace internal/app/metrics.go \
            --replace-fail 'http.ListenAndServe(":"+port' 'http.ListenAndServe("127.0.0.1:"+port'
        '';
      });

      sysmon-procs = pkgs.buildGoModule {
        pname = "sysmon-procs";
        version = "0";
        src = ./procs;
        vendorHash = null;
      };

      claude-limits = pkgs.buildGoModule {
        pname = "claude-limits";
        version = "0";
        src = ./claude-limits;
        vendorHash = null;
      };

      grafana = pkgs.fetchzip {
        name = "grafana-13.2.2";
        url = "https://dl.grafana.com/oss/release/grafana-13.2.2.darwin-arm64.tar.gz";
        hash = "sha256-DgGUmZOUaDOUmkjdgVLVSwjkLZP0S8SAAdghbZsHHKg=";
      };

      grafana-plugins = pkgs.fetchzip {
        name = "grafana-clickhouse-datasource-4.21.3";
        url = "https://grafana.com/api/plugins/grafana-clickhouse-datasource/versions/4.21.3/download?os=darwin&arch=arm64";
        extension = "zip";
        stripRoot = false;
        hash = "sha256-KIIWaR5gfv6M4VlaMtLeWd8YJMUBys1SuMZ+KUHhrYs=";
      };
    in
    {
      packages.aarch64-darwin = {
        inherit mactop sysmon-procs claude-limits grafana grafana-plugins;
        inherit (pkgs) victoriametrics clickhouse;
        otelcol-contrib = pkgs.opentelemetry-collector-contrib;
      };

      devShells.aarch64-darwin.default = pkgs.mkShellNoCC {
        packages = [ mactop sysmon-procs claude-limits pkgs.victoriametrics pkgs.go ];
      };
    };
}
