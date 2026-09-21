package gte

import (
	"encoding/binary"
	"errors"
	"io"
	"os"
)

// LoadTokenizer reads only the GTE1 header/vocabulary, never the model weights.
// The daemon can plan chunks without keeping the inference model resident.
func LoadTokenizer(path string) (*Tokenizer, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	return readTokenizer(io.LimitReader(file, 8<<20))
}

func readTokenizer(reader io.Reader) (*Tokenizer, error) {
	var header [28]byte
	if _, err := io.ReadFull(reader, header[:]); err != nil {
		return nil, err
	}
	if string(header[:4]) != "GTE1" {
		return nil, errors.New("invalid model magic")
	}
	count := binary.LittleEndian.Uint32(header[4:8])
	sequence := binary.LittleEndian.Uint32(header[24:28])
	if count <= TokenMASK || count > 1<<20 || sequence < 3 || sequence > 1<<20 {
		return nil, errors.New("invalid tokenizer dimensions")
	}
	words := make([]string, count)
	for i := range words {
		var size [2]byte
		if _, err := io.ReadFull(reader, size[:]); err != nil {
			return nil, err
		}
		word := make([]byte, binary.LittleEndian.Uint16(size[:]))
		if _, err := io.ReadFull(reader, word); err != nil {
			return nil, err
		}
		words[i] = string(word)
	}
	return NewTokenizer(words, int(sequence))
}

func (m *Model) Chunk(text string, tokens, overlap, chars int) ([]string, error) {
	return m.tokenizer.Chunk(text, tokens, overlap, chars)
}
