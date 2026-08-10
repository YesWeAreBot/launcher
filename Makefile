VERSION ?= 0.1.0
LDFLAGS := -s -w -X launcher/cmd.version=$(VERSION)

.PHONY: build dist test vet clean

build:
	CGO_ENABLED=0 go build -trimpath -o dist/yesimbot-cli -ldflags '$(LDFLAGS)' .

# Optional: requires upx (apt install upx-ucl). ~4MB -> ~1.6MB.
dist: build
	upx --best dist/yesimbot-cli

test:
	go test ./...

vet:
	go vet ./...

clean:
	rm -f dist/yesimbot-cli
