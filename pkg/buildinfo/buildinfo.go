// Package buildinfo reports which build of the application is running.
//
// Values are stamped in at link time by deploy.sh. When they are absent -- a
// plain `go build`, or `go run` during development -- it falls back to the VCS
// data the Go toolchain embeds automatically, so the footer still identifies
// the commit rather than showing nothing useful.
package buildinfo

import (
	"runtime/debug"
	"sync"
)

// Stamped at link time, e.g.
//
//	-X github.com/botginx/botginx/pkg/buildinfo.version=v2.34.3
//	-X github.com/botginx/botginx/pkg/buildinfo.build=247
var (
	version string
	build   string
)

// defaultVersion is used when nothing was stamped and there are no tags to
// describe. Keep it in step with the release the branch is working toward.
const defaultVersion = "v0.1.0"

var (
	once            sync.Once
	resolvedVersion string
	resolvedBuild   string
)

func resolve() {
	resolvedVersion = version
	resolvedBuild = build

	if resolvedBuild == "" {
		resolvedBuild = vcsBuild()
	}
	if resolvedVersion == "" {
		resolvedVersion = defaultVersion
	}
	if resolvedBuild == "" {
		resolvedBuild = "dev"
	}
}

// vcsBuild derives a build identifier from the VCS data Go stamps into any
// binary built inside a repository.
func vcsBuild() string {
	info, ok := debug.ReadBuildInfo()
	if !ok {
		return ""
	}

	var revision string
	var dirty bool

	for _, setting := range info.Settings {
		switch setting.Key {
		case "vcs.revision":
			revision = setting.Value
		case "vcs.modified":
			dirty = setting.Value == "true"
		}
	}

	if revision == "" {
		return ""
	}
	if len(revision) > 7 {
		revision = revision[:7]
	}
	if dirty {
		// Uncommitted changes mean the commit alone does not identify this
		// binary, so say so rather than implying a clean build.
		return revision + "-dirty"
	}
	return revision
}

// Version returns the release version, e.g. "v2.34.3".
func Version() string {
	once.Do(resolve)
	return resolvedVersion
}

// Build returns the build identifier: a commit count when stamped by deploy.sh,
// otherwise the short commit hash.
func Build() string {
	once.Do(resolve)
	return resolvedBuild
}
