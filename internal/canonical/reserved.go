package canonical

// reservedRuleNames maps a reserved rule slug to why it is reserved. A reserved
// slug cannot be authored as a canonical rule because it collides with a tool's
// catch-all output filename. This map is the single source of truth for both
// the name set and the rationale.
var reservedRuleNames = map[string]string{
	"general": "collides with Cursor's catch-all .cursor/rules/general.mdc",
}

// IsReservedRuleName reports whether slug is a reserved rule filename.
func IsReservedRuleName(slug string) bool {
	_, ok := reservedRuleNames[slug]
	return ok
}

// ReservedRuleReason returns why slug is reserved, or "" if it is not reserved.
func ReservedRuleReason(slug string) string {
	return reservedRuleNames[slug]
}
