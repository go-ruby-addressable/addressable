// Copyright (c) the go-ruby-addressable/addressable authors
//
// SPDX-License-Identifier: BSD-3-Clause

package addressable

import "strings"

// Character classes mirror Addressable::URI::CharacterClasses. Each set names the
// characters that are allowed to appear *unescaped* within a component; every other
// byte is percent-encoded by EncodeComponent.
const (
	// ClassUnreserved is RFC 3986 unreserved: ALPHA / DIGIT / "-" / "." / "_" / "~".
	ClassUnreserved = "A-Za-z0-9\\-._~"
	// ClassReserved is RFC 3986 reserved (gen-delims + sub-delims).
	ClassReserved = ":/?#\\[\\]@!$&'()*+,;="
	// ClassPath is the set kept literal inside a normalized path.
	ClassPath = ClassUnreserved + ":@!$&'()*+,;=/"
	// ClassQuery is the set kept literal inside a normalized query.
	ClassQuery = ClassUnreserved + ":@!$'()*+,;=/?"
	// ClassFragment is the set kept literal inside a normalized fragment.
	ClassFragment = ClassUnreserved + ":@!$&'()*+,;=/?"
	// ClassAuthority is the set kept literal inside a normalized authority.
	ClassAuthority = ClassUnreserved + ":@!$&'()*+,;=\\[\\]"
	// ClassHost is the set kept literal inside a normalized host.
	ClassHost = ClassUnreserved + "!$&'()*+,;=\\[\\]"
	// ClassScheme is the set kept literal inside a scheme.
	ClassScheme = "A-Za-z0-9\\-+."
)

// buildAllowed expands a bracket-style character-class spec (as used above) into a
// 256-entry lookup table of allowed bytes.
func buildAllowed(spec string) *[256]bool {
	var tbl [256]bool
	i := 0
	// Unescape the "\\" and "\[" / "\]" forms used in the class constants.
	runes := []byte(spec)
	for i < len(runes) {
		c := runes[i]
		if c == '\\' && i+1 < len(runes) {
			tbl[runes[i+1]] = true
			i += 2
			continue
		}
		// Range like A-Z: a literal, a dash not at the ends, then a literal.
		if i+2 < len(runes) && runes[i+1] == '-' && runes[i+2] != '\\' {
			lo, hi := runes[i], runes[i+2]
			for b := int(lo); b <= int(hi); b++ {
				tbl[b] = true
			}
			i += 3
			continue
		}
		tbl[c] = true
		i++
	}
	return &tbl
}

const upperHex = "0123456789ABCDEF"

func isHex(b byte) bool {
	return (b >= '0' && b <= '9') || (b >= 'a' && b <= 'f') || (b >= 'A' && b <= 'F')
}

func hexVal(b byte) byte {
	switch {
	case b >= '0' && b <= '9':
		return b - '0'
	case b >= 'a' && b <= 'f':
		return b - 'a' + 10
	default:
		return b - 'A' + 10
	}
}

// EncodeComponent percent-encodes every byte of s that is not present in the given
// character-class spec. It mirrors Addressable::URI.encode_component with a string
// character class.
func EncodeComponent(s, characterClass string) string {
	allowed := buildAllowed(characterClass)
	var b strings.Builder
	for i := 0; i < len(s); i++ {
		c := s[i]
		if allowed[c] {
			b.WriteByte(c)
			continue
		}
		b.WriteByte('%')
		b.WriteByte(upperHex[c>>4])
		b.WriteByte(upperHex[c&0x0f])
	}
	return b.String()
}

// UnencodeComponent percent-decodes every %XX sequence in s. Invalid escapes are
// left verbatim, matching Addressable::URI.unencode_component.
func UnencodeComponent(s string) string {
	if !strings.ContainsRune(s, '%') {
		return s
	}
	var b strings.Builder
	for i := 0; i < len(s); i++ {
		if s[i] == '%' && i+2 < len(s) && isHex(s[i+1]) && isHex(s[i+2]) {
			b.WriteByte(hexVal(s[i+1])<<4 | hexVal(s[i+2]))
			i += 2
			continue
		}
		b.WriteByte(s[i])
	}
	return b.String()
}
