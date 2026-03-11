{
  description = "durable-iot-migrate — Durable IoT platform migration on Temporal";

  inputs = {
    nixpkgs.url = "github:NixOS/nixpkgs/nixos-25.11";
    flake-utils.url = "github:numtide/flake-utils";
  };

  outputs = { self, nixpkgs, flake-utils }:
    flake-utils.lib.eachDefaultSystem (system:
      let
        pkgs = import nixpkgs { inherit system; };
      in
      {
        devShells.default = pkgs.mkShell {
          name = "durable-iot-migrate";

          buildInputs = with pkgs; [
            # Go toolchain
            go
            gopls
            golangci-lint
            delve

            # Temporal
            temporal-cli  # includes dev server (temporal server start-dev)

            # Database
            postgresql_16  # psql client for Doltgres

            # Tools
            jq
            curl
            git
          ];

          shellHook = ''
            echo "🔧 durable-iot-migrate dev environment"
            echo "   Go:           $(go version | cut -d' ' -f3)"
            echo "   Temporal CLI: $(temporal --version 2>/dev/null | head -1)"
            echo ""
            echo "Quick start:"
            echo "  temporal server start-dev    # Start dev server (gRPC:7233, UI:8233)"
            echo "  go test ./...                # Run tests"
            echo "  go run ./cmd/worker          # Start migration worker"
            echo ""
            export GOPATH="$HOME/go"
            export PATH="$GOPATH/bin:$PATH"
          '';
        };
      }
    );
}
