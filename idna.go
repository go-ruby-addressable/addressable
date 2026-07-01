// Copyright (c) the go-ruby-addressable/addressable authors
//
// SPDX-License-Identifier: BSD-3-Clause

package addressable

import "strings"

// idnToASCII converts a (already-lowercased) host to its ASCII-Compatible Encoding
// form: each dot-separated label that contains non-ASCII runes is Punycode-encoded
// and prefixed with "xn--". Pure-ASCII labels pass through unchanged. This mirrors
// Addressable::URI#normalized_host's IDN handling (the gem uses libidn / a pure-Ruby
// punycode fallback; we implement RFC 3492 directly so the build stays cgo-free).
func idnToASCII(host string) string {
	if isASCII(host) {
		return host
	}
	labels := strings.Split(host, ".")
	for i, label := range labels {
		if isASCII(label) {
			continue
		}
		labels[i] = "xn--" + punycodeEncode(label)
	}
	return strings.Join(labels, ".")
}

func isASCII(s string) bool {
	for i := 0; i < len(s); i++ {
		if s[i] >= 0x80 {
			return false
		}
	}
	return true
}

// RFC 3492 Punycode parameters (bootstring for IDNA).
const (
	pyBase        = 36
	pyTMin        = 1
	pyTMax        = 26
	pySkew        = 38
	pyDamp        = 700
	pyInitialBias = 72
	pyInitialN    = 128
)

func pyAdapt(delta, numPoints int, firstTime bool) int {
	if firstTime {
		delta /= pyDamp
	} else {
		delta /= 2
	}
	delta += delta / numPoints
	k := 0
	for delta > ((pyBase-pyTMin)*pyTMax)/2 {
		delta /= pyBase - pyTMin
		k += pyBase
	}
	return k + (pyBase-pyTMin+1)*delta/(delta+pySkew)
}

// pyDigit maps a value 0..35 to its Punycode ASCII character.
func pyDigit(d int) byte {
	if d < 26 {
		return byte('a' + d)
	}
	return byte('0' + d - 26)
}

// punycodeEncode encodes a Unicode label (no "xn--" prefix) per RFC 3492.
func punycodeEncode(input string) string {
	runes := []rune(input)
	var out strings.Builder

	// Copy basic (ASCII) code points to the output.
	basic := 0
	for _, r := range runes {
		if r < 0x80 {
			out.WriteByte(byte(r))
			basic++
		}
	}
	handled := basic
	if basic > 0 {
		out.WriteByte('-')
	}

	n := pyInitialN
	delta := 0
	bias := pyInitialBias

	total := len(runes)
	for handled < total {
		// Find the smallest code point >= n among the remaining.
		m := 0x7fffffff
		for _, r := range runes {
			if int(r) >= n && int(r) < m {
				m = int(r)
			}
		}
		delta += (m - n) * (handled + 1)
		n = m
		for _, r := range runes {
			if int(r) < n {
				delta++
			}
			if int(r) == n {
				q := delta
				for k := pyBase; ; k += pyBase {
					t := k - bias
					if t < pyTMin {
						t = pyTMin
					} else if t > pyTMax {
						t = pyTMax
					}
					if q < t {
						break
					}
					out.WriteByte(pyDigit(t + (q-t)%(pyBase-t)))
					q = (q - t) / (pyBase - t)
				}
				out.WriteByte(pyDigit(q))
				bias = pyAdapt(delta, handled+1, handled == basic)
				delta = 0
				handled++
			}
		}
		delta++
		n++
	}
	return out.String()
}
