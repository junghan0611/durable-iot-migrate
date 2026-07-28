{
  description = "durable-iot-migrate — place + recipe core for small smart-home hubs";

  inputs = {
    nixpkgs.url = "github:NixOS/nixpkgs/nixos-25.11";
    flake-utils.url = "github:numtide/flake-utils";
  };

  outputs = { self, nixpkgs, flake-utils }:
    flake-utils.lib.eachDefaultSystem (system:
      let
        pkgs = import nixpkgs { inherit system; };
        graalvm = pkgs.graalvmPackages.graalvm-ce;
      in
      {
        devShells = {
          # 기본: GraalVM (native-image 포함)
          default = pkgs.mkShell {
            name = "durable-iot-migrate";

            buildInputs = with pkgs; [
              clojure
              graalvm         # JDK + native-image

              # Go (archive reference)
              go

              # Tools
              jq
              curl
              git
            ];

            JAVA_HOME = graalvm;
            GRAALVM_HOME = graalvm;

            shellHook = ''
              echo "🔧 durable-iot-migrate dev shell (GraalVM)"
              echo "  ./run.sh test           — Clojure tests"
              echo "  ./run.sh native-build   — GraalVM native binary"
              echo ""
            '';
          };

          # JVM만 (가벼운 개발용)
          jvm = pkgs.mkShell {
            name = "durable-iot-migrate-jvm";

            buildInputs = with pkgs; [
              clojure
              jdk17_headless
              go
              jq
              git
            ];

            shellHook = ''
              echo "🔧 durable-iot-migrate dev shell (JVM only)"
            '';
          };
        };
      });
}
