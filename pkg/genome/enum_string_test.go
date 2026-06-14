package genome

import "testing"

func TestSkeletonTypeStringValid(t *testing.T) {
	cases := map[SkeletonType]string{
		Shedding:    "shedding",
		TrickTaking: "trick_taking",
		Rummy:       "rummy",
		Climbing:    "climbing",
	}
	for s, want := range cases {
		if got := s.String(); got != want {
			t.Errorf("SkeletonType(%d).String() = %q, want %q", uint8(s), got, want)
		}
	}
}

func TestSkeletonTypeStringOutOfRange(t *testing.T) {
	// An out-of-range value (e.g. from a corrupted/garbage-deserialized
	// genome) must not panic; it should degrade to a labeled fallback like
	// the sibling enum String() methods.
	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("SkeletonType.String() panicked on out-of-range value: %v", r)
		}
	}()
	got := SkeletonType(99).String()
	if got != "SkeletonType(99)" {
		t.Errorf("SkeletonType(99).String() = %q, want %q", got, "SkeletonType(99)")
	}
}
