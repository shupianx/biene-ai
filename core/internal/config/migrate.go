package config

// CurrentConfigVersion is the schema version this build of core writes.
// Bump this whenever a new entry is appended to `configMigrations`.
const CurrentConfigVersion = 1

// configMigration is one step that brings a config file from
// (TargetVersion - 1) up to TargetVersion. Migrations run in ascending
// order until cfg.Version reaches CurrentConfigVersion.
type configMigration struct {
	TargetVersion int
	Description   string
	Apply         func(*Config) bool
}

// configMigrations is the ordered list of upgrade steps. Each migration
// must be idempotent enough to skip rows that already match the new
// shape (e.g. honor existing non-nil pointers) so re-running on
// already-migrated data is a no-op.
var configMigrations = []configMigration{
	{
		TargetVersion: 1,
		Description:   "Backfill images_available=false for known vision-incapable models",
		Apply:         migrateBackfillImagesAvailable,
	},
}

// Migrate brings cfg up to CurrentConfigVersion by applying every pending
// migration in order. Returns true when anything (including the version
// stamp itself) was changed so the caller can persist the upgrade.
func Migrate(cfg *Config) bool {
	if cfg == nil {
		return false
	}
	changed := false
	for _, m := range configMigrations {
		if cfg.Version >= m.TargetVersion {
			continue
		}
		if m.Apply != nil && m.Apply(cfg) {
			changed = true
		}
		cfg.Version = m.TargetVersion
		changed = true
	}
	if cfg.Version != CurrentConfigVersion {
		cfg.Version = CurrentConfigVersion
		changed = true
	}
	return changed
}

// ── Migration steps ──────────────────────────────────────────────────────

// migrateBackfillImagesAvailable: in v0 the schema had no
// `images_available` field, so legacy entries pointing at vision-incapable
// API models silently still showed the composer's image attach control.
// This step writes an explicit `images_available: false` for any matching
// entry that hasn't already declared a value. Keep the model list in sync
// with the `images_available: false` markers in
// renderer/src/constants/providerTemplates.ts.
func migrateBackfillImagesAvailable(cfg *Config) bool {
	blocked := map[string]struct{}{
		"deepseek-v4-pro":   {},
		"deepseek-v4-flash": {},
	}
	changed := false
	for i := range cfg.ModelList {
		entry := &cfg.ModelList[i]
		if entry.ImagesAvailable != nil {
			continue
		}
		if _, ok := blocked[entry.Model]; !ok {
			continue
		}
		f := false
		entry.ImagesAvailable = &f
		changed = true
	}
	return changed
}
