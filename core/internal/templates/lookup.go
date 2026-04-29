package templates

import "strings"

// LookupContextWindow returns the context-window size declared by the
// builtin template that matches the given (provider, model, baseURL)
// triple. Comparison is case-insensitive on provider and trims trailing
// slashes on baseURL so superficially-different but semantically-equal
// values still match.
//
// `ok` is false when no template matches OR when the matched template
// has no declared window — in both cases the caller should fall back
// to its own default rather than guess.
func LookupContextWindow(provider, model, baseURL string) (int, bool) {
	for _, vendor := range Builtin {
		if !providersEqual(vendor.Provider, provider) {
			continue
		}
		if !urlsEqual(vendor.BaseURL, baseURL) {
			continue
		}
		for _, t := range vendor.Models {
			if t.Model != model {
				continue
			}
			if t.ContextWindow > 0 {
				return t.ContextWindow, true
			}
			return 0, false
		}
	}
	return 0, false
}

func providersEqual(a, b string) bool {
	return strings.EqualFold(strings.TrimSpace(a), strings.TrimSpace(b))
}

func urlsEqual(a, b string) bool {
	return strings.TrimRight(strings.TrimSpace(a), "/") == strings.TrimRight(strings.TrimSpace(b), "/")
}

// ModelsForVendor returns the model strings declared under the given
// (provider, baseURL) vendor pair. Used by callers that need to
// enumerate "what models does this vendor expose?" — e.g. the ChatGPT
// OAuth path, which only routes to models OpenAI ships behind its
// account, but which we'd otherwise have to maintain in a second
// hardcoded list.
//
// Returns an empty slice when no vendor matches; never nil so callers
// can range over the result without a guard.
func ModelsForVendor(provider, baseURL string) []string {
	for _, vendor := range Builtin {
		if !providersEqual(vendor.Provider, provider) {
			continue
		}
		if !urlsEqual(vendor.BaseURL, baseURL) {
			continue
		}
		out := make([]string, 0, len(vendor.Models))
		for _, t := range vendor.Models {
			if t.Model != "" {
				out = append(out, t.Model)
			}
		}
		return out
	}
	return []string{}
}
