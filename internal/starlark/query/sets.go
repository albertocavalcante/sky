package query

// Union returns items in a or b (set union).
func Union(a, b *Result) *Result {
	if a == nil {
		return b
	}
	if b == nil {
		return a
	}

	seen := make(map[string]bool)
	var items []Item

	// Add all items from a
	for _, item := range a.Items {
		key := item.key()
		if !seen[key] {
			seen[key] = true
			items = append(items, item)
		}
	}

	// Add items from b that aren't in a
	for _, item := range b.Items {
		key := item.key()
		if !seen[key] {
			seen[key] = true
			items = append(items, item)
		}
	}

	return &Result{Items: items}
}

// Difference returns items in a but not b (set difference).
func Difference(a, b *Result) *Result {
	if a == nil {
		return &Result{}
	}
	if b == nil {
		return a
	}

	// Build set of keys in b
	bKeys := make(map[string]bool)
	for _, item := range b.Items {
		bKeys[item.key()] = true
	}

	// Keep items from a that aren't in b
	var items []Item
	for _, item := range a.Items {
		if !bKeys[item.key()] {
			items = append(items, item)
		}
	}

	return &Result{Items: items}
}

// Intersection returns items in both a and b (set intersection).
func Intersection(a, b *Result) *Result {
	if a == nil || b == nil {
		return &Result{}
	}

	// Build set of keys in b
	bKeys := make(map[string]bool)
	for _, item := range b.Items {
		bKeys[item.key()] = true
	}

	// Keep items from a that are also in b
	var items []Item
	seen := make(map[string]bool)
	for _, item := range a.Items {
		key := item.key()
		if bKeys[key] && !seen[key] {
			seen[key] = true
			items = append(items, item)
		}
	}

	return &Result{Items: items}
}
