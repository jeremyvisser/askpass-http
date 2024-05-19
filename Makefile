GOOS=linux
GOARCH=amd64
GO := GOOS=$(GOOS) GOARCH=$(GOARCH) go
GONATIVE := go

all: askpass-http rpm

rpm: askpass-http.rpm

askpass-http: *.go
	$(GO) build .

askpass-http.rpm: askpass-http util/build-deb/*.go
	$(GONATIVE) run ./util/build-deb

clean:
	rm -f \
		askpass-http \
		askpass-http.rpm

.PHONY: all rpm clean
