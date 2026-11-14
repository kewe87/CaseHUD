package sketch

import (
	"strings"
	"unicode/utf8"
)

// cp1252High maps the Windows-1252 characters of 0x80..0x9F back to their
// byte; every other character up to U+00FF is its own byte (Latin-1).
var cp1252High = map[rune]byte{
	'€': 0x80, '‚': 0x82, 'ƒ': 0x83, '„': 0x84, '…': 0x85, '†': 0x86, '‡': 0x87,
	'ˆ': 0x88, '‰': 0x89, 'Š': 0x8A, '‹': 0x8B, 'Œ': 0x8C, 'Ž': 0x8E,
	'‘': 0x91, '’': 0x92, '“': 0x93, '”': 0x94, '•': 0x95, '–': 0x96, '—': 0x97,
	'˜': 0x98, '™': 0x99, 'š': 0x9A, '›': 0x9B, 'œ': 0x9C, 'ž': 0x9E, 'Ÿ': 0x9F,
}

// FixMojibake repairs text that was UTF-8 encoded twice: a client read UTF-8
// bytes as Windows-1252 (or Latin-1) and encoded the result as UTF-8 again,
// so "Kühler" arrives as "KÃ¼hler". The text is changed only when every
// character maps back to one byte and those bytes are valid UTF-8 with at
// least one multi-byte character. Ordinary text with a stray "Ã" does not
// qualify (a lone lead byte is not valid UTF-8) and is returned unchanged.
func FixMojibake(s string) string {
	if !strings.ContainsAny(s, "ÃÂ") { // lead bytes of the Latin-1 range
		return s
	}
	b := make([]byte, 0, len(s))
	for _, r := range s {
		switch {
		case r < 0x80:
			b = append(b, byte(r))
		case r <= 0xFF && (r < 0x80 || r > 0x9F):
			b = append(b, byte(r))
		case r >= 0x80 && r <= 0x9F: // C1 control: Windows-1252 leaves some bytes undefined
			b = append(b, byte(r))
		default:
			c, ok := cp1252High[r]
			if !ok {
				return s
			}
			b = append(b, c)
		}
	}
	if !utf8.Valid(b) || utf8.RuneCount(b) == len(b) {
		return s
	}
	return string(b)
}

// repairText fixes the free text of a sketch (the part labels).
func (s *Sketch) repairText() {
	for i := range s.Parts {
		s.Parts[i].Label = FixMojibake(s.Parts[i].Label)
	}
}
