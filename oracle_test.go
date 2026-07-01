// Copyright (c) the go-ruby-addressable/addressable authors
//
// SPDX-License-Identifier: BSD-3-Clause

package addressable

import (
	"os/exec"
	"strings"
	"testing"
)

// rubyBin locates a usable `ruby` with the `addressable` gem and RUBY_VERSION >= 4.0
// once. The oracle tests skip themselves when it is absent (the qemu cross-arch
// lanes and the Windows lane), so the deterministic suite alone drives the 100%
// gate there.
func rubyBin(t *testing.T) string {
	t.Helper()
	path, err := exec.LookPath("ruby")
	if err != nil {
		t.Skip("ruby not on PATH; skipping addressable oracle")
	}
	// Gate: version >= 4.0 and the gem loads.
	out, err := exec.Command(path, "-e",
		`exit(RUBY_VERSION >= "4.0" ? 0 : 3)`).CombinedOutput()
	if err != nil {
		t.Skipf("ruby version gate not met (< 4.0): %s", strings.TrimSpace(string(out)))
	}
	if err := exec.Command(path, "-e", `require "addressable/uri"`).Run(); err != nil {
		t.Skip("addressable gem not installed; skipping oracle")
	}
	return path
}

func rubyOut(t *testing.T, bin, script string) string {
	t.Helper()
	cmd := exec.Command(bin, "-raddressable/uri", "-raddressable/template", "-e",
		"$stdout.binmode\n"+script)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("ruby error: %v\nscript:\n%s\noutput:\n%s", err, script, out)
	}
	return string(out)
}

// TestOracleNormalize checks Normalize against the gem for a corpus of URIs.
func TestOracleNormalize(t *testing.T) {
	bin := rubyBin(t)
	corpus := []string{
		"HTTP://Example.COM:80/a/./b/../c/",
		"http://example.com/%7euser",
		"http://example.com/a/b/../../../c",
		"http://example.com",
		"https://example.com:443/x",
		"http://例え.テスト/ぱす",
		"http://example.com/?b=2&a=1",
		"mailto:user@example.com",
		"urn:isbn:0451450523",
	}
	for _, uri := range corpus {
		want := strings.TrimRight(rubyOut(t, bin,
			`print Addressable::URI.parse(%q{`+uri+`}).normalize.to_s`), "\n")
		if got := Parse(uri).Normalize().String(); got != want {
			t.Errorf("Normalize(%q) = %q, gem = %q", uri, got, want)
		}
	}
}

// TestOracleJoin checks reference resolution against the gem (RFC 3986 §5.4).
func TestOracleJoin(t *testing.T) {
	bin := rubyBin(t)
	base := "http://a/b/c/d;p?q"
	refs := []string{"g", "./g", "/g", "//g", "?y", "#s", "../g", "../..", "g;x", "", "g/../h"}
	for _, r := range refs {
		want := strings.TrimRight(rubyOut(t, bin,
			`print (Addressable::URI.parse(%q{`+base+`}) + %q{`+r+`}).to_s`), "\n")
		if got := Parse(base).Join(r).String(); got != want {
			t.Errorf("Join(%q, %q) = %q, gem = %q", base, r, got, want)
		}
	}
}

// TestOracleExpand checks RFC 6570 expansion against the gem.
func TestOracleExpand(t *testing.T) {
	bin := rubyBin(t)
	preamble := `vars = {
      "var"=>"value","hello"=>"Hello World!","half"=>"50%",
      "x"=>"1024","y"=>"768","empty"=>"","path"=>"/foo/bar",
      "list"=>["red","green","blue"],
      "keys"=>{"semi"=>";","dot"=>".","comma"=>","}
    }` + "\n"
	tvars := map[string]Value{
		"var": "value", "hello": "Hello World!", "half": "50%",
		"x": "1024", "y": "768", "empty": "", "path": "/foo/bar",
		"list": []string{"red", "green", "blue"},
		"keys": [][2]string{{"semi", ";"}, {"dot", "."}, {"comma", ","}},
	}
	tpls := []string{
		"{var}", "{+hello}", "{#x,hello,y}", "X{.list*}", "{/list*,path:4}",
		"{;keys*}", "{?keys*}", "{&x,y,empty}", "{list}", "{keys}", "{+keys*}",
	}
	for _, tpl := range tpls {
		want := strings.TrimRight(rubyOut(t, bin, preamble+
			`print Addressable::Template.new(%q{`+tpl+`}).expand(vars).to_s`), "\n")
		if got := NewTemplate(tpl).Expand(tvars); got != want {
			t.Errorf("Expand(%q) = %q, gem = %q", tpl, got, want)
		}
	}
}
