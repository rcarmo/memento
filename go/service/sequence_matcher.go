package service

import "sort"

type matchBlock struct{ i, j, n int }

func sequenceRatio(aText, bText string) float64 {
	a, b := []rune(aText), []rune(bText)
	if len(a)+len(b) == 0 {
		return 1
	}
	b2j := map[rune][]int{}
	for j, value := range b {
		b2j[value] = append(b2j[value], j)
	}
	if len(b) >= 200 {
		threshold := len(b)/100 + 1
		for value, indexes := range b2j {
			if len(indexes) > threshold {
				delete(b2j, value)
			}
		}
	}
	queue := [][4]int{{0, len(a), 0, len(b)}}
	blocks := []matchBlock{}
	for len(queue) > 0 {
		region := queue[len(queue)-1]
		queue = queue[:len(queue)-1]
		block := longestSequenceMatch(a, b, b2j, region[0], region[1], region[2], region[3])
		if block.n == 0 {
			continue
		}
		blocks = append(blocks, block)
		if region[0] < block.i && region[2] < block.j {
			queue = append(queue, [4]int{region[0], block.i, region[2], block.j})
		}
		if block.i+block.n < region[1] && block.j+block.n < region[3] {
			queue = append(queue, [4]int{block.i + block.n, region[1], block.j + block.n, region[3]})
		}
	}
	merged := mergeMatchBlocks(blocks)
	matches := 0
	for _, block := range merged {
		matches += block.n
	}
	return 2 * float64(matches) / float64(len(a)+len(b))
}
func mergeMatchBlocks(blocks []matchBlock) []matchBlock {
	sort.Slice(blocks, func(i, j int) bool {
		if blocks[i].i != blocks[j].i {
			return blocks[i].i < blocks[j].i
		}
		return blocks[i].j < blocks[j].j
	})
	merged := []matchBlock{}
	for _, block := range blocks {
		if len(merged) > 0 {
			last := &merged[len(merged)-1]
			if last.i+last.n == block.i && last.j+last.n == block.j {
				last.n += block.n
				continue
			}
		}
		merged = append(merged, block)
	}
	return merged
}
func longestSequenceMatch(a, b []rune, b2j map[rune][]int, alo, ahi, blo, bhi int) matchBlock {
	best := matchBlock{alo, blo, 0}
	previous := map[int]int{}
	for i := alo; i < ahi; i++ {
		current := map[int]int{}
		for _, j := range b2j[a[i]] {
			if j < blo {
				continue
			}
			if j >= bhi {
				break
			}
			size := previous[j-1] + 1
			current[j] = size
			if size > best.n {
				best = matchBlock{i - size + 1, j - size + 1, size}
			}
		}
		previous = current
	}
	for best.i > alo && best.j > blo && a[best.i-1] == b[best.j-1] {
		best.i--
		best.j--
		best.n++
	}
	for best.i+best.n < ahi && best.j+best.n < bhi && a[best.i+best.n] == b[best.j+best.n] {
		best.n++
	}
	return best
}
