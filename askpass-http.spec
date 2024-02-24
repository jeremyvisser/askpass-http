Summary: Go Askpass web server
Name: askpass-http
Version: 0
Release: 0
License: MIT
BuildRequires: golang
Requires: systemd, dracut

%description
Lets you unlock your disk from the moon. Roaming charges may apply.

%build
go build .

%install
install -Dm755 \
    -t "%{buildroot}/usr/bin/" \
    askpass-http
install -Dm644 \
    -t "%{buildroot}/usr/lib/systemd/system/" \
    usr/lib/systemd/system/*
install -Dm755 \
    -t "%{buildroot}/usr/lib/dracut/modules.d/98askpasshttp/" \
    usr/lib/dracut/modules.d/98askpasshttp/module-setup.sh

%posttrans
systemctl daemon-reload
if [[ $1 -ge 1 ]]; then
    dracut -f
fi

%preun
if [[ $1 == 0 ]]; then
    systemctl disable --now \
        askpass-http.path \
        askpass-http.socket \
        askpass-http.service
    dracut -f
fi

%files
/usr/bin/askpass-http
/usr/lib/systemd/system/askpass-http.path
/usr/lib/systemd/system/askpass-http.service
/usr/lib/systemd/system/askpass-http.socket
/usr/lib/dracut/modules.d/98askpasshttp/module-setup.sh
