package ipmi

import (
	"fmt"
)

// SensorUnit defines the unit of a sensor. It is specified in 37.17 and 43.17
// of v1.5 and v2.0 respectively. It is an 8-bit uint on the wire.
type SensorUnit uint8

const (
	_ SensorUnit = iota
	SensorUnitCelsius
	SensorUnitFahrenheit
	SensorUnitKelvin
	SensorUnitVolts
	SensorUnitAmps
	SensorUnitWatts
	SensorUnitJoules
	SensorUnitCoulombs
	SensorUnitVoltamperes
	SensorUnitNits
	SensorUnitLumen
	SensorUnitLux
	SensorUnitCandela
	SensorUnitKilopascals
	SensorUnitPoundsPerSquareInch
	SensorUnitNewtons
	SensorUnitCubicFeetPerMinute
	SensorUnitRotationsPerMinute
	SensorUnitHertz
	SensorUnitMicroseconds
	SensorUnitMilliseconds
	SensorUnitSeconds
	SensorUnitMinutes
	SensorUnitHours
	SensorUnitDays
	SensorUnitWeeks
	SensorUnitMils
	SensorUnitInches
	SensorUnitFeet
	SensorUnitCubicInches
	SensorUnitCubicFeet
	SensorUnitMillimeters
	SensorUnitCentimeters
	SensorUnitMeters
	SensorUnitCubicCentimeters
	SensorUnitCubicMeters
	SensorUnitLiters
	SensorUnitFluidOunces
	SensorUnitRadians
	SensorUnitSteradians
	SensorUnitRevolutions
	SensorUnitCycles
	SensorUnitGravities
	SensorUnitOunces
	SensorUnitPounds
	SensorUnitFeetPounds
	SensorUnitOunceInches
	SensorUnitGauss
	SensorUnitGilberts
	SensorUnitHenry
	SensorUnitMillihenry
	SensorUnitFarad
	SensorUnitMicrofarad
	SensorUnitOhms
	SensorUnitSiemens
	SensorUnitMoles
	SensorUnitBecquerel
	SensorUnitPartsPerMillion
	_
	SensorUnitDecibels
	SensorUnitDecibelsAFilter
	SensorUnitDecibelsCFilter
	SensorUnitGray
	SensorUnitSieverts
	SensorUnitColorTempKelvin
	SensorUnitBits
	SensorUnitKilobits
	SensorUnitMegabits
	SensorUnitGigabits
	SensorUnitBytes
	SensorUnitKilobytes
	SensorUnitMegabytes
	SensorUnitGigabytes
	SensorUnitWords
	SensorUnitDwords
	SensorUnitQwords
	SensorUnitMemoryLines
	SensorUnitHits
	SensorUnitMisses
	SensorUnitRetries
	SensorUnitResets
	SensorUnitOverflows
	SensorUnitUnderruns
	SensorUnitCollisions
	SensorUnitPackets
	SensorUnitMessages
	SensorUnitCharacters
	SensorUnitErrors
	SensorUnitCorrectableErrors
	SensorUnitUncorrectableErrors
	SensorUnitFatal
	SensorUnitGrams
)

var (
	sensorUnitSymbols = map[SensorUnit]string{
		SensorUnitCelsius:             "C",
		SensorUnitFahrenheit:          "F",
		SensorUnitKelvin:              "K",
		SensorUnitVolts:               "V",
		SensorUnitAmps:                "A",
		SensorUnitWatts:               "W",
		SensorUnitJoules:              "J",
		SensorUnitCoulombs:            "C",
		SensorUnitVoltamperes:         "VA",
		SensorUnitNits:                "nt",
		SensorUnitLumen:               "lm",
		SensorUnitLux:                 "lx",
		SensorUnitCandela:             "cd",
		SensorUnitKilopascals:         "kPa",
		SensorUnitPoundsPerSquareInch: "psi",
		SensorUnitNewtons:             "nt",
		SensorUnitCubicFeetPerMinute:  "CFM",
		SensorUnitRotationsPerMinute:  "RPM",
		SensorUnitHertz:               "Hz",
		SensorUnitMicroseconds:        "μs",
		SensorUnitMilliseconds:        "ms",
		SensorUnitSeconds:             "s",
		SensorUnitMinutes:             "min",
		SensorUnitHours:               "hr",
		SensorUnitDays:                "d",
		SensorUnitWeeks:               "w",
		SensorUnitMils:                "mil",
		SensorUnitInches:              "in",
		SensorUnitFeet:                "ft",
		SensorUnitCubicInches:         "in³",
		SensorUnitCubicFeet:           "ft³",
		SensorUnitMillimeters:         "mm",
		SensorUnitCentimeters:         "cm",
		SensorUnitMeters:              "m",
		SensorUnitCubicCentimeters:    "cm³",
		SensorUnitCubicMeters:         "m³",
		SensorUnitLiters:              "l",
		SensorUnitFluidOunces:         "fl oz",
		SensorUnitRadians:             "rad",
		SensorUnitSteradians:          "sr",
		SensorUnitRevolutions:         "rev",
		SensorUnitCycles:              "Hz",
		SensorUnitGravities:           "g",
		SensorUnitOunces:              "oz",
		SensorUnitPounds:              "lb",
		SensorUnitFeetPounds:          "ft-lb",
		SensorUnitOunceInches:         "oz-in",
		SensorUnitGauss:               "G",
		SensorUnitGilberts:            "Gb",
		SensorUnitHenry:               "H",
		SensorUnitMillihenry:          "mH",
		SensorUnitFarad:               "F",
		SensorUnitMicrofarad:          "μF",
		SensorUnitOhms:                "Ω",
		SensorUnitSiemens:             "Ω⁻¹",
		SensorUnitMoles:               "mol",
		SensorUnitBecquerel:           "Bq",
		SensorUnitPartsPerMillion:     "ppm",
		SensorUnitDecibels:            "dB",
		SensorUnitDecibelsAFilter:     "dBA",
		SensorUnitDecibelsCFilter:     "dBC",
		SensorUnitGray:                "Gy",
		SensorUnitSieverts:            "Sv",
		SensorUnitColorTempKelvin:     "ColorK",
		SensorUnitBits:                "b",
		SensorUnitKilobits:            "Kb",
		SensorUnitMegabits:            "Mb",
		SensorUnitGigabits:            "Gb",
		SensorUnitBytes:               "B",
		SensorUnitKilobytes:           "KB",
		SensorUnitMegabytes:           "MB",
		SensorUnitGigabytes:           "GB",
		SensorUnitWords:               "word",
		SensorUnitDwords:              "dword",
		SensorUnitQwords:              "qword",
		SensorUnitMemoryLines:         "memory line",
		SensorUnitHits:                "hit",
		SensorUnitMisses:              "miss",
		SensorUnitRetries:             "retry",
		SensorUnitResets:              "reset",
		SensorUnitOverflows:           "overflow",
		SensorUnitUnderruns:           "underrun",
		SensorUnitCollisions:          "collision",
		SensorUnitPackets:             "pkt",
		SensorUnitMessages:            "msg",
		SensorUnitCharacters:          "char",
		SensorUnitErrors:              "err",
		SensorUnitCorrectableErrors:   "correctable err",
		SensorUnitUncorrectableErrors: "uncorrectable err",
		SensorUnitFatal:               "fatal",
		SensorUnitGrams:               "g",
	}
)

func (s SensorUnit) Symbol() string {
	if s == 0 {
		return "Unspecified/Unused"
	}
	if symbol, ok := sensorUnitSymbols[s]; ok {
		return symbol
	}
	return "Unknown"
}

func (s SensorUnit) String() string {
	return fmt.Sprintf("%#v(%v)", uint8(s), s.Symbol())
}
