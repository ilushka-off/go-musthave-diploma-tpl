package luhn

import "testing"

func TestValid(t *testing.T) {
	tests := []struct {
		name   string
		number string
		want   bool
	}{
		{name: "valid number from spec, odd length", number: "12345678903", want: true},
		{name: "valid number from spec, even length", number: "9278923470", want: true},
		{name: "valid withdrawal number from spec", number: "2377225624", want: true},
		{name: "wrong check digit", number: "12345678900", want: false},
		{name: "empty string", number: "", want: false},
		{name: "contains letter", number: "12f3", want: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := Valid(tt.number); got != tt.want {
				t.Errorf("Valid(%q) = %v, want %v", tt.number, got, tt.want)
			}
		})
	}
}
