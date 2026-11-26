PREFIX ?= $(HOME)/.local
BINDIR ?= $(PREFIX)/bin
SHAREDIR ?= $(PREFIX)/share/exptree

.PHONY: build build-linux test install clean

build:
	CGO_ENABLED=0 go build -o bin/exptree ./cmd/exptree

build-linux:
	mkdir -p bin
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o bin/exptree-linux-amd64 ./cmd/exptree
	CGO_ENABLED=0 GOOS=linux GOARCH=arm64 go build -o bin/exptree-linux-arm64 ./cmd/exptree

test:
	CGO_ENABLED=0 go test -count=1 ./...

install: build build-linux
	mkdir -p $(BINDIR) $(SHAREDIR)
	cp bin/exptree $(BINDIR)/exptree
	chmod +x $(BINDIR)/exptree
	ln -sf exptree $(BINDIR)/etree
	cp bin/exptree-linux-amd64 $(SHAREDIR)/exptree-linux-amd64
	cp bin/exptree-linux-arm64 $(SHAREDIR)/exptree-linux-arm64

clean:
	rm -rf bin/ examples/demo_workspace examples/synergy_workspace examples/pareto_workspace
