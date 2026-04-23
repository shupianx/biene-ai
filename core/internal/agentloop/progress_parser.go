package agentloop

import (
	"strings"
)

// writeProgressParser extracts the file_path (once it finishes streaming)
// and a running count of file_text bytes from the partial JSON input of a
// write-class tool call. It is tolerant of arbitrary key ordering and
// whitespace, and never rescans more of the buffer than necessary.
//
// The parser is stateless between calls: every invocation of Snapshot
// re-scans the current buffer. Inputs are small (a tool_use payload) so
// the cost is negligible.
type writeProgressParser struct {
	buf []byte
}

// writeProgressSnapshot is what the parser reports each time it is asked.
type writeProgressSnapshot struct {
	FilePath      string
	PathKnown     bool
	FileTextBytes int
}

func (p *writeProgressParser) Append(chunk string) {
	p.buf = append(p.buf, chunk...)
}

func (p *writeProgressParser) Snapshot() writeProgressSnapshot {
	var out writeProgressSnapshot
	s := string(p.buf)

	if idx := findValueStart(s, "file_path"); idx >= 0 {
		value, done := readJSONStringValue(s[idx:])
		if done {
			out.FilePath = value
			out.PathKnown = true
		}
	}

	if idx := findValueStart(s, "file_text"); idx >= 0 {
		out.FileTextBytes = countJSONStringBytes(s[idx:])
	}

	return out
}

// findValueStart searches for `"<key>"` followed (with optional
// whitespace) by `:` and an opening `"`. Returns the index just after
// the opening quote, or -1 if the pattern hasn't arrived yet.
func findValueStart(s, key string) int {
	needle := `"` + key + `"`
	search := 0
	for {
		rel := strings.Index(s[search:], needle)
		if rel < 0 {
			return -1
		}
		i := search + rel + len(needle)
		// Skip whitespace
		for i < len(s) && isJSONSpace(s[i]) {
			i++
		}
		if i >= len(s) || s[i] != ':' {
			// Not a key position (could be inside some other string value).
			// Advance past this occurrence and keep looking.
			search = search + rel + len(needle)
			continue
		}
		i++
		for i < len(s) && isJSONSpace(s[i]) {
			i++
		}
		if i >= len(s) || s[i] != '"' {
			return -1 // value hasn't started (or is null, etc.)
		}
		return i + 1
	}
}

// readJSONStringValue consumes a JSON string starting at the first
// content char (after the opening "). Returns the decoded value and true
// if the closing quote has arrived. If still streaming, returns what it
// has with done=false.
func readJSONStringValue(s string) (string, bool) {
	var sb strings.Builder
	i := 0
	for i < len(s) {
		c := s[i]
		if c == '\\' {
			if i+1 >= len(s) {
				return sb.String(), false
			}
			esc := s[i+1]
			switch esc {
			case '"', '\\', '/':
				sb.WriteByte(esc)
				i += 2
			case 'n':
				sb.WriteByte('\n')
				i += 2
			case 'r':
				sb.WriteByte('\r')
				i += 2
			case 't':
				sb.WriteByte('\t')
				i += 2
			case 'b':
				sb.WriteByte('\b')
				i += 2
			case 'f':
				sb.WriteByte('\f')
				i += 2
			case 'u':
				if i+6 > len(s) {
					return sb.String(), false
				}
				// Don't bother decoding the codepoint; values we care about
				// (file paths) rarely use \uXXXX. Just emit a placeholder
				// byte so byte counts stay rough but monotonic.
				sb.WriteString(s[i : i+6])
				i += 6
			default:
				sb.WriteByte(esc)
				i += 2
			}
			continue
		}
		if c == '"' {
			return sb.String(), true
		}
		sb.WriteByte(c)
		i++
	}
	return sb.String(), false
}

// countJSONStringBytes walks a JSON string body counting unescaped bytes
// until the closing quote (or end of buffer). Used for file_text where we
// want a growing size indicator without materializing the whole value.
func countJSONStringBytes(s string) int {
	count := 0
	i := 0
	for i < len(s) {
		c := s[i]
		if c == '\\' {
			if i+1 >= len(s) {
				return count
			}
			esc := s[i+1]
			if esc == 'u' {
				if i+6 > len(s) {
					return count
				}
				count++
				i += 6
				continue
			}
			count++
			i += 2
			continue
		}
		if c == '"' {
			return count
		}
		count++
		i++
	}
	return count
}

func isJSONSpace(c byte) bool {
	return c == ' ' || c == '\t' || c == '\n' || c == '\r'
}
