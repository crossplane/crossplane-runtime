# New to Nix? Start here:
#   Language basics:  https://nix.dev/tutorials/nix-language
#   Flakes intro:     https://zero-to-nix.com/concepts/flakes
{
  description = "Crossplane Runtime - Go library for building Crossplane providers and controllers";

  inputs = {
    nixpkgs.url = "github:NixOS/nixpkgs/nixos-25.11";

    # TODO(negz): Unpin once https://github.com/nix-community/gomod2nix/pull/231 is released.
    gomod2nix = {
      url = "github:nix-community/gomod2nix/49662a44272806ff785df2990a420edaaca15db4";
      inputs.nixpkgs.follows = "nixpkgs";
    };
  };

  outputs =
    {
      self,
      nixpkgs,
      gomod2nix,
    }:
    let
      # Systems where Nix runs (dev machines, CI).
      supportedSystems = [
        "x86_64-linux"
        "aarch64-linux"
        "x86_64-darwin"
        "aarch64-darwin"
      ];

      # Helpers for per-system outputs.
      forAllSystems = f: nixpkgs.lib.genAttrs supportedSystems (system: forSystem system f);
      forSystem =
        system: f:
        f {
          inherit system;
          pkgs = import nixpkgs {
            inherit system;
            overlays = [ gomod2nix.overlays.default ];
          };
        };

    in
    {
      # CI checks (nix flake check).
      checks = forAllSystems (
        { pkgs, ... }:
        let
          checks = import ./nix/checks.nix { inherit pkgs self; };
        in
        {
          test = checks.test { };
          generate = checks.generate { };
          go-lint = checks.goLint { };
          nix-lint = checks.nixLint { };
        }
      );

      # Development commands (nix run .#<app>).
      apps = forAllSystems (
        { pkgs, ... }:
        let
          apps = import ./nix/apps.nix { inherit pkgs; };
        in
        {
          test = apps.test { };
          lint = apps.lint { fix = true; };
          generate = apps.generate { };
          tidy = apps.tidy { };
        }
      );

      # Development shell (nix develop).
      devShells = forAllSystems (
        { pkgs, ... }:
        {
          default = pkgs.mkShell {
            buildInputs = [
              pkgs.coreutils
              pkgs.gnused
              pkgs.ncurses
              pkgs.go
              pkgs.golangci-lint
              pkgs.gomod2nix

              # Code generation
              pkgs.buf
              pkgs.protoc-gen-go
              pkgs.protoc-gen-go-grpc
              pkgs.kubernetes-controller-tools

              # Nix
              pkgs.nixfmt-rfc-style
            ];

            shellHook = ''
              export PS1='\[\033[38;2;243;128;123m\][cros\[\033[38;2;255;205;60m\]spla\[\033[38;2;53;208;186m\]ne-rt]\[\033[0m\] \w \$ '

              echo "Crossplane Runtime development shell ($(go version | cut -d' ' -f3))"
              echo ""
              echo "  nix run .#test          nix run .#generate"
              echo "  nix run .#lint          nix run .#tidy"
              echo ""
              echo "  nix flake check"
              echo ""
            '';
          };
        }
      );
    };
}
