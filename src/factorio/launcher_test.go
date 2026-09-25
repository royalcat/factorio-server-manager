package factorio

import "testing"

func TestHasNativeARMBuild(t *testing.T) {
	tests := []struct {
		name    string
		version Version
		want    bool
	}{
		{name: "first published arm64 build", version: Version{2, 1, 18, 0}, want: true},
		{name: "newer 2.1 build", version: Version{2, 1, 20, 0}, want: true},
		{name: "2.2 build", version: Version{2, 2, 0, 0}, want: true},
		{name: "version before first arm64 build", version: Version{2, 1, 17, 0}, want: false},
		{name: "2.0 release", version: Version{2, 0, 77, 0}, want: false},
		{name: "unknown version", version: NilVersion, want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := hasNativeARMBuild(tt.version); got != tt.want {
				t.Fatalf("hasNativeARMBuild(%v) = %v, want %v", tt.version, got, tt.want)
			}
		})
	}
}

func TestNeedsEmulationFor(t *testing.T) {
	tests := []struct {
		name    string
		goarch  string
		version Version
		want    bool
	}{
		{name: "amd64 runs 2.0 natively", goarch: "amd64", version: Version{2, 0, 77, 0}, want: false},
		{name: "amd64 runs old versions natively", goarch: "amd64", version: Version{0, 18, 0, 0}, want: false},
		{name: "arm64 runs first native build natively", goarch: "arm64", version: Version{2, 1, 18, 0}, want: false},
		{name: "arm64 runs 2.1.20 natively", goarch: "arm64", version: Version{2, 1, 20, 0}, want: false},
		{name: "arm64 emulates 2.1.17", goarch: "arm64", version: Version{2, 1, 17, 0}, want: true},
		{name: "arm64 emulates 2.0", goarch: "arm64", version: Version{2, 0, 77, 0}, want: true},
		{name: "arm64 emulates unknown version", goarch: "arm64", version: NilVersion, want: true},
		{name: "riscv64 always emulates", goarch: "riscv64", version: Version{2, 1, 20, 0}, want: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := needsEmulationFor(tt.goarch, tt.version); got != tt.want {
				t.Fatalf("needsEmulationFor(%q, %v) = %v, want %v", tt.goarch, tt.version, got, tt.want)
			}
		})
	}
}

func TestFactorioDownloadPlatform(t *testing.T) {
	tests := []struct {
		name    string
		goarch  string
		version Version
		want    string
	}{
		{name: "amd64 uses linux64", goarch: "amd64", version: Version{2, 1, 20, 0}, want: "linux64"},
		{name: "arm64 uses linux-arm64 for first native build", goarch: "arm64", version: Version{2, 1, 18, 0}, want: "linux-arm64"},
		{name: "arm64 uses linux-arm64 for 2.1.20", goarch: "arm64", version: Version{2, 1, 20, 0}, want: "linux-arm64"},
		{name: "arm64 uses linux64 before first native build", goarch: "arm64", version: Version{2, 1, 17, 0}, want: "linux64"},
		{name: "arm64 uses linux64 for 2.0", goarch: "arm64", version: Version{2, 0, 77, 0}, want: "linux64"},
		{name: "arm64 uses linux64 for unknown version", goarch: "arm64", version: NilVersion, want: "linux64"},
		{name: "riscv64 uses linux64", goarch: "riscv64", version: Version{2, 1, 20, 0}, want: "linux64"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := factorioDownloadPlatform(tt.goarch, tt.version); got != tt.want {
				t.Fatalf("factorioDownloadPlatform(%q, %v) = %q, want %q", tt.goarch, tt.version, got, tt.want)
			}
		})
	}
}
