package sentencepiece

import (
	"encoding/binary"
	"strings"
	"unicode/utf8"
)

const spaceSymbol = "▁"

type charsmap struct {
	units       []uint32
	replacement []byte
}

func decodeCharsmap(blob []byte) *charsmap {
	if len(blob) <= 4 {
		return nil
	}
	size := int(binary.LittleEndian.Uint32(blob))
	if size >= len(blob)-4 || size%4 != 0 {
		return nil
	}
	m := &charsmap{replacement: append([]byte(nil), blob[4+size:]...), units: make([]uint32, size/4)}
	for i := range m.units {
		m.units[i] = binary.LittleEndian.Uint32(blob[4+i*4:])
	}
	return m
}
func unitOffset(unit uint32) uint32 { return (unit >> 10) << ((unit & (1 << 9)) >> 6) }
func (m *charsmap) prefix(input []byte) ([]byte, int) {
	length, value := 0, 0
	if len(m.units) > 0 {
		position := uint64(unitOffset(m.units[0]))
		for i, b := range input {
			position ^= uint64(b)
			if position >= uint64(len(m.units)) {
				break
			}
			unit := m.units[position]
			if unit&(0x80000000|0xff) != uint32(b) {
				break
			}
			position ^= uint64(unitOffset(unit))
			if unit>>8&1 == 1 && position < uint64(len(m.units)) {
				length = i + 1
				value = int(m.units[position] & 0x7fffffff)
			}
		}
	}
	if length == 0 || value >= len(m.replacement) {
		_, n := utf8.DecodeRune(input)
		return input[:n], n
	}
	replacement := m.replacement[value:]
	for i, b := range replacement {
		if b == 0 {
			return replacement[:i], length
		}
	}
	return replacement, length
}
func (m *model) normalize(text string) string {
	space := " "
	if m.normalizer.escape {
		space = spaceSymbol
	}
	out := make([]byte, 0, len(text)*2)
	if !m.suffix && m.normalizer.dummy {
		out = append(out, space...)
	}
	previous := m.normalizer.removeExtra
	cm := decodeCharsmap(m.normalizer.charsmap)
	input := []byte(text)
	for len(input) > 0 {
		_, n := utf8.DecodeRune(input)
		replacement := input[:n]
		if cm != nil {
			replacement, n = cm.prefix(input)
		}
		for previous && len(replacement) > 0 && replacement[0] == ' ' {
			replacement = replacement[1:]
		}
		if len(replacement) > 0 {
			for _, b := range replacement {
				if b == ' ' {
					out = append(out, space...)
				} else {
					out = append(out, b)
				}
			}
			previous = replacement[len(replacement)-1] == ' '
		}
		input = input[n:]
		if !m.normalizer.removeExtra {
			previous = false
		}
	}
	if m.normalizer.removeExtra {
		for strings.HasSuffix(string(out), space) {
			out = out[:len(out)-len(space)]
		}
	}
	if m.suffix && m.normalizer.dummy {
		out = append(out, space...)
	}
	if !utf8.Valid(out) {
		return ""
	}
	return string(out)
}
