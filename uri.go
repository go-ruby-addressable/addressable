// Copyright (c) the go-ruby-addressable/addressable authors
//
// SPDX-License-Identifier: BSD-3-Clause

// Package addressable is a pure-Go (no cgo) reimplementation of the Ruby
// `addressable` gem: RFC 3986 URI parsing/normalization/reference-resolution and
// RFC 6570 URI Templates (all four levels), byte-exact to the gem.
package addressable

import (
	"sort"
	"strconv"
	"strings"
)

// URI is a parsed, mutable URI, mirroring Addressable::URI. A nil-valued component
// is represented by an empty *string (Go's zero *string is nil = "component
// absent"); the path is always a plain string (never nil, matching the gem, which
// reports "" rather than nil for a missing path).
type URI struct {
	scheme   *string
	userinfo *string
	host     *string
	port     *int
	path     string
	query    *string
	fragment *string
}

func strp(s string) *string { return &s }
func intp(i int) *int       { return &i }

// defaultPorts maps a lowercased scheme to its default port, used by normalization
// and Origin.
var defaultPorts = map[string]int{
	"http":     80,
	"https":    443,
	"ftp":      21,
	"tftp":     69,
	"sftp":     22,
	"ssh":      22,
	"svn+ssh":  22,
	"telnet":   23,
	"nntp":     119,
	"gopher":   70,
	"wais":     210,
	"ldap":     389,
	"prospero": 1525,
	"ws":       80,
	"wss":      443,
}

// splitURI splits a URI reference into its five RFC 3986 components using the
// reference-implementation regexp from Appendix B, adapted to explicit scanning so
// the pure-Go build carries no regexp dependency for the hot path.
//
// It returns (scheme, hasScheme, authority, hasAuthority, path, query, hasQuery,
// fragment, hasFragment).
func splitURI(uri string) (scheme string, hasScheme bool, authority string, hasAuthority bool, path string, query string, hasQuery bool, fragment string, hasFragment bool) {
	rest := uri

	// fragment
	if i := strings.IndexByte(rest, '#'); i >= 0 {
		fragment = rest[i+1:]
		hasFragment = true
		rest = rest[:i]
	}
	// query
	if i := strings.IndexByte(rest, '?'); i >= 0 {
		query = rest[i+1:]
		hasQuery = true
		rest = rest[:i]
	}
	// scheme: ALPHA *( ALPHA / DIGIT / "+" / "-" / "." ) ":" before any "/"
	if i := strings.IndexByte(rest, ':'); i > 0 && validScheme(rest[:i]) {
		// A ':' only introduces a scheme when it precedes the first '/' (so that
		// "foo/bar:baz" is a relative path, not a scheme).
		slash := strings.IndexByte(rest, '/')
		if slash < 0 || i < slash {
			scheme = rest[:i]
			hasScheme = true
			rest = rest[i+1:]
		}
	}
	// authority
	if strings.HasPrefix(rest, "//") {
		rest = rest[2:]
		end := len(rest)
		if j := strings.IndexAny(rest, "/"); j >= 0 {
			end = j
		}
		authority = rest[:end]
		hasAuthority = true
		path = rest[end:]
	} else {
		path = rest
	}
	return
}

func validScheme(s string) bool {
	if s == "" {
		return false
	}
	c := s[0]
	if !((c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z')) {
		return false
	}
	for i := 1; i < len(s); i++ {
		c := s[i]
		if !((c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9') || c == '+' || c == '-' || c == '.') {
			return false
		}
	}
	return true
}

// splitAuthority breaks "user:pass@host:port" into its parts.
func splitAuthority(auth string) (userinfo *string, host string, port *int) {
	rest := auth
	if i := strings.LastIndexByte(rest, '@'); i >= 0 {
		userinfo = strp(rest[:i])
		rest = rest[i+1:]
	}
	// host may be an IP-literal "[...]" which itself contains ':'
	if strings.HasPrefix(rest, "[") {
		if j := strings.IndexByte(rest, ']'); j >= 0 {
			host = rest[1:j]
			after := rest[j+1:]
			if strings.HasPrefix(after, ":") {
				port = parsePort(after[1:])
			}
			return
		}
	}
	if i := strings.LastIndexByte(rest, ':'); i >= 0 {
		host = rest[:i]
		port = parsePort(rest[i+1:])
		return
	}
	host = rest
	return
}

// hostForOutput re-wraps an IPv6 (or otherwise colon-bearing) host literal in the
// "[...]" brackets that parsing stripped, so serialization round-trips.
func hostForOutput(h string) string {
	if strings.ContainsRune(h, ':') && !strings.HasPrefix(h, "[") {
		return "[" + h + "]"
	}
	return h
}

func parsePort(s string) *int {
	if s == "" {
		return nil
	}
	n, err := strconv.Atoi(s)
	if err != nil {
		return nil
	}
	return intp(n)
}

// Parse parses a URI reference into a *URI, mirroring Addressable::URI.parse.
func Parse(uri string) *URI {
	scheme, hasScheme, authority, hasAuthority, path, query, hasQuery, fragment, hasFragment := splitURI(uri)
	u := &URI{path: path}
	if hasScheme {
		u.scheme = strp(scheme)
	}
	if hasAuthority {
		ui, host, port := splitAuthority(authority)
		u.userinfo = ui
		u.host = strp(host)
		u.port = port
	}
	if hasQuery {
		u.query = strp(query)
	}
	if hasFragment {
		u.fragment = strp(fragment)
	}
	return u
}

// Component accessors. A nil pointer means the component is absent (Ruby nil).

// Scheme returns the scheme component, or nil if absent.
func (u *URI) Scheme() *string { return u.scheme }

// Userinfo returns the userinfo component, or nil if absent.
func (u *URI) Userinfo() *string { return u.userinfo }

// Host returns the host component, or nil if absent.
func (u *URI) Host() *string { return u.host }

// Port returns the explicit port, or nil if absent.
func (u *URI) Port() *int { return u.port }

// Path returns the path component (never nil; "" when empty).
func (u *URI) Path() string { return u.path }

// Query returns the query component, or nil if absent.
func (u *URI) Query() *string { return u.query }

// Fragment returns the fragment component, or nil if absent.
func (u *URI) Fragment() *string { return u.fragment }

// Authority reconstructs the authority ("user@host:port"), or nil when the URI has
// no host.
func (u *URI) Authority() *string {
	if u.host == nil {
		return nil
	}
	var b strings.Builder
	if u.userinfo != nil {
		b.WriteString(*u.userinfo)
		b.WriteByte('@')
	}
	b.WriteString(hostForOutput(*u.host))
	if u.port != nil {
		b.WriteByte(':')
		b.WriteString(strconv.Itoa(*u.port))
	}
	s := b.String()
	return &s
}

// NormalizedHost returns the host lowercased and IDN-punycoded (ASCII-Compatible
// Encoding), mirroring Addressable::URI#normalized_host.
func (u *URI) NormalizedHost() *string {
	if u.host == nil {
		return nil
	}
	h := strings.ToLower(*u.host)
	h = idnToASCII(h)
	return &h
}

// String serializes the URI back to its RFC 3986 string form (Addressable::URI#to_s).
func (u *URI) String() string {
	var b strings.Builder
	if u.scheme != nil {
		b.WriteString(*u.scheme)
		b.WriteByte(':')
	}
	if u.host != nil {
		b.WriteString("//")
		if a := u.Authority(); a != nil {
			b.WriteString(*a)
		}
	}
	b.WriteString(u.path)
	if u.query != nil {
		b.WriteByte('?')
		b.WriteString(*u.query)
	}
	if u.fragment != nil {
		b.WriteByte('#')
		b.WriteString(*u.fragment)
	}
	return b.String()
}

// clone returns a deep copy so mutating methods stay non-destructive where the gem
// is (normalize returns a new URI; the in-place bang variants are not modeled).
func (u *URI) clone() *URI {
	c := &URI{path: u.path}
	if u.scheme != nil {
		c.scheme = strp(*u.scheme)
	}
	if u.userinfo != nil {
		c.userinfo = strp(*u.userinfo)
	}
	if u.host != nil {
		c.host = strp(*u.host)
	}
	if u.port != nil {
		c.port = intp(*u.port)
	}
	if u.query != nil {
		c.query = strp(*u.query)
	}
	if u.fragment != nil {
		c.fragment = strp(*u.fragment)
	}
	return c
}

// Normalize returns a normalized copy of the URI per RFC 3986 §6:
// lowercased scheme/host, IDN punycode, percent-encoding normalization,
// dot-segment removal, and default-port stripping. Mirrors
// Addressable::URI#normalize.
func (u *URI) Normalize() *URI {
	n := u.clone()
	if n.scheme != nil {
		s := strings.ToLower(*n.scheme)
		n.scheme = &s
	}
	if n.host != nil {
		h := idnToASCII(strings.ToLower(*n.host))
		n.host = &h
	}
	if n.userinfo != nil {
		ui := normalizePercent(*n.userinfo, ClassUnreserved+":@!$&'()*+,;=")
		n.userinfo = &ui
	}
	// drop a port equal to the scheme default
	if n.port != nil && n.scheme != nil {
		if dp, ok := defaultPorts[*n.scheme]; ok && dp == *n.port {
			n.port = nil
		}
	}
	// path: percent-normalize then dot-segment removal
	p := normalizePercent(n.path, ClassPath)
	if n.host != nil || n.scheme != nil {
		p = removeDotSegments(p)
		if p == "" {
			// an authority with an empty path normalizes to "/"
			p = "/"
		}
	}
	n.path = p
	if n.query != nil {
		q := normalizePercent(*n.query, ClassQuery+"&=")
		n.query = &q
	}
	if n.fragment != nil {
		f := normalizePercent(*n.fragment, ClassFragment)
		n.fragment = &f
	}
	return n
}

// normalizePercent re-encodes s so that every byte is either a member of the
// allowed class or a normalized (uppercase-hex) percent escape, while decoding any
// escape whose byte is actually a member of the allowed (unreserved) set.
func normalizePercent(s, characterClass string) string {
	allowed := buildAllowed(characterClass)
	// unreserved bytes are always decoded even if they arrive escaped
	unreserved := buildAllowed(ClassUnreserved)
	var b strings.Builder
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c == '%' && i+2 < len(s) && isHex(s[i+1]) && isHex(s[i+2]) {
			dec := hexVal(s[i+1])<<4 | hexVal(s[i+2])
			if unreserved[dec] {
				b.WriteByte(dec)
			} else {
				b.WriteByte('%')
				b.WriteByte(upperHex[dec>>4])
				b.WriteByte(upperHex[dec&0x0f])
			}
			i += 2
			continue
		}
		if allowed[c] {
			b.WriteByte(c)
		} else {
			b.WriteByte('%')
			b.WriteByte(upperHex[c>>4])
			b.WriteByte(upperHex[c&0x0f])
		}
	}
	return b.String()
}

// removeDotSegments implements RFC 3986 §5.2.4.
func removeDotSegments(path string) string {
	var out []byte
	in := []byte(path)
	for len(in) > 0 {
		switch {
		case hasPrefixB(in, "../"):
			in = in[3:]
		case hasPrefixB(in, "./"):
			in = in[2:]
		case hasPrefixB(in, "/./"):
			in = append([]byte("/"), in[3:]...)
		case eqB(in, "/."):
			in = []byte("/")
		case hasPrefixB(in, "/../"):
			in = append([]byte("/"), in[4:]...)
			out = popSegment(out)
		case eqB(in, "/.."):
			in = []byte("/")
			out = popSegment(out)
		case eqB(in, "."):
			in = nil
		case eqB(in, ".."):
			in = nil
		default:
			// move the initial segment (including its leading "/") to out
			i := 1
			if in[0] != '/' {
				i = 0
			}
			for i < len(in) && in[i] != '/' {
				i++
			}
			out = append(out, in[:i]...)
			in = in[i:]
		}
	}
	return string(out)
}

func popSegment(out []byte) []byte {
	for i := len(out) - 1; i >= 0; i-- {
		if out[i] == '/' {
			return out[:i]
		}
	}
	return out[:0]
}

func hasPrefixB(b []byte, p string) bool { return strings.HasPrefix(string(b), p) }
func eqB(b []byte, s string) bool        { return string(b) == s }

// Join resolves reference against the (base) URI per RFC 3986 §5.2, returning a new
// absolute URI. Mirrors Addressable::URI#join / #+.
func (u *URI) Join(reference string) *URI {
	r := Parse(reference)
	return u.JoinURI(r)
}

// JoinURI is Join with an already-parsed reference.
func (u *URI) JoinURI(r *URI) *URI {
	base := u
	t := &URI{}
	if r.scheme != nil {
		t.scheme = strp(*r.scheme)
		t.copyAuthorityFrom(r)
		t.path = removeDotSegments(r.path)
		t.query = r.query
	} else {
		if r.host != nil {
			t.copyAuthorityFrom(r)
			t.path = removeDotSegments(r.path)
			t.query = r.query
		} else {
			if r.path == "" {
				t.path = base.path
				if r.query != nil {
					t.query = r.query
				} else {
					t.query = base.query
				}
			} else {
				if strings.HasPrefix(r.path, "/") {
					t.path = removeDotSegments(r.path)
				} else {
					t.path = removeDotSegments(mergePath(base, r.path))
				}
				t.query = r.query
			}
			t.copyAuthorityFrom(base)
		}
		t.scheme = base.scheme
	}
	t.fragment = r.fragment
	return t
}

func (t *URI) copyAuthorityFrom(o *URI) {
	if o.userinfo != nil {
		t.userinfo = strp(*o.userinfo)
	}
	if o.host != nil {
		t.host = strp(*o.host)
	}
	if o.port != nil {
		t.port = intp(*o.port)
	}
}

// mergePath implements RFC 3986 §5.3 merge.
func mergePath(base *URI, refPath string) string {
	if base.host != nil && base.path == "" {
		return "/" + refPath
	}
	if i := strings.LastIndexByte(base.path, '/'); i >= 0 {
		return base.path[:i+1] + refPath
	}
	return refPath
}

// Origin returns the RFC 6454 origin ("scheme://host[:port]"), or "null" when the
// URI is not a hierarchical http(s)-style URI. Mirrors Addressable::URI#origin.
func (u *URI) Origin() string {
	if u.scheme == nil || u.host == nil {
		return "null"
	}
	n := u.Normalize()
	var b strings.Builder
	b.WriteString(*n.scheme)
	b.WriteString("://")
	b.WriteString(*n.host)
	if n.port != nil {
		b.WriteByte(':')
		b.WriteString(strconv.Itoa(*n.port))
	}
	return b.String()
}

// Omit returns a copy of the URI with the named components removed. Recognized
// names: "scheme", "userinfo", "host", "port", "path", "query", "fragment".
// Mirrors Addressable::URI#omit.
func (u *URI) Omit(components ...string) *URI {
	c := u.clone()
	for _, name := range components {
		switch name {
		case "scheme":
			c.scheme = nil
		case "userinfo":
			c.userinfo = nil
		case "host":
			c.host = nil
		case "port":
			c.port = nil
		case "path":
			c.path = ""
		case "query":
			c.query = nil
		case "fragment":
			c.fragment = nil
		}
	}
	return c
}

// QueryValues parses the query string into ordered key/value pairs. Duplicate keys
// are preserved in order. Mirrors Addressable::URI#query_values(Array).
func (u *URI) QueryValues() [][2]string {
	if u.query == nil {
		return nil
	}
	var out [][2]string
	for _, pair := range strings.Split(*u.query, "&") {
		if pair == "" {
			continue
		}
		k, v, found := strings.Cut(pair, "=")
		key := UnencodeComponent(k)
		val := ""
		if found {
			val = UnencodeComponent(v)
		}
		out = append(out, [2]string{key, val})
	}
	return out
}

// QueryValuesHash collapses QueryValues into a map keeping the *last* value for a
// duplicated key, mirroring Addressable::URI#query_values (Hash, the default).
func (u *URI) QueryValuesHash() map[string]string {
	pairs := u.QueryValues()
	if pairs == nil {
		return nil
	}
	m := make(map[string]string, len(pairs))
	for _, p := range pairs {
		m[p[0]] = p[1]
	}
	return m
}

// SetQueryValues sets the query component from ordered pairs, percent-encoding keys
// and values. A value list under one key repeats the key. Mirrors the assignment
// side of Addressable::URI#query_values=.
func (u *URI) SetQueryValues(pairs [][2]string) {
	if pairs == nil {
		u.query = nil
		return
	}
	parts := make([]string, 0, len(pairs))
	for _, p := range pairs {
		k := EncodeComponent(p[0], ClassUnreserved)
		v := EncodeComponent(p[1], ClassUnreserved)
		parts = append(parts, k+"="+v)
	}
	q := strings.Join(parts, "&")
	u.query = &q
}

// SetQueryValuesMap sets the query from a map. Keys are emitted in sorted order for
// determinism; a []string value under a key repeats the key. This matches the gem's
// observable output for the sorted-hash cases the golden vectors cover.
func (u *URI) SetQueryValuesMap(m map[string]any) {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	var pairs [][2]string
	for _, k := range keys {
		switch v := m[k].(type) {
		case string:
			pairs = append(pairs, [2]string{k, v})
		case []string:
			for _, item := range v {
				pairs = append(pairs, [2]string{k, item})
			}
		}
	}
	u.SetQueryValues(pairs)
}
