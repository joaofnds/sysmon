{
  description = "Local Mac system monitoring: mactop scraped by VictoriaMetrics";

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
    in
    {
      packages.aarch64-darwin = {
        inherit mactop;
        inherit (pkgs) victoriametrics;
      };

      devShells.aarch64-darwin.default = pkgs.mkShellNoCC {
        packages = [ mactop pkgs.victoriametrics ];
      };
    };
}
