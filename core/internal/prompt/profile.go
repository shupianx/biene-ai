package prompt

import "strings"

type Domain string

const (
	DomainGeneral Domain = "general"
	DomainCoding  Domain = "coding"
)

type Style string

const (
	StyleBalanced  Style = "balanced"
	StyleConcise   Style = "concise"
	StyleThorough  Style = "thorough"
	StyleSkeptical Style = "skeptical"
	StyleProactive Style = "proactive"
)

type AgentProfile struct {
	Domain             Domain `json:"domain"`
	Style              Style  `json:"style"`
	CustomInstructions string `json:"custom_instructions"`
}

func DefaultProfile() AgentProfile {
	return AgentProfile{
		Domain: DomainCoding,
		Style:  StyleBalanced,
	}
}

func NormalizeProfile(profile AgentProfile) AgentProfile {
	switch profile.Domain {
	case DomainGeneral, DomainCoding:
	default:
		profile.Domain = DomainCoding
	}

	switch profile.Style {
	case StyleBalanced, StyleConcise, StyleThorough, StyleSkeptical, StyleProactive:
	default:
		profile.Style = StyleBalanced
	}

	profile.CustomInstructions = strings.TrimSpace(profile.CustomInstructions)
	return profile
}
