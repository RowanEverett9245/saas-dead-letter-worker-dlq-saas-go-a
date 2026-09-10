package jobs

import "fmt"

const MaxAttempts = 3

type Kind string

const (
	TenantOnboarding Kind = "tenant_onboarding"
	AccountLifecycle Kind = "account_lifecycle"
	AdminOperation   Kind = "admin_operation"
)

type Job struct {
	ID       string         `json:"job_id"`
	TenantID string         `json:"tenant_id"`
	Kind     Kind           `json:"kind"`
	Attempt  int            `json:"attempt"`
	Input    map[string]any `json:"input"`
}

type DeadLetter struct {
	OriginalJob Job    `json:"original_job"`
	Reason      string `json:"reason"`
	FailedAt    string `json:"failed_at"`
}

func (j Job) Validate() error {
	if j.ID == "" || j.TenantID == "" {
		return fmt.Errorf("job_id and tenant_id are required")
	}
	switch j.Kind {
	case TenantOnboarding, AccountLifecycle, AdminOperation:
		return nil
	default:
		return fmt.Errorf("unknown job kind %q", j.Kind)
	}
}

func ShouldDeadLetter(job Job) bool {
	return job.Attempt >= MaxAttempts
}
