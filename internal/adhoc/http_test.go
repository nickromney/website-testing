package adhoc

import "testing"

func TestNormalizeTarget(t *testing.T) {
	t.Run("adds https to bare host", func(t *testing.T) {
		got, err := NormalizeTarget("google.com")
		if err != nil {
			t.Fatalf("NormalizeTarget() err = %v", err)
		}
		if got != "https://google.com" {
			t.Fatalf("target = %q", got)
		}
	})

	t.Run("preserves explicit scheme", func(t *testing.T) {
		got, err := NormalizeTarget("http://localhost:3000/health")
		if err != nil {
			t.Fatalf("NormalizeTarget() err = %v", err)
		}
		if got != "http://localhost:3000/health" {
			t.Fatalf("target = %q", got)
		}
	})
}
