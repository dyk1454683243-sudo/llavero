# Packaging files

Reference copies of everything that has to be installed outside the binary.
Distro packages should use these rather than asking users to paste from the
top-level README.

| File | Install to |
|---|---|
| `70-uhid-uaccess.rules` | `/usr/lib/udev/rules.d/` |
| `70-tpmrm-uaccess.rules` | `/usr/lib/udev/rules.d/` (only needed for TPM unlock) |
| `llavero.service` | `/usr/lib/systemd/user/` |
| `user@.service.d-20-memlock.conf` | `/etc/systemd/system/user@.service.d/20-memlock.conf` |

Both udev rules are mandatory for the features they enable, not optional
extras. A package that installs only the binary will look broken: `/dev/uhid`
is root-only and `/dev/tpmrm0` is `root:tss`.

The `user@.service.d` drop-in raises the locked-memory ceiling. It is separate
because a user unit cannot raise a hard rlimit above the one the
`systemd --user` manager holds, so `LimitMEMLOCK` in `llavero.service` is
silently ignored without it. It also needs a reboot, since that manager
commonly survives a logout.
