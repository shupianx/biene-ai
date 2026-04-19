package tools

// SkillActivator is the narrow interface a session exposes to the use_skill
// tool. Activation loads the full skill body, marks the skill active on the
// session, and returns the instructions so the tool result carries them back
// to the model.
type SkillActivator interface {
	ActivateSkill(name string) (instructions string, err error)
}
