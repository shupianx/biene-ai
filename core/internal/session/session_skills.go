package session

import (
	"fmt"
	"log"
	"strings"

	"biene/internal/skills"
)

// ActivateSkill implements tools.SkillActivator. It loads the skill body from
// the agent workspace, marks the skill active on the session, emits a realtime
// event, persists the updated meta, and returns the instructions so the
// use_skill tool result carries them back to the model.
func (s *Session) ActivateSkill(name string) (string, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return "", fmt.Errorf("skill name is required")
	}

	metas, err := skills.ScanForWorkDir(s.WorkDir)
	if err != nil {
		return "", fmt.Errorf("scan skills: %w", err)
	}

	var match *skills.Metadata
	for i := range metas {
		if strings.EqualFold(metas[i].Name, name) {
			match = &metas[i]
			break
		}
	}
	if match == nil {
		return "", fmt.Errorf("skill %q is not installed in this agent workspace", name)
	}

	def, err := skills.LoadDefinition(*match)
	if err != nil {
		return "", fmt.Errorf("load skill %q: %w", match.Name, err)
	}

	s.mu.Lock()
	added := appendUniqueLocked(&s.activeSkills, def.Name)
	meta := s.metaLocked()
	persistedMeta := s.persistentMetaLocked()
	s.mu.Unlock()

	if added {
		if s.store != nil {
			if err := s.store.SaveMeta(persistedMeta); err != nil {
				log.Printf("persist active skills for %s: %v", s.ID, err)
			}
		}
		s.send(makeFrame("skill_activated", skillActivatedPayload{SkillName: def.Name}))
		s.notifyMetaChanged(meta)
	}

	return def.Instructions, nil
}

func appendUniqueLocked(list *[]string, name string) bool {
	for _, existing := range *list {
		if existing == name {
			return false
		}
	}
	*list = append(*list, name)
	return true
}
