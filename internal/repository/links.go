package repository

import (
	"fmt"
	"path"
	"regexp"
	"sort"
	"strings"

	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/text"
	"github.com/yuin/goldmark/util"
	"golang.org/x/net/idna"
)

type MarkdownLink struct {
	Href string `json:"href"`
	Text string `json:"text"`
	Line int    `json:"line"`
}
type RenameRewriteResult struct {
	Content string `json:"content"`
	Changed bool   `json:"changed"`
}

var uriScheme = regexp.MustCompile(`^[A-Za-z][A-Za-z0-9+.-]*:`)

func IsExternalLink(href string) bool {
	return strings.HasPrefix(href, "//") || uriScheme.MatchString(href)
}

// ResolveLinkPath canonicalizes an internal Markdown link against its source
// concept. Anchor-only links resolve to the source concept itself.
func ResolveLinkPath(sourcePath, href string) (string, bool) {
	if IsExternalLink(href) {
		return "", false
	}
	linkPath, _, _ := strings.Cut(href, "#")
	if linkPath == "" {
		return sourcePath, true
	}
	if strings.HasPrefix(linkPath, "/") {
		return path.Clean(linkPath), true
	}
	return path.Join(path.Dir(sourcePath), linkPath), true
}

// ResolveAssetLinkPath returns the manifest-relative target for links that can
// refer to a file in the source concept's accepted asset pack.
func ResolveAssetLinkPath(href string) (string, bool) {
	if IsExternalLink(href) {
		return "", false
	}
	linkPath, _, _ := strings.Cut(href, "#")
	if linkPath == "" || strings.HasPrefix(linkPath, "/") {
		return "", false
	}
	target := path.Clean(linkPath)
	if target == "." || target == ".." || strings.HasPrefix(target, "../") {
		return "", false
	}
	return strings.TrimPrefix(target, "./"), true
}

func markdownSource(content string) []byte {
	return []byte(strings.ReplaceAll(strings.ReplaceAll(strings.ReplaceAll(content, "\r\n", "\n"), "\r", "\n"), "\x00", "�"))
}
func decodedMarkdown(raw []byte) string {
	return string(util.ResolveEntityNames(util.ResolveNumericReferences(util.UnescapePunctuations(raw))))
}
func acceptedLink(href string) bool {
	lower := strings.ToLower(strings.TrimFunc(href, conceptSpace))
	if strings.HasPrefix(lower, "javascript:") || strings.HasPrefix(lower, "vbscript:") || strings.HasPrefix(lower, "file:") {
		return false
	}
	if strings.HasPrefix(lower, "data:") {
		for _, kind := range []string{"gif", "png", "jpeg", "webp"} {
			if strings.HasPrefix(lower, "data:image/"+kind+";") {
				return true
			}
		}
		return false
	}
	return true
}

func normaliseLink(href string) string {
	href = recodeLinkHost(href, false)
	const safe = ";/?:@&=+$,-_.!~*'()#"
	var out strings.Builder
	for i := 0; i < len(href); i++ {
		c := href[i]
		if c == '%' && i+2 < len(href) && hex(href[i+1]) && hex(href[i+2]) {
			out.WriteString(href[i : i+3])
			i += 2
			continue
		}
		if c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c >= '0' && c <= '9' || strings.IndexByte(safe, c) >= 0 {
			out.WriteByte(c)
		} else {
			fmt.Fprintf(&out, "%%%02X", c)
		}
	}
	return out.String()
}
func hex(c byte) bool { return c >= '0' && c <= '9' || c >= 'a' && c <= 'f' || c >= 'A' && c <= 'F' }
func recodeLinkHost(href string, decode bool) string {
	start := 0
	protocol := uriScheme.FindString(href)
	if protocol != "" {
		start = len(protocol)
		lower := strings.ToLower(protocol)
		if lower != "http:" && lower != "https:" && lower != "mailto:" {
			return href
		}
	}
	if strings.HasPrefix(href[start:], "//") {
		start += 2
	} else if !strings.EqualFold(protocol, "mailto:") {
		return href
	}
	end := len(href)
	if i := strings.IndexAny(href[start:], "/?#"); i >= 0 {
		end = start + i
	}
	authority := href[start:end]
	if at := strings.LastIndexByte(authority, '@'); at >= 0 {
		start += at + 1
		authority = authority[at+1:]
	}
	if strings.HasPrefix(authority, "[") {
		return href
	}
	if colon := strings.LastIndexByte(authority, ':'); colon >= 0 {
		end = start + colon
		authority = authority[:colon]
	}
	var host string
	var err error
	if decode {
		host, err = idna.Punycode.ToUnicode(authority)
	} else {
		host, err = idna.Punycode.ToASCII(authority)
	}
	if err != nil {
		return href
	}
	return href[:start] + host + href[end:]
}
func normaliseLinkText(value string) string {
	value = recodeLinkHost(value, true)
	const keep = ";/?:@&=+$,#%"
	var out strings.Builder
	for i := 0; i < len(value); {
		if value[i] == '%' && i+2 < len(value) && hex(value[i+1]) && hex(value[i+2]) {
			var b byte
			for _, c := range []byte(value[i+1 : i+3]) {
				b *= 16
				switch {
				case c <= '9':
					b += c - '0'
				case c <= 'F':
					b += c - 'A' + 10
				default:
					b += c - 'a' + 10
				}
			}
			if strings.IndexByte(keep, b) < 0 {
				out.WriteByte(b)
			} else {
				out.WriteString(strings.ToUpper(value[i : i+3]))
			}
			i += 3
			continue
		}
		out.WriteByte(value[i])
		i++
	}
	return strings.ToValidUTF8(out.String(), "�")
}

func linkLine(node ast.Node, source []byte) int {
	for parent := node.Parent(); parent != nil; parent = parent.Parent() {
		if parent.Type() == ast.TypeBlock && parent.Lines().Len() > 0 {
			return 1 + strings.Count(string(source[:parent.Lines().At(0).Start]), "\n")
		}
	}
	return 0
}
func inImage(node ast.Node) bool {
	for p := node.Parent(); p != nil; p = p.Parent() {
		if _, ok := p.(*ast.Image); ok {
			return true
		}
	}
	return false
}
func linkText(node ast.Node, source []byte) string {
	var out strings.Builder
	var visit func(ast.Node)
	visit = func(n ast.Node) {
		switch v := n.(type) {
		case *ast.CodeSpan, *ast.Image, *ast.RawHTML:
			return
		case *ast.Text:
			out.WriteString(decodedMarkdown(v.Value(source)))
			return
		case *ast.String:
			out.WriteString(decodedMarkdown(v.Value))
			return
		}
		for child := n.FirstChild(); child != nil; child = child.NextSibling() {
			visit(child)
		}
	}
	for child := node.FirstChild(); child != nil; child = child.NextSibling() {
		visit(child)
	}
	return out.String()
}
func parsedLinks(content string) ([]MarkdownLink, []string) {
	source := markdownSource(content)
	document := goldmark.New().Parser().Parse(text.NewReader(source))
	links := []MarkdownLink{}
	destinations := []string{}
	_ = ast.Walk(document, func(node ast.Node, entering bool) (ast.WalkStatus, error) {
		if !entering {
			return ast.WalkContinue, nil
		}
		switch n := node.(type) {
		case *ast.Link:
			// Both parsers retain nested autolinks, but the source extractor
			// has one current-href slot: the inner link replaces the outer.
			// Destination comparisons must still include both nodes.
			nested := false
			_ = ast.Walk(n, func(child ast.Node, enter bool) (ast.WalkStatus, error) {
				if _, ok := child.(*ast.AutoLink); ok && enter {
					nested = true
				}
				return ast.WalkContinue, nil
			})
			href := decodedMarkdown(n.Destination)
			if acceptedLink(href) {
				href = normaliseLink(href)
				destinations = append(destinations, href)
				if !nested && !inImage(n) {
					links = append(links, MarkdownLink{Href: href, Text: linkText(n, source), Line: linkLine(n, source)})
				}
			}
		case *ast.Image:
			href := decodedMarkdown(n.Destination)
			if acceptedLink(href) {
				destinations = append(destinations, normaliseLink(href))
			}
		case *ast.AutoLink:
			href := string(n.URL(source))
			if n.AutoLinkType == ast.AutoLinkEmail {
				href = "mailto:" + href
			}
			if acceptedLink(href) {
				destinations = append(destinations, normaliseLink(href))
				if !inImage(n) {
					links = append(links, MarkdownLink{Href: normaliseLink(href), Text: normaliseLinkText(string(n.Label(source))), Line: linkLine(n, source)})
				}
			}
		}
		return ast.WalkContinue, nil
	})
	return links, destinations
}

func ExtractStructuralLinks(content string) []MarkdownLink {
	links, _ := parsedLinks(content)
	return links
}
func RewriteLinksForRename(content, oldPath, newPath string) RenameRewriteResult {
	return RewriteLinksForRenameFrom(content, "", oldPath, newPath)
}

func RewriteLinksForRenameFrom(content, sourcePath, oldPath, newPath string) RenameRewriteResult {
	if oldPath == "" {
		return RenameRewriteResult{Content: content}
	}
	return rewriteLinkDestinations(content, func(href string) string {
		return rewriteHrefFrom(href, sourcePath, oldPath, newPath)
	})
}

// RebaseRelativeLinks preserves a moved concept's internal link targets.
func RebaseRelativeLinks(content, oldSourcePath, newSourcePath string, localResources ...map[string]bool) RenameRewriteResult {
	return rewriteLinkDestinations(content, func(href string) string {
		if resource, ok := ResolveAssetLinkPath(href); ok {
			for _, resources := range localResources {
				if resources[resource] {
					return href // Attached files move with their owning concept.
				}
			}
		}
		linkPath, anchor, hasAnchor := strings.Cut(href, "#")
		if linkPath == "" || strings.HasPrefix(linkPath, "/") || IsExternalLink(href) {
			return href
		}
		target, _ := ResolveLinkPath(oldSourcePath, href)
		replacement := relativeLinkPath(path.Dir(newSourcePath), target)
		if hasAnchor {
			return replacement + "#" + anchor
		}
		return replacement
	})
}

func rewriteLinkDestinations(content string, rewrite func(string) string) RenameRewriteResult {
	original := content
	_, destinations := parsedLinks(content)
	type replacement struct {
		start         int
		before, after string
	}
	replacements := []replacement{}
	changes := map[string]string{}
	for _, href := range destinations {
		if after := rewrite(href); after != href {
			changes[href] = after
		}
	}
	for before, after := range changes {
		forms := [][2]string{{before, after}}
		beforePath, beforeAnchor, beforeHasAnchor := strings.Cut(before, "#")
		afterPath, afterAnchor, afterHasAnchor := strings.Cut(after, "#")
		if beforeHasAnchor && afterHasAnchor && beforeAnchor == afterAnchor && beforePath != afterPath {
			// Replacing only the path preserves entity/escape spelling inside
			// the fragment; parsed destination checks still reject prose/code.
			forms = append(forms, [2]string{beforePath, afterPath})
		}
		// Goldmark normalizes spaces and other bytes in destinations. Search the
		// decoded form as well so angle-bracket destinations retain their source
		// spelling while the parsed href remains canonical.
		if rawBefore, rawAfter := normaliseLinkText(before), normaliseLinkText(after); rawBefore != before {
			forms = append(forms, [2]string{rawBefore, rawAfter})
		}
		for _, form := range forms {
			for start := 0; start < len(content); {
				index := strings.Index(content[start:], form[0])
				if index < 0 {
					break
				}
				position := start + index
				replacements = append(replacements, replacement{position, form[0], form[1]})
				start = position + len(form[0])
			}
		}
	}
	sort.Slice(replacements, func(i, j int) bool {
		if replacements[i].start == replacements[j].start {
			return len(replacements[i].before) > len(replacements[j].before)
		}
		return replacements[i].start > replacements[j].start
	})
	boundary := len(content)
	for _, item := range replacements {
		// Destinations may be prefixes of one another or have both encoded and
		// decoded source spellings. Never apply overlapping source edits twice.
		if item.start+len(item.before) > boundary {
			continue
		}
		candidate := content[:item.start] + item.after + content[item.start+len(item.before):]
		_, updated := parsedLinks(candidate)
		if len(updated) != len(destinations) {
			continue
		}
		differences := false
		valid := true
		for j, before := range destinations {
			if before != updated[j] {
				differences = true
				if rewrite(before) != updated[j] {
					valid = false
					break
				}
			}
		}
		if differences && valid {
			content = candidate
			destinations = updated
			boundary = item.start
		}
	}
	return RenameRewriteResult{Content: content, Changed: content != original}
}
func rewriteHrefFrom(href, sourcePath, oldPath, newPath string) string {
	linkPath, anchor, hasAnchor := strings.Cut(href, "#")
	resolved, internal := ResolveLinkPath(sourcePath, href)
	if !internal || resolved != oldPath || linkPath == "" {
		return href
	}
	replacement := newPath
	if !strings.HasPrefix(linkPath, "/") {
		replacement = relativeLinkPath(path.Dir(sourcePath), newPath)
	}
	if hasAnchor {
		return replacement + "#" + anchor
	}
	return replacement
}

func relativeLinkPath(fromDir, target string) string {
	fromDir = strings.Trim(fromDir, "/")
	if fromDir == "." {
		fromDir = ""
	}
	from := strings.Split(fromDir, "/")
	to := strings.Split(strings.Trim(target, "/"), "/")
	if len(from) == 1 && from[0] == "" {
		from = nil
	}
	if len(to) == 1 && to[0] == "" {
		to = nil
	}
	for len(from) > 0 && len(to) > 0 && from[0] == to[0] {
		from, to = from[1:], to[1:]
	}
	parts := make([]string, 0, len(from)+len(to))
	for range from {
		parts = append(parts, "..")
	}
	parts = append(parts, to...)
	if len(parts) == 0 {
		return "."
	}
	return strings.Join(parts, "/")
}
