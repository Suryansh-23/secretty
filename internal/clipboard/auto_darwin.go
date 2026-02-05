//go:build darwin
// +build darwin

package clipboard

func autoBackendCandidates() []Backend {
	return []Backend{BackendPbcopy}
}
