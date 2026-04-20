package xfyun

import "testing"

func TestParseSize(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		width   int
		height  int
		wantErr bool
	}{
		{name: "square", input: "1024x1024", width: 1024, height: 1024},
		{name: "trim and uppercase", input: " 768X1024 ", width: 768, height: 1024},
		{name: "invalid", input: "512x512", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotW, gotH, err := ParseSize(tt.input)
			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if gotW != tt.width || gotH != tt.height {
				t.Fatalf("unexpected size: got %dx%d want %dx%d", gotW, gotH, tt.width, tt.height)
			}
		})
	}
}
