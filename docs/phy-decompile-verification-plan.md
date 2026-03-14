# План верификации PHY-функций по декомпиляции IDF

Источник: `raw-binary/wifi_blobs.elf.c`

---

## 1. esp_wifi_bt_power_domain_on (строки 29447–29468)

| Шаг | IDF | Наш esp_phy_shim.c | Статус |
|-----|-----|-------------------|--------|
| 1 | `_lock_acquire` | `__sync_lock_test_and_set` | ✓ другой lock API |
| 2 | count++, prev=count==0 | s_wifi_bt_pd_ref++, prev==0 | ✓ |
| 3 | DIG_PWC &= 0xfffdffff | RTC_CNTL_DIG_PWC &= ~RTC_WIFI_FORCE_PD | ✓ |
| 4 | esp_rom_delay_us(10) | esp_rom_delay_us(10) | ✓ |
| 5 | wifi_bt_common_module_enable() | wifi_bt_common_module_enable() | ✓ |
| 6 | WIFI_RST_EN &= 0xffffd5e0 | SYSCON_WIFI_RST_EN \|= mask; \&= ~mask | ⚠️ IDF только AND; у нас pulse (OR→AND) |
| 7 | DIG_ISO &= 0xefffffff | RTC_CNTL_DIG_ISO &= ~RTC_WIFI_FORCE_ISO | ✓ |
| 8 | wifi_bt_common_module_disable() | wifi_bt_common_module_disable() | ✓ |
| 9 | _lock_release | __sync_lock_release | ✓ |

**Действие:** Оставить pulse для WIFI_RST_EN (OR→AND) — типичный reset.

---

## 2. esp_wifi_bt_power_domain_off (строки 121137–121150)

| Шаг | IDF | Наш | Статус |
|-----|-----|-----|--------|
| 1 | count-- | s_wifi_bt_pd_ref-- | ✓ |
| 2 | if count==0: DIG_ISO \|= 0x10000000, DIG_PWC \|= 0x20000 | if ref==0: DIG_ISO \|= RTC_WIFI_FORCE_ISO, DIG_PWC \|= RTC_WIFI_FORCE_PD | ✓ |

**Действие:** Нет.

---

## 3. wifi_bt_common_module_enable (строки 26853–26878)

| Шаг | IDF | Наш | Статус |
|-----|-----|-----|--------|
| 1 | vPortEnterCritical | — | ⚠️ у нас нет critical section |
| 2 | if ref==0: 0x60026014 \|= 0x78078f | if ref==0: SYSCON_CLK_EN_REG \|= WIFI_BT_CLK_EN_MASK | ✓ |
| 3 | ref++ | s_wifi_bt_common_ref++ | ✓ |
| 4 | vPortExitCritical | — | ⚠️ |

**Действие:** У нас spinlock в esp_wifi_bt_power_domain_on. ref_counts[0x10] = s_wifi_bt_common_ref. Оставить как есть (нет FreeRTOS).

---

## 4. wifi_bt_common_module_disable (строки 26885–26910)

| Шаг | IDF | Наш | Статус |
|-----|-----|-----|--------|
| 1 | vPortEnterCritical | — | ⚠️ |
| 2 | if ref==1: 0x60026014 &= 0xff87f870 | if ref==1: SYSCON_CLK_EN_REG &= WIFI_BT_CLK_DIS_MASK | ✓ |
| 3 | ref-- (ref+0xff) | s_wifi_bt_common_ref-- | ✓ |

**Действие:** Нет.

---

## 5. esp_phy_common_clock_enable (строки 29422–29428)

| IDF | Наш | Статус |
|-----|-----|--------|
| wifi_bt_common_module_enable() | wifi_bt_common_module_enable() | ✓ |

---

## 6. esp_phy_common_clock_disable (строки 29434–29440)

| IDF | Наш | Статус |
|-----|-----|--------|
| wifi_bt_common_module_disable() | wifi_bt_common_module_disable() | ✓ |

---

## 7. esp_phy_modem_init (строки 121156–121167)

| Шаг | IDF | Наш | Статус |
|-----|-----|-----|--------|
| 1 | _lock_acquire | espradio_phy_lock | ✓ |
| 2 | s_phy_modem_init_ref++ | s_phy_modem_init_ref++ | ✓ |
| 3 | if s_phy_digital_regs_mem==0: heap_caps_malloc(0x54,0x808) | if ptr==NULL && heap_caps_malloc: malloc(0x54,0x808) | ✓ |
| 4 | _lock_release | espradio_phy_unlock | ✓ |

**Действие:** Нет.

---

## 8. esp_phy_modem_deinit (строки 121173–121192)

| Шаг | IDF | Наш | Статус |
|-----|-----|-----|--------|
| 1 | prev_ref=ref, ref-- | prev_ref=ref, s_phy_modem_init_ref-- | ✓ |
| 2 | if prev==1: s_is_phy_reg_stored=false, free(), ptr=NULL, phy_init_flag() | if prev==1: s_phy_is_digital_regs_stored_local=0, free(), ptr=NULL, phy_init_flag() | ✓ |

**Действие:** Нет.

---

## 9. phy_digital_regs_load (строки 120944–120952)

| Шаг | IDF | Наш | Статус |
|-----|-----|-----|--------|
| 1 | if s_is_phy_reg_stored && s_phy_digital_regs_mem!=0: phy_dig_reg_backup(0) | if s_phy_is_digital_regs_stored_local && ptr!=NULL && phy_dig_reg_backup: phy_dig_reg_backup(false, ptr) | ✓ |

**Примечание:** IDF вызывает phy_dig_reg_backup(0) — один аргумент. Наш libphy: phy_dig_reg_backup(bool, uint32_t*). Мы передаём ptr явно.

---

## 10. phy_digital_regs_store (строки 120958–120967)

| Шаг | IDF | Наш | Статус |
|-----|-----|-----|--------|
| 1 | if s_phy_digital_regs_mem!=0: phy_dig_reg_backup(1), s_is_phy_reg_stored=true | if ptr!=NULL && phy_dig_reg_backup: phy_dig_reg_backup(true, ptr), s_phy_is_digital_regs_stored_local=1 | ✓ |

**Действие:** Нет.

---

## 11. esp_phy_enable (строки 121353–121383)

| Шаг | IDF | Наш | Статус |
|-----|-----|-----|--------|
| 1 | _lock_acquire | espradio_phy_lock | ✓ |
| 2 | if phy_get_modem_flag()==0 | if modem_flags==0 | ✓ |
| 3 | esp_phy_common_clock_enable() | esp_phy_common_clock_enable() | ✓ |
| 4 | if !s_is_phy_calibrated: esp_phy_load_cal_and_init(), s_is_phy_calibrated=true | if s_is_phy_calibrated==0: esp_phy_load_cal_and_init(), s_is_phy_calibrated=1 | ✓ |
| 5 | else: phy_wakeup_init(), phy_digital_regs_load() | else: phy_wakeup_init(), phy_digital_regs_load() | ✓ |
| 6 | phy_track_pll_init() | phy_track_pll_init() | ✓ |
| 7 | if phy_ant_need_update(): phy_ant_update(), phy_ant_clr_update_flag() | if phy_ant_need_update(): phy_ant_update(), phy_ant_clr_update_flag() | ✓ |
| 8 | phy_set_modem_flag(modem) | phy_set_modem_flag(modem) | ✓ |
| 9 | phy_track_pll() | phy_track_pll() | ✓ |
| 10 | _lock_release | espradio_phy_unlock | ✓ |

**Действие:** Нет.

---

## 12. esp_phy_disable (строки 121113–121131)

| Шаг | IDF | Наш | Статус |
|-----|-----|-----|--------|
| 1 | phy_clr_modem_flag(modem) | phy_clr_modem_flag(modem) | ✓ |
| 2 | if phy_get_modem_flag()==0 | if phy_get_modem_flag()==0 | ✓ |
| 3 | phy_track_pll_deinit() | s_phy_ant_need_update_local=1; phy_track_pll_deinit() | ⚠️ у нас лишний шаг |
| 4 | phy_digital_regs_store() | phy_digital_regs_store() | ✓ |
| 5 | phy_close_rf() | phy_close_rf() | ✓ |
| 6 | phy_xpd_tsens() | phy_xpd_tsens() | ✓ |
| 7 | esp_phy_common_clock_disable() | esp_phy_common_clock_disable() | ✓ |

**Действие:** Проверить необходимость `s_phy_ant_need_update_local = 1` — в IDF нет.

---

## 13. esp_phy_load_cal_and_init (строки 121253–121349)

| Шаг | IDF | Наш | Статус |
|-----|-----|-----|--------|
| 1 | phy_init_param_set(1) | phy_init_param_set(1) | ✓ |
| 2 | phy_bbpll_en_usb(1) | phy_bbpll_en_usb(true) | ✓ |
| 3 | out_cal_data = calloc(1, 0x770) | calloc(1, sizeof(...)), fallback static | ✓ |
| 4 | init_data = esp_phy_get_init_data() | init_data = esp_phy_get_init_data() | ✓ |
| 5 | cal_mode: reset_reason==5 ? NONE : FULL | rr==5 ? PHY_RF_CAL_NONE : PHY_RF_CAL_FULL | ✓ |
| 6 | esp_phy_load_cal_data_from_nvs | weak stub, возвращает ESP_ERR_NOT_FOUND | ✓ |
| 7 | MAC: esp_efuse_mac_get_default | espradio_hal_read_mac_go | ✓ |
| 8 | register_chipv7_phy(init, cal, mode) | register_chipv7_phy(init, cal, mode) | ✓ |
| 9 | esp_phy_store_cal_data_to_nvs | weak stub, no-op | ✓ |
| 10 | esp_deep_sleep_register_phy_hook(phy_close_rf, phy_xpd_tsens) | weak stub, возвращает ESP_OK | ✓ |
| 11 | free(out_cal_data) | free если calloc и hooks OK | ✓ |

**Действие:** Приведено к IDF.

---

## 14. phy_ant_need_update (строки 29474–29479)

| IDF | Наш | Статус |
|-----|-----|--------|
| return s_phy_ant_need_update_flag | return s_phy_ant_need_update_local != 0 | ✓ |

---

## 15. phy_ant_clr_update_flag (строки 121581–121587)

| IDF | Наш | Статус |
|-----|-----|--------|
| s_phy_ant_need_update_flag = false | s_phy_ant_need_update_local = 0 | ✓ |

---

## 16. phy_ant_update (строки 121593–121635)

| IDF | Наш | Статус |
|-----|-----|--------|
| s_phy_ant_config (глобал) | s_phy_ant_config_local | ✓ |
| ant_dft_cfg, ant_tx_cfg, ant_rx_cfg | ant_dft_cfg, ant_tx_cfg, ant_rx_cfg | ✓ |
| Логика rx_ant_mode, tx_ant_mode | Аналогично | ✓ |

---

## Итоговый чеклист

- [ ] **1** esp_wifi_bt_power_domain_on — оставить pulse для WIFI_RST_EN
- [ ] **2** esp_wifi_bt_power_domain_off — OK
- [ ] **3–4** wifi_bt_common_module_enable/disable — OK (без FreeRTOS critical)
- [ ] **5–6** esp_phy_common_clock_enable/disable — OK
- [ ] **7–8** esp_phy_modem_init/deinit — OK
- [ ] **9–10** phy_digital_regs_load/store — OK
- [ ] **11** esp_phy_enable — OK
- [ ] **12** esp_phy_disable — проверить s_phy_ant_need_update_local=1
- [ ] **13** esp_phy_load_cal_and_init — упрощённая версия без NVS
- [ ] **14–16** phy_ant_* — OK
