# Дизассемблированные бинарники для отладки

Папка для разбора крэшей (например `pc: nil`, `mcause: 0x2` — Illegal Instruction).

- `firmware.elf` — собранный образ приложения (TinyGo + blobs).
- `firmware.elf.dis` — полный дизассемблер firmware.elf (секции .text, .iram и т.д.).
- `firmware.elf.syms` — таблица символов (nm -n), для поиска адреса по имени и наоборот.
- `libs/esp32c3/*.dis` — дизассемблер каждой библиотеки из `blobs/libs/esp32c3/`.
- `INVESTIGATION_PLAN.md` — пошаговый план разбора крэша после coex_wifi_release.

**Псевдокод:** Установлены retdec и Ghidra. Для RISC-V/ESP32-C3 используй **Ghidra**: `ghidraRun`, затем File → Import File → выбери `raw-binary/firmware.elf`, дождись анализа, переходи по адретам (например 0x420151b4) и смотри вкладку Decompiler.

Пересоздать всё: из корня репо запустить `./raw-binary/disasm.sh`.

Для поиска по адресу (например из лога исключения):
- Сначала искать в `firmware.elf.dis` (код приложения и линкованные символы).
- Если адрес в диапазоне блобов (см. карту в .elf или в логе загрузки), искать в соответствующем `libs/esp32c3/<lib>.dis`.
