package ideconfig

import "strings"

// StripComments removes JSONC comments (line and block) from the input string,
// preserving comment-like sequences inside quoted strings. It also removes
// trailing commas before } or ].
func StripComments(input string) string {
	var buf strings.Builder
	buf.Grow(len(input))

	i := 0
	n := len(input)

	for i < n {
		ch := input[i]

		// --- Quoted string: copy verbatim, handling escapes ---
		if ch == '"' {
			buf.WriteByte(ch)
			i++
			for i < n {
				sc := input[i]
				buf.WriteByte(sc)
				if sc == '\\' {
					// Escaped character — copy the next byte too.
					i++
					if i < n {
						buf.WriteByte(input[i])
						i++
					}
					continue
				}
				if sc == '"' {
					i++
					break
				}
				i++
			}
			continue
		}

		// --- Line comment: // ---
		if ch == '/' && i+1 < n && input[i+1] == '/' {
			// Skip to end of line (but keep the newline itself).
			i += 2
			for i < n && input[i] != '\n' {
				i++
			}
			continue
		}

		// --- Block comment: /* ... */ ---
		if ch == '/' && i+1 < n && input[i+1] == '*' {
			i += 2
			for i < n {
				if input[i] == '*' && i+1 < n && input[i+1] == '/' {
					i += 2
					break
				}
				i++
			}
			continue
		}

		// --- Normal character ---
		buf.WriteByte(ch)
		i++
	}

	return removeTrailingCommas(buf.String())
}

// removeTrailingCommas strips commas that appear (possibly with whitespace)
// directly before a closing } or ].
func removeTrailingCommas(s string) string {
	var buf strings.Builder
	buf.Grow(len(s))

	n := len(s)
	for i := 0; i < n; i++ {
		ch := s[i]

		// Inside a string — copy verbatim.
		if ch == '"' {
			buf.WriteByte(ch)
			i++
			for i < n {
				sc := s[i]
				buf.WriteByte(sc)
				if sc == '\\' {
					i++
					if i < n {
						buf.WriteByte(s[i])
						i++
					}
					continue
				}
				if sc == '"' {
					break
				}
				i++
			}
			continue
		}

		if ch == ',' {
			// Look ahead past whitespace to see if the next non-whitespace
			// character is } or ].
			j := i + 1
			for j < n && isJSONWhitespace(s[j]) {
				j++
			}
			if j < n && (s[j] == '}' || s[j] == ']') {
				// Drop the comma; keep the whitespace and the closing bracket
				// for the next iteration.
				continue
			}
		}

		buf.WriteByte(ch)
	}

	return buf.String()
}

// isJSONWhitespace returns true for characters that are insignificant
// whitespace in JSON.
func isJSONWhitespace(c byte) bool {
	return c == ' ' || c == '\t' || c == '\n' || c == '\r'
}

// ExtractPreamble splits input into the text before the first '{' (preamble)
// and the text from the first '{' onwards (jsonBody). If no '{' is found,
// the entire input is returned as preamble and jsonBody is empty.
func ExtractPreamble(input string) (preamble string, jsonBody string) {
	idx := strings.IndexByte(input, '{')
	if idx < 0 {
		return input, ""
	}
	return input[:idx], input[idx:]
}
