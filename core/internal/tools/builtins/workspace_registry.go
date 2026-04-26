package builtins

import "biene/internal/tools"

// RegistryForWorkDir builds a registry with workspace-scoped built-in tools.
func RegistryForWorkDir(workDir string) *tools.Registry {
	r := tools.NewRegistry()
	r.Register(NewListSkillsToolInDir(workDir))
	r.Register(NewListFilesToolInDir(workDir))
	r.Register(NewGlobToolInDir(workDir))
	r.Register(NewGrepToolInDir(workDir))
	r.Register(NewFileReadToolInDir(workDir))
	r.Register(NewFileWriteToolInDir(workDir))
	r.Register(NewFileEditToolInDir(workDir))
	r.Register(NewRunCommandToolInDir(workDir))
	return r
}
