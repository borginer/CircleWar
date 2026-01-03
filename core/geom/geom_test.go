package geom

import "testing"

func TestLimited_Table(t *testing.T) {
	tests := []struct {
		name           string
		vec            Vector2
		lx, ux, ly, uy float32
		want           Vector2
	}{
		{"inside bounds", NewVector(2, 2), 0, 4, 0, 4, NewVector(2, 2)},
		{"outside bounds", NewVector(6, -1), 0, 4, 0, 4, NewVector(4, 0)},
		{"edge of bounds", NewVector(4, 0), 0, 4, 0, 4, NewVector(4, 0)},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got := test.vec.Limited(test.lx, test.ly, test.ux, test.uy)
			if got != test.want {
				t.Errorf("got %s - want %s", got, test.want)
			}
		})
	}
}
