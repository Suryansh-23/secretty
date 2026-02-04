//go:build linux
// +build linux

package clipboard

func autoBackendCandidates() []Backend {
	var candidates []Backend
	if isWayland() {
		candidates = append(candidates, BackendWlCopy)
	}
	if isX11() {
		candidates = append(candidates, BackendXclip, BackendXsel)
	}
	if len(candidates) == 0 {
		return nil
	}
	return candidates
}
