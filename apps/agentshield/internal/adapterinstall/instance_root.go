package adapterinstall

import (
	"os"

	"siq-agent-security/apps/agentshield/internal/hermeshome"
)

// Use handle-derived FileInfo: on Windows path-based Lstat can defer reading
// the file ID until SameFile, which would otherwise observe a replacement.
// Closing the read-only handle avoids retaining resources for expired previews.
func inspectInstanceRoot(instance *InstanceTarget) (os.FileInfo, error) {
	before, err := os.Lstat(instance.ConfigDir)
	if err != nil || !before.IsDir() || before.Mode()&os.ModeSymlink != 0 || hermeshome.Identifier(instance.ConfigDir) != instance.ID {
		return nil, ErrPlanChanged
	}
	f, err := os.Open(instance.ConfigDir)
	if err != nil {
		return nil, ErrPlanChanged
	}
	opened, statErr := f.Stat()
	closeErr := f.Close()
	if statErr != nil || closeErr != nil || !opened.IsDir() || !os.SameFile(before, opened) {
		return nil, ErrPlanChanged
	}
	after, err := os.Lstat(instance.ConfigDir)
	if err != nil || !after.IsDir() || after.Mode()&os.ModeSymlink != 0 || !os.SameFile(opened, after) {
		return nil, ErrPlanChanged
	}
	return opened, nil
}

func (p *Plan) verifyInstanceRoot() error {
	if p.payload.Options.Instance == nil {
		return nil
	}
	if p.instanceRoot == nil {
		return ErrPlanChanged
	}
	current, err := inspectInstanceRoot(p.payload.Options.Instance)
	if err != nil || !os.SameFile(p.instanceRoot, current) {
		return ErrPlanChanged
	}
	return nil
}
