package graphiceq

import (
	"github.com/cwbudde/algo-mixedphase/internal/fftx"
)

// fftWorkspace adapts [fftx.Workspace] to this package's short call sites and
// its error prefix.
type fftWorkspace struct {
	*fftx.Workspace
}

func newFFTWorkspace(size int) (*fftWorkspace, error) {
	workspace, err := fftx.New(size, "graphiceq")
	if err != nil {
		return nil, err
	}

	return &fftWorkspace{Workspace: workspace}, nil
}

func (w *fftWorkspace) forwardReal(input []float64) ([]complex128, error) {
	return w.ForwardReal(input)
}

func (w *fftWorkspace) inverseReal(input []complex128) ([]float64, error) {
	return w.InverseReal(input)
}
