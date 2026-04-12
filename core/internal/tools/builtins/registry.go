package builtins

import "biene/internal/tools"

// RegistryForWorkDir returns a registry pre-loaded with file tools rooted at workDir.
func RegistryForWorkDir(workDir string) *tools.Registry {
	r := tools.NewRegistry()
	r.Register(NewFileReadToolInDir(workDir))
	r.Register(NewFileWriteToolInDir(workDir))
	r.Register(NewFileEditToolInDir(workDir))
	return r
}
