# Interactive development commands for Crossplane Runtime.
#
# Apps run outside the Nix sandbox with full filesystem and network access.
# They're designed for local development where Go modules are already available.
#
# All apps are builder functions that take an attrset of arguments and return a
# complete app definition ({ type, meta.description, program }). Most use
# writeShellApplication to create the program. The text block is preprocessed:
#
#   ${somePkg}/bin/foo   -> /nix/store/.../bin/foo  (Nix store path)
#   ''${SOME_VAR}        -> ${SOME_VAR}             (shell variable, escaped)
#
# Each app declares its tool dependencies via runtimeInputs, with inheritPath
# set to false. This ensures apps only use explicitly declared tools.
{ pkgs }:
{
  # Run Go unit tests.
  test = _: {
    type = "app";
    meta.description = "Run unit tests";
    program = pkgs.lib.getExe (
      pkgs.writeShellApplication {
        name = "crossplane-runtime-test";
        runtimeInputs = [ pkgs.go ];
        inheritPath = false;
        text = ''
          export CGO_ENABLED=0
          go test ./apis/... ./pkg/... "$@"
        '';
      }
    );
  };

  # Run golangci-lint.
  lint =
    {
      fix ? false,
    }:
    {
      type = "app";
      meta.description = "Run golangci-lint" + (if fix then " with auto-fix" else "");
      program = pkgs.lib.getExe (
        pkgs.writeShellApplication {
          name = "crossplane-runtime-lint";
          runtimeInputs = [
            pkgs.go
            pkgs.golangci-lint
          ];
          inheritPath = false;
          text = ''
            export CGO_ENABLED=0
            export GOLANGCI_LINT_CACHE="''${XDG_CACHE_HOME:-$HOME/.cache}/golangci-lint"
            golangci-lint run ${if fix then "--fix" else ""} "$@"
          '';
        }
      );
    };

  # Run code generation.
  generate = _: {
    type = "app";
    meta.description = "Run code generation";
    program = pkgs.lib.getExe (
      pkgs.writeShellApplication {
        name = "crossplane-runtime-generate";
        runtimeInputs = [
          pkgs.coreutils
          pkgs.go
          pkgs.buf
          pkgs.protoc-gen-go
          pkgs.protoc-gen-go-grpc
          pkgs.kubernetes-controller-tools
        ];
        inheritPath = false;
        text = ''
          export CGO_ENABLED=0

          echo "Running go generate..."
          go generate -tags 'generate' ./...

          echo "Done"
        '';
      }
    );
  };

  # Run go mod tidy and regenerate gomod2nix.toml.
  tidy = _: {
    type = "app";
    meta.description = "Run go mod tidy and regenerate gomod2nix.toml";
    program = pkgs.lib.getExe (
      pkgs.writeShellApplication {
        name = "crossplane-runtime-tidy";
        runtimeInputs = [
          pkgs.go
          pkgs.gomod2nix
        ];
        inheritPath = false;
        text = ''
          export CGO_ENABLED=0
          echo "Running go mod tidy..."
          go mod tidy
          echo "Running go mod verify..."
          go mod verify
          echo "Regenerating gomod2nix.toml..."
          gomod2nix generate
          echo "Done"
        '';
      }
    );
  };
}
