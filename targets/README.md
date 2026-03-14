# Цели для Wi‑Fi (ESP32-C3)

Таргет **esp32c3-wifi.json** — копия конфигурации ESP32-C3 с увеличенным стеком горутин:

- **default-stack-size: 8192** — стек 8 KB для wifi task и колбэков OSI (по умолчанию ~2 KB может приводить к `pc: nil` при старте Wi‑Fi).

Сборка и прошивка с этой целью (из корня репозитория):

```bash
tinygo flash -target=targets/esp32c3-wifi.json -monitor --port /dev/cu.usbmodem2101 examples/wifi/main.go
```

Пути `linkerscript` и `extra-files` в JSON заданы относительно корня TinyGo (например `targets/esp32c3.ld`), поэтому компилятор берёт их из своей установки. Либо используй `flash.sh`, он передаёт `-stack-size=8192` для обычного таргета `esp32c3-supermini`.
