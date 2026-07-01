// Copyright (c) the go-ruby-addressable/addressable authors
//
// SPDX-License-Identifier: BSD-3-Clause

package addressable

import (
	"regexp"
	"strings"
)

// Template is a compiled RFC 6570 URI Template (all four levels), mirroring
// Addressable::Template.
type Template struct {
	pattern string
	parts   []tpart // literal text and expression parts, in order
}

// tpart is either a literal run of text (expr == nil) or a single {…} expression.
type tpart struct {
	literal string
	expr    *texpr
}

// texpr is one {op vars} expression.
type texpr struct {
	op   byte // 0 for the default operator, else one of + # . / ; ? &
	vars []tvar
}

// tvar is one variable spec inside an expression, e.g. list*, path:4.
type tvar struct {
	name    string
	explode bool
	prefix  int // >0 when a :N modifier is present
}

// Value is the dynamic value bound to a template variable: a string, a []string
// (list), or a [][2]string (ordered key/value pairs, an "associative array").
type Value any

// operatorInfo describes the expansion behavior of one RFC 6570 operator.
type operatorInfo struct {
	first    string // string emitted before the first value if any value present
	sep      string // separator between values
	named    bool   // ";x=1" / "?x=1" style (name=value)
	ifEmpty  string // string between name and value when the value is empty
	allowRes bool   // reserved characters allowed unescaped (+ and #)
}

func opInfo(op byte) operatorInfo {
	switch op {
	case '+':
		return operatorInfo{first: "", sep: ",", allowRes: true}
	case '#':
		return operatorInfo{first: "#", sep: ",", allowRes: true}
	case '.':
		return operatorInfo{first: ".", sep: "."}
	case '/':
		return operatorInfo{first: "/", sep: "/"}
	case ';':
		return operatorInfo{first: ";", sep: ";", named: true, ifEmpty: ""}
	case '?':
		return operatorInfo{first: "?", sep: "&", named: true, ifEmpty: "="}
	case '&':
		return operatorInfo{first: "&", sep: "&", named: true, ifEmpty: "="}
	default:
		return operatorInfo{first: "", sep: ","}
	}
}

// NewTemplate compiles a template pattern.
func NewTemplate(pattern string) *Template {
	t := &Template{pattern: pattern}
	i := 0
	for i < len(pattern) {
		open := strings.IndexByte(pattern[i:], '{')
		if open < 0 {
			t.parts = append(t.parts, tpart{literal: pattern[i:]})
			break
		}
		open += i
		if open > i {
			t.parts = append(t.parts, tpart{literal: pattern[i:open]})
		}
		close := strings.IndexByte(pattern[open:], '}')
		if close < 0 {
			// Unterminated expression: treat the remainder as literal.
			t.parts = append(t.parts, tpart{literal: pattern[open:]})
			break
		}
		close += open
		t.parts = append(t.parts, tpart{expr: parseExpr(pattern[open+1 : close])})
		i = close + 1
	}
	return t
}

// Pattern returns the original template string.
func (t *Template) Pattern() string { return t.pattern }

func parseExpr(body string) *texpr {
	e := &texpr{}
	if body == "" {
		return e
	}
	switch body[0] {
	case '+', '#', '.', '/', ';', '?', '&':
		e.op = body[0]
		body = body[1:]
	}
	for _, spec := range strings.Split(body, ",") {
		if spec == "" {
			continue
		}
		v := tvar{}
		if strings.HasSuffix(spec, "*") {
			v.explode = true
			spec = spec[:len(spec)-1]
		} else if idx := strings.IndexByte(spec, ':'); idx >= 0 {
			n := 0
			for _, c := range spec[idx+1:] {
				if c < '0' || c > '9' {
					n = -1
					break
				}
				n = n*10 + int(c-'0')
			}
			if n >= 0 {
				v.prefix = n
				spec = spec[:idx]
			}
		}
		v.name = spec
		e.vars = append(e.vars, v)
	}
	return e
}

// Expand expands the template with the given variable bindings, returning the
// resulting URI string. Mirrors Addressable::Template#expand(vars).to_s.
func (t *Template) Expand(vars map[string]Value) string {
	var b strings.Builder
	for _, p := range t.parts {
		if p.expr == nil {
			b.WriteString(p.literal)
			continue
		}
		b.WriteString(expandExpr(p.expr, vars))
	}
	return b.String()
}

func expandExpr(e *texpr, vars map[string]Value) string {
	info := opInfo(e.op)
	var pieces []string
	for _, v := range e.vars {
		val, ok := vars[v.name]
		if !ok || val == nil {
			continue
		}
		s, present := expandVar(v, val, info)
		if !present {
			continue
		}
		pieces = append(pieces, s)
	}
	if len(pieces) == 0 {
		return ""
	}
	return info.first + strings.Join(pieces, info.sep)
}

// expandVar expands one variable and returns (rendered, present). present is false
// when the value is undefined for expansion (e.g. an empty list/hash), which the
// caller drops.
func expandVar(v tvar, val Value, info operatorInfo) (string, bool) {
	switch x := val.(type) {
	case string:
		return expandString(v, x, info), true
	case []string:
		if len(x) == 0 {
			return "", false
		}
		return expandList(v, x, info), true
	case [][2]string:
		if len(x) == 0 {
			return "", false
		}
		return expandAssoc(v, x, info), true
	default:
		return "", false
	}
}

func encVal(s string, info operatorInfo) string {
	if info.allowRes {
		// + and # allow reserved + pct-encoded (encode only what is neither
		// unreserved nor reserved nor an existing pct-escape).
		return encodeAllowReserved(s)
	}
	return EncodeComponent(s, ClassUnreserved)
}

// encodeAllowReserved percent-encodes s keeping unreserved + reserved characters and
// existing percent-escapes literal (the "U+R" set of RFC 6570 for the + and #
// operators).
func encodeAllowReserved(s string) string {
	unreserved := buildAllowed(ClassUnreserved)
	reserved := buildAllowed(ClassReserved)
	var b strings.Builder
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c == '%' && i+2 < len(s) && isHex(s[i+1]) && isHex(s[i+2]) {
			b.WriteByte(c)
			continue
		}
		if unreserved[c] || reserved[c] {
			b.WriteByte(c)
			continue
		}
		b.WriteByte('%')
		b.WriteByte(upperHex[c>>4])
		b.WriteByte(upperHex[c&0x0f])
	}
	return b.String()
}

func expandString(v tvar, s string, info operatorInfo) string {
	val := s
	if v.prefix > 0 && v.prefix < len([]rune(s)) {
		val = string([]rune(s)[:v.prefix])
	}
	enc := encVal(val, info)
	if info.named {
		return namePair(v.name, enc, s == "", info)
	}
	return enc
}

func namePair(name, enc string, empty bool, info operatorInfo) string {
	if empty {
		return name + info.ifEmpty
	}
	return name + "=" + enc
}

func expandList(v tvar, list []string, info operatorInfo) string {
	if v.explode {
		parts := make([]string, len(list))
		for i, item := range list {
			if info.named {
				parts[i] = namePair(v.name, encVal(item, info), item == "", info)
			} else {
				parts[i] = encVal(item, info)
			}
		}
		return strings.Join(parts, info.sep)
	}
	parts := make([]string, len(list))
	for i, item := range list {
		parts[i] = encVal(item, info)
	}
	joined := strings.Join(parts, ",")
	if info.named {
		return v.name + "=" + joined
	}
	return joined
}

func expandAssoc(v tvar, pairs [][2]string, info operatorInfo) string {
	if v.explode {
		parts := make([]string, len(pairs))
		for i, kv := range pairs {
			k := encVal(kv[0], info)
			val := encVal(kv[1], info)
			if info.named {
				parts[i] = namePair(k, val, kv[1] == "", info)
			} else {
				parts[i] = k + "=" + val
			}
		}
		return strings.Join(parts, info.sep)
	}
	parts := make([]string, 0, len(pairs)*2)
	for _, kv := range pairs {
		parts = append(parts, encVal(kv[0], info), encVal(kv[1], info))
	}
	joined := strings.Join(parts, ",")
	if info.named {
		return v.name + "=" + joined
	}
	return joined
}

// Extract reverse-matches a URI against the template, returning the variable
// bindings, or nil when the URI does not match. Mirrors
// Addressable::Template#extract(uri). List/hash values are returned as []string /
// map[string]string where the template variable exploded.
func (t *Template) Extract(uri string) map[string]any {
	re, names, specs := t.buildRegexp()
	m := re.FindStringSubmatch(uri)
	if m == nil {
		return nil
	}
	out := make(map[string]any, len(names))
	for i, name := range names {
		raw := m[i+1]
		out[name] = decodeExtracted(raw, specs[i])
	}
	return out
}

// extractKind classifies how a captured group is post-processed.
type extractKind int

const (
	kindSimple extractKind = iota // single string value
	kindList                      // separator-joined list -> []string
	kindAssoc                     // query-style pairs -> map[string]string
)

// decodeSpec records how one captured group is decoded: its kind and, for a list,
// the separator its items are joined by (empty means the value is a single item).
type decodeSpec struct {
	kind extractKind
	sep  string
}

func decodeExtracted(raw string, spec decodeSpec) any {
	switch spec.kind {
	case kindList:
		var items []string
		if spec.sep == "" {
			items = []string{raw}
		} else {
			items = strings.Split(raw, spec.sep)
		}
		out := make([]string, len(items))
		for i, it := range items {
			out[i] = UnencodeComponent(it)
		}
		return out
	case kindAssoc:
		m := map[string]string{}
		for _, pair := range strings.Split(raw, "&") {
			if pair == "" {
				continue
			}
			k, v, _ := strings.Cut(pair, "=")
			m[UnencodeComponent(k)] = UnencodeComponent(v)
		}
		return m
	default:
		if raw == "" {
			return nil
		}
		return UnencodeComponent(raw)
	}
}

// buildRegexp turns the template into an anchored regexp and the ordered list of
// captured variable names + their decode specs.
func (t *Template) buildRegexp() (*regexp.Regexp, []string, []decodeSpec) {
	var b strings.Builder
	b.WriteString("^")
	var names []string
	var specs []decodeSpec
	for _, p := range t.parts {
		if p.expr == nil {
			b.WriteString(regexp.QuoteMeta(p.literal))
			continue
		}
		writeExprRegexp(&b, p.expr, &names, &specs)
	}
	b.WriteString("$")
	return regexp.MustCompile(b.String()), names, specs
}

func writeExprRegexp(b *strings.Builder, e *texpr, names *[]string, specs *[]decodeSpec) {
	info := opInfo(e.op)
	// A leading operator injects a fixed prefix character (., /, ;, ?, &, #).
	if info.first != "" {
		b.WriteString(regexp.QuoteMeta(info.first))
	}
	for i, v := range e.vars {
		if i > 0 {
			b.WriteString(regexp.QuoteMeta(info.sep))
		}
		spec := decodeSpec{kind: kindSimple}
		if v.explode {
			if info.named {
				spec.kind = kindAssoc
			} else {
				spec.kind = kindList
				// A default-operator ({list*}) explode has no reversible item
				// separator, so the gem returns the raw run as a single element;
				// operator explodes ({/list*}, {.list*}) split on the operator's
				// separator character.
				if e.op != 0 {
					spec.sep = info.sep
				}
			}
		}
		*names = append(*names, v.name)
		*specs = append(*specs, spec)
		// Named operators (; ? &) emit "name=value"; for a non-exploded var consume
		// the literal "name=" prefix outside the capture so only the value is
		// captured. An exploded assoc keeps the whole "k=v&k=v" run (kindAssoc
		// re-parses it), so no fixed prefix is stripped.
		if info.named && !v.explode {
			b.WriteString(regexp.QuoteMeta(v.name) + "=")
		}
		switch {
		case info.allowRes:
			b.WriteString("(.*?)")
		case v.explode:
			// An exploded value spans its separators; capture the whole run up to
			// the next fixed delimiter (?, #, or end-of-input handled by anchoring).
			b.WriteString("(.+?)")
		default:
			b.WriteString("([^" + regexp.QuoteMeta(splitCharset(info.sep)) + "/?#]*?)")
		}
	}
}

// splitCharset returns the separator characters to exclude from a non-greedy match,
// keeping the regexp class small. It is only ever called with a non-empty operator
// separator.
func splitCharset(sep string) string {
	return sep + ","
}
