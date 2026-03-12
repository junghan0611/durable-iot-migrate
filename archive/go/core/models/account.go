package models

// Account represents an end-user account on an IoT platform.
// In a real migration (e.g., 1.4M accounts), each account owns N devices.
// Migration is orchestrated per-account to maintain atomicity:
// all devices in an account succeed, or all roll back.
type Account struct {
	ID       string `json:"id"`
	Email    string `json:"email,omitempty"`
	Region   string `json:"region"`    // "kr", "us", "eu", "jp", "cn"
	Tier     string `json:"tier"`      // "free", "premium", "enterprise"
	DevCount int    `json:"dev_count"` // Pre-computed device count for planning
}

// AccountBatchConfig extends BatchConfig for account-level orchestration.
type AccountBatchConfig struct {
	CohortID     string    `json:"cohort_id"`     // Canary cohort identifier
	Accounts     []Account `json:"accounts"`      // Accounts in this cohort
	MaxParallel  int       `json:"max_parallel"`  // Max concurrent account migrations
	BatchConfig                                   // Embedded: threshold, concurrency, etc.
}

// AccountBatchResult extends BatchResult with account-level tracking.
type AccountBatchResult struct {
	CohortID        string           `json:"cohort_id"`
	TotalAccounts   int              `json:"total_accounts"`
	AccountResults  []AccountResult  `json:"account_results"`
	AccountsFailed  int              `json:"accounts_failed"`
	AccountsSuccess int              `json:"accounts_success"`
	CriticalFailed  int              `json:"critical_failed"`  // Safety-critical device failures
}

// AccountResult tracks migration result for a single account.
type AccountResult struct {
	AccountID      string      `json:"account_id"`
	Region         string      `json:"region"`
	DevicesMigrated int       `json:"devices_migrated"`
	DevicesFailed   int       `json:"devices_failed"`
	CriticalFailed  int       `json:"critical_failed"` // camera/lock failures
	BatchResult    BatchResult `json:"batch_result"`    // Detailed device-level results
}

// AccountSuccessRate returns the account-level success rate.
func (r *AccountBatchResult) AccountSuccessRate() float64 {
	if r.TotalAccounts == 0 {
		return 1.0
	}
	return float64(r.AccountsSuccess) / float64(r.TotalAccounts)
}

// HasCriticalFailures returns true if any safety-critical device failed.
func (r *AccountBatchResult) HasCriticalFailures() bool {
	return r.CriticalFailed > 0
}
