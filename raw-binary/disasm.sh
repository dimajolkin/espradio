#!/usr/bin/env bash
set -euo pipefail
SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"
RAW="$SCRIPT_DIR"
LIBS_SRC="${ROOT}/blobs/libs/esp32c3"
LIBS_DST="${RAW}/libs/esp32c3"
TINYGO="${TINYGO:-/Users/dimajolkin/GolandProjects/origin_tinygo/build/tinygo}"
TARGET="${TARGET:-esp32c3-supermini}"
PKG="${1:-./examples/wifi}"
OBJDUMP="${OBJDUMP:-}"

if [ -z "$OBJDUMP" ]; then
  for d in "$HOME/.espressif/tools/riscv32-esp-elf/"*"/riscv32-esp-elf/bin/riscv32-esp-elf-objdump" \
           "/opt/homebrew/opt/riscv32-esp-elf/bin/riscv32-esp-elf-objdump"; do
    if [ -x "$d" ]; then OBJDUMP="$d"; break; fi
  done
fi
[ -n "$OBJDUMP" ] && [ -x "$OBJDUMP" ] || { echo "riscv32-esp-elf-objdump not found. Set OBJDUMP= or install ESP-IDF toolchain."; exit 1; }

mkdir -p "$LIBS_DST"
cd "$ROOT"

echo "Building firmware.elf..."
"$TINYGO" build -target "$TARGET" -o "$RAW/firmware.elf" "$PKG"

echo "Disassembling firmware.elf -> firmware.elf.dis..."
"$OBJDUMP" -d -S -l "$RAW/firmware.elf" > "$RAW/firmware.elf.dis" 2>/dev/null || "$OBJDUMP" -d "$RAW/firmware.elf" > "$RAW/firmware.elf.dis"
OBJDUMP_DIR="$(dirname "$OBJDUMP")"
NM="${OBJDUMP_DIR}/riscv32-esp-elf-nm"
[ -x "$NM" ] && { echo "Exporting symbols -> firmware.elf.syms"; "$NM" -n "$RAW/firmware.elf" > "$RAW/firmware.elf.syms"; } || true

for a in "$LIBS_SRC"/*.a; do
  [ -f "$a" ] || continue
  name="$(basename "$a" .a)"
  echo "Disassembling $name -> libs/esp32c3/${name}.dis"
  "$OBJDUMP" -d "$a" > "$LIBS_DST/${name}.dis" 2>/dev/null || true
done

if command -v retdec-decompiler >/dev/null 2>&1; then
  echo "Decompiling firmware.elf -> firmware_decompiled.c (optional, may fail for RISC-V)..."
  if retdec-decompiler -s "$RAW/firmware.elf" -o "$RAW/firmware_decompiled.c" 2>/dev/null; then
    echo "Decompiled to $RAW/firmware_decompiled.c"
  else
    echo "retdec skipped (unsupported arch/compiler; use Ghidra for RISC-V pseudo-code)"
  fi
fi

echo "Done. See $RAW/firmware.elf.dis and $RAW/libs/esp32c3/*.dis"
