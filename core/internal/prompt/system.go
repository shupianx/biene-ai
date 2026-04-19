package prompt

import (
	"fmt"
	"runtime"
	"strings"

	"biene/internal/skills"
	"biene/internal/tools"
)

type AgentIdentity struct {
	ID      string
	Name    string
	WorkDir string
}

// Build constructs the system prompt that is sent with every API request.
func Build(
	registry *tools.Registry,
	cwd string,
	profile AgentProfile,
	self AgentIdentity,
	installedSkills []skills.Metadata,
	activatedSkills []skills.Definition,
) string {
	profile = NormalizeProfile(profile)
	catalog := CurrentCatalog()
	domain := catalog.domainDefinition(profile.Domain)
	style := catalog.styleDefinition(profile.Style)

	var sb strings.Builder

	writeSection(&sb, "Base", []string{
		"You are an AI assistant running inside a desktop workspace.",
		"Help solve the user's task clearly, accurately, and with useful next steps.",
		"Your default response mode is plain text. Do not call tools unless they are necessary to complete the task or the current message gives an explicit instruction that requires a tool-mediated action.",
		"If this agent has installed skills relevant to the current task, follow any activated skill instructions as part of your normal behavior.",
		"When the user asks what skills are installed or available for this agent, use list_skills instead of guessing from memory.",
		"Use run_command only when command output is actually needed, such as running tests, builds, linters, generators, or project CLIs. Prefer direct file tools when a command is not necessary.",
		"Use write_file or edit_file only when the user gives a concrete instruction to create or modify workspace files. Answering a question by writing the answer into a file is incorrect.",
		"Use send_to_agent only when the user clearly wants agent collaboration, file handoff, or delegation, or when you are sending work, answers, or results back to another agent.",
		"If an incoming agent message is asking you for work, an answer, a decision, or a result that should go back to the sender, do not answer only in the local chat. Use send_to_agent to send that response back to the sender.",
		"Do not fabricate facts, results, files, or actions.",
		"If important information is missing, state the assumption or ask for clarification instead of guessing.",
		"Be concise when possible, but include enough detail to make the answer actionable.",
		"Do not mention internal protocol fields or bookkeeping to the user, such as parameter names, thread IDs, message IDs, or reply-tracking state.",
		"When coordinating with other agents, describe the collaboration in natural language instead of exposing internal mechanics.",
	})

	writeSection(&sb, "Domain", domain.Rules)
	writeSection(&sb, "Style", style.Rules)

	writeSection(&sb, "Current Agent", []string{
		fmt.Sprintf("Agent name: %s", self.Name),
		fmt.Sprintf("Agent ID: %s", self.ID),
		fmt.Sprintf("Agent workspace: %s", self.WorkDir),
		"If you need to confirm your own identity or compare yourself with other agents, use this section or list_agents. list_agents also shows your current agent entry.",
	})

	if profile.CustomInstructions != "" {
		writeSection(&sb, "Custom Instructions", []string{profile.CustomInstructions})
	}

	if len(installedSkills) > 0 {
		skillLines := make([]string, 0, len(installedSkills)+1)
		skillLines = append(skillLines, "This agent has the following installed skills available in its own workspace. Only load the full instructions for the skill(s) activated for the current request.")
		for _, skill := range installedSkills {
			skillLines = append(skillLines, fmt.Sprintf("**%s**: %s", skill.Name, firstLine(skill.Description)))
		}
		writeSection(&sb, "Installed Skills", skillLines)
	}

	if len(activatedSkills) > 0 {
		activationLines := make([]string, 0, len(activatedSkills))
		for _, skill := range activatedSkills {
			activationLines = append(activationLines, fmt.Sprintf("%s: %s", skill.Name, firstLine(skill.Description)))
		}
		writeSection(&sb, "Activated Skills", activationLines)

		for _, skill := range activatedSkills {
			fmt.Fprintf(&sb, "## Skill: %s\n", skill.Name)
			sb.WriteString(skill.Instructions)
			sb.WriteString("\n\n")
		}
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
