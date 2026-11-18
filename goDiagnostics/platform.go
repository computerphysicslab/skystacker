package diagnostics

import "context"

// platform groups the operating-system-specific collectors. Core code only
// talks to this interface, so adding a new OS needs one new file with build
// tags and no changes to the report format.
type platform interface {
	Host(ctx context.Context) (uptimeSec int64, bootTime string, err error)
	OS(ctx context.Context) (OSInfo, error)
	CPU(ctx context.Context) (CPUInfo, error)
	Memory(ctx context.Context) (MemoryInfo, error)
	Disks(ctx context.Context) ([]DiskInfo, error)
	Processes(ctx context.Context, n int) ([]ProcessInfo, error)
	Updates(ctx context.Context) ([]PackageInfo, error)
}

var platformFactory = func() platform { return newPlatformImpl() }

func newPlatform() platform { return platformFactory() }
