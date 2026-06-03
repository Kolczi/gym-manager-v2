APP_NAME := gym-manager
VERSION  := $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
BUILD    := $(shell date -u +%Y%m%d%H%M%S)
LDFLAGS  := -s -w -X main.version=$(VERSION) -X main.build=$(BUILD)

.PHONY: all build web migrate-mongo clean deb dev test release

all: build web

## Build Wails desktop app (requires GTK4 + webkitgtk)
build:
	go build -ldflags "$(LDFLAGS)" -o bin/gym-manager-v2 .

## Build headless web server (VPS mode)
web:
	go build -ldflags "$(LDFLAGS)" -o bin/gym-web ./cmd/web/

## Build MongoDB migration tool
migrate-mongo:
	go build -ldflags "$(LDFLAGS)" -o bin/migrate-mongo ./cmd/migrate-mongo/

## Run dev server (HTTP, no Wails)
dev:
	GYM_DEV_HTTP=1 go run .

## Generate sqlc
sqlc:
	sqlc generate

## Run goose migrations
migrate-up:
	goose -dir sql/migrations sqlite3 "$(DATABASE_PATH)" up

migrate-down:
	goose -dir sql/migrations sqlite3 "$(DATABASE_PATH)" down

## Create backup
backup:
	./scripts/backup.sh

## Sync to VPS
sync:
	./scripts/sync.sh $(VPS_HOST)

## Build .deb package
deb: web
	$(eval TMPDIR := $(shell mktemp -d))
	mkdir -p $(TMPDIR)/opt/gym-manager/bin
	mkdir -p $(TMPDIR)/opt/gym-manager/internal/templates
	mkdir -p $(TMPDIR)/opt/gym-manager/frontend/dist
	mkdir -p $(TMPDIR)/opt/gym-manager/sql/migrations
	mkdir -p $(TMPDIR)/opt/gym-manager/scripts
	mkdir -p $(TMPDIR)/opt/gym-manager/deploy
	mkdir -p $(TMPDIR)/DEBIAN

	cp bin/gym-web $(TMPDIR)/opt/gym-manager/bin/
	cp -r internal/templates/* $(TMPDIR)/opt/gym-manager/internal/templates/
	cp -r frontend/dist/* $(TMPDIR)/opt/gym-manager/frontend/dist/
	cp -r sql/migrations/* $(TMPDIR)/opt/gym-manager/sql/migrations/
	cp scripts/backup.sh $(TMPDIR)/opt/gym-manager/scripts/
	cp deploy/gym-manager.service $(TMPDIR)/opt/gym-manager/deploy/
	cp deploy/postinstall.sh $(TMPDIR)/DEBIAN/postinst
	chmod 755 $(TMPDIR)/DEBIAN/postinst

	@echo "Package: $(APP_NAME)" > $(TMPDIR)/DEBIAN/control
	@echo "Version: $(VERSION)" >> $(TMPDIR)/DEBIAN/control
	@echo "Section: web" >> $(TMPDIR)/DEBIAN/control
	@echo "Priority: optional" >> $(TMPDIR)/DEBIAN/control
	@echo "Architecture: amd64" >> $(TMPDIR)/DEBIAN/control
	@echo "Depends: " >> $(TMPDIR)/DEBIAN/control
	@echo "Maintainer: Gym Manager <gym@local>" >> $(TMPDIR)/DEBIAN/control
	@echo "Description: Gym Manager — system zarządzania siłownią" >> $(TMPDIR)/DEBIAN/control

	dpkg-deb --root-owner-group --build $(TMPDIR) bin/$(APP_NAME)_$(VERSION)_amd64.deb
	rm -rf $(TMPDIR)
	@echo "→ bin/$(APP_NAME)_$(VERSION)_amd64.deb"

## Clean build artifacts
clean:
	rm -rf bin/

## Build all release artifacts
dist: web migrate-mongo
	@echo "Release artifacts in bin/"
	@ls -lh bin/

## Tag + GitHub Release with all binaries
## Usage: make release TAG=v0.2.0
release: dist
	@test -n "$(TAG)" || (echo "Usage: make release TAG=v0.2.0" && exit 1)
	git tag -a $(TAG) -m "Release $(TAG)"
	git push origin $(TAG)
	gh release create $(TAG) bin/gym-web bin/migrate-mongo \
		--title "$(TAG)" \
		--generate-notes
