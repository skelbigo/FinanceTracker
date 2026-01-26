package db

import "testing"

func TestMaskPostgresURL(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		{
			name: "URL with password masks userinfo password",
			in:   "postgres://u:p@h/db",
			want: "postgres://u:******@h/db",
		},
		{
			name: "URL without password does not add password",
			in:   "postgres://u@h/db",
			want: "postgres://u@h/db",
		},
		{
			name: "Query masks password param",
			in:   "postgres://u@h/db?password=123&x=1",
			want: "postgres://u@h/db?password=******&x=1",
		},
		{
			name: "Conninfo masks quoted password with spaces",
			in:   "password='a b' host=localhost user=me",
			want: "password='******' host=localhost user=me",
		},
		{
			name: "Conninfo masks pass key",
			in:   "host=localhost pass=secret user=me",
			want: "host=localhost pass=****** user=me",
		},
		{
			name: "Conninfo masks pwd key",
			in:   `host=localhost pwd="s e c" user=me`,
			want: `host=localhost pwd="******" user=me`,
		},
		{
			name: "Fallback masks userinfo password when url.Parse fails",
			in:   "postgres://u:p@h/db%zz",
			want: "postgres://u:******@h/db%zz",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := MaskPostgresURL(tc.in)
			if got != tc.want {
				t.Fatalf("MaskPostgresURL(%q)\n got:  %q\n want: %q", tc.in, got, tc.want)
			}
		})
	}
}
