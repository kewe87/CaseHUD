package app

// markMatched declares pumps proven, as the start of a check that passed.
func (b *boardPumps) markMatched(ids ...string) {
	for _, id := range ids {
		b.setState(id, pumpMatched)
	}
}
