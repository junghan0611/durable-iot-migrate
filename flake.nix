{
  description = "durable-iot-migrate — Durable IoT platform migration with Clojure semantic layer";

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
            # Clojure
            clojure
            jdk17_headless

            # Temporal
            temporal-cli

            # Database
            postgresql_16  # psql client for Doltgres

            # Go (archive — reference implementation)
            go

            # Tools
            jq
            curl
            git
          ];

          shellHook = ''
            echo "🔧 durable-iot-migrate dev environment"
            echo "   Clojure:      $(clj --version 2>&1)"
            echo "   Java:         $(java -version 2>&1 | head -1)"
            echo "   Temporal CLI: $(temporal --version 2>/dev/null | head -1)"
            echo ""
            echo "Quick start:"
            echo "  clj -M:test                  # Run Clojure tests"
            echo "  clj -M:repl                  # Start nREPL"
            echo "  temporal server start-dev    # Start dev server"
            echo ""
            echo "Archive (Go reference):"
            echo "  cd archive/go && go test ./..."
            echo ""
          '';
        };
      }
    );
}
