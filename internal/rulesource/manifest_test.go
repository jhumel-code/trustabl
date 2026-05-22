package rulesource

import (
	"testing"
	"testing/fstest"
)

func TestCompatible(t *testing.T) {
	cases := []struct {
		name    string
		fsys    fstest.MapFS
		support int
		want    bool
	}{
		{"equal version", fstest.MapFS{"manifest.yaml": &fstest.MapFile{Data: []byte("schema_version: 1\n")}}, 1, true},
		{"older pack", fstest.MapFS{"manifest.yaml": &fstest.MapFile{Data: []byte("schema_version: 1\n")}}, 2, true},
		{"newer pack", fstest.MapFS{"manifest.yaml": &fstest.MapFile{Data: []byte("schema_version: 3\n")}}, 2, false},
		{"missing manifest", fstest.MapFS{}, 1, false},
		{"malformed manifest", fstest.MapFS{"manifest.yaml": &fstest.MapFile{Data: []byte("not: [valid")}}, 1, false},
		{"zero version", fstest.MapFS{"manifest.yaml": &fstest.MapFile{Data: []byte("schema_version: 0\n")}}, 1, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := compatible(tc.fsys, tc.support); got != tc.want {
				t.Errorf("compatible = %v, want %v", got, tc.want)
			}
		})
	}
}
