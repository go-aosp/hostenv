// Copyright (c) the go-aosp authors. All rights reserved.
//
// SPDX-License-Identifier: BSD-3-Clause

package hostenv

import (
	"fmt"
	"os"
	"strings"
)

// The directory variables a host fills, named once so three hosts and their
// payloads cannot drift apart on the spelling.
//
// ⛔ WHY A PAYLOAD CANNOT FIX THESE ITSELF, ONLY REPORT THEM. An Android app
// process inherits no home directory: os.UserHomeDir and os.UserConfigDir both
// fail outright, and the first real application packaged this way died on
// exactly that before drawing a pixel (go-widgets/android's README records it).
// Only the host knows the answers -- getFilesDir() is persistent, getCacheDir()
// is what the system reclaims under pressure -- so the host sets them and a
// payload's job is to notice early and say so, rather than fail later inside
// whatever library first asked for a home.
const (
	// EnvHome is the app's persistent directory: getFilesDir().
	EnvHome = "HOME"
	// EnvTmp is a scratch directory the system may reclaim: getCacheDir().
	EnvTmp = "TMPDIR"
	// EnvCache is where caches belong, so Android can delete them.
	EnvCache = "XDG_CACHE_HOME"
)

// Dirs is what the host provided. An empty field means the host did not set it.
type Dirs struct {
	Home, Tmp, Cache string
}

// ReadDirs reports the directory variables the host set.
func ReadDirs(getenv func(string) string) Dirs {
	return Dirs{
		Home:  strings.TrimSpace(getenv(EnvHome)),
		Tmp:   strings.TrimSpace(getenv(EnvTmp)),
		Cache: strings.TrimSpace(getenv(EnvCache)),
	}
}

// Check reports what a payload will not be able to do, before it tries.
//
// ⛔ IT NAMES THE CALL THAT WILL FAIL, NOT JUST THE VARIABLE. "HOME is unset"
// sends a reader looking for whoever should have set it; "HOME is unset, so
// os.UserHomeDir will fail" tells them which of their own lines is about to
// break. A missing HOME is the difference between a program that reports a
// configuration problem and one that dies in a dependency.
//
// Only Home is an error. A missing TMPDIR leaves os.TempDir answering /tmp,
// which exists on Android and is writable by the app, so it is a warning at
// most and not worth refusing a run over.
func (d Dirs) Check() error {
	if d.Home == "" {
		return fmt.Errorf("%s is unset: an Android app process inherits no home directory, so os.UserHomeDir and os.UserConfigDir will fail — the host must set %s to getFilesDir()",
			EnvHome, EnvHome)
	}
	return nil
}

// Missing lists the variables the host did not set, in declaration order, so a
// caller can log the whole picture rather than the first problem.
func (d Dirs) Missing() []string {
	var out []string
	for _, v := range []struct {
		name, val string
	}{
		{EnvHome, d.Home},
		{EnvTmp, d.Tmp},
		{EnvCache, d.Cache},
	} {
		if v.val == "" {
			out = append(out, v.name)
		}
	}
	return out
}

// Ready is the one call a payload makes at start-up: install the resolver if
// the host named servers, and refuse if the host left no home.
//
// It returns whether a resolver was installed, and an error when the
// environment cannot support the payload at all.
func Ready() (resolver bool, err error) { return ready(os.Getenv) }

func ready(getenv func(string) string) (bool, error) {
	if err := ReadDirs(getenv).Check(); err != nil {
		return false, err
	}
	return installResolver(getenv), nil
}
