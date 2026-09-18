package graphdebug

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"html"
	"math"
	"strconv"
	"strings"

	"github.com/rcarmo/memento/go/internal/pyjson"
)

func ExportJSON(nodes []Node, edges []Edge, revisions Revisions, settings map[string]any) ([]byte, error) {
	if nodes == nil {
		nodes = []Node{}
	}
	if edges == nil {
		edges = []Edge{}
	}
	if settings == nil {
		settings = map[string]any{}
	}
	raw, err := json.Marshal(map[string]any{"schema_version": 1, "revisions": revisions, "nodes": nodes, "edges": edges, "settings": settings})
	if err != nil {
		return nil, err
	}
	value, _ := pyjson.Parse(string(raw))
	for _, item := range value.(map[string]any)["nodes"].([]any) {
		node := item.(map[string]any)
		embedding := node["embedding"].(map[string]any)
		delete(embedding, "error")
		position := node["coarse_position"].(map[string]any)
		for _, axis := range []string{"x", "y", "z"} {
			number := position[axis].(json.Number)
			value, _ := number.Float64()
			position[axis] = json.Number(strconv.FormatFloat(value, 'f', 1, 64))
			if value != math.Trunc(value) {
				position[axis] = number
			}
		}
	}
	encoded, err := pyjson.DumpsCompact(value)
	if err != nil {
		return nil, err
	}
	lower := strings.ToLower(encoded)
	for _, forbidden := range []string{"embedding_blob", "bearer", "authorization", "token_env"} {
		if strings.Contains(lower, forbidden) {
			return nil, errors.New("graph export contains forbidden fields")
		}
	}
	return []byte(encoded), nil
}
func ExportSVG(nodes []Node, edges []Edge, width, height int) []byte {
	byID := map[string]Node{}
	for _, node := range nodes {
		byID[node.ID] = node
	}
	scale := float64(min(width, height)) * .09
	centerX, centerY := float64(width)/2, float64(height)/2
	point := func(node Node) (float64, float64) {
		return centerX + node.CoarsePosition.X*scale, centerY - node.CoarsePosition.Y*scale
	}
	var out bytes.Buffer
	fmt.Fprintf(&out, `<svg xmlns="http://www.w3.org/2000/svg" width="%d" height="%d" viewBox="0 0 %d %d">`, width, height, width, height)
	out.WriteString(`<style>text{font:12px system-ui,sans-serif;fill:#1a2a40}.edge{fill:none;stroke:#7090b0;stroke-opacity:.55}.node{fill:#2b6cb0;stroke:#1a2a40}</style><rect width="100%" height="100%" fill="#e8eff6"/>`)
	for _, edge := range edges {
		source, sok := byID[edge.Source]
		if edge.Target == nil {
			continue
		}
		target, tok := byID[*edge.Target]
		if !sok || !tok {
			continue
		}
		x1, y1 := point(source)
		x2, y2 := point(target)
		cx, cy := (x1+x2)/2, (y1+y2)/2-math.Min(120, math.Abs(x2-x1)*.18+20)
		fmt.Fprintf(&out, `<path class="edge" d="M%.2f,%.2f Q%.2f,%.2f %.2f,%.2f"/>`, x1, y1, cx, cy, x2, y2)
	}
	for _, node := range nodes {
		x, y := point(node)
		radius := math.Max(4, math.Min(24, 4+float64(len(integerText(int64(max(1, node.CombinedBytes)))))*1.4))
		fmt.Fprintf(&out, `<circle class="node" cx="%.2f" cy="%.2f" r="%.2f"/>`, x, y, radius)
		fmt.Fprintf(&out, `<text x="%.2f" y="%.2f">%s</text>`, x+radius+4, y+4, html.EscapeString(node.Title))
	}
	out.WriteString(`</svg>`)
	return out.Bytes()
}
