#!/usr/bin/env bash
set -euo pipefail
cd "$(dirname "$0")"
ROOT="$(pwd)"
TINYGO="${TINYGO:-/Users/dimajolkin/GolandProjects/origin_tinygo/build/tinygo}"
PORT="${PORT:-/dev/cu.usbmodem2101}"
TARGET="${TARGET:-esp32c3-supermini}"
IDF_PROJECT="${ROOT}/build/idf-wifi-project"
BOOTLOADER_BIN="${IDF_PROJECT}/build/bootloader/bootloader.bin"
PARTITION_BIN="${IDF_PROJECT}/build/partition_table/partition-table.bin"
FIRMWARE_BIN="${ROOT}/firmware.bin"

WITH_BOOTLOADER=
PKG=.
for arg in "$@"; do
  case "$arg" in
    --bootloader|-b) WITH_BOOTLOADER=1 ;;
    *)               PKG="$arg" ; break ;;
  esac
done

if [ -n "$WITH_BOOTLOADER" ]; then
  if [ ! -f "$BOOTLOADER_BIN" ] || [ ! -f "$PARTITION_BIN" ]; then
    echo "Building IDF bootloader and partition table..."
    if [ -z "${IDF_PATH:-}" ]; then
      echo "Set IDF_PATH (e.g. export IDF_PATH=\$HOME/esp/esp-idf) and run: . \$IDF_PATH/export.sh"
      exit 1
    fi
    ( cd "$IDF_PROJECT" && idf.py set-target esp32c3 2>/dev/null || true
      idf.py build )
  fi
  [ -f "$BOOTLOADER_BIN" ] || { echo "Missing: $BOOTLOADER_BIN — run: . \$IDF_PATH/export.sh && idf.py build in $IDF_PROJECT"; exit 1; }
  [ -f "$PARTITION_BIN" ] || { echo "Missing: $PARTITION_BIN"; exit 1; }
  echo "Building firmware..."
  "$TINYGO" build -stack-size=8192 -target "$TARGET" -o "$FIRMWARE_BIN" "$PKG"
  echo "Flashing with bootloader..."
  if command -v espflash >/dev/null 2>&1; then
    espflash flash --bootloader "$BOOTLOADER_BIN" --partition-table "$PARTITION_BIN" --port "$PORT" "$FIRMWARE_BIN"
    espflash monitor --port "$PORT"
  else
    esptool.py -p "$PORT" write_flash \
      0x0 "$BOOTLOADER_BIN" \
      0x8000 "$PARTITION_BIN" \
      0x10000 "$FIRMWARE_BIN"
    exec "$TINYGO" monitor --port "$PORT"
  fi
else
  exec "$TINYGO" flash -stack-size=8192 -monitor -target "$TARGET" --port "$PORT" "$PKG"
fi
