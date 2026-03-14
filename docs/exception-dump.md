# Расширенный вывод при исключении (trap)

При падении по исключению (например `pc: nil`, Illegal Instruction) в лог UART выводится дополнительная строка **до** стандартного сообщения TinyGo:

```
*** Trap: mepc=0x... mcause=0x... (code=...) mtval=0x...
*** Illegal instruction at 0x... (mtval=instruction or 0)

*** Exception:     pc: nil
...
```

- **mepc** — адрес инструкции, которая вызвала исключение (или куда вернуться).
- **mcause** — причина (2 = Illegal instruction, 0 = fetch address misaligned, 1 = fetch access fault, и т.д.).
- **mtval** — для Illegal instruction часто 0; для fault по адресу — адрес доступа.

Перехват устанавливается в `Enable()` через `espradio_trap_handler_install()`: текущий `mtvec` сохраняется, в него записывается `espradio_trap_entry` (asm). При трапе сначала вызывается наш вывод, затем управление передаётся исходному обработчику runtime.

Файл: `trap_handler.c` (naked-функция + inline asm).
