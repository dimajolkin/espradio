# espradio

## Wi‑Fi blobs и OSI‑адаптер

Этот репозиторий использует бинарные Wi‑Fi/BT драйверы из проекта [`esp-wifi-sys`](https://github.com/esp-rs/esp-wifi-sys) (через `esp-wifi`):

- `Makefile`:
  - `git clone https://github.com/esp-rs/esp-wifi` в поддиректорию `esp-wifi`.
  - `build-blobs` вызывает `./build/build-idf-blobs-docker.sh`, который собирает/выгружает IDF‑совместимые библиотеки и заголовки в `blobs/`.
- Для `esp32c3` используются библиотеки из `blobs/libs/esp32c3/`:
  - `libnet80211.a` — Wi‑Fi MAC/management.
  - `libcoexist.a` — Wi‑Fi/BT coexistence.
  - `libcore.a`, `libpp.a`, `libphy.a`, `libwpa_supplicant.a` и др.

Основной интерфейс между Go‑кодом и блобами — структура `wifi_osi_funcs_t` из `blobs/include/esp_private/wifi_os_adapter.h`. Реализация этой структуры находится в `osi.c`:

- Группы функций:
  - задачи и планировщик: `_task_create*_`, `_task_delete`, `_task_delay`, `_task_get_current_task` и т.п. (проброшены в Go через `radio.go`);
  - очереди и семафоры: `_queue_*`, `_semphr_*`, `_wifi_thread_semphr_get` (реализованы поверх Go‑каналов в `sync.go`);
  - группы событий: `_event_group_*` (на `sync.Cond`);
  - таймеры: `_timer_setfn`, `_timer_arm(_us)`, `_timer_done` (`espradio_timer_*` + goroutine‑таймеры в `radio.go`);
  - память и логирование: `_malloc*`, `_free`, `_log_write*`, `_log_timestamp`;
  - события: `_event_post` → очередь событий; dispatch в `espradio_event_loop_run_once()` из горутины; минимальный esp_event (`esp_event_loop_create_default`, `esp_event_handler_register`). См. `docs/rust-vs-espradio.md`.
  - coexistence: `_coex_*` и `_coex_schm_*` (пока реализованы как заглушки, часть из них только логирует вызов и/или паникует).

Инициализация стека Wi‑Fi выполняется через `espradio_wifi_init` в `radio.c`:

- Заполняется `wifi_init_config_t cfg = WIFI_INIT_CONFIG_DEFAULT();` из `blobs/include/esp_wifi.h`.
- Поля переопределяются:
  - `cfg.osi_funcs = &espradio_osi_funcs;`
  - `cfg.nvs_enable = 0;` (отключён NVS внутри драйвера).
- Далее вызывается `esp_wifi_init_internal(&cfg)` из `libnet80211.a`.

По `nm` для `libnet80211.a` и `libcoexist.a`:

- В `libnet80211.a` есть функции:
  - управление «домашним» каналом: `wifi_check_home_channel`, `wifi_set_home_channel_process`, `esp_wifi_set_home_channel`, а также вспомогательный модуль канал‑менеджера `chm_*` (`chm_get_home_channel`, `chm_set_home_channel`, `chm_return_home_channel`, `chm_is_at_home_channel` и др.);
  - генерация событий: `wifi_event_post`, `wifi_mesh_event_post`, множество внутренних `*_process` вызывают `wifi_event_post`.
- В `libcoexist.a` экспортируются:
  - `coex_init`, `coex_enable`, `coex_disable`, `coex_event_duration_get`, `coex_status_get` и др.;
  - `coex_wifi_request`, `coex_wifi_release`, `coex_bt_request/release`;
  - `coex_register_start_cb`, `coex_core_*`, а также `esp_coex_adapter_register` и MD5‑проверки адаптера.
  Эти функции ожидают, что вызовы к coexistence‑слою будут маршрутизированы через указатели, заданные в `wifi_osi_funcs_t` (поля `_coex_*`, `_coex_schm_*` и `_coex_schm_flexible_*`).

### Известная особенность: WIFI_EVENT_HOME_CHANNEL_CHANGE

При вызове `Scan()` (см. `wifi_scan.go`) наблюдается последовательность:

- драйвер инициализируется, создаёт очередь и семафоры (`espradio_wifi_create_queue`, `espradio_semphr_create`), запускает Wi‑Fi‑задачу;
- в процессе инициализации coexistence вызываются:
  - `coex_register_start_cb(cb=...)`;
  - `coex_schm_register_cb(type=0, cb=...)`;
- перед событием `WIFI_EVENT_HOME_CHANNEL_CHANGE` драйвер вызывает:
  - `coex_enable` → заглушка в `osi.c` только логирует и возвращает `0`;
  - затем `_event_post` с `event_base = "WIFI_EVENT"` и `event_id = 43` (`WIFI_EVENT_HOME_CHANNEL_CHANGE` по `esp_wifi_types_generic.h`);
  - внутри `_event_post` вызывается зарегистрированный из Go колбэк (`espradio_dummy_event_cb`), он отрабатывает успешно.

После возврата из `_event_post` выполнение продолжается уже внутри блоба и на некоторых конфигурациях платы/прошивки может завершиться исключением ядра (`pc = 0`, `mcause = 2`, т.е. RISC‑V Illegal Instruction). Это указывает на возможное рассогласование:

- между версией blobs (из `esp-wifi-sys`/`esp-wifi`) и локальной реализацией `wifi_osi_funcs_t`/`sdkconfig.h`, или
- на баг внутри самой бинарной библиотеки.

Локальный код старается 1:1 следовать макету `wifi_osi_funcs_t` из `blobs/include/esp_private/wifi_os_adapter.h`; все поля до `_magic` заполнены, включая последние `_coex_schm_flexible_period_*` и `_coex_schm_get_phase_by_idx`. Если поведение блобов меняется между версиями `esp-wifi-sys`, необходимо сверять `osi.c` с актуальным OSI‑адаптером из репозитория `esp-wifi-sys`/`esp-radio` для выбранной цели (ESP32‑C3).

### Запуск с IDF bootloader

Сейчас TinyGo по умолчанию прошивает один образ с 0x0 без второго этапа загрузчика. Если Wi‑Fi ведёт себя нестабильно или падает после `coex_wifi_release`, стоит попробовать схему **bootloader @ 0x0 + приложение @ 0x10000**: второй этап загрузчика переконфигурирует SPI flash по заголовку приложения и даёт окружение, как в IDF. Инструкция: **`docs/bootloader.md`**.





Notes:

По псевдокоду в firmware.elf.c цепочка для event=36 сосредоточена в wifi_reset_mac(): request(36) → вызов по 0xf4 → release(36) → обращение к _g_wdev_last_desc_reset_ptr. Наиболее правдоподобная причина pc: nil — вызов по нулевому указателю, соответствующему в псевдокоде _g_wdev_last_desc_reset_ptr (или эквивалентному месту в дизассемблере сразу после возврата из coex_wifi_release). Дальше имеет смысл искать в дизассе точную инструкцию после coex_wifi_release (load + jalr/jr) и при необходимости добавить в наш код инициализацию/заглушку для этого callback’а.



Ошибка теперь точно не в OSI‑слотах _event_post, _mutex_lock/_mutex_unlock и _timer_setfn, а в самом коде блоба между wifi_event_post(43, ...) и первым следующим OSI‑вызовом (внутри chm_set_current_channel/PHY).


pc:nil после event_post id=43 с высокой вероятностью происходит глубже — в set_chanfreq / ic_mac_* / pm_*, которые опираются на “железные” регистры и внутренние глобалы, и ожидают:
полный IDF‑бут‑путь (bootloader, init PLL/BBPLL/XTAL, RF‑init),
“родной” layout памяти и ROM‑функций.