# Сравнение с декомпилированным IDF (wifi_blobs.elf.c)

## 1. esp_wifi_init flow

### IDF (строки 67673–67700)
```
esp_wifi_set_sleep_min_active_time(50000);
esp_wifi_set_keep_alive_time(10000000);
esp_wifi_set_sleep_wait_broadcast_data_time(15000);
esp_wifi_set_log_level();
esp_wifi_bt_power_domain_on();
eVar1 = esp_wifi_init_internal(config);
if (eVar1 == 0) {
  esp_phy_modem_init();           // ← сразу после init_internal
  eVar1 = esp_supplicant_init();
  ...
}
```

### Наш radio.c
```
esp_wifi_set_sleep_min_active_time(50000);
esp_wifi_set_keep_alive_time(10000000);
esp_wifi_set_sleep_wait_broadcast_data_time(15000);
esp_wifi_bt_power_domain_on();
esp_wifi_init_internal(&cfg);
wifi_init_completed();            // в отдельном вызове
```

**Отличия:**
- `esp_wifi_set_log_level()` — у нас через `espradio_set_blob_log_level` в Go (g_log_level)
- `esp_phy_modem_init()` — **не вызываем сразу**. У нас вызывается из `phy_enable` OSI callback, когда blob запрашивает PHY
- `esp_supplicant_init()` — не вызываем (WPA)

---

## 2. esp_wifi_bt_power_domain_on

### IDF (строки 29447–29468)
```c
_DAT_ram_60008088 &= 0xfffdffff;   // DIG_PWC: clear WIFI_FORCE_PD (bit 17)
esp_rom_delay_us(10);
wifi_bt_common_module_enable();
_DAT_ram_60026018 &= 0xffffd5e0;   // WIFI_RST_EN: AND only (0xffffd5e0 = ~0x2a1f)
_DAT_ram_6000808c &= 0xefffffff;   // DIG_ISO: clear WIFI_FORCE_ISO (bit 28)
wifi_bt_common_module_disable();
```

### Наш esp_phy_shim.c
```c
RTC_CNTL_DIG_PWC &= ~RTC_WIFI_FORCE_PD;   // 0x60008088, bit 17 ✓
esp_rom_delay_us(10);                      // ✓
wifi_bt_common_module_enable();            // ✓
SYSCON_WIFI_RST_EN |= MODEM_RESET_WHEN_PU; // OR then AND (reset pulse)
SYSCON_WIFI_RST_EN &= ~MODEM_RESET_WHEN_PU;
RTC_CNTL_DIG_ISO &= ~RTC_WIFI_FORCE_ISO;  // ✓
wifi_bt_common_module_disable();           // ✓
```

**Отличие:** WIFI_RST_EN — IDF только `&= 0xffffd5e0` (очистка битов). У нас `|= mask` затем `&= ~mask` (импульс сброса). Типичный reset: assert → deassert. Декомпиляция могла скрыть OR.

---

## 3. wifi_bt_common_module_enable/disable

### IDF (строки 26853–26911)
- `enable`: если ref==0 → `0x60026014 |= 0x78078f`, ref++
- `disable`: если ref==1 → `0x60026014 &= 0xff87f870`, ref--

### Наш esp_phy_shim.c
- Маски 0x78078f / 0xff87f870 — совпадают ✓
- ref_counts[0x10] в IDF = наш s_wifi_bt_common_ref

---

## 4. esp_phy_modem_init

### IDF (строки 121156–121169)
```c
s_phy_modem_init_ref++;
if (s_phy_digital_regs_mem == 0)
  s_phy_digital_regs_mem = heap_caps_malloc(0x54, 0x808);
```

### Наш esp_phy_shim.c
- Логика совпадает (ref++, malloc 0x54/0x808 или fallback на локальный буфер)

**Вызов:** IDF — сразу после esp_wifi_init_internal. У нас — из `phy_enable` callback (osi.c).

---

## 5. esp_wifi_bt_power_domain_off

### IDF (строки 121137–121151)
```c
count--;
if (count == 0) {
  DIG_ISO |= 0x10000000;   // bit 28
  DIG_PWC |= 0x20000;      // bit 17
}
```

### Наш esp_phy_shim.c
- Совпадает ✓

---

## Рекомендации

1. **esp_phy_modem_init** — попробовать вызывать сразу после `esp_wifi_init_internal` в radio.c, как в IDF.
2. **WIFI_RST_EN** — оставить reset pulse (OR→AND), это стандартный паттерн сброса.
