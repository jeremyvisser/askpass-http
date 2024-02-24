default: askpass-http

askpass-http: *.go
	go build .

rpm: askpass-http
	rpmbuild -bb --build-in-place askpass-http.spec

install:
	install -v -m 644 \
		usr/lib/systemd/system/* \
		/usr/lib/systemd/system/
	install -v -m 755 \
		askpass-http \
		/usr/bin/
	install -v -D -m 755 \
		usr/lib/dracut/modules.d/98askpasshttp/module-setup.sh \
		/usr/lib/dracut/modules.d/98askpasshttp/

.PHONY: install
