package skilldata

import _ "embed"

// Markdown is skills/freedisk/SKILL.md (own folder so skills.sh can index it).
//
//go:embed freedisk/SKILL.md
var Markdown []byte
