// Package domain holds DTOs for samples http and service contracts
package domain

// SamplesInput is the input for fetching samples
type SamplesInput struct {
	Repo     string `json:"repo,omitempty" validate:"omitempty,min=1,max=200" example:"golang/go"`
	Lang     string `json:"lang,omitempty" validate:"omitempty,alpha" example:"en"`
	Category string `json:"category,omitempty" validate:"omitempty,printascii" example:"bot-directed"`
	Severity string `json:"severity,omitempty" validate:"omitempty,oneof=info low medium high" example:"medium"`
	Limit    int    `json:"limit,omitempty" validate:"omitempty,min=1,max=200" example:"50"`
}

// Sample represents a detected utterance sample
type Sample struct {
	UtteranceID  string `json:"utterance_id"`
	Repo         string `json:"repo"`
	Lang         string `json:"lang"`
	Source       string `json:"source"`
	SourceDetail string `json:"source_detail"`
	Text         string `json:"text"`
	Term         string `json:"term"`
	Category     string `json:"category"`
	Severity     string `json:"severity"`
	DetectorVer  int    `json:"detector_version"`
	CreatedAt    string `json:"created_at"`
}
