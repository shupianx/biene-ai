package prompt

import "strings"

type Domain string

type Style string

type AgentProfile struct {
	Domain             Domain `json:"domain"`
	Style              Style  `json:"style"`
	CustomInstructions string `json:"custom_instructions"`
}

func DefaultProfile() AgentProfile {
	catalog := CurrentCatalog()
	return AgentProfile{
		Domain: catalog.DefaultDomain,
		Style:  catalog.DefaultStyle,
	}
}

func NormalizeProfile(profile AgentProfile) AgentProfile {
	catalog := CurrentCatalog()

	profile.Domain = Domain(strings.TrimSpace(string(profile.Domain)))
	if profile.Domain == "" || !catalog.hasDomain(profile.Domain) {
		profile.Domain = catalog.DefaultDomain
	}

	profile.Style = Style(strings.TrimSpace(string(profile.Style)))
	if profile.Style == "" || !catalog.hasStyle(profile.Style) {
		profile.Style = catalog.DefaultStyle
	}

	profile.CustomInstructions = strings.TrimSpace(profile.CustomInstructions)
	return profile
}
