package catalogdata

import _ "embed"

// YAML is the bundled macos-hotspots catalog, so the CLI works without a checkout.
//
//go:embed macos-hotspots.yaml
var YAML []byte
