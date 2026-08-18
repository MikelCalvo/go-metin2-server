package worldruntime

import "testing"

func TestApproxDistanceMatchesLegacyOctileApproximation(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		dx   int32
		dy   int32
		want int32
	}{
		{name: "origin", dx: 0, dy: 0, want: 0},
		{name: "axis under gate", dx: 1000, dy: 0, want: 960},
		{name: "negative axis under gate", dx: -1000, dy: 0, want: 960},
		{name: "axis just inside", dx: 1040, dy: 0, want: 999},
		{name: "axis at gate", dx: 1041, dy: 0, want: 1000},
		{name: "axis beyond gate", dx: 1042, dy: 0, want: 1001},
		{name: "equal axes", dx: 1000, dy: 1000, want: 1359},
		{name: "diagonal under edge", dx: 700, dy: 700, want: 951},
		{name: "mixed quadrant", dx: -800, dy: 600, want: 1007},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if got := ApproxDistance(tc.dx, tc.dy); got != tc.want {
				t.Fatalf("ApproxDistance(%d, %d) = %d, want %d", tc.dx, tc.dy, got, tc.want)
			}
		})
	}
}

func TestWithinExchangeDistanceRequiresSameMapAndStrictMaxDistance(t *testing.T) {
	t.Parallel()

	left := visibilityCharacter("Left", 0x02040101, 42, 1100, 2100)
	near := visibilityCharacter("Near", 0x02040102, 42, 1100+1040, 2100)
	edge := visibilityCharacter("Edge", 0x02040103, 42, 1100+1041, 2100)
	far := visibilityCharacter("Far", 0x02040104, 42, 1100+1042, 2100)
	otherMap := visibilityCharacter("OtherMap", 0x02040105, 43, 1100, 2100)

	if !WithinExchangeDistance(left, near) {
		t.Fatal("expected near peer inside exchange distance")
	}
	if WithinExchangeDistance(left, edge) {
		t.Fatal("expected ApproxDistance >= EXCHANGE_MAX_DISTANCE peer to fail closed")
	}
	if WithinExchangeDistance(left, far) {
		t.Fatal("expected far peer outside exchange distance")
	}
	if WithinExchangeDistance(left, otherMap) {
		t.Fatal("expected different-map peer to fail closed")
	}
}
