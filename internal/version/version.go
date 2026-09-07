package version

// Version is the CLI version string. Keep this a var (not a const) so
// releases can stamp it with:
//
//	-X github.com/CiprianSpiridon/free-disk-space/internal/version.Version={{.Version}}
//
// Default is for `go run` / builds without ldflags.
var Version = "0.1.0"
