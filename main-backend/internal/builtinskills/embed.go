// Package builtinskills exposes the curated skills that ship inside Rescene.
package builtinskills

import "embed"

// Files contains the reusable, runtime-independent factory skills.
//
//go:embed *.json
var Files embed.FS
