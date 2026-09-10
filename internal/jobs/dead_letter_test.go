package jobs

import "testing"

func TestShouldDeadLetter(t *testing.T) {
	tests := []struct {
		name    string
		kind    Kind
		attempt int
		want    bool
	}{
		{"onboarding retries", TenantOnboarding, 2, false},
		{"onboarding poison job", TenantOnboarding, 3, true},
		{"lifecycle poison job", AccountLifecycle, 4, true},
		{"admin first failure", AdminOperation, 1, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			job := Job{ID: "job-42", TenantID: "tenant-7", Kind: tt.kind, Attempt: tt.attempt}
			if got := ShouldDeadLetter(job); got != tt.want {
				t.Fatalf("ShouldDeadLetter(attempt=%d) = %v, want %v", tt.attempt, got, tt.want)
			}
		})
	}
}
