#!/usr/bin/env bash
# durable-iot-migrate — 커맨드 모음
set -euo pipefail
cd "$(dirname "$0")"

CMD="${1:-help}"
shift 2>/dev/null || true

case "$CMD" in
  ## --- Development ---
  test)
    echo "=== Clojure 테스트 ==="
    clj -M:test
    ;;
  test-go)
    echo "=== Go 아카이브 테스트 ==="
    cd archive/go && go test ./... -count=1
    ;;
  repl)
    clj -M:repl
    ;;
  run)
    clj -M:run "$@"
    ;;

  ## --- Native Image ---
  uberjar)
    echo "=== Uberjar 빌드 ==="
    mkdir -p target
    clj -J-Dclojure.compiler.direct-linking=true -T:build uber
    echo "→ target/durable-iot-migrate.jar"
    ;;
  native-build)
    echo "=== GraalVM Native Image 빌드 ==="
    # 1. uberjar
    mkdir -p target
    clj -J-Dclojure.compiler.direct-linking=true -T:build uber
    # 2. native-image
    native-image \
      -jar target/durable-iot-migrate.jar \
      -o target/durable-iot-migrate \
      -H:Name=durable-iot-migrate \
      --no-fallback \
      --initialize-at-build-time \
      -H:+ReportExceptionStackTraces \
      --enable-native-access=ALL-UNNAMED \
      -J-Xmx4g \
      2>&1
    echo ""
    echo "→ target/durable-iot-migrate ($(du -h target/durable-iot-migrate | cut -f1))"
    ;;
  native-run)
    ./target/durable-iot-migrate "$@"
    ;;

  ## --- 관리 ---
  clean)
    echo "=== 정리 ==="
    rm -rf .cpcache/ target/
    echo "done"
    ;;
  help|*)
    echo "durable-iot-migrate — IoT 플랫폼 마이그레이션 CLI"
    echo ""
    echo "Usage: ./run.sh <command> [args]"
    echo ""
    echo "Development (JVM):"
    echo "  test           Clojure 테스트"
    echo "  test-go        Go 아카이브 테스트"
    echo "  run [args]     CLI 실행"
    echo "  repl           Clojure REPL"
    echo ""
    echo "Native (GraalVM):"
    echo "  uberjar        AOT uberjar 빌드"
    echo "  native-build   GraalVM native binary 빌드"
    echo "  native-run     native binary로 실행"
    echo ""
    echo "Management:"
    echo "  clean          캐시, target 삭제"
    echo ""
    echo "Shells:"
    echo "  nix develop          — GraalVM (native-image 포함)"
    echo "  nix develop .#jvm    — JVM only (가벼운 개발)"
    ;;
esac
