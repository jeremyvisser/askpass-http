# askpass-http

Copyright © 2024 Jeremy Visser

An insecure but convenient way to enter systemd-ask-password prompts remotely over HTTP. Useful for situations where hardware tokens aren't available or practical, and locally stored keys are undesirable.

## Requirements

- systemd
- Distros:
  - Fedora / Red Hat with Dracut

## Features

- Reads systemd-ask-password prompts
- Can run from initramfs or regular system

## Caveats

- No verification by default. Your connection might have been MITM'ed.
  Take appropriate precautions.

## Initramfs network access

With Dracut, you can add the following to your boot command line (if not already present) to have it set up networking:

```
ip=dhcp rd.neednet=1
```

(In addition to any command line args already configured.)

See [dracut.cmdline(7)](man:dracut.cmdline(7)) for more config options.
