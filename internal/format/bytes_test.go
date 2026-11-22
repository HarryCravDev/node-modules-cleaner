package format

import "testing"

func TestBytes(t *testing.T) {
	tests := []struct {
		input int64
		want  string
	}{
		{0, "0 B"},
		{512, "512 B"},
		{1024, "1.0 KB"},
		{1536, "1.5 KB"},
		{1024 * 1024, "1.0 MB"},
		{256 * 1024 * 1024, "256.0 MB"},
		{2 * 1024 * 1024 * 1024, "2.00 GB"},
	}

	for _, tc := range tests {
		got := Bytes(tc.input)
		if got != tc.want {
			t.Errorf("Bytes(%d) = %q, want %q", tc.input, got, tc.want)
		}
	}
}
