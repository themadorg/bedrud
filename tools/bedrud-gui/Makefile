# Stable flag so the cgo cache is reused. gotk4 `free()` prototype warning is harmless.
export CGO_CFLAGS := -Wno-builtin-declaration-mismatch

HOST_EMBED := internal/hostbin/bedrud-host

.PHONY: test build run tidy embed-host appimage

embed-host:
	@$(MAKE) -C ../bedrud-host build
	@cp -f ../bedrud-host/bedrud-host $(HOST_EMBED)

test: embed-host
	go test ./internal/hostcli ./internal/tray ./internal/hostbin

tidy:
	go mod tidy

build: embed-host
	@echo "==> building bedrud-gui (embeds bedrud-host; GTK cgo; first compile can take several minutes)"
	go build -v -o bedrud-gui ./cmd/bedrud-gui

run: build
	./bedrud-gui

# Standalone AppImage (downloads linuxdeploy + gtk plugin + appimagetool on first run).
appimage: build
	sh packaging/mk-appimage.sh
