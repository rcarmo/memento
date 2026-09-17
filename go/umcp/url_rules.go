package umcp

import (
	"fmt"
	"net/netip"
	"regexp"
	"strconv"
	"strings"
	"unicode"

	"golang.org/x/text/unicode/norm"
)

var urlScheme = regexp.MustCompile(`^[A-Za-z][A-Za-z0-9+.-]*$`)
var ipvFuture = regexp.MustCompile(`^v[a-fA-F0-9]+\..+$`)

// RequestTarget is urllib.urlsplit's raw-string component representation.
// Percent escapes are preserved rather than decoded as by net/url.URL.Path.
type RequestTarget struct {
	Scheme   string
	Netloc   string
	Path     string
	Query    string
	Fragment string
}

// SplitRequestTarget mirrors the source URL preprocessing, bracket validation
// and NFKC delimiter checks. Invalid bracketed hosts return an error to the
// caller, as urlsplit raises before origin_is_allowed's port validation.
func SplitRequestTarget(target string) (RequestTarget, error) {
	target = strings.TrimLeftFunc(target, func(r rune) bool { return r <= 0x20 })
	target = strings.NewReplacer("\t", "", "\r", "", "\n", "").Replace(target)
	var out RequestTarget
	if i := strings.IndexByte(target, ':'); i > 0 && urlScheme.MatchString(target[:i]) {
		out.Scheme = strings.ToLower(target[:i])
		target = target[i+1:]
	}
	if strings.HasPrefix(target, "//") {
		target = target[2:]
		i := strings.IndexAny(target, "/?#")
		if i < 0 {
			i = len(target)
		}
		out.Netloc = target[:i]
		target = target[i:]
		if err := validateNetloc(out.Netloc); err != nil {
			return RequestTarget{}, err
		}
	}
	if i := strings.IndexByte(target, '#'); i >= 0 {
		out.Fragment = target[i+1:]
		target = target[:i]
	}
	if i := strings.IndexByte(target, '?'); i >= 0 {
		out.Query = target[i+1:]
		target = target[:i]
	}
	out.Path = target
	return out, nil
}

func validateNetloc(netloc string) error {
	hostinfo := netloc
	if i := strings.LastIndexByte(hostinfo, '@'); i >= 0 {
		hostinfo = hostinfo[i+1:]
	}
	left, right := strings.Contains(netloc, "["), strings.Contains(netloc, "]")
	if left != right {
		return fmt.Errorf("Invalid IPv6 URL")
	}
	if left && right {
		start := strings.IndexByte(hostinfo, '[')
		end := strings.IndexByte(hostinfo, ']')
		if start != 0 || end <= start || (end+1 < len(hostinfo) && hostinfo[end+1] != ':') {
			return fmt.Errorf("Invalid IPv6 URL")
		}
		host := hostinfo[start+1 : end]
		if strings.HasPrefix(host, "v") {
			if !ipvFuture.MatchString(host) {
				return fmt.Errorf("IPvFuture address is invalid")
			}
		} else {
			address, err := netip.ParseAddr(host)
			if err != nil {
				return fmt.Errorf("%s does not appear to be an IPv4 or IPv6 address", pythonRepr(host))
			}
			if address.Is4() {
				return fmt.Errorf("An IPv4 address cannot be in brackets")
			}
		}
	}
	n := strings.NewReplacer("@", "", ":", "", "#", "", "?", "").Replace(netloc)
	normalized := norm.NFKC.String(n)
	if n != normalized && strings.ContainsAny(normalized, "/?#@:") {
		return fmt.Errorf("netloc '%s' contains invalid characters under NFKC normalization", netloc)
	}
	return nil
}

// RequestTargetPath returns the raw path or '/', without decoding escapes.
func RequestTargetPath(target string) (string, error) {
	parts, err := SplitRequestTarget(target)
	if err != nil {
		return "", err
	}
	if parts.Path == "" {
		return "/", nil
	}
	return parts.Path, nil
}

// OriginIsAllowed keeps the pinned source's exact allowlist/authority checks.
// The error is a source urlparse failure; transports must reject it, not treat
// malformed input as an allowlist match.
func OriginIsAllowed(origin string, allowed []string, localBind bool, authority string) (bool, error) {
	if origin == "" {
		return false, nil
	}
	parts, err := SplitRequestTarget(origin)
	if err != nil {
		return false, err
	}
	if parts.Scheme != "http" && parts.Scheme != "https" {
		return false, nil
	}
	if strings.Contains(parts.Netloc, "@") {
		return false, nil
	}
	host, port := parts.Netloc, ""
	if strings.HasPrefix(host, "[") {
		end := strings.IndexByte(host, ']')
		host = host[1:end]
		_, port, _ = strings.Cut(parts.Netloc[end+1:], ":")
	} else {
		host, port, _ = strings.Cut(host, ":")
	}
	if host == "" || parts.Path != "" || parts.Query != "" || parts.Fragment != "" {
		return false, nil
	}
	if port != "" {
		for _, r := range port {
			if r < '0' || r > '9' {
				return false, nil
			}
		}
		p, err := strconv.ParseUint(strings.TrimLeft(port, "0"), 10, 64)
		if err != nil && strings.TrimLeft(port, "0") != "" {
			return false, nil
		}
		if p > 65535 {
			return false, nil
		}
	}
	for _, entry := range allowed {
		if origin == entry {
			return true, nil
		}
	}
	if authority != "" && origin == "http://"+authority {
		return true, nil
	}
	if len(allowed) > 0 || !localBind {
		return false, nil
	}
	host = strings.ToLower(host)
	return host == "127.0.0.1" || host == "localhost" || host == "::1", nil
}

func pythonRepr(s string) string {
	// Host parse errors use repr: prefer single quotes unless only they need escaping.
	quote := byte('\'')
	if strings.Contains(s, "'") && !strings.Contains(s, `"`) {
		quote = '"'
	}
	var b strings.Builder
	b.WriteByte(quote)
	for _, r := range s {
		switch r {
		case '\\':
			b.WriteString(`\\`)
		case '\n':
			b.WriteString(`\n`)
		case '\r':
			b.WriteString(`\r`)
		case '\t':
			b.WriteString(`\t`)
		default:
			if !unicode.IsPrint(r) {
				switch {
				case r < 256:
					fmt.Fprintf(&b, `\x%02x`, r)
				case r < 65536:
					fmt.Fprintf(&b, `\u%04x`, r)
				default:
					fmt.Fprintf(&b, `\U%08x`, r)
				}
			} else {
				if r == rune(quote) {
					b.WriteByte('\\')
				}
				b.WriteRune(r)
			}
		}
	}
	b.WriteByte(quote)
	return b.String()
}
