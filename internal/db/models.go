package db

import (
	"time"

	"gorm.io/gorm"
)

// Status types
type BuildStatus string
type JobStatus string
type TriggerType string

const (
	BuildStatusPending   BuildStatus = "pending"
	BuildStatusRunning   BuildStatus = "running"
	BuildStatusSuccess   BuildStatus = "success"
	BuildStatusFailed    BuildStatus = "failed"
	BuildStatusCancelled BuildStatus = "cancelled"

	JobStatusPending JobStatus = "pending"
	JobStatusRunning JobStatus = "running"
	JobStatusSuccess JobStatus = "success"
	JobStatusFailed  JobStatus = "failed"
	JobStatusSkipped JobStatus = "skipped"

	TriggerPush   TriggerType = "push"
	TriggerPR     TriggerType = "pull_request"
	TriggerManual TriggerType = "manual"
)

// Pipeline is a registered repository with its webhook config.
type Pipeline struct {
	gorm.Model
	Name          string  `gorm:"uniqueIndex;not null"`
	RepoURL       string  `gorm:"not null"`
	DefaultBranch string  `gorm:"default:main"`
	ConfigPath    string  `gorm:"default:.forge.yml"`
	WebhookSecret string
	GitHubRepo    string  // "owner/repo"
	Builds        []Build `gorm:"foreignKey:PipelineID"`
	Secrets       []Secret `gorm:"foreignKey:PipelineID"`
}

// Secret stores an encrypted environment variable for a pipeline.
type Secret struct {
	gorm.Model
	PipelineID uint   `gorm:"index;not null;uniqueIndex:idx_pipeline_secret_name"`
	Pipeline   Pipeline `gorm:"constraint:OnDelete:CASCADE"`
	Name       string `gorm:"not null;uniqueIndex:idx_pipeline_secret_name"`
	Ciphertext string `gorm:"not null"`
	Nonce      string `gorm:"not null"`
}

// Build is a single triggered execution of a pipeline.
type Build struct {
	gorm.Model
	PipelineID uint        `gorm:"index;not null"`
	Pipeline   Pipeline    `gorm:"constraint:OnDelete:CASCADE"`
	Trigger    TriggerType `gorm:"not null"`
	CommitSHA  string
	Branch     string
	CommitMsg  string
	AuthorName string
	Status     BuildStatus `gorm:"default:pending"`
	StartedAt  *time.Time
	FinishedAt *time.Time
	Jobs       []Job `gorm:"foreignKey:BuildID"`
}

// Job is a single job within a build (maps to one Docker container).
type Job struct {
	gorm.Model
	BuildID     uint      `gorm:"index;not null"`
	Build       Build     `gorm:"constraint:OnDelete:CASCADE"`
	Name        string    `gorm:"not null"`
	Image       string
	Status      JobStatus `gorm:"default:pending"`
	ContainerID string
	ExitCode    int        `gorm:"default:-1"`
	StartedAt   *time.Time
	FinishedAt  *time.Time
	Logs        []LogLine `gorm:"foreignKey:JobID"`
}

// LogLine is a single line of output from a job container.
type LogLine struct {
	ID        uint      `gorm:"primaryKey;autoIncrement"`
	JobID     uint      `gorm:"index;not null"`
	Seq       int       `gorm:"not null"`
	Stream    string    // "stdout" or "stderr"
	Text      string
	Timestamp time.Time
}
