package model

import (
	"time"
)

// BuildVersionInfo is the version of a Bacalhau binary (either client or server)
type BuildVersionInfo struct {
	// Client Version: version.Info{Major:"1", Minor:"24", GitVersion:"v1.24.0",
	// GitCommit:"4ce5a8954017644c5420bae81d72b09b735c21f0", GitTreeState:"clean",
	// BuildDate:"2022-05-03T13:46:05Z", GoVersion:"go1.18.1", Compiler:"gc", Platform:"darwin/arm64"}

	Major      string    `json:"major,omitempty"`
	Minor      string    `json:"minor,omitempty"`
	GitVersion string    `json:"gitversion"`
	GitCommit  string    `json:"gitcommit"`
	BuildDate  time.Time `json:"builddate"`
	GOOS       string    `json:"goos"`
	GOARCH     string    `json:"goarch"`
}
