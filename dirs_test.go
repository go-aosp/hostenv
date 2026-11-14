// Copyright (c) the go-aosp authors. All rights reserved.
//
// SPDX-License-Identifier: BSD-3-Clause

package hostenv

import (
	"net"
	"reflect"
	"strings"
	"testing"
)

func TestReadDirsTrimsAndReportsWhatTheHostSet(t *testing.T) {
	d := ReadDirs(env(map[string]string{
		EnvHome:  " /data/user/0/org.example/files ",
		EnvTmp:   "/data/user/0/org.example/cache",
		EnvCache: "",
	}))
	if d.Home != "/data/user/0/org.example/files" {
		t.Errorf("Home = %q, want it trimmed", d.Home)
	}
	if d.Tmp == "" {
		t.Error("Tmp was set by the host and came back empty")
	}
	if d.Cache != "" {
		t.Errorf("Cache = %q, want empty: the host did not set it", d.Cache)
	}
}

// ⛔ THE ERROR MUST NAME THE CALL THAT WILL FAIL. A message that names only the
// variable sends the reader hunting for whoever should have set it; naming
// os.UserHomeDir tells them which of their own lines is about to break. This is
// the defect that killed the first packaged application before it drew a pixel.
func TestCheckRefusesAMissingHomeAndSaysWhatWillBreak(t *testing.T) {
	err := ReadDirs(env(map[string]string{})).Check()
	if err == nil {
		t.Fatal("a missing HOME was accepted")
	}
	for _, want := range []string{EnvHome, "os.UserHomeDir", "getFilesDir()"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("error %q does not mention %q", err, want)
		}
	}
}

// A missing TMPDIR is NOT an error: os.TempDir still answers /tmp, which exists
// on Android and is writable by the app, so refusing a run over it would be
// stricter than the platform.
func TestCheckAcceptsAMissingTmpAndCache(t *testing.T) {
	if err := ReadDirs(env(map[string]string{EnvHome: "/h"})).Check(); err != nil {
		t.Fatalf("Check() = %v, want nil: only HOME is fatal", err)
	}
}

func TestMissingListsEveryUnsetVariableInOrder(t *testing.T) {
	got := ReadDirs(env(map[string]string{EnvTmp: "/t"})).Missing()
	if want := []string{EnvHome, EnvCache}; !reflect.DeepEqual(got, want) {
		t.Fatalf("Missing() = %v, want %v", got, want)
	}
	if got := ReadDirs(env(map[string]string{EnvHome: "/h", EnvTmp: "/t", EnvCache: "/c"})).Missing(); got != nil {
		t.Fatalf("Missing() = %v, want nil when the host set all three", got)
	}
}

// Ready is the one call a payload makes, so the order matters: a missing home
// must be refused BEFORE a resolver is installed, or a process that cannot run
// has already had its global resolver replaced.
func TestReadyRefusesBeforeTouchingTheResolver(t *testing.T) {
	before := net.DefaultResolver
	t.Cleanup(func() { net.DefaultResolver = before })

	ok, err := ready(env(map[string]string{EnvDNS: "8.8.8.8"}))
	if err == nil {
		t.Fatal("Ready() accepted an environment with no HOME")
	}
	if ok {
		t.Error("Ready() reported a resolver although it refused")
	}
	if net.DefaultResolver != before {
		t.Error("the resolver was replaced in a run that was refused")
	}
}

func TestReadyInstallsTheResolverWhenTheEnvironmentIsWhole(t *testing.T) {
	before := net.DefaultResolver
	t.Cleanup(func() { net.DefaultResolver = before })

	ok, err := ready(env(map[string]string{EnvHome: "/h", EnvDNS: "8.8.8.8"}))
	if err != nil {
		t.Fatalf("Ready() = %v, want nil", err)
	}
	if !ok {
		t.Error("Ready() did not install a resolver although servers were named")
	}

	// And a host that gave a home but no servers is the ordinary non-network
	// application: no error, no resolver.
	net.DefaultResolver = before
	ok, err = ready(env(map[string]string{EnvHome: "/h"}))
	if err != nil {
		t.Fatalf("Ready() = %v, want nil", err)
	}
	if ok {
		t.Error("Ready() installed a resolver with no servers named")
	}
	if net.DefaultResolver != before {
		t.Error("the default resolver was replaced with no servers named")
	}
}

// Ready reads the real environment; this proves the exported wrapper is wired
// to the seam the tests above drive.
func TestReadyReadsTheRealEnvironment(t *testing.T) {
	before := net.DefaultResolver
	t.Cleanup(func() { net.DefaultResolver = before })
	t.Setenv(EnvHome, "")
	if _, err := Ready(); err == nil {
		t.Fatal("Ready() accepted an empty HOME from the real environment")
	}
	t.Setenv(EnvHome, "/h")
	t.Setenv(EnvDNS, "")
	t.Setenv(EnvDNSAlias, "")
	if ok, err := Ready(); err != nil || ok {
		t.Fatalf("Ready() = %v, %v; want false, nil", ok, err)
	}
}
