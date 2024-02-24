#!/bin/bash

check() {
    return 0
}

depends() {
    echo systemd
    return 0
}

install() {
    inst_multiple \
        /usr/bin/askpass-http \
        "${systemdsystemunitdir}/askpass-http.path" \
        "${systemdsystemunitdir}/askpass-http.service" \
        "${systemdsystemunitdir}/askpass-http.socket"

    ln_r "${systemdsystemunitdir}/askpass-http.path" \
         "${systemdsystemunitdir}/sysinit.target.wants/askpass-http.path"
}
