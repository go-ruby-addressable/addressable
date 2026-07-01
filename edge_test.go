// Copyright (c) the go-ruby-addressable/addressable authors
//
// SPDX-License-Identifier: BSD-3-Clause

package addressable

import (
	"reflect"
	"testing"
)

func TestIDNAEdge(t *testing.T) {
	// mixed ASCII + non-ASCII labels: the ASCII label is passed through, the
	// unicode label is punycoded.
	if got := idnToASCII("www.例え.com"); got != "www.xn--r8jz45g.com" {
		t.Errorf("mixed IDN = %q", got)
	}
	// a label with leading basic chars plus a non-basic char (exercises the
	// basic>0 "-" delimiter and the delta accumulation over basic points).
	if got := idnToASCII("aあ"); got == "" || got[:4] != "xn--" {
		t.Errorf("basic+nonbasic label = %q", got)
	}
	// pure ASCII host: fast path, unchanged.
	if got := idnToASCII("example.com"); got != "example.com" {
		t.Errorf("ascii host = %q", got)
	}
}

func TestParseExprEdge(t *testing.T) {
	// empty spec between commas is skipped; ":x" (non-digit) prefix is left as a
	// literal name (no prefix applied).
	tpl := NewTemplate("{a,,b}")
	got := tpl.Expand(map[string]Value{"a": "1", "b": "2"})
	if got != "1,2" {
		t.Errorf("empty spec = %q", got)
	}
	// non-numeric prefix: the whole "var:x" is treated as the variable name, which
	// is unbound, so it expands to nothing.
	if got := NewTemplate("{var:x}").Expand(map[string]Value{"var": "value"}); got != "" {
		t.Errorf("bad prefix = %q", got)
	}
	// expandVar with an unsupported dynamic type is dropped.
	if got := NewTemplate("{n}").Expand(map[string]Value{"n": 42}); got != "" {
		t.Errorf("unsupported type = %q", got)
	}
}

func TestEncodeAllowReservedExisting(t *testing.T) {
	// an existing %XX escape is preserved verbatim under the +/# operators.
	got := NewTemplate("{+v}").Expand(map[string]Value{"v": "a%20b c"})
	if got != "a%20b%20c" {
		t.Errorf("allow-reserved existing escape = %q", got)
	}
}

func TestExtractListSep(t *testing.T) {
	// operator explode splits on the operator separator.
	got := NewTemplate("http://x{/list*}").Extract("http://x/red/green/blue")
	if !reflect.DeepEqual(got["list"], []string{"red", "green", "blue"}) {
		t.Errorf("op explode list = %#v", got["list"])
	}
	// assoc explode with an empty trailing pair segment is skipped.
	got2 := NewTemplate("http://x/{?m*}").Extract("http://x/?a=1&")
	if !reflect.DeepEqual(got2["m"], map[string]string{"a": "1"}) {
		t.Errorf("assoc explode = %#v", got2["m"])
	}
	// splitCharset with the default (empty) separator: a default-operator var.
	got3 := NewTemplate("http://x/{a}").Extract("http://x/hello")
	if got3["a"] != "hello" {
		t.Errorf("default charset = %#v", got3["a"])
	}
}

func TestValidSchemeEdge(t *testing.T) {
	// A leading digit is not a valid scheme, so "1http://x" parses as a path.
	u := Parse("1abc:def")
	if u.Scheme() != nil {
		t.Errorf("digit-leading scheme wrongly accepted: %q", d(u.Scheme()))
	}
	// A scheme with an invalid character mid-token is rejected.
	u2 := Parse("ht_tp:x")
	if u2.Scheme() != nil {
		t.Errorf("underscore scheme wrongly accepted: %q", d(u2.Scheme()))
	}
	// Empty candidate before ':' (":path") is not a scheme.
	u3 := Parse(":path")
	if u3.Scheme() != nil {
		t.Errorf("empty scheme wrongly accepted")
	}
}

func TestParsePortEdge(t *testing.T) {
	// empty port after ':' -> nil
	u := Parse("http://host:/x")
	if u.Port() != nil {
		t.Errorf("empty port = %v", *u.Port())
	}
	// non-numeric port -> nil (tolerant; the gem raises, we degrade gracefully)
	u2 := Parse("http://host:bad/x")
	if u2.Port() != nil {
		t.Errorf("bad port = %v", *u2.Port())
	}
}

func TestRemoveDotSegmentsLeading(t *testing.T) {
	// Leading "../" and "./" are stripped (RFC 3986 §5.2.4 case A).
	if got := removeDotSegments("../a"); got != "a" {
		t.Errorf("../a = %q", got)
	}
	if got := removeDotSegments("./a"); got != "a" {
		t.Errorf("./a = %q", got)
	}
	// bare "." and ".." reduce to nothing.
	if got := removeDotSegments("."); got != "" {
		t.Errorf(". = %q", got)
	}
	if got := removeDotSegments(".."); got != "" {
		t.Errorf(".. = %q", got)
	}
	// popSegment with an empty output stack (leading "/../").
	if got := removeDotSegments("/../a"); got != "/a" {
		t.Errorf("/../a = %q", got)
	}
}

func TestCopyAuthorityNilParts(t *testing.T) {
	// Reference with a host but no userinfo/port: copyAuthorityFrom skips those.
	base := Parse("http://a/b/c")
	got := base.Join("//other/x").String()
	if got != "http://other/x" {
		t.Errorf("authority-only ref = %q", got)
	}
	// Reference carrying userinfo + port: those branches are copied through.
	got2 := base.Join("//user:pass@other:99/x").String()
	if got2 != "http://user:pass@other:99/x" {
		t.Errorf("full-authority ref = %q", got2)
	}
}

func TestValidSchemeDirect(t *testing.T) {
	// Directly exercise the defensive empty-string guard (unreachable via Parse,
	// where a scheme candidate always has length > 0).
	if validScheme("") {
		t.Error("empty string is not a valid scheme")
	}
}
