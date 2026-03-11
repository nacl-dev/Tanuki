package downloader

import "testing"

func TestValidateCronExpressionAcceptsStandardFiveFieldFormat(t *testing.T) {
	t.Parallel()

	normalized, nextRun, err := ValidateCronExpression("  0   3   *  *  * ")
	if err != nil {
		t.Fatalf("expected cron to be valid, got error: %v", err)
	}
	if normalized != "0 3 * * *" {
		t.Fatalf("unexpected normalization: %q", normalized)
	}
	if nextRun.IsZero() {
		t.Fatalf("expected next run to be populated")
	}
}

func TestValidateCronExpressionRejectsSixFieldFormat(t *testing.T) {
	t.Parallel()

	if _, _, err := ValidateCronExpression("0 0 3 * * *"); err == nil {
		t.Fatalf("expected six-field cron expression to be rejected")
	}
}
