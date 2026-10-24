package auth

import (
	"reflect"
	"testing"
)

func TestCandidatesPaths(t *testing.T) {
	cases := []struct {
		path string
		want []string
	}{
		{
			path: "",
			want: []string{"/", ""},
		},
		{
			path: "/",
			want: []string{"/", ""},
		},
		{
			path: "a",
			want: []string{"/a/", "/a", "/", ""},
		},
		{
			path: "/a",
			want: []string{"/a/", "/a", "/", ""},
		},
		{
			path: "/a/",
			want: []string{"/a/", "/a", "/", ""},
		},
		{
			path: "a/b",
			want: []string{"/a/b/", "/a/b", "/a/", "/a", "/", ""},
		},
		{
			path: "/a/b",
			want: []string{"/a/b/", "/a/b", "/a/", "/a", "/", ""},
		},
		{
			path: "/a/b/",
			want: []string{"/a/b/", "/a/b", "/a/", "/a", "/", ""},
		},
	}

	for _, c := range cases {
		got := []string{}
		candidatesPaths(c.path)(func(s string) bool {
			got = append(got, s)
			return true
		})
		if len(got) != len(c.want) {
			t.Fatalf("CandidatesPaths(%q) = %v, want %v", c.path, got, c.want)
		}
		if !reflect.DeepEqual(got, c.want) {
			t.Fatalf("CandidatesPaths(%q) = %v, want %v", c.path, got, c.want)
		}
	}
}
