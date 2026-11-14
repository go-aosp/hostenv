# hostenv

What an Android Java host hands a **CGO-free Go payload**: the DNS servers,
`HOME`, `TMPDIR`, `XDG_CACHE_HOME` — named once, in a module a program can
import without a GUI toolkit.

```go
resolver, err := hostenv.Ready()
if err != nil {
    log.Fatal(err) // the host left no HOME; nothing below will work
}
log.Printf("dns installed: %v", resolver)
```

## Why this exists as its own module

Android gives a spawned process almost nothing a Unix program expects, and two
of those gaps stop real programs dead:

- **No `/etc/resolv.conf`.** Android's resolver lives behind libc, which is
  behind cgo — the one thing a CGO-free payload does not have. Go's pure
  resolver finds no nameserver, falls back to `127.0.0.1:53`, and every lookup
  dies with a connection refused. Measured on a Pixel 11 Pro Fold, Android 17:

  ```
  /etc/resolv.conf   does not exist
  getprop net.dns1   empty
  ping 1.1.1.1       64 bytes, 9.19 ms      <- IP connectivity is fine
  ```

  So the network works and names do not. A host can see the real servers
  through `ConnectivityManager`, so it passes them in the environment and
  [InstallResolver] points `net.DefaultResolver` at them.

- **No `HOME`.** `os.UserHomeDir` and `os.UserConfigDir` both fail outright.
  Only the host knows the answer — `getFilesDir()` is persistent,
  `getCacheDir()` is what the system reclaims under pressure — so the host sets
  it and a payload's job is to notice early. [Dirs.Check] names the call that
  will fail, not just the variable.

The mechanism is **not new**: it was built and proven in
[go-widgets/android](https://github.com/go-widgets/android), on a real
installable APK, and `resolver.go` here is ported from it with its attribution
intact. What is new is only the layer. That package is a widget back-end
carrying a socket protocol, a client, an a11y bridge and twelve indirect
dependencies (gfx, iconoir, opentype, crdt…); a package manager or a daemon
cannot depend on it for seventy lines of stdlib. This module imports **nothing
but the standard library**.

## The contract

A host sets these when it spawns the payload. A payload reads them through this
package rather than by name, so three hosts cannot drift apart on a spelling.

| Variable | Set by the host to | Required |
| --- | --- | --- |
| `GW_ANDROID_DNS` | the active network's DNS servers, comma separated | for any lookup |
| `AOSP_DNS` | the same, vendor-neutral spelling; read **first** | alternative to the above |
| `HOME` | `getFilesDir()` — persistent | **yes** |
| `TMPDIR` | `getCacheDir()` — reclaimable | no (`os.TempDir` answers `/tmp`) |
| `XDG_CACHE_HOME` | `getCacheDir()` | no |

Reading the DNS servers needs `ACCESS_NETWORK_STATE`, so a host leaves them
unset for an application that does not want the network — and then
[InstallResolver] declines and changes nothing, rather than rewiring the
default resolver to an empty list.

### ⛔ Why `GW_ANDROID_DNS` keeps its prefix

It is a **published identity**: go-widgets/android's host already sets it, and
so does anything built on that host. Renaming it would not replace a spelling,
it would add a generation that every host and every payload must then
understand. `AOSP_DNS` exists for a new host that prefers a neutral name, is
read first, and **neither name will be removed**. The suite carries a case per
generation, because a test that covers one of them goes green while the other
silently stops being read.

## Who this is for

Anything in the fleet that runs as a CGO-free Go payload under an Android host:
[go-widgets/android](https://github.com/go-widgets/android) (widgets),
[go-xrkit/android](https://github.com/go-xrkit/android) (screen capture),
[go-sicp/androidsvc](https://github.com/go-sicp/androidsvc) (foreground
services), and `go-zpkg`'s installer, which is what prompted the extraction —
it hit both gaps on a real phone and was one commit away from writing a second
resolver.

## Building a payload

`android/arm64` is the only Android target Go links CGO-free:

```sh
GOOS=android GOARCH=arm64 CGO_ENABLED=0 go build -o /tmp/payload ./cmd/payload
```

go-widgets/android's `host/build.sh` packages any such binary into an APK with
`APP_BIN=`, and sets the whole contract above.
