package session

import (
	"fmt"
	"log"
	"strings"

	"tinte/internal/skills"
)

// InstallSkillFromRepository copies a skill from ~/.tinte/skills into this
// agent's workspace so the agent can discover and activate it. Any existing
// installation at the destination is overwritten; frontend callers should
// confirm with the user before calling when the skill is already in the
// session's InstalledSkillIDs list.
func (s *Session) InstallSkillFromRepository(id string) (string, error) {
	name, err := skills.InstallSkillByID(s.WorkDir, id)
	if err != nil {
		return "", err
	}
	s.refreshInstalledSkillsAndNotify()
	return name, nil
}

// UninstallSkill removes one installed skill from this agent's workspace and
// drops it from the active-skills list if it was loaded.
func (s *Session) UninstallSkill(id string) (string, error) {
	name, err := skills.UninstallSkillByID(s.WorkDir, id)
	if err != nil {
		return "", err
	}

	s.mu.Lock()
	changed := removeFromListLocked(&s.activeSkills, name)
	ids, scanErr := skills.InstalledSkillIDsForWorkDir(s.WorkDir)
	if scanErr != nil {
		log.Printf("scan installed skills for %s: %v", s.ID, scanErr)
	} else {
		s.installedSkillIDs = ids
	}
	meta := s.metaLocked()
	persistedMeta := s.persistentMetaLocked()
	s.mu.Unlock()

	if changed && s.store != nil {
		if err := s.store.SaveMeta(persistedMeta); err != nil {
			log.Printf("persist active skills for %s: %v", s.ID, err)
		}
	}
	s.notifyMetaChanged(meta)

	return name, nil
}

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

// refreshInstalledSkillsAndNotify rescans <WorkDir>/.tinte/skills, updates
// the cache, and broadcasts a meta_changed so the UI reflects the new state.
func (s *Session) refreshInstalledSkillsAndNotify() {
	ids, err := skills.InstalledSkillIDsForWorkDir(s.WorkDir)
	if err != nil {
		log.Printf("scan installed skills for %s: %v", s.ID, err)
		return
	}
	s.mu.Lock()
	s.installedSkillIDs = ids
	meta := s.metaLocked()
	s.mu.Unlock()
	s.notifyMetaChanged(meta)
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

func removeFromListLocked(list *[]string, name string) bool {
	if name == "" {
		return false
	}
	for i, existing := range *list {
		if existing == name {
			*list = append((*list)[:i], (*list)[i+1:]...)
			return true
		}
	}
	return false
}
