//go:build !linux && !darwin && !windows

package diagnostics

import "context"

type otherPlatform struct{}

func newPlatformImpl() platform { return otherPlatform{} }

func (otherPlatform) Host(context.Context) (int64, string, error) { return 0, "", ErrUnsupported }
func (otherPlatform) OS(context.Context) (OSInfo, error) {
	return OSInfo{GOOS: "unknown", GOARCH: "unknown"}, ErrUnsupported
}
func (otherPlatform) CPU(context.Context) (CPUInfo, error)       { return CPUInfo{}, ErrUnsupported }
func (otherPlatform) Memory(context.Context) (MemoryInfo, error) { return MemoryInfo{}, ErrUnsupported }
func (otherPlatform) Disks(context.Context) ([]DiskInfo, error)  { return nil, ErrUnsupported }
func (otherPlatform) Processes(context.Context, int) ([]ProcessInfo, error) {
	return nil, ErrUnsupported
}
func (otherPlatform) Packages(context.Context) ([]PackageInfo, error) { return nil, ErrUnsupported }
