package prompt

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
)

//go:embed catalog.json
var embeddedCatalogJSON []byte

type DomainDefinition struct {
	Value Domain   `json:"value"`
	Rules []string `json:"rules"`
}

type StyleDefinition struct {
	Value Style    `json:"value"`
	Rules []string `json:"rules"`
}

type Catalog struct {
	DefaultDomain Domain             `json:"default_domain"`
	DefaultStyle  Style              `json:"default_style"`
	Domains       []DomainDefinition `json:"domains"`
	Styles        []StyleDefinition  `json:"styles"`
}

var (
	catalogOnce sync.Once
	catalogMu   sync.RWMutex
	catalogData = Catalog{}
)

func CurrentCatalog() Catalog {
	ensureCatalogLoaded()

	catalogMu.RLock()
	defer catalogMu.RUnlock()
	return cloneCatalog(catalogData)
}

func ensureCatalogLoaded() {
	catalogOnce.Do(func() {
		var catalog Catalog
		if err := json.Unmarshal(embeddedCatalogJSON, &catalog); err != nil {
			panic(fmt.Errorf("parse embedded prompt catalog: %w", err))
		}
		normalized := normalizeCatalog(catalog)

		catalogMu.Lock()
		catalogData = normalized
		catalogMu.Unlock()
	})
}

func cloneCatalog(src Catalog) Catalog {
	clone := Catalog{
		DefaultDomain: src.DefaultDomain,
		DefaultStyle:  src.DefaultStyle,
		Domains:       make([]DomainDefinition, len(src.Domains)),
		Styles:        make([]StyleDefinition, len(src.Styles)),
	}
	for i, domain := range src.Domains {
		clone.Domains[i] = domain
		clone.Domains[i].Rules = append([]string(nil), domain.Rules...)
	}
	for i, style := range src.Styles {
		clone.Styles[i] = style
		clone.Styles[i].Rules = append([]string(nil), style.Rules...)
	}
	return clone
}

func normalizeCatalog(catalog Catalog) Catalog {
	catalog.Domains = normalizeDomains(catalog.Domains)
	catalog.Styles = normalizeStyles(catalog.Styles)

	if len(catalog.Domains) == 0 {
		panic("prompt catalog must define at least one domain")
	}
	if len(catalog.Styles) == 0 {
		panic("prompt catalog must define at least one style")
	}

	if catalog.DefaultDomain == "" || !catalog.hasDomain(catalog.DefaultDomain) {
		catalog.DefaultDomain = catalog.Domains[0].Value
	}
	if catalog.DefaultStyle == "" || !catalog.hasStyle(catalog.DefaultStyle) {
		catalog.DefaultStyle = catalog.Styles[0].Value
	}

	return catalog
}

func normalizeDomains(domains []DomainDefinition) []DomainDefinition {
	seen := make(map[Domain]struct{}, len(domains))
	out := make([]DomainDefinition, 0, len(domains))
	for _, domain := range domains {
		domain.Value = Domain(strings.TrimSpace(string(domain.Value)))
		if domain.Value == "" {
			continue
		}
		if _, ok := seen[domain.Value]; ok {
			continue
		}
		seen[domain.Value] = struct{}{}
		domain.Rules = normalizeRules(domain.Rules)
		out = append(out, domain)
	}
	return out
}

func normalizeStyles(styles []StyleDefinition) []StyleDefinition {
	seen := make(map[Style]struct{}, len(styles))
	out := make([]StyleDefinition, 0, len(styles))
	for _, style := range styles {
		style.Value = Style(strings.TrimSpace(string(style.Value)))
		if style.Value == "" {
			continue
		}
		if _, ok := seen[style.Value]; ok {
			continue
		}
		seen[style.Value] = struct{}{}
		style.Rules = normalizeRules(style.Rules)
		out = append(out, style)
	}
	return out
}

func normalizeRules(rules []string) []string {
	out := make([]string, 0, len(rules))
	for _, rule := range rules {
		rule = strings.TrimSpace(rule)
		if rule == "" {
			continue
		}
		out = append(out, rule)
	}
	return out
}

func (c Catalog) hasDomain(value Domain) bool {
	for _, domain := range c.Domains {
		if domain.Value == value {
			return true
		}
	}
	return false
}

func (c Catalog) hasStyle(value Style) bool {
	for _, style := range c.Styles {
		if style.Value == value {
			return true
		}
	}
	return false
}

func (c Catalog) domainDefinition(value Domain) DomainDefinition {
	for _, domain := range c.Domains {
		if domain.Value == value {
			return domain
		}
	}
	for _, domain := range c.Domains {
		if domain.Value == c.DefaultDomain {
			return domain
		}
	}
	return c.Domains[0]
}

func (c Catalog) styleDefinition(value Style) StyleDefinition {
	for _, style := range c.Styles {
		if style.Value == value {
			return style
		}
	}
	for _, style := range c.Styles {
		if style.Value == c.DefaultStyle {
			return style
		}
	}
	return c.Styles[0]
}
