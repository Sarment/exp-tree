PREFIX ?= $(HOME)/.local
BINDIR ?= $(PREFIX)/bin

.PHONY: build test install clean

build:
	CGO_ENABLED=0 go build -o bin/etree ./cmd/etree

test:
	CGO_ENABLED=0 go test -count=1 ./...

install: build
	mkdir -p $(BINDIR)
	cp bin/etree $(BINDIR)/etree
	chmod +x $(BINDIR)/etree

clean:
	rm -rf bin/ examples/demo_workspace examples/synergy_workspace examples/pareto_workspace
