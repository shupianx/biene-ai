package skills

import (
	"slices"
	"strings"
	"unicode"
)

// SelectForText chooses at most one best-matching skill for the given user text.
// Explicit `$skill-name` references win immediately. Otherwise a lightweight
// lexical score is used against the skill name and description.
func SelectForText(text string, metas []Metadata) *Metadata {
	text = strings.TrimSpace(text)
	if text == "" || len(metas) == 0 {
		return nil
	}

	if explicit := selectExplicit(text, metas); explicit != nil {
		return explicit
	}

	msgNorm := normalizeText(text)
	msgTokens := tokenSet(msgNorm)
	if len(msgTokens) == 0 {
		return nil
	}

	var best *Metadata
	bestScore := 0.0

	for i := range metas {
		score := scoreMetadata(msgNorm, msgTokens, metas[i])
		if score > bestScore {
			best = &metas[i]
			bestScore = score
		}
	}

	if bestScore < 1.6 {
		return nil
	}
	return best
}

func selectExplicit(text string, metas []Metadata) *Metadata {
	lower := strings.ToLower(text)
	for i := range metas {
		name := normalizeSkillName(metas[i].Name)
		if name == "" {
			continue
		}
		if strings.Contains(lower, "$"+name) {
			return &metas[i]
		}
	}
	return nil
}

func scoreMetadata(msgNorm string, msgTokens []string, meta Metadata) float64 {
	nameNorm := normalizeText(strings.ReplaceAll(meta.Name, "-", " "))
	descNorm := normalizeText(meta.Description)
	score := 0.0

	if nameNorm != "" && strings.Contains(msgNorm, nameNorm) {
		score += 2.5
	}
	if descNorm != "" && strings.Contains(msgNorm, descNorm) {
		score += 1.75
	}

	nameTokens := tokenSet(nameNorm)
	descTokens := tokenSet(descNorm)
	overlap := overlapCount(msgTokens, nameTokens)
	if overlap > 0 {
		score += float64(overlap) * 1.2
	}

	descOverlap := overlapCount(msgTokens, descTokens)
	if descOverlap >= 2 {
		score += float64(descOverlap) * 0.45
	}

	if overlap == 0 && descOverlap < 2 {
		return 0
	}
	return score
}

func overlapCount(a, b []string) int {
	if len(a) == 0 || len(b) == 0 {
		return 0
	}
	count := 0
	for _, item := range a {
		if slices.Contains(b, item) {
			count++
		}
	}
	return count
}

func normalizeSkillName(name string) string {
	name = strings.ToLower(strings.TrimSpace(name))
	name = strings.ReplaceAll(name, "_", "-")
	name = strings.ReplaceAll(name, " ", "-")
	return name
}

func normalizeText(text string) string {
	text = strings.ToLower(strings.TrimSpace(text))
	var b strings.Builder
	lastSpace := false
	for _, r := range text {
		switch {
		case unicode.IsLetter(r), unicode.IsNumber(r):
			b.WriteRune(r)
			lastSpace = false
		case r == '-' || r == '_':
			if !lastSpace {
				b.WriteByte(' ')
				lastSpace = true
			}
		default:
			if !lastSpace {
				b.WriteByte(' ')
				lastSpace = true
			}
		}
	}
	return strings.TrimSpace(b.String())
}

func tokenSet(text string) []string {
	if text == "" {
		return nil
	}
	parts := strings.Fields(text)
	seen := make(map[string]struct{}, len(parts))
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		if len([]rune(part)) < 2 {
			continue
		}
		if _, ok := seen[part]; ok {
			continue
		}
		seen[part] = struct{}{}
		out = append(out, part)
	}
	return out
}
