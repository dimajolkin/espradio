# Использование IDF bootloader с TinyGo

Сейчас TinyGo прошивает один образ по адресу **0x0**: ROM загрузчик грузит его и сразу передаёт управление в `call_start_cpu0`. Второй этап загрузки (IDF bootloader) не используется.

## Зачем может понадобиться bootloader

Второй этап загрузчика в ESP-IDF ([Bootloader](https://docs.espressif.com/projects/esp-idf/en/latest/esp32c3/api-guides/bootloader.html)):

- Делает минимальную инициализацию модулей.
- Читает **partition table** (по умолчанию с 0x8000), выбирает раздел приложения (например factory @ 0x10000).
- Загружает образ приложения в RAM и передаёт ему управление.
- **Переконфигурирует SPI flash** по заголовку **образа приложения** (режим, частота, размер). ROM и bootloader делают это по-разному; blobs Wi‑Fi/PHY могут ожидать, что к моменту их работы flash уже настроен так, как после bootloader’а.

Если крэш или нестабильная работа Wi‑Fi связаны с отличием инициализации (flash cache, такты и т.п.), имеет смысл попробовать схему **bootloader @ 0x0 + приложение @ 0x10000**.

## Как попробовать

### 1. Собрать bootloader и partition table в idf-wifi-project

```bash
cd build/idf-wifi-project
. $IDF_PATH/export.sh
idf.py set-target esp32c3
idf.py build
```

В каталоге `build/` появятся:

- `build/bootloader/bootloader.bin` — второй этап загрузчика;
- `build/partition_table/partition-table.bin` — таблица разделов (если включена в проекте);
- по умолчанию в IDF partition table лежит по 0x8000, приложение factory — с **0x10000**.

### 2. Собрать приложение TinyGo

Используется **тот же линкер-скрипт** (`build/esp32c3.ld` / `blobs/esp32c3.ld`), что и при прошивке без bootloader: в нём заданы только адреса выполнения (DRAM/IRAM/DROM/IROM), а не смещение в flash. Образ в формате ESP (заголовок + сегменты) один и тот же; меняется только адрес прошивки (0x0 или 0x10000). Отдельный .ld для режима с bootloader не нужен.

Как обычно, например:

```bash
tinygo build -target=esp32c3 -o=firmware.bin ./examples/wifi
```

### 3. Прошить с bootloader’ом

С **espflash** (рекомендуется для TinyGo): espflash сам подставит смещение приложения из partition table (обычно factory @ 0x10000).

```bash
espflash flash --bootloader build/idf-wifi-project/build/bootloader/bootloader.bin \
  --partition-table build/idf-wifi-project/build/partition_table/partition-table.bin \
  firmware.bin
```

Или вручную через esptool.py (адреса по умолчанию для одной app-партиции):

```bash
esptool.py -p /dev/cu.usbmodem* write_flash \
  0x0   build/idf-wifi-project/build/bootloader/bootloader.bin \
  0x8000 build/idf-wifi-project/build/partition_table/partition-table.bin \
  0x10000 firmware.bin
```

Проверь, что в твоём `partition_table/` лежит раздел типа `app` с offset 0x10000 (например `factory`).

### 4. Ожидаемое поведение

- После сброса ROM загружает bootloader с 0x0.
- Bootloader читает partition table, находит приложение (0x10000), настраивает flash по заголовку этого образа, загружает образ в RAM и прыгает в него.
- Дальше выполняется уже TinyGo-приложение. Wi‑Fi blobs работают в окружении, максимально близком к типичному IDF (один раз уже прошёл второй этап загрузки и переконфиг flash).

## Если используешь только TinyGo (без IDF bootloader)

Текущий режим: один образ по 0x0, entry `call_start_cpu0`, без partition table. Так тоже допустимо: ROM сам грузит образ и настраивает flash по его заголовку. Разница только в том, что не выполняется код второго этапа (IDF bootloader). Если проблема воспроизводится только без bootloader’а — значит, стоит зафиксировать схему с bootloader’ом и, при необходимости, доставить недостающую инициализацию в ранний старт TinyGo (аналог того, что делает bootloader).
