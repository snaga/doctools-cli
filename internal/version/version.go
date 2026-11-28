package version

// Version is the current version of doctools.
// This value is injected at build time via ldflags:
//
//	go build -ldflags="-X 'doctools-cli/internal/version.Version=0.8.0'" ./cmd/doctools-cli
var Version = "0.8.0"
