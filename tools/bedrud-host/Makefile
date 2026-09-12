.PHONY: help test vet build clean init list status

BIN := bedrud-host

help:
	@echo "bedrud-host"
	@echo "  make test    go test ./..."
	@echo "  make vet     go vet ./..."
	@echo "  make build   go build -o $(BIN)"
	@echo "  make init    build + ./$(BIN) init   (flags: make init ARGS='--linode-token …')"
	@echo "  make list    ./$(BIN) list (builds only if missing)"
	@echo "  make status  ./$(BIN) status (builds only if missing)"
	@echo "  make clean   remove binary"

test:
	go test -count=1 ./...

vet:
	go vet ./...

build:
	@tail -c 40 $(BIN) 2>/dev/null > .$(BIN).keytail || true
	go build -o $(BIN) .
	@if [ -s .$(BIN).keytail ]; then \
	  python3 -c "d=open('.$(BIN).keytail','rb').read(); raise SystemExit(0 if d.endswith(b'BHKEY1\\x00\\x00') else 1)" \
	  && cat .$(BIN).keytail >> $(BIN) && echo "preserved key trailer"; \
	  fi
	@rm -f .$(BIN).keytail

init: build
	./$(BIN) init $(ARGS)

list:
	test -x $(BIN) || $(MAKE) build
	./$(BIN) list

status:
	test -x $(BIN) || $(MAKE) build
	./$(BIN) status

clean:
	rm -f $(BIN) $(BIN).exe
