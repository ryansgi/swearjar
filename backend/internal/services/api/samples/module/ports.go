package module

import (
	"context"

	samplesdom "swearjar/internal/services/api/samples/domain"
	samplessvc "swearjar/internal/services/api/samples/service"
)

// Ports returns the module ports
func (m *Module) Ports() any { return m.ports }

// adaptSamplesPort adapts the samples service to the domain port interface
type adaptSamplesPort struct{ svc samplessvc.Service }

// Recent implements the domain ServicePort interface
func (a adaptSamplesPort) Recent(ctx context.Context, in samplesdom.SamplesInput) ([]samplesdom.Sample, error) {
	return a.svc.Recent(ctx, in)
}
