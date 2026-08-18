package quality

import (
	"reflect"
	"testing"
)

func TestIdentifiersInSourceTracksLines(t *testing.T) {
	source := "first second\n\nthird = fourth\nfifth"
	want := []identifierAtLine{
		{name: "first", line: 1},
		{name: "second", line: 1},
		{name: "third", line: 3},
		{name: "fourth", line: 3},
		{name: "fifth", line: 4},
	}

	if got := identifiersInSource(source); !reflect.DeepEqual(got, want) {
		t.Fatalf("identifiersInSource() = %#v, want %#v", got, want)
	}
}
