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
		"If this agent has installed skills relevant to the current task, call use_skill to load the relevant skill, then follow its instructions as part of your normal behavior.",
		"When the user asks what skills are installed or available for this agent, use list_skills instead of guessing from memory.",
		"Use run_command only when command output is actually needed, such as running tests, builds, linters, generators, or project CLIs. Prefer direct file tools when a command is not necessary.",
		"Use write_file or edit_file only when the user gives a concrete instruction to create or modify workspace files. Answering a question by writing the answer into a file is incorrect.",
		"Use send_to_agent only when the user clearly wants agent collaboration, file handoff, or delegation, or when you are sending work, answers, or results back to another agent.",
		"If an incoming agent message is asking you for work, an answer, a decision, or a result that should go back to the sender, do not answer only in the local chat. Use send_to_agent to send that response back to the sender.",
		"Resolving a target agent: when the user refers to another agent by name or description, call list_agents first (unless you already have a current result in this conversation), then match against the Name column. Only proceed with send_to_agent when exactly one listed agent clearly matches. If nothing matches or several do, do not guess — reply in the local chat with the candidates (name + ID) and ask the user to pick.",
		"A mention in the user's message of the form @[Name](agent:<ID>) is the user's explicit choice. Use the embedded ID directly as target_agent_id without further list_agents disambiguation.",
		"A token in the user's message of the form /[Name](skill:<Name>) is the user's explicit request to load that installed skill. Call use_skill with the embedded Name immediately — do not list_skills first or ask the user.",
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
		skillLines := make([]string, 0, len(installedSkills)+2)
		skillLines = append(skillLines, "This agent has the following installed skills available in its own workspace. Each entry shows the skill's name and a short description.")
		skillLines = append(skillLines, "When an installed skill looks relevant to the user's request, call use_skill with its exact name to load its full instructions. Do not pre-load a skill that is not clearly relevant. Once a skill has been loaded in this conversation, its guidance remains available — do not re-load it for follow-up turns.")
		for _, skill := range installedSkills {
			skillLines = append(skillLines, fmt.Sprintf("**%s**: %s", skill.Name, firstLine(skill.Description)))
		}
		writeSection(&sb, "Installed Skills", skillLines)
	}

	writeSection(&sb, "Environment", []string{
		fmt.Sprintf("OS: %s", runtime.GOOS),
		fmt.Sprintf("Working directory: %s", cwd),
	})

	writeSection(&sb, "Inbox", []string{
		"Incoming files arrive under the inbox/ directory in your workspace, grouped by sender.",
		"Files uploaded by the user are placed in inbox/user/.",
		"Files delivered by another agent are placed in inbox/<sender-agent-id>/. Use list_agents to resolve a sender ID to its human-readable name.",
		"When you need to work on an incoming file, read it from its inbox path rather than assuming a top-level location.",
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
