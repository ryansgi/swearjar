package domain

import "context"

// ServicePort defines the service contract for samples
type ServicePort interface {
	Recent(ctx context.Context, in SamplesInput) ([]Sample, error)
}
