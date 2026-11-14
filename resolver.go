// Copyright (c) the go-aosp authors. All rights reserved.
//
// Ported from go-widgets/android's resolver.go, which carries
// "Copyright (c) the go-widgets/android authors" under the same licence. The
// mechanism, the round-robin and the GW_ANDROID_DNS name are theirs; what is
// new here is only the move to a module a non-GUI program can import.
//
// SPDX-License-Identifier: BSD-3-Clause

package hostenv

import (
	"context"
	"net"
	"os"
	"strings"
	"sync/atomic"
	"time"
)

// EnvDNS names the environment variable a Java host sets to the system's DNS
// servers, comma separated, when it spawns the payload.
//
// Android has no /etc/resolv.conf, and its resolver lives behind libc, which is
// behind cgo -- the one thing a CGO-free payload does not have. Go's pure
// resolver therefore finds no nameserver, falls back to localhost, and every
// lookup fails with a connection refused. A host can see the real servers
// through ConnectivityManager, so it passes them here.
//
// ⛔ THE NAME CARRIES A `GW_` PREFIX ON PURPOSE, AND IT IS NOT A MISTAKE TO
// TIDY. It is a PUBLISHED identity: go-widgets/android's host already sets it,
// and so does anything built on that host. Renaming it would not replace a
// spelling, it would ADD a generation that every host and every payload then
// has to understand -- the failure this fleet has recorded four times in one
// day. So this stays canonical for ever, and [EnvDNSAlias] is accepted beside
// it for a host that prefers a vendor-neutral name.
const EnvDNS = "GW_ANDROID_DNS"

// EnvDNSAlias is a vendor-neutral spelling a NEW host may set instead.
//
// It is read FIRST, so a host that sets both wins with the neutral one; but
// neither name will ever be removed, and a payload must not care which it got.
const EnvDNSAlias = "AOSP_DNS"

// resolverDial is the dialer InstallResolver uses, replaceable in tests.
var resolverDial = (&net.Dialer{Timeout: 5 * time.Second}).DialContext

// DNSServers reports the servers the host named, in dial order, or nil.
//
// It reads [EnvDNSAlias] before [EnvDNS]: see the note on the constants about
// why both exist and why neither goes away.
func DNSServers(getenv func(string) string) []string {
	if s := parseServers(getenv(EnvDNSAlias)); len(s) > 0 {
		return s
	}
	return parseServers(getenv(EnvDNS))
}

// InstallResolver points net.DefaultResolver at the DNS servers the host named,
// and reports whether it did.
//
// It returns false, changing nothing, when no variable is set or none lists a
// usable server: off Android, or in a package whose application never asked for
// ACCESS_NETWORK_STATE, the default resolver is either already correct or
// deliberately absent, and silently rewiring it would be worse than leaving it.
//
// ⛔ IT REPLACES net.DefaultResolver RATHER THAN BUILDING A CLIENT. Every
// net.Dial, every http.Client and every library in the process then resolves,
// including the ones the payload does not own. A per-client resolver fixes
// whichever call site its author remembered, which on a package manager pulling
// from a registry, a pantry and a mirror is the wrong number of places.
//
// Dial is called with the address Go picked from its (empty) configuration and
// ignores it, substituting a configured server chosen by attempt: the first is
// tried first, and a retry moves to the next, so a dead server does not pin
// every lookup to it. Each server keeps the network Go asked for -- "udp" or
// "tcp", the latter when a truncated answer must be re-fetched -- because the
// choice is the resolver's, not ours.
func InstallResolver() bool { return installResolver(os.Getenv) }

func installResolver(getenv func(string) string) bool {
	servers := DNSServers(getenv)
	if len(servers) == 0 {
		return false
	}
	var attempt atomic.Uint64
	net.DefaultResolver = &net.Resolver{
		PreferGo: true,
		Dial: func(ctx context.Context, network, _ string) (net.Conn, error) {
			n := attempt.Add(1) - 1
			return resolverDial(ctx, network, servers[n%uint64(len(servers))])
		},
	}
	return true
}

// parseServers splits a host's comma-separated list into dialable addresses,
// dropping empties and bracketing IPv6 literals, which need it before a port.
func parseServers(raw string) []string {
	var out []string
	for _, s := range strings.Split(raw, ",") {
		s = strings.TrimSpace(s)
		if s == "" {
			continue
		}
		out = append(out, net.JoinHostPort(s, "53"))
	}
	return out
}
