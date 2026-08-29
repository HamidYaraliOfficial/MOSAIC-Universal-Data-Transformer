package security

import (
	"fmt"
	"strings"
)

// SandboxPolicy is the per-Script-Node permission set configured from the
// node's Inspector panel: MOSAIC never grants a script unrestricted access
// by default.
type SandboxPolicy struct {
	AllowFileSystem bool     `json:"allowFileSystem"`
	AllowedPaths    []string `json:"allowedPaths,omitempty"`
	AllowNetwork    bool     `json:"allowNetwork"`
	AllowedHosts    []string `json:"allowedHosts,omitempty"`
	MaxMemoryMB     int      `json:"maxMemoryMb"`
	TimeoutSeconds  int      `json:"timeoutSeconds"`
}

// DefaultSandboxPolicy denies everything by default — the user must
// explicitly opt a Script Node into filesystem or network access.
func DefaultSandboxPolicy() SandboxPolicy {
	return SandboxPolicy{MaxMemoryMB: 256, TimeoutSeconds: 10}
}

// CheckPathAccess validates a script's requested file path against policy,
// used by the Script Node runtime before any file operation.
func (p SandboxPolicy) CheckPathAccess(path string) error {
	if !p.AllowFileSystem {
		return fmt.Errorf("security: script attempted file access but AllowFileSystem is disabled")
	}
	for _, allowed := range p.AllowedPaths {
		if strings.HasPrefix(path, allowed) {
			return nil
		}
	}
	return fmt.Errorf("security: path %q is outside the script's allowed paths", path)
}

// CheckNetworkAccess validates a script's requested host against policy.
func (p SandboxPolicy) CheckNetworkAccess(host string) error {
	if !p.AllowNetwork {
		return fmt.Errorf("security: script attempted network access but AllowNetwork is disabled")
	}
	for _, allowed := range p.AllowedHosts {
		if allowed == host || strings.HasSuffix(host, "."+allowed) {
			return nil
		}
	}
	return fmt.Errorf("security: host %q is not in the script's allowed hosts", host)
}
