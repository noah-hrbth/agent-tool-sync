package tools

// All returns all built-in tools in a stable order. This slice is the single
// source of truth for adapter ordering and Names().
func All() []Tool {
	return []Tool{
		{Meta: claudeMeta, Render: renderClaude},
		{Meta: openCodeMeta, Render: renderOpenCode},
		{Meta: cursorMeta, Render: renderCursor},
		{Meta: geminiMeta, Render: renderGemini},
		{Meta: codexMeta, Render: renderCodex},
		{Meta: zedMeta, Render: renderZed},
		{Meta: clineMeta, Render: renderCline},
		{Meta: junieMeta, Render: renderJunie},
		{Meta: vibeMeta, Render: renderVibe},
		{Meta: copilotMeta, Render: renderCopilot},
		{Meta: piMeta, Render: renderPi},
	}
}

// Names returns the display name of each tool from All().
func Names() []string {
	all := All()
	names := make([]string, len(all))
	for i, t := range all {
		names[i] = t.Meta.Name
	}
	return names
}
