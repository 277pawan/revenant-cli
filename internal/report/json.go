// Package report writes the durable evidence file for a verify run.
//
// Phase 1 is JSON only. Markdown / Prometheus belong in this package later
// as extra Write* functions — keep the Report struct the single source of truth.
package report

import (
	"encoding/json"
	"fmt"
	"os"
	"time"

	"github.com/pawan-bisht/revenant/internal/checks"
)

const (
	StatusPass = "PASS"
	StatusFail = "FAIL"
)

type Report struct {
	Plan     string          `json:"plan"`
	Status   string          `json:"status"`
	Duration string          `json:"duration"`
	Checks   []checks.Result `json:"checks"`
}

func Build(plan string, results []checks.Result, d time.Duration) Report {
	status := StatusPass
	for _, r := range results {
		if r.Status != checks.StatusPass {
			status = StatusFail
			break
		}
	}

	return Report{
		Plan:     plan,
		Status:   status,
		Duration: d.Round(time.Millisecond).String(),
		Checks:   results,
	}
}

func WriteJSON(path string, r Report) error {
	data, err := json.MarshalIndent(r, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal report: %w", err)
	}
	data = append(data, '\n')

	if err := os.WriteFile(path, data, 0o644); err != nil {
		return fmt.Errorf("write %s: %w", path, err)
	}
	return nil
}
