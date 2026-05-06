package tools

// All returns all built-in adapters in a stable order.
func All() []Adapter {
	return []Adapter{
		&claudeAdapter{},
		&openCodeAdapter{},
		&cursorAdapter{},
		&geminiAdapter{},
		&codexAdapter{},
		&zedAdapter{},
		&clineAdapter{},
		&junieAdapter{},
	}
}

// Names returns the Name() of each adapter from All().
func Names() []string {
	all := All()
	names := make([]string, len(all))
	for i, a := range all {
		names[i] = a.Name()
	}
	return names
}
