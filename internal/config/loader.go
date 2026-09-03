// Package config turns revenant.yaml into Go structs the rest of the
// program can use. Nothing in this package talks to the database.
package config

import (
	"fmt"
	"os"
	"strings"

	"gopkg.in/yaml.v3"
)

// File is the top-level shape of revenant.yaml.
//
// YAML field names use snake_case (expect_tables). Go fields use CamelCase.
// The `yaml:"..."` tags are how yaml.v3 maps between the two.
type File struct {
	Plan     string   `yaml:"plan"`
	Database Database `yaml:"database"`
	Checks   []Check  `yaml:"checks"`
}

type Database struct {
	Engine     string `yaml:"engine"`     // phase 1: must be "postgres"
	Connection string `yaml:"connection"` // usually "${DATABASE_URL}"
}

// Check is a *union* of every check type we support.
// Unused fields stay empty depending on `type`.
//
//   type: schema      -> ExpectTables
//   type: row_count   -> Table, Min
//   type: foreign_key -> Table, References  (stub in checks/foreignkey.go)
//
// When you add golden_query later, add Query / ExpectMin here and a new
// case in internal/checks/runner.go — you do not need a new yaml file format.
type Check struct {
	Type         string   `yaml:"type"`
	ExpectTables []string `yaml:"expect_tables,omitempty"`
	Table        string   `yaml:"table,omitempty"`
	Min          *int     `yaml:"min,omitempty"` // pointer so we can tell "missing" from "0"
	References   string   `yaml:"references,omitempty"`
}

// Load reads path, unmarshals YAML, and expands ${ENV} in the connection string.
func Load(path string) (*File, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read config %s: %w", path, err)
	}

	var cfg File
	if err := yaml.Unmarshal(raw, &cfg); err != nil {
		return nil, fmt.Errorf("parse yaml %s: %w", path, err)
	}

	if cfg.Plan == "" {
		return nil, fmt.Errorf("revenant.yaml: plan is required")
	}
	if cfg.Database.Connection == "" {
		return nil, fmt.Errorf("revenant.yaml: database.connection is required")
	}
	if len(cfg.Checks) == 0 {
		return nil, fmt.Errorf("revenant.yaml: at least one check is required")
	}

	cfg.Database.Connection = expandEnv(cfg.Database.Connection)
	if cfg.Database.Connection == "" {
		return nil, fmt.Errorf("database.connection expanded to empty string (is DATABASE_URL set?)")
	}

	return &cfg, nil
}

// expandEnv supports the yaml style we document: ${DATABASE_URL}
// os.ExpandEnv already understands $FOO and ${FOO}.
func expandEnv(s string) string {
	return strings.TrimSpace(os.ExpandEnv(s))
}
