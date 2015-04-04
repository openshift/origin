package types

import "fmt"

type Version struct {
	X, Y, Z int
}

func (a Version) Eq(b Version) bool {
	return a.X == b.X && a.Y == b.Y && a.Z == b.Z
}

func (a Version) Gt(b Version) bool {
	if a.X > b.X {
		return true
	}
	if a.X < b.X {
		return false
	} // so, Xs are equal
	if a.Y > b.Y {
		return true
	}
	if a.Y < b.Y {
		return false
	} // so, Ys are equal
	if a.Z > b.Z {
		return true
	}
	return false
}

func (v Version) GoString() string {
	return fmt.Sprintf("%d.%d.%d", v.X, v.Y, v.Z)
}

func (v Version) NonZero() bool {
	return !v.Eq(Version{0, 0, 0})
}
