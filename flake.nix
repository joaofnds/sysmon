{
  description = "Local Mac system monitoring: mactop, per-app usage and Claude plan limits scraped by VictoriaMetrics";

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
    in
    {
      packages.aarch64-darwin = {
        inherit mactop sysmon-procs claude-limits;
        inherit (pkgs) victoriametrics;
      };

      devShells.aarch64-darwin.default = pkgs.mkShellNoCC {
        packages = [ mactop sysmon-procs claude-limits pkgs.victoriametrics pkgs.go ];
      };
    };
}
