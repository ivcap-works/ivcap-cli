{
  description = "IVCAP Command Line Interface (CLI)";

  inputs = {
    nixpkgs.url = "github:nixos/nixpkgs?ref=nixos-unstable";
    flake-utils.url = "github:numtide/flake-utils";
  };

  outputs = { self, nixpkgs, flake-utils }:
    let
      supportedSystems = [ "x86_64-linux" "x86_64-darwin" "i686-linux" "aarch64-linux" "aarch64-darwin" ];
      forAllSystems = f: nixpkgs.lib.genAttrs supportedSystems (system: f system);
    in {
        overlay = final: prev: {
          ivcap-cli = with final; stdenv.mkDerivation {
            name = "ivcap-cli";
            src = self;
            buildPhase = "make";
            installPhase = "make install";
            buildInputs = with pkgs; [
              go
              addlicense
              golangci-lint
              go-critic
              go-tools
              gosec
              govulncheck
            ];
          };
        };

        defaultPackage = forAllSystems(system: (import nixpkgs {
          inherit system;
          overlays = [ self.overlay ];
        }).ivcap-cli);

        devShells = forAllSystems(system:
          let pkgs = nixpkgs.legacyPackages.${system};
          in {
            default = pkgs.mkShell {
              buildInputs = with pkgs; [
                go
                addlicense
                golangci-lint
                go-critic
                go-tools
                gosec
                govulncheck
              ];
            };
          });
        };
}
