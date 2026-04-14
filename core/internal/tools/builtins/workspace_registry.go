package builtins

import "biene/internal/tools"

// RegistryForWorkDir builds a registry with workspace-scoped file tools.
func RegistryForWorkDir(workDir string) *tools.Registry {
	r := tools.NewRegistry()
	r.Register(NewListFilesToolInDir(workDir))
	r.Register(NewFileReadToolInDir(workDir))
	r.Register(NewFileWriteToolInDir(workDir))
	r.Register(NewFileEditToolInDir(workDir))
	return r
}
