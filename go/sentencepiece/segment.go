package sentencepiece

import "math"

type symbol struct{ start, length, previous, next int }
type pair struct {
	score             float32
	left, right, size int
}
type agenda []pair

func (a agenda) Len() int { return len(a) }
func (a agenda) Less(i, j int) bool {
	left, right := floatOrder(a[i].score), floatOrder(a[j].score)
	if left == right {
		return a[i].left < a[j].left
	}
	return left > right
}
func (a agenda) Swap(i, j int) { a[i], a[j] = a[j], a[i] }
func (a *agenda) push(value pair) {
	*a = append(*a, value)
	for child := len(*a) - 1; child > 0; {
		parent := (child - 1) / 2
		if !a.Less(child, parent) {
			break
		}
		a.Swap(parent, child)
		child = parent
	}
}
func (a *agenda) pop() pair {
	last := len(*a) - 1
	a.Swap(0, last)
	for parent := 0; ; {
		left := parent*2 + 1
		if left >= last {
			break
		}
		child := left
		if right := left + 1; right < last && a.Less(right, left) {
			child = right
		}
		if !a.Less(child, parent) {
			break
		}
		a.Swap(parent, child)
		parent = child
	}
	value := (*a)[last]
	*a = (*a)[:last]
	return value
}

// Rust f32::total_cmp order, including signed zero and NaNs.
func floatOrder(value float32) int32 {
	bits := int32(math.Float32bits(value))
	bits ^= int32(uint32(bits>>31) >> 1)
	return bits
}

func (p *Processor) bpe(text []byte) []span {
	symbols := []symbol{}
	for i := 0; i < len(text); {
		size := min(utf8Length(text[i]), len(text)-i)
		symbols = append(symbols, symbol{i, size, len(symbols) - 1, len(symbols) + 1})
		i += size
	}
	if len(symbols) == 0 {
		return []span{}
	}
	symbols[len(symbols)-1].next = -1
	queue := agenda{}
	add := func(left, right int) {
		if left < 0 || right < 0 {
			return
		}
		start := symbols[left].start
		size := symbols[left].length + symbols[right].length
		if id, ok := p.ids[string(text[start:start+size])]; ok && id != p.model.unk {
			queue.push(pair{p.model.pieces[id].Score, left, right, size})
		}
	}
	for i := 0; i < len(symbols)-1; i++ {
		add(i, i+1)
	}
	for len(queue) > 0 {
		top := queue.pop()
		left, right := &symbols[top.left], &symbols[top.right]
		if left.length == 0 || right.length == 0 {
			continue
		}
		if left.length+right.length != top.size || left.next != top.right {
			continue
		}
		left.length += right.length
		left.next = right.next
		if right.next >= 0 {
			symbols[right.next].previous = top.left
		}
		right.length = 0
		add(left.previous, top.left)
		add(top.left, left.next)
	}
	out := []span{}
	for i := 0; i >= 0; i = symbols[i].next {
		s := symbols[i]
		id, ok := p.ids[string(text[s.start:s.start+s.length])]
		if !ok {
			id = p.model.unk
		}
		out = append(out, span{s.start, s.length, id})
	}
	return out
}

type trieNode struct {
	children map[byte]int
	id       int
}

func (p *Processor) buildTrie() {
	p.trie = []trieNode{{children: make(map[byte]int), id: -1}}
	for id, piece := range p.model.pieces {
		if piece.Kind != Normal && piece.Kind != UserDefined {
			continue
		}
		node := 0
		for _, b := range []byte(piece.Text) {
			next, ok := p.trie[node].children[b]
			if !ok {
				next = len(p.trie)
				p.trie = append(p.trie, trieNode{children: make(map[byte]int), id: -1})
				p.trie[node].children[b] = next
			}
			node = next
		}
		p.trie[node].id = id
	}
}

type bestNode struct {
	id    int
	score float32
	start int
}

func (p *Processor) unigram(text []byte) []span {
	if len(text) == 0 {
		return []span{}
	}
	best := make([]bestNode, len(text)+1)
	for i := range best {
		best[i].id = -1
		best[i].start = -1
	}
	relax := func(start, length, id int, score float32) {
		candidate := float32(score + best[start].score)
		target := &best[start+length]
		if target.start < 0 || candidate > target.score {
			*target = bestNode{id, candidate, start}
		}
	}
	for start := 0; start < len(text); {
		size := min(utf8Length(text[start]), len(text)-start)
		single := false
		node := 0
		for i, b := range text[start:] {
			next, ok := p.trie[node].children[b]
			if !ok {
				break
			}
			node = next
			id := p.trie[node].id
			if id >= 0 {
				length := i + 1
				score := p.model.pieces[id].Score
				if p.model.pieces[id].Kind == UserDefined {
					score = float32(0.1 * (float64(length) - 1))
				}
				relax(start, length, id, score)
				if length == size {
					single = true
				}
			}
		}
		if !single {
			relax(start, size, p.model.unk, float32(p.minScore-10))
		}
		start += size
	}
	out := []span{}
	for end := len(text); end > 0; {
		n := best[end]
		out = append(out, span{n.start, end - n.start, n.id})
		end = n.start
	}
	for left, right := 0, len(out)-1; left < right; left, right = left+1, right-1 {
		out[left], out[right] = out[right], out[left]
	}
	return out
}
