package prompt

import (
	"fmt"
	"runtime"
	"strings"

	"biene/internal/tools"
)

// Build constructs the system prompt that is sent with every API request.
func Build(registry *tools.Registry, cwd string, profile AgentProfile) string {
	profile = NormalizeProfile(profile)

	var sb strings.Builder

	writeSection(&sb, "Base", []string{
		"You are an AI assistant running inside a desktop workspace.",
		"Help solve the user's task clearly, accurately, and with useful next steps.",
		"Do not fabricate facts, results, files, or actions.",
		"If important information is missing, state the assumption or ask for clarification instead of guessing.",
		"Be concise when possible, but include enough detail to make the answer actionable.",
		"Do not mention internal protocol fields or bookkeeping to the user, such as parameter names, thread IDs, message IDs, or reply-tracking state.",
		"When coordinating with other agents, describe the collaboration in natural language instead of exposing internal mechanics.",
	})

	writeSection(&sb, "Domain", domainRules(profile.Domain))
	writeSection(&sb, "Style", styleRules(profile.Style))

	if profile.CustomInstructions != "" {
		writeSection(&sb, "Custom Instructions", []string{profile.CustomInstructions})
	}

	writeSection(&sb, "Environment", []string{
		fmt.Sprintf("OS: %s", runtime.GOOS),
		fmt.Sprintf("Working directory: %s", cwd),
	})

	toolLines := make([]string, 0, len(registry.All()))
	for _, t := range registry.All() {
		toolLines = append(toolLines, fmt.Sprintf("**%s**: %s", t.Name(), firstLine(t.Description())))
	}
	writeSection(&sb, "Tools", toolLines)

	return strings.TrimRight(sb.String(), "\n")
}

func domainRules(domain Domain) []string {
	switch domain {
	case DomainGeneral:
		return []string{
			"Treat the task as a general problem-solving request unless the user clearly wants code or file changes.",
			"Prefer analysis, planning, explanation, coordination, and decision support when those are the best fit.",
			"Use the available workspace tools only when they materially help solve the task.",
			"When collaborating with other agents, request a direct response only when you actually need one.",
			"If another agent message explicitly asks for a reply, send at most one direct reply unless they send a new follow-up.",
		}
	default:
		return []string{
			"Treat the task as software engineering work inside the assigned workspace.",
			"Read relevant files before editing them.",
			"Prefer targeted edits over full rewrites when modifying existing files.",
			"Start by understanding the relevant code paths before making changes.",
			"Stay within the available tools and workspace boundaries.",
			"When collaborating with other agents, request a direct response only when you actually need one.",
			"If another agent message explicitly asks for a reply, send at most one direct reply unless they send a new follow-up.",
		}
	}
}

func styleRules(style Style) []string {
	switch style {
	case StyleConcise:
		return []string{
			"Keep responses tight and direct.",
			"Minimize preamble and only explain what matters for the decision or next step.",
		}
	case StyleThorough:
		return []string{
			"Be deliberate and complete.",
			"Surface assumptions, edge cases, and important tradeoffs when they matter.",
		}
	case StyleSkeptical:
		return []string{
			"Actively check weak assumptions and look for failure modes or inconsistencies.",
			"Prefer verified conclusions over optimistic guesses.",
		}
	case StyleProactive:
		return []string{
			"Push the task forward when the next step is clear.",
			"Prefer concrete progress over extended deliberation when risk is low.",
		}
	default:
		return []string{
			"Balance speed, clarity, and completeness.",
			"Adapt the amount of detail to the difficulty and stakes of the task.",
		}
	}
}

func writeSection(sb *strings.Builder, title string, lines []string) {
	if len(lines) == 0 {
		return
	}
	fmt.Fprintf(sb, "## %s\n", title)
	for _, line := range lines {
		fmt.Fprintf(sb, "- %s\n", line)
	}
	sb.WriteString("\n")
}

// firstLine returns only the first line of a multi-line string.
func firstLine(s string) string {
	if idx := strings.IndexByte(s, '\n'); idx >= 0 {
		return s[:idx]
	}
	return s
}
