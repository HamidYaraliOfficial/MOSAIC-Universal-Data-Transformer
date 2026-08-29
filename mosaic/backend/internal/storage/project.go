// Package storage implements MOSAIC's Project System: saving/loading
// Projects (pipelines, datasets metadata, connections, templates and
// execution history) to disk as versioned JSON, plus Autosave/Crash
// Recovery. Deliberately dependency-free (no cgo sqlite driver) so the
// engine builds and runs anywhere the Go toolchain does.
package storage

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"time"

	"mosaic/internal/pipeline"
)

// Project is the top-level container: everything the Project Explorer
// panel lists for one MOSAIC workspace.
type Project struct {
	ID          string                 `json:"id"`
	Name        string                 `json:"name"`
	CreatedAt   time.Time              `json:"createdAt"`
	UpdatedAt   time.Time              `json:"updatedAt"`
	Pipelines   []pipeline.Definition  `json:"pipelines"`
	Connections []Connection           `json:"connections"`
	Tags        []string               `json:"tags,omitempty"`
	Favorite    bool                   `json:"favorite"`
}

// Connection is a saved Database/API connector reference. Credentials are
// never stored inline here — see security.Vault; this struct only holds a
// vault key pointing at the encrypted secret.
type Connection struct {
	ID         string `json:"id"`
	Name       string `json:"name"`
	Kind       string `json:"kind"` // postgres|mysql|sqlite|sqlserver|rest|graphql
	Host       string `json:"host,omitempty"`
	Database   string `json:"database,omitempty"`
	VaultKey   string `json:"vaultKey,omitempty"`
}

// HistoryEntry records one saved pipeline version for the History System /
// Data Lineage timeline.
type HistoryEntry struct {
	Timestamp time.Time `json:"timestamp"`
	Summary   string    `json:"summary"`
	Snapshot  pipeline.Definition `json:"snapshot"`
}

// Store manages Projects on disk under a root directory, one subdirectory
// per project plus a shared autosave slot.
type Store struct {
	Root string
}

// NewStore ensures the storage root (and its autosave subfolder) exist.
func NewStore(root string) (*Store, error) {
	if err := os.MkdirAll(filepath.Join(root, "autosave"), 0o755); err != nil {
		return nil, err
	}
	if err := os.MkdirAll(filepath.Join(root, "projects"), 0o755); err != nil {
		return nil, err
	}
	return &Store{Root: root}, nil
}

func (s *Store) projectPath(id string) string {
	return filepath.Join(s.Root, "projects", id+".json")
}

// Save persists a project as pretty-printed JSON (human-diffable, which
// matters for the "versioned and migratable" pipeline format requirement).
func (s *Store) Save(p *Project) error {
	p.UpdatedAt = time.Now()
	data, err := json.MarshalIndent(p, "", "  ")
	if err != nil {
		return err
	}
	tmp := s.projectPath(p.ID) + ".tmp"
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, s.projectPath(p.ID)) // atomic on POSIX filesystems
}

// Load reads a single project by ID.
func (s *Store) Load(id string) (*Project, error) {
	data, err := os.ReadFile(s.projectPath(id))
	if err != nil {
		return nil, err
	}
	var p Project
	if err := json.Unmarshal(data, &p); err != nil {
		return nil, fmt.Errorf("storage: corrupt project file %q: %w", id, err)
	}
	return &p, nil
}

// List returns lightweight metadata for every saved project, most recently
// updated first, for the Start Center's "Recent Projects" list.
func (s *Store) List() ([]*Project, error) {
	entries, err := os.ReadDir(filepath.Join(s.Root, "projects"))
	if err != nil {
		return nil, err
	}
	var out []*Project
	for _, e := range entries {
		if e.IsDir() || filepath.Ext(e.Name()) != ".json" {
			continue
		}
		id := e.Name()[:len(e.Name())-len(".json")]
		p, err := s.Load(id)
		if err != nil {
			continue // skip corrupt files rather than failing the whole list
		}
		out = append(out, p)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].UpdatedAt.After(out[j].UpdatedAt) })
	return out, nil
}

// Autosave writes a crash-recovery snapshot of the currently open pipeline.
// On next launch, RecoverAutosave lets the UI offer "restore unsaved work".
func (s *Store) Autosave(projectID string, def *pipeline.Definition) error {
	data, err := json.Marshal(def)
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(s.Root, "autosave", projectID+".json"), data, 0o644)
}

// RecoverAutosave returns a pending autosave snapshot for a project, if any.
func (s *Store) RecoverAutosave(projectID string) (*pipeline.Definition, bool) {
	data, err := os.ReadFile(filepath.Join(s.Root, "autosave", projectID+".json"))
	if err != nil {
		return nil, false
	}
	var def pipeline.Definition
	if json.Unmarshal(data, &def) != nil {
		return nil, false
	}
	return &def, true
}

// ClearAutosave removes the recovery snapshot after a successful manual save.
func (s *Store) ClearAutosave(projectID string) {
	os.Remove(filepath.Join(s.Root, "autosave", projectID+".json"))
}
