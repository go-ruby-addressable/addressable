// Copyright (c) the go-ruby-addressable/addressable authors
//
// SPDX-License-Identifier: BSD-3-Clause

package addressable

import (
	"reflect"
	"testing"
)

func d(s *string) string {
	if s == nil {
		return "<nil>"
	}
	return *s
}

func di(p *int) string {
	if p == nil {
		return "<nil>"
	}
	return string(rune('0' + *p)) // only used in messages for small values
}

// --- Parse component vectors (captured from the addressable gem) ---

func TestParseComponents(t *testing.T) {
	type want struct {
		scheme, user, host, path, query, frag, authority, normhost string
		port                                                       *int
	}
	p := func(i int) *int { return &i }
	cases := []struct {
		uri string
		w   want
	}{
		{"http://example.com/foo", want{"http", "<nil>", "example.com", "/foo", "<nil>", "<nil>", "example.com", "example.com", nil}},
		{"http://User:Pass@Example.COM:80/a/./b/../c/?x=1&y=2#frag", want{"http", "User:Pass", "Example.COM", "/a/./b/../c/", "x=1&y=2", "frag", "User:Pass@Example.COM:80", "example.com", p(80)}},
		{"HTTP://example.com", want{"HTTP", "<nil>", "example.com", "", "<nil>", "<nil>", "example.com", "example.com", nil}},
		{"//example.com/path", want{"<nil>", "<nil>", "example.com", "/path", "<nil>", "<nil>", "example.com", "example.com", nil}},
		{"mailto:user@example.com", want{"mailto", "<nil>", "<nil>", "user@example.com", "<nil>", "<nil>", "<nil>", "<nil>", nil}},
		{"http://xn--nxasmq6b.com", want{"http", "<nil>", "xn--nxasmq6b.com", "", "<nil>", "<nil>", "xn--nxasmq6b.com", "xn--nxasmq6b.com", nil}},
		{"http://例え.テスト/", want{"http", "<nil>", "例え.テスト", "/", "<nil>", "<nil>", "例え.テスト", "xn--r8jz45g.xn--zckzah", nil}},
		{"http://example.com/%7euser", want{"http", "<nil>", "example.com", "/%7euser", "<nil>", "<nil>", "example.com", "example.com", nil}},
		{"urn:isbn:0451450523", want{"urn", "<nil>", "<nil>", "isbn:0451450523", "<nil>", "<nil>", "<nil>", "<nil>", nil}},
		{"foo/bar", want{"<nil>", "<nil>", "<nil>", "foo/bar", "<nil>", "<nil>", "<nil>", "<nil>", nil}},
		{"?query", want{"<nil>", "<nil>", "<nil>", "", "query", "<nil>", "<nil>", "<nil>", nil}},
		{"#frag", want{"<nil>", "<nil>", "<nil>", "", "<nil>", "frag", "<nil>", "<nil>", nil}},
		{"http://[::1]:8080/", want{"http", "<nil>", "::1", "/", "<nil>", "<nil>", "[::1]:8080", "::1", p(8080)}},
		{"http://[::1]/", want{"http", "<nil>", "::1", "/", "<nil>", "<nil>", "[::1]", "::1", nil}},
		{"", want{"<nil>", "<nil>", "<nil>", "", "<nil>", "<nil>", "<nil>", "<nil>", nil}},
	}
	for _, c := range cases {
		u := Parse(c.uri)
		got := want{
			scheme: d(u.Scheme()), user: d(u.Userinfo()), host: d(u.Host()),
			path: u.Path(), query: d(u.Query()), frag: d(u.Fragment()),
			authority: d(u.Authority()), normhost: d(u.NormalizedHost()), port: u.Port(),
		}
		if got.scheme != c.w.scheme || got.user != c.w.user || got.host != c.w.host ||
			got.path != c.w.path || got.query != c.w.query || got.frag != c.w.frag ||
			got.authority != c.w.authority || got.normhost != c.w.normhost {
			t.Errorf("Parse(%q) = %+v\n want %+v", c.uri, got, c.w)
		}
		if (got.port == nil) != (c.w.port == nil) || (got.port != nil && *got.port != *c.w.port) {
			t.Errorf("Parse(%q) port = %s want %s", c.uri, di(got.port), di(c.w.port))
		}
		// round-trip
		if s := u.String(); s != c.uri {
			t.Errorf("String round-trip %q -> %q", c.uri, s)
		}
	}
}

func TestNormalize(t *testing.T) {
	cases := map[string]string{
		"HTTP://Example.COM:80/a/./b/../c/": "http://example.com/a/c/",
		"http://example.com/%7euser":        "http://example.com/~user",
		"http://example.com/foo%20bar":      "http://example.com/foo%20bar",
		"http://example.com/a/b/../../../c": "http://example.com/c",
		"http://example.com":                "http://example.com/",
		"http://example.com:8080/":          "http://example.com:8080/",
		"http://example.com/?b=2&a=1":       "http://example.com/?b=2&a=1",
		"http://例え.テスト/ぱす":                  "http://xn--r8jz45g.xn--zckzah/%E3%81%B1%E3%81%99",
		"https://example.com:443/x":         "https://example.com/x",
		"http://example.com/a%2fb":          "http://example.com/a%2Fb",
		"foo/../bar":                        "foo/../bar", // no scheme/host -> no dot removal
		"mailto:x@y.com":                    "mailto:x@y.com",
	}
	for in, wantS := range cases {
		if got := Parse(in).Normalize().String(); got != wantS {
			t.Errorf("Normalize(%q) = %q want %q", in, got, wantS)
		}
	}
}

func TestJoinRFC3986(t *testing.T) {
	base := Parse("http://a/b/c/d;p?q")
	cases := map[string]string{
		"g": "http://a/b/c/g", "./g": "http://a/b/c/g", "g/": "http://a/b/c/g/",
		"/g": "http://a/g", "//g": "http://g", "?y": "http://a/b/c/d;p?y",
		"g?y": "http://a/b/c/g?y", "#s": "http://a/b/c/d;p?q#s", "g#s": "http://a/b/c/g#s",
		"g?y#s": "http://a/b/c/g?y#s", ";x": "http://a/b/c/;x", "g;x": "http://a/b/c/g;x",
		"": "http://a/b/c/d;p?q", "../": "http://a/b/", "../g": "http://a/b/g",
		"../..": "http://a/", "../../g": "http://a/g", "/./g": "http://a/g",
		"/../g": "http://a/g", "g.": "http://a/b/c/g.", "../../../g": "http://a/g",
		"../../../../g": "http://a/g", ".": "http://a/b/c/", "./": "http://a/b/c/",
		"..": "http://a/b/", "g/./h": "http://a/b/c/g/h", "g/../h": "http://a/b/c/h",
	}
	for r, wantS := range cases {
		if got := base.Join(r).String(); got != wantS {
			t.Errorf("Join(%q) = %q want %q", r, got, wantS)
		}
	}
	// join with scheme-bearing reference and a relative base path
	if got := Parse("/base").Join("http://other/x").String(); got != "http://other/x" {
		t.Errorf("abs ref join = %q", got)
	}
	// merge against a base with no slash in path
	if got := Parse("http://h").Join("rel").String(); got != "http://h/rel" {
		t.Errorf("merge empty base path = %q", got)
	}
	if got := Parse("norel").Join("rel").String(); got != "rel" {
		t.Errorf("merge no-host no-slash = %q", got)
	}
	// JoinURI directly
	if got := base.JoinURI(Parse("g")).String(); got != "http://a/b/c/g" {
		t.Errorf("JoinURI = %q", got)
	}
}

func TestQueryValues(t *testing.T) {
	u := Parse("http://example.com/?a=1&b=2&a=3")
	pairs := u.QueryValues()
	wantPairs := [][2]string{{"a", "1"}, {"b", "2"}, {"a", "3"}}
	if !reflect.DeepEqual(pairs, wantPairs) {
		t.Errorf("QueryValues = %v want %v", pairs, wantPairs)
	}
	h := u.QueryValuesHash()
	if h["a"] != "3" || h["b"] != "2" {
		t.Errorf("QueryValuesHash = %v", h)
	}
	// nil query
	if Parse("http://x/").QueryValues() != nil || Parse("http://x/").QueryValuesHash() != nil {
		t.Error("expected nil query values")
	}
	// empty segments and key-only
	p2 := Parse("http://x/?&k&m=").QueryValues()
	if !reflect.DeepEqual(p2, [][2]string{{"k", ""}, {"m", ""}}) {
		t.Errorf("edge query values = %v", p2)
	}
	// percent-decoding
	p3 := Parse("http://x/?a%20b=c%2Fd").QueryValues()
	if p3[0][0] != "a b" || p3[0][1] != "c/d" {
		t.Errorf("decoded qv = %v", p3)
	}
}

func TestSetQueryValues(t *testing.T) {
	u := Parse("http://example.com/")
	u.SetQueryValues([][2]string{{"a b", "c/d"}, {"x", "y"}})
	if d(u.Query()) != "a%20b=c%2Fd&x=y" {
		t.Errorf("SetQueryValues = %q", d(u.Query()))
	}
	u.SetQueryValues(nil)
	if u.Query() != nil {
		t.Error("nil pairs should clear query")
	}
	u2 := Parse("http://example.com/")
	u2.SetQueryValuesMap(map[string]any{"a": "1", "b": []string{"x", "y"}, "ignored": 42})
	if d(u2.Query()) != "a=1&b=x&b=y" {
		t.Errorf("SetQueryValuesMap = %q", d(u2.Query()))
	}
}

func TestOmitOrigin(t *testing.T) {
	u := Parse("http://user:pass@example.com:8080/p?q=1#f")
	if got := u.Omit("userinfo", "query").String(); got != "http://example.com:8080/p#f" {
		t.Errorf("Omit = %q", got)
	}
	// exercise every omit branch
	all := u.Omit("scheme", "host", "port", "path", "fragment").String()
	if all != "?q=1" {
		t.Errorf("Omit all = %q", all)
	}
	u.Omit("bogus") // unknown name is a no-op
	if u.Origin() != "http://example.com:8080" {
		t.Errorf("Origin = %q", u.Origin())
	}
	if Parse("http://example.com").Origin() != "http://example.com" {
		t.Error("origin default port")
	}
	if Parse("mailto:x@y.com").Origin() != "null" {
		t.Error("origin non-hierarchical")
	}
	if Parse("//host/x").Origin() != "null" {
		t.Error("origin no scheme")
	}
}

func TestEncodeDecode(t *testing.T) {
	if got := EncodeComponent("one two", ClassUnreserved); got != "one%20two" {
		t.Errorf("encode = %q", got)
	}
	if got := EncodeComponent("a b/c?d", ClassPath); got != "a%20b/c%3Fd" {
		t.Errorf("encode path = %q", got)
	}
	if got := UnencodeComponent("one%20two"); got != "one two" {
		t.Errorf("decode = %q", got)
	}
	// invalid escape left verbatim
	if got := UnencodeComponent("a%2"); got != "a%2" {
		t.Errorf("bad escape = %q", got)
	}
	if got := UnencodeComponent("a%zzb"); got != "a%zzb" {
		t.Errorf("bad hex = %q", got)
	}
	if got := UnencodeComponent("no-escape"); got != "no-escape" {
		t.Errorf("no-escape fast path = %q", got)
	}
	// buildAllowed range + escaped-bracket path
	tbl := buildAllowed("a-c\\[")
	if !tbl['a'] || !tbl['b'] || !tbl['c'] || !tbl['['] || tbl['d'] {
		t.Error("buildAllowed range/escape wrong")
	}
	// trailing dash literal (not a range)
	tbl2 := buildAllowed("x-")
	if !tbl2['x'] || !tbl2['-'] {
		t.Error("buildAllowed trailing dash")
	}
}
