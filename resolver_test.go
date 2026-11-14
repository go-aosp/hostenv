// Copyright (c) the go-aosp authors. All rights reserved.
//
// The first three tests are ported from go-widgets/android's resolver_test.go
// ("Copyright (c) the go-widgets/android authors", same licence); the
// generation and Ready tests are new.
//
// SPDX-License-Identifier: BSD-3-Clause

package hostenv

import (
	"context"
	"errors"
	"net"
	"reflect"
	"testing"
)

// env builds a getenv from a map, so a test states exactly what the host set
// and nothing leaks in from the machine running the suite.
func env(m map[string]string) func(string) string {
	return func(k string) string { return m[k] }
}

func TestParseServers(t *testing.T) {
	for _, tc := range []struct {
		name string
		raw  string
		want []string
	}{
		{"empty", "", nil},
		{"blanks only", " , ,, ", nil},
		{"one", "8.8.8.8", []string{"8.8.8.8:53"}},
		{"several, spaced", " 8.8.8.8 , 1.1.1.1 ", []string{"8.8.8.8:53", "1.1.1.1:53"}},
		// An IPv6 literal must be bracketed before a port, which is the whole
		// reason this goes through net.JoinHostPort rather than a concatenation.
		{"ipv6", "fd00::1", []string{"[fd00::1]:53"}},
		{"mixed", "fd00::1,8.8.8.8", []string{"[fd00::1]:53", "8.8.8.8:53"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := parseServers(tc.raw); !reflect.DeepEqual(got, tc.want) {
				t.Fatalf("parseServers(%q) = %v, want %v", tc.raw, got, tc.want)
			}
		})
	}
}

// ⛔ ONE CASE PER GENERATION OF THE NAME. The published name and the neutral
// alias must BOTH work, for ever: a host already in the field sets the first,
// and a test covering only one generation goes green while the other silently
// stops being read — which is exactly how a rename turns into an outage here.
func TestBothGenerationsOfTheDNSVariableAreRead(t *testing.T) {
	for _, tc := range []struct {
		name string
		set  map[string]string
		want []string
	}{
		{"published name alone", map[string]string{EnvDNS: "8.8.8.8"}, []string{"8.8.8.8:53"}},
		{"neutral alias alone", map[string]string{EnvDNSAlias: "9.9.9.9"}, []string{"9.9.9.9:53"}},
		{
			"both set: the neutral one wins, and the other is not merged in",
			map[string]string{EnvDNSAlias: "9.9.9.9", EnvDNS: "8.8.8.8"},
			[]string{"9.9.9.9:53"},
		},
		{"neither", map[string]string{}, nil},
		{"alias present but empty falls through to the published name",
			map[string]string{EnvDNSAlias: " , ", EnvDNS: "1.1.1.1"},
			[]string{"1.1.1.1:53"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := DNSServers(env(tc.set)); !reflect.DeepEqual(got, tc.want) {
				t.Fatalf("DNSServers = %v, want %v", got, tc.want)
			}
		})
	}
}

// TestInstallResolverDeclinesWithoutServers pins the rule that matters most:
// off Android the default resolver is already correct, so a no-op must leave it
// exactly as it was rather than rewire it to nothing.
func TestInstallResolverDeclinesWithoutServers(t *testing.T) {
	before := net.DefaultResolver
	t.Cleanup(func() { net.DefaultResolver = before })

	for _, raw := range []string{"", " , "} {
		if installResolver(env(map[string]string{EnvDNS: raw})) {
			t.Fatalf("installResolver() = true for %q, want false", raw)
		}
		if net.DefaultResolver != before {
			t.Fatalf("the default resolver was replaced for %q", raw)
		}
	}
}

// TestInstallResolverDialsTheConfiguredServers proves the substitution: the
// address Go hands the Dial hook is ignored, and a configured server is used
// instead. Recording the network too pins that "tcp" is passed through rather
// than forced to "udp" -- the resolver asks for tcp to re-fetch a truncated
// answer, and overriding that would break long replies.
func TestInstallResolverDialsTheConfiguredServers(t *testing.T) {
	before, beforeDial := net.DefaultResolver, resolverDial
	t.Cleanup(func() { net.DefaultResolver, resolverDial = before, beforeDial })

	var gotNet, gotAddr []string
	sentinel := errors.New("dialed")
	resolverDial = func(_ context.Context, network, addr string) (net.Conn, error) {
		gotNet = append(gotNet, network)
		gotAddr = append(gotAddr, addr)
		return nil, sentinel
	}

	if !installResolver(env(map[string]string{EnvDNS: "8.8.8.8,1.1.1.1"})) {
		t.Fatal("installResolver() = false with two servers")
	}
	r := net.DefaultResolver
	if !r.PreferGo {
		t.Fatal("the installed resolver must PreferGo: the cgo path is what is missing")
	}
	for i, network := range []string{"udp", "udp", "tcp"} {
		if _, err := r.Dial(context.Background(), network, "127.0.0.1:53"); !errors.Is(err, sentinel) {
			t.Fatalf("dial %d: err = %v, want the sentinel", i, err)
		}
	}
	// Round-robin by attempt: server 1, server 2, then back to server 1.
	wantAddr := []string{"8.8.8.8:53", "1.1.1.1:53", "8.8.8.8:53"}
	if !reflect.DeepEqual(gotAddr, wantAddr) {
		t.Fatalf("dialed %v, want %v", gotAddr, wantAddr)
	}
	if wantNet := []string{"udp", "udp", "tcp"}; !reflect.DeepEqual(gotNet, wantNet) {
		t.Fatalf("networks %v, want %v", gotNet, wantNet)
	}
}

// InstallResolver is the exported wrapper over the tested seam; it reads the
// real environment, and this is what proves the two are wired together.
func TestInstallResolverReadsTheRealEnvironment(t *testing.T) {
	before := net.DefaultResolver
	t.Cleanup(func() { net.DefaultResolver = before })
	t.Setenv(EnvDNSAlias, "")
	t.Setenv(EnvDNS, "")
	if InstallResolver() {
		t.Fatal("InstallResolver() = true with both variables empty")
	}
	t.Setenv(EnvDNS, "8.8.8.8")
	if !InstallResolver() {
		t.Fatal("InstallResolver() = false although the published variable is set")
	}
}
