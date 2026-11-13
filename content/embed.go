// Package content embeds the agent and topic definitions shipped with be.
package content

import "embed"

// FS holds agents/*.yaml and topics/*.yaml.
//
//go:embed agents/*.yaml topics/*.yaml
var FS embed.FS
