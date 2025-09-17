package detector

// A tiny stdlib-only Aho-Corasick automaton for byte strings.
// Inputs are normalized to lowercased UTF-8. We use a fixed 256-way
// transition table per node to avoid map lookups in hot paths

type acNode struct {
	// trans[b] = next state or -1 if absent
	trans  [256]int
	fail   int
	output []int // lemma IDs ending at this node
}

type acAutomaton struct {
	nodes []acNode
}

func newAutomaton() *acAutomaton {
	a := &acAutomaton{nodes: make([]acNode, 1)}
	// init root transitions to -1
	for i := range a.nodes[0].trans {
		a.nodes[0].trans[i] = -1
	}
	a.nodes[0].fail = 0
	return a
}

// AddPattern inserts a pattern and associates it with an integer ID
func (a *acAutomaton) AddPattern(pat []byte, id int) {
	if len(pat) == 0 {
		return
	}
	state := 0
	for _, b := range pat {
		nxt := a.nodes[state].trans[b]
		if nxt == -1 {
			nxt = len(a.nodes)
			a.nodes[state].trans[b] = nxt
			var n acNode
			for i := range n.trans {
				n.trans[i] = -1
			}
			a.nodes = append(a.nodes, n)
		}
		state = nxt
	}
	a.nodes[state].output = append(a.nodes[state].output, id)
}

// Build finalizes failure links using a simple queue
func (a *acAutomaton) Build() {
	// Initialize depth-1 fail links to root
	q := make([]int, 0, 64)
	for b := range 256 {
		s := a.nodes[0].trans[byte(b)]
		if s != -1 {
			a.nodes[s].fail = 0
			q = append(q, s)
		}
	}

	// BFS over trie
	for qi := 0; qi < len(q); qi++ {
		r := q[qi]
		for b := range 256 {
			s := a.nodes[r].trans[byte(b)]
			if s == -1 {
				continue
			}
			q = append(q, s)

			// compute fail(s)
			f := a.nodes[r].fail
			for f != 0 && a.nodes[f].trans[byte(b)] == -1 {
				f = a.nodes[f].fail
			}
			if nxt := a.nodes[f].trans[byte(b)]; nxt != -1 {
				a.nodes[s].fail = nxt
			} else {
				a.nodes[s].fail = 0
			}

			// merge output
			a.nodes[s].output = append(a.nodes[s].output, a.nodes[a.nodes[s].fail].output...)
		}
	}
}

// FindAll scans text and calls cb(endIndex, patternID) for each match.
// If cb returns false, scanning stops early
func (a *acAutomaton) FindAll(text []byte, cb func(end int, id int) bool) {
	state := 0
	for i, b := range text {
		// follow fail links while missing edge
		for state != 0 && a.nodes[state].trans[b] == -1 {
			state = a.nodes[state].fail
		}
		if nxt := a.nodes[state].trans[b]; nxt != -1 {
			state = nxt
		}
		if out := a.nodes[state].output; len(out) > 0 {
			for _, id := range out {
				if !cb(i+1, id) { // endIndex is i+1
					return
				}
			}
		}
	}
}
