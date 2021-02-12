package main

import (
	"fmt"
)

type Units struct {
	scale uint64
	base  string
	units []string
}

var (
	binaryUnits = &Units{
		scale: 1024,
		base:  "",
		units: []string{"KB", "MB", "GB", "TB", "PB"},
	}
	timeUnitsUs = &Units{
		scale: 1000,
		base:  "us",
		units: []string{"ms", "s"},
	}
	timeUnitsS = &Units{
		scale: 60,
		base:  "s",
		units: []string{"m", "h"},
	}
)

func FormatUnits(n float64, m *Units, prec int) string {
	amt := n
	unit := m.base

	scale := float64(m.scale) * 0.85

	for i := 0; i < len(m.units) && amt >= scale; i++ {
		amt /= float64(m.scale)
		unit = m.units[i]
	}
	return fmt.Sprintf("%.*f%s", prec, amt, unit)
}

func FormatBinary(n float64) string {
	return FormatUnits(n, binaryUnits, 2)
}

func FormatTimeUs(n float64) string {
	units := timeUnitsUs
	if n >= 1000000.0 {
		n /= 1000000.0
		units = timeUnitsS
	}
	return FormatUnits(n, units, 2)
}
