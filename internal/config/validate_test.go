package config

import "testing"

func TestValidateChecksRejectsTypo(t *testing.T) {
	err := ValidateChecks([]Check{{Type: "rowcount", Table: "orders", Min: ptr(1)}})
	if err == nil {
		t.Fatal("expected error for rowcount typo")
	}
}

func TestValidateChecksRowCountMaxLessThanMin(t *testing.T) {
	err := ValidateChecks([]Check{{Type: "row_count", Table: "orders", Min: ptr(10), Max: ptr(5)}})
	if err == nil {
		t.Fatal("expected error when max < min")
	}
}

func ptr(n int) *int { return &n }
