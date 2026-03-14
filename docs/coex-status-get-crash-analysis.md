# Разбор падения после coex_status_get (pc: nil)

## Цепочка по логу

```
osi: phy_enable
osi: coex_status_get -> 0
[wifi] Set ps type: 0, coexist: 0
osi: coex_status_get -> 0
*** Exception:     pc: nil
*** Exception:   code: 2
*** Exception: mcause: 0x00000002  (Illegal instruction)
```

`pc: nil` — переход по адресу 0 (вызов функции по нулевому указателю).

## Путь выполнения по декомпилятору (wifi_blobs.elf.c)

**pm_start** (строки 111855–111946) использует `_g_osi_funcs_p` — указатель на таблицу `wifi_osi_funcs_t`:

```c
// Строка 111901 — первый coex_status_get (offset 400)
iVar1 = (**(code **)(_g_osi_funcs_p + 400))();
if (iVar1 != 0) {
  (**(code **)(_g_osi_funcs_p + 0x1b4))(DAT_ram_3fc993fc / 100);  // coex_schm_interval_set
}
// ...
// Строка 111909 — второй coex_status_get
DAT_ram_3fc993d5 = (**(code **)(_g_osi_funcs_p + 400))();
// ...
// Строка 111924 — вызов coex_pti_get (offset 0x1a8 = 424) → CRASH если NULL
(**(code **)(_g_osi_funcs_p + 0x1a8))(0,&uStack_39);
hal_set_rx_beacon_pti(uStack_39);
```

Если `*(_g_osi_funcs_p + 0x1a8)` == 0, происходит переход по NULL.

`_g_osi_funcs_p` — ячейка по адресу **0x3fcdf954** (EXT_ram_3fcdf954 в wifi_blobs.elf.c:21286). Туда записываем `&espradio_osi_funcs` в `espradio_ensure_osi_ptr()` и `espradio_esp_wifi_start()`.

## Путь по дизассемблеру (libpp.dis)

В libpp.dis pm_start загружает таблицу через s1 (релокация на глобальную переменную), затем:
- offset 400 → _coex_status_get
- offset 424 (0x1a8) → _coex_pti_get

## Смещения в wifi_osi_funcs_t (wifi_os_adapter.h)

| Offset | Поле |
|--------|------|
| 400 (0x190) | _coex_status_get |
| 404 (0x194) | _coex_condition_set |
| 424 (0x1a8) | **_coex_pti_get** |
| 436 (0x1b4) | _coex_schm_interval_set |

## Проверка espradio_osi_funcs (osi.c)

В `espradio_osi_funcs` поле `_coex_pti_get` задано:

```c
._coex_pti_get = espradio_coex_pti_get,
```

`espradio_coex_pti_get` реализован в coex.c и возвращает 0 в `*pti`.

## Возможные причины crash

1. **Blob использует другую таблицу** — не `espradio_osi_funcs`, а `g_wifi_osi_funcs` или указатель из `G_OSI_FUNCS_P_ADDR` (0x3fcdf954). Если `g_wifi_osi_funcs` не заполнен до вызова pm_start, слот 424 = 0.

2. **s1 указывает не на osi_funcs** — в pm_start s1 загружается как `lui s1,0x0` (релокация на глобальную переменную). Если это не структура с указателем на osi_funcs по offset 0, то `lw a5,0(s1)` даёт неверный base, и `lw a5,424(a5)` читает из неправильного места.

3. **Порядок инициализации** — pm_start может вызываться до `memcpy(&g_wifi_osi_funcs, ...)` или до записи в `G_OSI_FUNCS_P_ADDR`.

4. **Другой путь** — crash может быть не в pm_start, а в pm_on_channel (строка 4747 libpp.dis) или pm_coex_* (строка 33573), где тоже есть `lw a5,424(a5); jalr a5`.

## Находка: блоб обнуляет g_osi_funcs_p в конце pm_start

В libpp.dis (строки 6800–6803) в конце pm_start:
```
22e:  lui   a5, 0x0      # релокация на g_osi_funcs_p
232:  sw    zero, 0(a5)   # *(g_osi_funcs_p) = 0
236:  lui   a5, 0x0      # вторая релокация
23a:  sw    zero, 0(a5)   # ещё одна запись 0
```

Блоб обнуляет ячейку 0x3fcdf954 после успешного завершения pm_start. Вызов `ensure_osi_ptr` в coex_status_get/phy_enable не устраняет crash — stored и slot_1a8 корректны, но падение сохраняется.

## Рекомендации

1. **Использовать espradio_esp_wifi_start вместо esp_wifi_start** — `espradio_esp_wifi_start` перед вызовом blob записывает `G_OSI_FUNCS_P_ADDR` и `g_osi_funcs_p`. Если между `Enable()` и `Start()` что-то перезаписывает таблицу, повторная запись перед start должна помочь. **Сделано** в wifi_scan.go.

2. **Добавить лог в espradio_coex_pti_get** — если лог появляется до crash, вызов доходит, и проблема в коде после return. Если crash до лога — указатель по offset 424 действительно NULL.

3. **Проверить инициализацию** — в `espradio_wifi_init` перед `esp_wifi_init_internal`:
   - `memcpy(&g_wifi_osi_funcs, &espradio_osi_funcs, sizeof(wifi_osi_funcs_t))`
   - `*(volatile uint32_t *)G_OSI_FUNCS_P_ADDR = (uint32_t)&espradio_osi_funcs`
   Убедиться, что это выполняется до любого вызова pm_*.

4. **Проверить pm_update_params** — в libpp s1 при втором вызове указывает на глобальную структуру (релокация в 6682). Определить, какой символ линкуется, и что по offset 0 лежит указатель на заполненную osi-таблицу.

---

## Дополнительные вызовы _coex_pti_get (0x1a8) в wifi_blobs.elf.c

- **111924** — pm_start (основной путь crash)
- **41297** — другой путь (coex config)
