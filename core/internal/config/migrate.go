package config

import "biene/internal/templates"

// CurrentConfigVersion is the schema version this build of core writes.
// Bump this whenever a new entry is appended to `configMigrations`.
const CurrentConfigVersion = 3

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
	{
		TargetVersion: 2,
		Description:   "Seed compaction block with built-in defaults",
		Apply:         migrateSeedCompaction,
	},
	{
		TargetVersion: 3,
		Description:   "Backfill context_window on model entries from the builtin template registry",
		Apply:         migrateBackfillContextWindow,
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
// migrateSeedCompaction: v1 had no compaction block. Seed the default one
// so existing installs auto-enable context compression on first launch.
// Already-set blocks are kept as-is.
func migrateSeedCompaction(cfg *Config) bool {
	if cfg.Compaction != nil {
		return false
	}
	def := DefaultCompactionConfig()
	cfg.Compaction = &def
	return true
}

// migrateBackfillContextWindow: v2 added the context_window field but did
// nothing to populate it on entries that pre-dated the change. Without
// this fill-in, those entries fall back to DefaultContextWindow (32K) at
// runtime, which silently breaks compaction's math on models that
// actually have 128K/200K windows. Walk model_list and copy the
// authoritative window from the builtin template registry whenever
//
//   - the entry has no explicit context_window (== 0), AND
//   - (provider, model, base_url) matches a known template that
//     declares a window.
//
// Entries that don't match any template (truly custom user models) are
// left at 0 — the runtime fallback handles them, and overwriting with
// a guess would do more harm than good.
func migrateBackfillContextWindow(cfg *Config) bool {
	changed := false
	for i := range cfg.ModelList {
		entry := &cfg.ModelList[i]
		if entry.ContextWindow > 0 {
			continue
		}
		window, ok := templates.LookupContextWindow(entry.Provider, entry.Model, entry.BaseURL)
		if !ok {
			continue
		}
		entry.ContextWindow = window
		changed = true
	}
	return changed
}

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
