package database

import "testing"

func TestBuildPortablePathWindows(t *testing.T) {
	tests := []struct {
		name      string
		root      portablePath
		remainder []string
		want      string
	}{
		{
			name: "drive root without segments",
			root: portablePath{
				style:      portablePathStyleWindows,
				rootPrefix: `C:`,
			},
			want: `C:\`,
		},
		{
			name: "drive root with segments",
			root: portablePath{
				style:      portablePathStyleWindows,
				rootPrefix: `C:`,
			},
			remainder: []string{"users", "alice"},
			want:      `C:\users\alice`,
		},
		{
			name: "unc root without segments",
			root: portablePath{
				style:      portablePathStyleWindows,
				rootPrefix: `\\host\share`,
			},
			want: `\\host\share`,
		},
		{
			name: "unc root with segments",
			root: portablePath{
				style:      portablePathStyleWindows,
				rootPrefix: `\\host\share`,
			},
			remainder: []string{"users", "alice"},
			want:      `\\host\share\users\alice`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := buildPortablePath(tt.root, tt.remainder)
			if got != tt.want {
				t.Fatalf("buildPortablePath() = %q, want %q", got, tt.want)
			}
		})
	}
}
