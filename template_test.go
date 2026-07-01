// Copyright (c) the go-ruby-addressable/addressable authors
//
// SPDX-License-Identifier: BSD-3-Clause

package addressable

import (
	"reflect"
	"testing"
)

func tvars() map[string]Value {
	return map[string]Value{
		"count": []string{"one", "two", "three"},
		"dom":   []string{"example", "com"},
		"dub":   "me/too",
		"hello": "Hello World!",
		"half":  "50%",
		"var":   "value",
		"who":   "fred",
		"base":  "http://example.com/home/",
		"path":  "/foo/bar",
		"list":  []string{"red", "green", "blue"},
		"keys":  [][2]string{{"semi", ";"}, {"dot", "."}, {"comma", ","}},
		"v":     "6",
		"x":     "1024",
		"y":     "768",
		"empty": "",
		"undef": nil,
	}
}

func TestExpandGolden(t *testing.T) {
	vars := tvars()
	cases := map[string]string{
		"{var}": "value", "{hello}": "Hello%20World%21", "{half}": "50%25",
		"{x,y}": "1024,768", "{x,hello,y}": "1024,Hello%20World%21,768",
		"{+var}": "value", "{+hello}": "Hello%20World!", "{+path}/here": "/foo/bar/here",
		"{+x,hello,y}": "1024,Hello%20World!,768",
		"{#var}":       "#value", "{#hello}": "#Hello%20World!", "{#x,hello,y}": "#1024,Hello%20World!,768",
		"X{.var}": "X.value", "X{.x,y}": "X.1024.768",
		"{/var}": "/value", "{/var,x}/here": "/value/1024/here",
		"{;x,y}": ";x=1024;y=768", "{;x,y,empty}": ";x=1024;y=768;empty",
		"{?x,y}": "?x=1024&y=768", "{?x,y,empty}": "?x=1024&y=768&empty=",
		"{&x,y,empty}": "&x=1024&y=768&empty=", "?fixed=yes{&x}": "?fixed=yes&x=1024",
		"{var:3}": "val", "{var:30}": "value",
		"{list}": "red,green,blue", "{list*}": "red,green,blue",
		"{keys}": "semi,%3B,dot,.,comma,%2C", "{keys*}": "semi=%3B,dot=.,comma=%2C",
		"{+list}": "red,green,blue", "{+list*}": "red,green,blue",
		"{+keys}": "semi,;,dot,.,comma,,", "{+keys*}": "semi=;,dot=.,comma=,",
		"{#list}": "#red,green,blue", "{#list*}": "#red,green,blue",
		"{#keys}": "#semi,;,dot,.,comma,,", "{#keys*}": "#semi=;,dot=.,comma=,",
		"X{.list}": "X.red,green,blue", "X{.list*}": "X.red.green.blue",
		"X{.keys}": "X.semi,%3B,dot,.,comma,%2C", "X{.keys*}": "X.semi=%3B.dot=..comma=%2C",
		"{/list}": "/red,green,blue", "{/list*}": "/red/green/blue",
		"{/list*,path:4}": "/red/green/blue/%2Ffoo",
		"{/keys}":         "/semi,%3B,dot,.,comma,%2C", "{/keys*}": "/semi=%3B/dot=./comma=%2C",
		"{;keys}": ";keys=semi,%3B,dot,.,comma,%2C", "{;keys*}": ";semi=%3B;dot=.;comma=%2C",
		"{?keys}": "?keys=semi,%3B,dot,.,comma,%2C", "{?keys*}": "?semi=%3B&dot=.&comma=%2C",
		"{?list}": "?list=red,green,blue", "{?list*}": "?list=red&list=green&list=blue",
		"{count}": "one,two,three", "{count*}": "one,two,three", "{+count}": "one,two,three",
		"{.count}": ".one,two,three", "{/count}": "/one,two,three", "{;count}": ";count=one,two,three",
		"{?count}": "?count=one,two,three", "{&count}": "&count=one,two,three",
		// undefined and missing variables drop out
		"{undef}": "", "{missing}": "", "O{.undef}": "O", "{?undef}": "",
		// literal-only and unterminated
		"plain/path":    "plain/path",
		"a{var}b":       "avalueb",
		"open{unclosed": "open{unclosed",
	}
	for tpl, want := range cases {
		if got := NewTemplate(tpl).Expand(vars); got != want {
			t.Errorf("Expand(%q) = %q want %q", tpl, got, want)
		}
	}
	// empty list / hash values expand to nothing
	ev := map[string]Value{"e": []string{}, "h": [][2]string{}}
	if got := NewTemplate("{e}{h}").Expand(ev); got != "" {
		t.Errorf("empty containers = %q", got)
	}
	if got := NewTemplate("{?e,h}").Expand(ev); got != "" {
		t.Errorf("empty named = %q", got)
	}
	// prefix on multibyte
	if got := NewTemplate("{var:2}").Expand(map[string]Value{"var": "héllo"}); got != "h%C3%A9" {
		t.Errorf("multibyte prefix = %q", got)
	}
	// Pattern accessor
	if NewTemplate("x{y}").Pattern() != "x{y}" {
		t.Error("Pattern")
	}
	// empty expression {}
	if got := NewTemplate("a{}b").Expand(vars); got != "ab" {
		t.Errorf("empty expr = %q", got)
	}
}

func TestExtractGolden(t *testing.T) {
	cases := []struct {
		tpl  string
		uri  string
		want map[string]any
	}{
		{"http://example.com/{var}", "http://example.com/value", map[string]any{"var": "value"}},
		{"http://example.com/{x,y}", "http://example.com/1024,768", map[string]any{"x": "1024", "y": "768"}},
		{"http://example.com/search{?q,lang}", "http://example.com/search?q=hello&lang=en", map[string]any{"q": "hello", "lang": "en"}},
		{"http://example.com/{+path}", "http://example.com/foo/bar", map[string]any{"path": "foo/bar"}},
		{"{scheme}://{host}/", "http://example.com/", map[string]any{"scheme": "http", "host": "example.com"}},
		{"http://example.com/{seg}/end", "http://example.com/mid/end", map[string]any{"seg": "mid"}},
		{"http://example.com/search{?q*}", "http://example.com/search?a=1&b=2", map[string]any{"q": map[string]string{"a": "1", "b": "2"}}},
		{"http://example.com{/seg}", "http://example.com/foo", map[string]any{"seg": "foo"}},
		{"http://example.com{.fmt}", "http://example.com.json", map[string]any{"fmt": "json"}},
		{"http://example.com/{a}{b}", "http://example.com/xy", map[string]any{"a": nil, "b": "xy"}},
	}
	for _, c := range cases {
		got := NewTemplate(c.tpl).Extract(c.uri)
		if !reflect.DeepEqual(got, c.want) {
			t.Errorf("Extract(%q, %q) = %#v want %#v", c.tpl, c.uri, got, c.want)
		}
	}
	// non-match returns nil
	if NewTemplate("http://example.com/nomatch").Extract("http://example.com/other") != nil {
		t.Error("expected nil for non-match")
	}
	// list explode extract
	got := NewTemplate("http://example.com/{list*}").Extract("http://example.com/red,green,blue")
	if !reflect.DeepEqual(got["list"], []string{"red,green,blue"}) {
		t.Errorf("list extract = %#v", got["list"])
	}
}
