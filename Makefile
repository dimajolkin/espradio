update: esp-wifi/README.md
	rm -rf blobs/headers
	rm -rf blobs/include
	rm -rf blobs/libs
	mkdir -p blobs/libs/esp32c3 blobs/esp32c3
	cp -rp esp-wifi/c/headers                          blobs
	cp -rp esp-wifi/c/include                          blobs
	cp -rp esp-wifi/esp-wifi-sys-esp32c3/libs/*.a      blobs/libs/esp32c3
	cp -f blobs/esp32c3.ld targets/esp32c3.ld
	dd if=build/esp-idf/components/esp_phy/esp32c3/phy_multiple_init_data.bin of=blobs/esp32c3/phy_init_data.bin bs=1 skip=8 count=128 2>/dev/null || true
	[ -f blobs/esp32c3/phy_init_data.bin ] && xxd -i -n phy_init_data_bytes blobs/esp32c3/phy_init_data.bin | sed 's/^unsigned char/static const unsigned char/' > phy_init_data.inc || true
	[ -f build/esp-idf/components/esp_phy/esp32c3/include/phy_init_data.h ] && cp build/esp-idf/components/esp_phy/esp32c3/include/phy_init_data.h blobs/include/esp32c3/phy_init_data_idf.h || true

esp-wifi/README.md:
	git clone https://github.com/esp-rs/esp-wifi

.PHONY: build-blobs build-blogs sync-ld
build-blobs: esp-wifi/README.md
	./build/build-idf-blobs-docker.sh

build-blogs: build-blobs