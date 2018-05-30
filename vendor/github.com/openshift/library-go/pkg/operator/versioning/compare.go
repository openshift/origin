package versioning

import (
	"github.com/blang/semver"
)

type VersionRange interface {
	Between(needle *semver.Version) bool
	BetweenOrEmpty(needle *semver.Version) bool
}

type versionRange struct {
	lowerInclusive bool
	lower          semver.Version

	upperInclusive bool
	upper          semver.Version
}

// NewRange is the "normal" [1.1.0, 1.2)
func NewRange(lowerInclusive, upperExclusive string) (VersionRange, error) {
	lower, err := semver.Parse(lowerInclusive)
	if err != nil {
		return nil, err
	}
	upper, err := semver.Parse(upperExclusive)
	if err != nil {
		return nil, err
	}

	return &versionRange{
		lowerInclusive: true,
		lower:          lower,
		upper:          upper,
	}, nil
}

func NewRangeOrDie(lowerInclusive, upperExclusive string) VersionRange {
	ret, err := NewRange(lowerInclusive, upperExclusive)
	if err != nil {
		panic(err)
	}
	return ret
}

func (r versionRange) Between(needle *semver.Version) bool {
	switch {
	case r.lowerInclusive && !r.upperInclusive:
		return needle.GTE(r.lower) && needle.LT(r.upper)
	case r.lowerInclusive && r.upperInclusive:
		return needle.GTE(r.lower) && needle.LTE(r.upper)
	case !r.lowerInclusive && !r.upperInclusive:
		return needle.GT(r.lower) && needle.LT(r.upper)
	case !r.lowerInclusive && r.upperInclusive:
		return needle.GT(r.lower) && needle.LTE(r.upper)

	}

	panic("math broke")
}

func (r versionRange) BetweenOrEmpty(needle *semver.Version) bool {
	if needle == nil {
		return true
	}
	return r.Between(needle)
}
