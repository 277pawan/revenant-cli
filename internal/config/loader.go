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
	Recovery Recovery `yaml:"recovery,omitempty"`
	Checks   []Check  `yaml:"checks"`
}

type Database struct {
	Engine     string `yaml:"engine"`     // phase 1: must be "postgres"
	Connection string `yaml:"connection"` // usually "${DATABASE_URL}"
}

type Recovery struct {
	Engine               string `yaml:"engine,omitempty"`                 // "aws-rds" or "none"
	SourceIdentifier     string `yaml:"source_identifier,omitempty"`      // RDS instance identifier or snapshot ARN
	SandboxInstanceClass string `yaml:"sandbox_instance_class,omitempty"` // e.g. "db.t4g.micro" or "db.t3.micro" for AWS free tier
	MaxSandboxAge        string `yaml:"max_sandbox_age,omitempty"`        // e.g. "2h" for reaper
	Region               string `yaml:"region,omitempty"`                 // AWS region e.g. "us-east-1"
	UseFreetier          bool   `yaml:"use_freetier,omitempty"`           // If true, uses db.t3.micro (AWS free-tier compatible) + minimal storage
}

// Check is a *union* of every check type we support.
// Unused fields stay empty depending on `type`.
//
//	type: schema       -> ExpectTables
//	type: row_count    -> Table, Min
//	type: foreign_key  -> Table, References
//	type: golden_query -> Query, ExpectMin
//
// When you add freshness/RPO later, add Column / MaxAge here and a new
// case in internal/checks/runner.go — you do not need a new yaml file format.
type Check struct {
	Type         string   `yaml:"type"`
	ExpectTables []string `yaml:"expect_tables,omitempty"`
	Table        string   `yaml:"table,omitempty"`
	Min          *int     `yaml:"min,omitempty"` // pointer so we can tell "missing" from "0"
	References   string   `yaml:"references,omitempty"`
	Query        string   `yaml:"query,omitempty"`
	ExpectMin    *int     `yaml:"expect_min,omitempty"`
	Column       string   `yaml:"column,omitempty"`
	MaxAge       string   `yaml:"max_age,omitempty"`
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

	var missing []string
	cfg.Database.Connection, missing = expandEnv(cfg.Database.Connection)
	for _, name := range missing {
		if name == "SANDBOX_ENDPOINT" && cfg.Recovery.Engine == "aws-rds" {
			continue
		}
		return nil, fmt.Errorf("database.connection references unset environment variable %q", name)
	}

	return &cfg, nil
}

// expandEnv supports the yaml style we document: ${DATABASE_URL}.
// Missing variables are retained so AWS can fill SANDBOX_ENDPOINT after it
// discovers the restored instance.
func expandEnv(s string) (string, []string) {
	var missing []string
	expanded := os.Expand(s, func(name string) string {
		value, ok := os.LookupEnv(name)
		if !ok {
			missing = append(missing, name)
			return "${" + name + "}"
		}
		return value
	})
	return strings.TrimSpace(expanded), missing
}
