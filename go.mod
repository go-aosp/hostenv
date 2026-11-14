module github.com/go-aosp/hostenv

// ⛔ THE MINIMUM THIS PACKAGE NEEDS, NOT THE TOOLCHAIN IT WAS WRITTEN ON.
// `go mod init` wrote 1.27.4 because that is what was installed. The first
// consumer to add this module -- go-widgets/android, on 1.27.2 -- had its own
// `go` directive silently RAISED by `go mod tidy` as a side effect of taking a
// seventy-line stdlib dependency. A shared library that drags every consumer's
// toolchain forward for nothing is a library people vendor a copy of instead.
//
// 1.27.2 is the oldest version a consumer is known to be on, and CI runs
// exactly this line (go-version-file: go.mod), so what is declared is what is
// exercised. Nothing here needs more: atomic.Uint64, net.Resolver.Dial,
// net.JoinHostPort and t.Setenv are all far older.
go 1.27.2
