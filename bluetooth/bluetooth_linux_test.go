// +build veyronbluetooth,!android

package bluetooth

import (
	"math"
	"testing"

	"veyron/lib/unit"
)

func TestDistanceFromRSSI(t *testing.T) {
	testcases := []struct {
		rssi int
		dist unit.Distance
	}{
		{-45, 1 * unit.Meter},
		{-50, 1 * unit.Meter},
		{-55, 2 * unit.Meter},
		{-60, 4 * unit.Meter},
		{-65, 8 * unit.Meter},
		{-70, 13 * unit.Meter},
		{-80, 39 * unit.Meter},
		{-90, 113 * unit.Meter},
	}
	for _, tc := range testcases {
		d := distanceFromRSSI(tc.rssi)
		if math.Trunc(d.Meters()) != tc.dist.Meters() {
			t.Errorf("distanceFromRSSI(%d) = %v; want %v", tc.rssi, d, tc.dist)
		}
	}
}
