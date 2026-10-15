# Setting it up on Ubuntu with Plasma

End to end: build, install, configure, start, and get a browser on another machine to stop
complaining. Every command below was run while this page was written and its output is pasted
from that run, so where the wording of an error matters you are reading the real wording.

What you need: the machine you are installing on (root), and one other device on the same
network to act as the parent's browser — a phone is fine.

---

## 1. Build and install

```
make install
```

That runs the repo's gate first (`make verify`) and refuses to install a build that fails it,
then drops:

| path | mode | what it is |
|---|---|---|
| `/usr/local/bin/plock` | 0755 | the one binary |
| `/etc/systemd/system/plock.service` | 0644 | the unit |
| `/etc/plock/` | 0700 | the policy file goes here |
| `/var/lib/plock/` | 0700 | the certificate and its key |
| `/var/log/plock/` | 0750 | the audit trail |
| `/usr/share/doc/plock/docs/` | 0644 | these docs, for `systemctl status` |

It does **not** start or enable anything, and it does not write a config file. Both of those
are decisions you make in the next two steps.

To take it back off: `make uninstall`. That leaves `/etc/plock`, `/var/log/plock` and
`/var/lib/plock` alone — the policy and the audit trail are your family's records, not the
program's scratch space, and a refresh install should not need the rules rewritten.
`make uninstall PURGE=1` deletes them, and nothing can reconstruct the trail afterwards.

## 2. Create the policy

`plock init` is non-interactive: it needs the account name and where the password comes from.
Use `--password-stdin`.

```
sudo /usr/local/bin/plock init --username parent --password-stdin
```

A password in argv is readable by every user on the machine for as long as the process runs,
and stays in your shell history afterwards; stdin does neither. The password is stored only
as a PBKDF2-SHA256 verifier — the config never holds anything recoverable.

`init` refuses to overwrite an existing administrator. That is what `--force` is for, and
note what it means: replacing the credential is how you lock yourself out of the web UI if
you mistype it, and existing admin sessions keep working until they expire (see
[docs/security.md](security.md) §6).

Now add the rules. As of this release the policy is edited as JSON — there is no CLI editor
yet, and the ones that will replace hand-editing (`plock profile`, `plock user`,
`plock override`) are ticket #11 and not in this build. So:

```
sudo editor /etc/plock/config.json
```

Write it with the file already in place rather than by redirecting a heredoc into it, so a
mistake leaves the previous policy on disk, and check it parses before restarting the daemon:

```
sudo python3 -c "import json,sys; json.load(open('/etc/plock/config.json'))"
```

The daemon validates on load and refuses a file it cannot make sense of, which shows up as a
crash loop rather than a silent partial policy — the right way round, but easier to fix while
you still have the terminal open.

Deliberately **not** `systemctl restart plock` here: §4 is where the service is enabled and
started, and a restart in this section would start it two steps early while making §3's "check
this machine can lock before you enable anything" false. Reading the daemon's own startup lines
is still the check that matters, so it lives in §4 where there is a daemon to read them from.

## 3. Check this machine can lock at all

```
sudo /usr/local/bin/plock doctor
```

Do this before enabling the service. `doctor` asks each configured backend whether it is
usable and, critically, whether locking can be *confirmed* — and confirmation is the part
that is machine-specific. On Plasma the answer depends on ksmserver being alive and on
`kscreenlocker_greet` being parented by it, which is exactly the kind of thing a distribution
update changes.

If `doctor` says nothing is usable, `plock serve` will refuse to start. That refusal is the
correct behaviour and the next section explains why it is not a bug to work around.

## 4. Start it

```
sudo systemctl enable --now plock
journalctl -u plock -f
```

A healthy start looks like this, from a real run:

```
plock serve: starting, config /etc/plock/config.json
plock serve: backend logind is available
web: listening on https and serving a self-signed certificate at /var/lib/plock/tls/plock.crt. The browser will warn once; trusting this file stops it: /var/lib/plock/tls/plock.crt
enforce: running, every 30s, config /etc/plock/config.json
web: listening on https://127.0.0.1:18443
```

The `enforce: running` line is the one to look for. It exists because its absence used to be
invisible: `serve` started the web UI, logged a cheerful line about listening, and enforced
nothing. If the daemon comes up without that line, it is not enforcing.

Read the whole startup block rather than scanning for that one line. A rule that is wrong in a
permissive direction is otherwise invisible until the week you notice nothing was ever locked,
and the startup lines are where a degraded schedule announces itself — an unknown timezone
falling back to a default, or a rule that can never fire, is reported here and nowhere else.

### If it crash-loops instead

`Restart=always` with a config the daemon refuses produces a crash loop, and that is
deliberate: the alternative is a unit that is quietly inactive, which is how a child who
deleted the policy file would switch enforcement off with a green `systemctl is-enabled`.

```
systemctl status plock --no-pager
journalctl -u plock -n 40 --no-pager
```

The journal line names the reason. The three that happen in practice are a missing config, a
config that fails validation, and no usable backend.

## 5. The certificate

`tls: "auto"` (the default) mints a self-signed certificate into `/var/lib/plock/tls/` on
first start — `plock.crt` and `plock.key`, the key at mode 0600. It is valid 825 days and
re-mints itself when fewer than 30 days remain, so it is not something to schedule.

Its subject alternative names are taken from the machine at mint time: `localhost`, the
hostname, loopback, and every address the machine currently has. That is what lets you reach
the dashboard by LAN IP without a name mismatch. It also means **a certificate minted before
you plugged in the ethernet cable, or before a Docker network added a bridge address, will not
cover the address you later type.** If the browser complains about the name rather than the
authority, delete `plock.crt` and restart the daemon; it re-mints with today's addresses.

The browser will warn once, because nothing signed this. Three ways to stop it, in the order
most people try them:

**Trust the file on the parent's device.** On Ubuntu:

```
sudo cp plock.crt /usr/local/share/ca-certificates/plock.crt
sudo update-ca-certificates
```

then restart the browser. On Android and iOS, installing a CA certificate is a settings
screen and each browser decides for itself whether to honour it; expect Chrome on Android to
accept it and Safari on iOS to be unreliable about it.

**Bring your own.** `tls: "custom"` with `certFile` and `keyFile` uses your pair instead —
that is the option that removes the warning entirely, and it is the right one if you already
run a private CA at home. `step-ca`, smallstep, or a corporate CA all work; the daemon checks
the pair at startup and refuses a key that does not match the certificate or a certificate
that has already expired.

**Turn TLS off, knowing what that costs.** `tls: "off"` serves plain HTTP and the daemon says
so on stderr every single start, in the loudest sentence it has, because the admin password
then crosses your network in clear text. It is there for a laptop-on-localhost development
loop, not for a house.

### "Client sent an HTTP request to an HTTPS server."

That exact line means you typed `http://` at a TLS port. Go answers it rather than hanging, so
it is a fast mistake to fix: put the `s` in.

## 6. Reach it from another device

Find the address the daemon is actually on. The listener comes from the config, so read it
from the config rather than guessing:

```
sudo grep -o '"listen"[^,]*' /etc/plock/config.json
ip -4 -br addr | awk '$2 ~ /^up/ {print $1, $3}'
```

If `listen` names `127.0.0.1`, the dashboard is only reachable on the daemon's own machine —
that is the loopback address, not a default to be surprised by. Set `listen` to
`0.0.0.0:18443` (or the machine's specific address) to publish it to the network.

Then, from the other device, with the minted certificate trusted:

```
curl -s --cacert plock.crt -o /dev/null -w 'HTTP %{http_code} -> %{redirect_url}\n' https://ADDRESS:18443/
```

```
HTTP 303 -> https://ADDRESS:18443/login
```

A 303 to `/login` is the healthy answer: the server is up, and it is refusing to show you
anything until you authenticate.

There is no firewall rule opened for you. If the dashboard is unreachable from the other
device while `curl` from the daemon's own machine works, that is `ufw`/`firewalld` and it is
yours to decide — publishing a root admin interface on the LAN is a choice worth making
consciously rather than having a script make it for you.

## 7. Verifying the hardening

`deploy/plock.service` enables a long list of directives and refuses a few others, with the
reason for each written next to it. Three of those reasons are claims about behaviour that
only a live machine can settle, and they are marked `PROVE:` in the file. This is that check.
It takes about five minutes and it is worth doing after any distribution update, because the
thing most likely to change is not the unit — it is what the desktop exposes.

Run all three with the service enabled and a session logged in on the machine.

**Step 1 — `NoNewPrivileges` and the privilege drop.** The unit sets `NoNewPrivileges=true`
on the reasoning that the daemon only ever *drops* privilege, which NNP does not restrict.
Confirm the drop still happens. Restart, then let the daemon actually lock someone: put a
profile's window over the current time, wait one tick, and watch for the locker process.

```
sudo systemctl restart plock
journalctl -u plock -f
```

In another terminal, on the machine being locked:

```
watch -n2 'pgrep -a kscreenlocker_greet || echo "no locker"'
```

Within about thirty seconds the locker should appear. No "operation not permitted" in the
journal, and a locker in the process list, is the claim holding. If NNP were the problem you
would see the permission error once per tick and no locker — and the screen would stay
unlocked, which is the part that matters, because the daemon keeps trying and the dashboard
keeps looking normal.

**Step 2 — `ProtectSystem=strict`, `ProtectHome=read-only`, and the session bus.** The unit
makes the whole filesystem read-only except the declared paths, and mounts `/home`, `/root` and
`/run/user` read-only rather than masked, on the reasoning that `connect(2)` to another user's
socket at `/run/user/<uid>/bus` checks permission on the socket inode and not writability of the
mount. `ProtectHome` is the directive that actually names `/run/user`, so it is the one to reach
for first when this check fails; `ProtectSystem` is the one that makes everything outside the
declared paths unwritable. If the reasoning is wrong, the desktop backend cannot reach the bus:

```
sudo systemctl restart plock
sudo /usr/local/bin/plock doctor
```

`doctor` reports each backend. A `kde` or `gnome` line that goes from "available" to a bus
error after this change is the claim falsified — in which case the fix is
`ReadWritePaths=/run/user` in the `[Service]` section, not backing `ProtectSystem` off to
`full` and losing the rest.

**Step 3 — `RestrictAddressFamilies`.** The unit allows `AF_UNIX AF_INET AF_INET6 AF_NETLINK`
and nothing else. AF_UNIX is the bus, the two INET families are the listener, and AF_NETLINK is
the certificate's SAN list: `internal/web/tls.go` falls back to `net.Interfaces()`, which on
Linux is `syscall.NetlinkRIB(RTM_GETLINK, …)` rather than `getifaddrs`. It is a read-only dump,
not a listener, so allowing it costs nothing the filter is buying.

Without it the failure is quiet and expensive: mint logs one line and carries on with a
**loopback-only** certificate, the daemon starts, and everything looks healthy until you open
the dashboard by LAN address and get a name mismatch. So do not check this step by `curl`-ing
and calling it done — read the names back:

```
sudo openssl x509 -in /var/lib/plock/tls/plock.crt -noout -text | grep -A1 'Subject Alternative Name'
```

A SAN list containing only `localhost` and `127.0.0.1` on a machine with real interfaces means
the netlink socket was refused. Note that deleting the certificate and restarting — the
documented remedy for a name mismatch — re-mints loopback-only again, so if you are in that
loop, the family list is the thing to look at, not the certificate.

**And the one that must stay off.** `ProtectProc` is written at its default on purpose, and the
two keys that used to sit beside it in this file — `PIDNamespace=` and `HidePID=` — are not
systemd directives at all, so `systemd-analyze verify` was discarding them with a warning on
every daemon-reload while this section described them as deliberate. Read the unit's comment for
the argument, which is the part worth keeping. The behaviour it argues for is real: the KDE
locker is corroborated by reading `/proc/<pid>/status` and `/proc/<pid>/cmdline` to prove the
locker process is parented by ksmserver — the second signal behind issue #29, because logind's
`LockedHint` is writable by the session user with no privilege at all. Hide other users'
processes and the corroborator finds no locker, every lock reports unconfirmed, and enforcement
re-arms forever while the dashboard says the machine cannot confirm. If you harden this unit
further, `ProtectProc=invisible` and `PrivatePIDs=` (removed in v255) are the settings to leave
alone — verify any directive you add with `systemd-analyze verify deploy/plock.service`, which
is how the three phantom keys were caught.

## 8. Confirming it survives

Two things the unit promises, both cheap to check and both worth checking once rather than
trusting:

```
sudo kill -9 $(pgrep -x plock)
watch -n1 -d 'systemctl is-active plock'
```

`Restart=always` with `RestartSec=5` brings it back inside about five seconds.

Then, as the account you are locking out — not as root:

```
systemctl stop plock
```

This should be refused. The guarantee that a child cannot stop enforcement is not implemented
in this program; it is systemd's system manager refusing an unauthorised stop on a unit owned
by root. It is worth typing once on your own machine, because it depends on your polkit
policy and not on our code, and if your distribution has granted `manage-units` to active
users then this product's central promise has a hole in it that no test in this repository
would catch.

## 9. Where to look when something is wrong

| symptom | look here |
|---|---|
| screen never locks | `journalctl -u plock`, then `plock doctor` |
| "unconfirmed" in the log | [docs/platforms.md](platforms.md) §5 and §7 |
| dashboard unreachable | `listen` in the config, then the firewall |
| browser name mismatch | §5 — re-mint by deleting `plock.crt` |
| service in `failed` | `journalctl -u plock -n 40`; the refusal names its reason |
| `will NOT be locked` at startup | You started `plock serve` from the desktop you mean to enforce, so the daemon is inside that session and locking it would lock the screen you stop the daemon from. Install the unit (§1) — a system unit shares no login session with anyone and the rule goes away. [docs/security.md](security.md) §4. |
| enforcement stopped at some hour | `/var/log/plock/audit.jsonl` — one JSON event per line |

The audit trail is the record of what enforcement did and who overrode it. It is 0600 root and
the daemon refuses to start if its directory is world-writable, because a trail anyone can
edit is not evidence.
