.PHONY: test build preview firmware release
test: build
	bash scripts/test-native.sh
	GLANCE_INSTALL_TEST=1 E1003_RENDERER_TEST="$(CURDIR)/bin/glance-render" go test -race ./...
	go vet ./...
	bash scripts/test-launchagent.sh
build:
	bash scripts/build.sh
preview: build
	bin/glance preview --output var/preview.png
firmware:
	bash scripts/build-firmware.sh
release:
	bash scripts/package-release.sh
