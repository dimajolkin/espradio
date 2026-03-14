# Сравнение с esp-rs (Rust) — что ещё не перенесено

Rust-стек [esp-wifi](https://github.com/esp-rs/esp-wifi) / [esp-idf-svc](https://github.com/esp-rs/esp-idf-svc) использует те же Wi‑Fi blobs (из ESP-IDF / esp-wifi-sys) и полный **esp_event** из IDF. Ниже — что у них есть, а у espradio пока упрощено или отсутствует.

## 1. Event loop (esp_event)

| В Rust/IDF | У нас |
|------------|--------|
| `esp_event_loop_create_default()` — создаёт default loop с отдельной FreeRTOS-задачей, которая блокируется на очереди событий. | Реализовано минимально: очередь в C, `esp_event_loop_create_default()` + `esp_event_handler_register()`; dispatch в `espradio_event_loop_run_once()`, вызывается из Go-горутины по таймеру. |
| `esp_event_post` копирует данные в очередь; обработчики вызываются в контексте задачи event loop. | `_event_post` (osi) кладёт копию события в очередь и возвращает 0; обработчики вызываются в `run_once()` (другая горутина). |
| Поддержка `esp_event_post_to()`, `esp_event_isr_post`, отдельных loop’ов. | Только default loop, только синхронный post из драйвера (через osi `_event_post`). |

Итог: минимальный esp_event для Wi‑Fi событий есть; полноценный многозадачный loop как в IDF — нет.

## 2. FreeRTOS и задачи

| В Rust/IDF | У нас |
|------------|--------|
| Wi‑Fi драйвер крутится в отдельной FreeRTOS-задаче; event loop — в своей задаче. | Wi‑Fi код выполняется в горутине(ах) TinyGo; таймеры и event run_once — в других горутинах. |
| Реальные семафоры/очереди FreeRTOS (`xQueueCreate`, `xSemaphoreTake` и т.д.). | Очереди и семафоры эмулированы на Go-каналах и мьютексах (`sync.go`). |
| `portENTER_CRITICAL` / `portEXIT_CRITICAL` и прерывания. | `interrupt.Disable` / `Restore` в Go. |

Итог: поведение «одна задача драйвера + отдельная задача event loop» приближено за счёт горутин и run_once, но не один в один с FreeRTOS.

## 3. Coexistence (Wi‑Fi/BT)

| В Rust/IDF | У нас |
|------------|--------|
| Реальные `coex_*` и планировщик (schm), таймеры, прерывания. | В основном заглушки: логирование и возврат 0 или panic (см. `osi.c`: `espradio_coex_*`). |

Итог: для «только Wi‑Fi» этого может хватать; для одновременного Wi‑Fi + BLE нужна полноценная реализация или отключение coexistence в конфиге blobs.

## 4. NVS, PHY, часы

| В Rust/IDF | У нас |
|------------|--------|
| NVS для хранения настроек Wi‑Fi (и др.). | `nvs_enable = 0` в init; все `_nvs_*` в osi — panic/todo. |
| Калибровка PHY, RTC, часы. | Часть заглушек (`_phy_*`, `_esp_timer_get_time`, `_get_time` и т.д. — todo/panic). |
| `esp_phy_modem_init` и т.п. | Не вызываются (используются только пребилты из IDF). |

Итог: для базового scan/connect часто достаточно; для продакшена и точного соответствия IDF нужны NVS, PHY и время.

## 5. Что уже близко к Rust/IDF

- **wifi_osi_funcs_t**: все поля до `_magic` заполнены (в т.ч. `_event_post`, `_coex_*`, таймеры, очереди, семафоры, event_group).
- **События**: драйвер шлёт события через `_event_post`; мы кладём их в очередь и обрабатываем в `espradio_event_loop_run_once()` с зарегистрированным обработчиком (dummy или своим).
- **Таймеры**: `_timer_setfn` / `_timer_arm` / `_timer_done` + горутины в Go для срабатывания.
- **Заголовки и blobs**: те же источники (IDF, esp-wifi-sys для C3), те же имена библиотек.

## 6. Практические шаги для сближения с Rust

1. **Event loop**: оставить текущую схему (очередь + run_once) или позже заменить на вызов реального `libesp_event.a` из IDF, если появятся линковка и зависимости.
2. **Coexistence**: либо отключить в конфиге blobs (если возможно), либо портировать минимальный набор `coex_*` из esp-rs/IDF.
3. **NAN/NDP**: на C3 официально не поддерживаются; при появлении событий 35–42 мы их кладём в очередь и обрабатываем (или игнорируем) в обработчике, не возвращая -1 из `_event_post`, чтобы драйвер не шёл по падающему error-path.
