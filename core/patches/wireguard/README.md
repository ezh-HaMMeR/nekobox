# Windows AWG UDP binding

Windows returns WSAEINVAL (10022) for IPV6_UNICAST_IF when IPv6 is disabled
on the selected adapter. Legacy AWG asks the dialer for an unspecified IPv4
destination; the dialer opens a dual-stack socket and tries IPv6 interface
binding. This reproduces `create ipv4 connection: listen udp :0` on Ethernet
index 17 with IPv6 disabled.

The currently pinned extended core uses WireGuard endpoints with nested
`amnezia` options, rather than the legacy `awg` endpoint. The GUI now emits
that schema and still imports both legacy and new JSON formats. Stored
profiles and AWG obfuscation parameters remain in their existing format.

The new WireGuard Windows bind also requires IPv6 by default. The overlay
maps WSAEINVAL **only from the IPv6 interface control** to EAFNOSUPPORT and
falls back from WinRingBind to StdNetBind. IPv4 interface protection stays
enabled, other errors propagate, and the actual ephemeral IPv4 port is
preserved if IPv6 is unavailable. Healthy dual-stack connections retain
WinRingBind. This affects WireGuard and AWG, only on Windows.

CMake applies this overlay for both module and vendor builds. The module
cache is never edited: module builds copy the pinned WireGuard source to
the build directory and use a build-local modfile. Building with raw
`go build` requires supplying that modfile and overlay explicitly.

Run on Windows with the Go version required by `core/server/go.mod`:

```powershell
./core/patches/wireguard/test.ps1
```

Tests cover UDP send/receive, a nonzero ephemeral port, repeated Open and
Close, real IPv4-only adapters (when present), healthy dual-stack binds,
and propagation of IPv4 control and other IPv6 errors. No VPN credentials
or external traffic are needed.

Validation on 2026-10-07: all five tests passed, including physical Ethernet
index 17. The project core built with Go 1.27.1. A synthetic AWG endpoint
with auto_detect_interface enabled reached `udp bind has been updated`
and interface state Up, where the unpatched core reported WSAEINVAL.
These checks verify local socket startup, not a handshake with a VPN server.
The GUI must also be rebuilt; the old 5.11.28.3 GUI emits the old core schema.
