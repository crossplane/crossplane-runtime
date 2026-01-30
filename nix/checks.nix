# CI check builders for Crossplane Runtime.
#
# Checks run inside the Nix sandbox without network or filesystem access. This
# makes them fully reproducible but means Go modules must come from gomod2nix.
#
# Most checks use buildGoApplication, which sets up the Go environment with
# modules from gomod2nix.toml. This is different from apps, which run outside
# the sandbox and can access Go modules normally.
#
# All checks are builder functions that take an attrset of arguments and return
# a derivation. The actual check definitions live in flake.nix.
{ pkgs, self }:
{
  # Run Go unit tests with coverage.
  test =
    _:
    pkgs.buildGoApplication {
      pname = "crossplane-runtime-test";
      version = "0.0.0";
      src = self;
      pwd = self;
      modules = ../gomod2nix.toml;

      CGO_ENABLED = "0";

      dontBuild = true;

      checkPhase = ''
        runHook preCheck
        export HOME=$TMPDIR
        go test -covermode=count -coverprofile=coverage.txt ./apis/... ./pkg/...
        runHook postCheck
      '';

      installPhase = ''
        mkdir -p $out
        cp coverage.txt $out/
      '';
    };

  # Run golangci-lint (without --fix, since source is read-only).
  goLint =
    _:
    pkgs.buildGoApplication {
      pname = "crossplane-runtime-go-lint";
      version = "0.0.0";
      src = self;
      pwd = self;
      modules = ../gomod2nix.toml;

      CGO_ENABLED = "0";

      nativeBuildInputs = [ pkgs.golangci-lint ];

      dontBuild = true;

      checkPhase = ''
        runHook preCheck
        export HOME=$TMPDIR
        export GOLANGCI_LINT_CACHE=$TMPDIR/.cache/golangci-lint
        golangci-lint run
        runHook postCheck
      '';

      installPhase = ''
        mkdir -p $out
        touch $out/.lint-passed
      '';
    };

  # Verify generated code matches committed code.
  generate =
    _:
    pkgs.buildGoApplication {
      pname = "crossplane-runtime-generate-check";
      version = "0.0.0";
      src = self;
      pwd = self;
      modules = ../gomod2nix.toml;

      CGO_ENABLED = "0";

      nativeBuildInputs = [
        pkgs.buf
        pkgs.protoc-gen-go
        pkgs.protoc-gen-go-grpc
        pkgs.kubernetes-controller-tools
      ];

      dontBuild = true;

      checkPhase = ''
        runHook preCheck
        export HOME=$TMPDIR

        echo "Running go generate..."
        go generate -tags generate ./apis/...

        echo "Comparing against committed source..."
        if ! diff -rq apis ${self}/apis > /dev/null 2>&1; then
          echo "ERROR: Generated code is out of date. Run 'nix run .#generate' and commit."
          diff -r apis ${self}/apis || true
          exit 1
        fi

        runHook postCheck
      '';

      installPhase = ''
        mkdir -p $out
        touch $out/.generate-passed
      '';
    };

  # Run Nix linters (statix, deadnix, nixfmt).
  nixLint =
    _:
    pkgs.runCommand "crossplane-runtime-nix-lint"
      {
        nativeBuildInputs = [
          pkgs.statix
          pkgs.deadnix
          pkgs.nixfmt-rfc-style
        ];
      }
      ''
        statix check ${self}
        deadnix --fail ${self}/flake.nix ${self}/nix
        nixfmt --check ${self}/flake.nix ${self}/nix/*.nix
        mkdir -p $out
        touch $out/.nix-lint-passed
      '';
}
