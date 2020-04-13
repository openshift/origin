// Copyright ©2019 The Gonum Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package kdtree_test

import (
	"fmt"
	"math"

	"gonum.org/v1/gonum/spatial/kdtree"
)

func Example_accessiblePublicTransport() {
	// Construct a k-d tree of train station locations
	// to identify accessible public transport for the
	// elderly.
	t := kdtree.New(stations, false)

	// Residence.
	q := place{lat: 51.501476, lon: -0.140634}

	var keep kdtree.Keeper

	// Find all stations within 0.75 of the residence.
	keep = kdtree.NewDistKeeper(0.75 * 0.75) // Distances are squared.
	t.NearestSet(keep, q)

	fmt.Println(`Stations within 750 m of 51.501476N 0.140634W.`)
	for _, c := range keep.(*kdtree.DistKeeper).Heap {
		p := c.Comparable.(place)
		fmt.Printf("%s: %0.3f km\n", p.name, math.Sqrt(p.Distance(q)))
	}
	fmt.Println()

	// Find the five closest stations to the residence.
	keep = kdtree.NewNKeeper(5)
	t.NearestSet(keep, q)

	fmt.Println(`5 closest stations to 51.501476N 0.140634W.`)
	for _, c := range keep.(*kdtree.NKeeper).Heap {
		p := c.Comparable.(place)
		fmt.Printf("%s: %0.3f km\n", p.name, math.Sqrt(p.Distance(q)))
	}

	// Output:
	//
	// Stations within 750 m of 51.501476N 0.140634W.
	// St. James's Park: 0.545 km
	// Green Park: 0.600 km
	// Victoria: 0.621 km
	//
	// 5 closest stations to 51.501476N 0.140634W.
	// St. James's Park: 0.545 km
	// Green Park: 0.600 km
	// Victoria: 0.621 km
	// Hyde Park Corner: 0.846 km
	// Picadilly Circus: 1.027 km
}

// stations is a list of railways stations satisfying the
// kdtree.Interface.
var stations = places{
	{name: "Bond Street", lat: 51.5142, lon: -0.1494},
	{name: "Charing Cross", lat: 51.508, lon: -0.1247},
	{name: "Covent Garden", lat: 51.5129, lon: -0.1243},
	{name: "Embankment", lat: 51.5074, lon: -0.1223},
	{name: "Green Park", lat: 51.5067, lon: -0.1428},
	{name: "Hyde Park Corner", lat: 51.5027, lon: -0.1527},
	{name: "Leicester Square", lat: 51.5113, lon: -0.1281},
	{name: "Marble Arch", lat: 51.5136, lon: -0.1586},
	{name: "Oxford Circus", lat: 51.515, lon: -0.1415},
	{name: "Picadilly Circus", lat: 51.5098, lon: -0.1342},
	{name: "Pimlico", lat: 51.4893, lon: -0.1334},
	{name: "Sloane Square", lat: 51.4924, lon: -0.1565},
	{name: "South Kensington", lat: 51.4941, lon: -0.1738},
	{name: "St. James's Park", lat: 51.4994, lon: -0.1335},
	{name: "Temple", lat: 51.5111, lon: -0.1141},
	{name: "Tottenham Court Road", lat: 51.5165, lon: -0.131},
	{name: "Vauxhall", lat: 51.4861, lon: -0.1253},
	{name: "Victoria", lat: 51.4965, lon: -0.1447},
	{name: "Waterloo", lat: 51.5036, lon: -0.1143},
	{name: "Westminster", lat: 51.501, lon: -0.1254},
}

// place is a kdtree.Comparable implementations.
type place struct {
	name     string
	lat, lon float64
}

// Compare satisfies the axis comparisons method of the kdtree.Comparable interface.
// The dimensions are:
//  0 = lat
//  1 = lon
func (p place) Compare(c kdtree.Comparable, d kdtree.Dim) float64 {
	q := c.(place)
	switch d {
	case 0:
		return p.lat - q.lat
	case 1:
		return p.lon - q.lon
	default:
		panic("illegal dimension")
	}
}

// Dims returns the number of dimensions to be considered.
func (p place) Dims() int { return 2 }

// Distance returns the distance between the receiver and c.
func (p place) Distance(c kdtree.Comparable) float64 {
	q := c.(place)
	d := haversine(p.lat, p.lon, q.lat, q.lon)
	return d * d
}

// haversine returns the distance between two geographic coordinates.
func haversine(lat1, lon1, lat2, lon2 float64) float64 {
	const r = 6371 // km
	sdLat := math.Sin(radians(lat2-lat1) / 2)
	sdLon := math.Sin(radians(lon2-lon1) / 2)
	a := sdLat*sdLat + math.Cos(radians(lat1))*math.Cos(radians(lat2))*sdLon*sdLon
	d := 2 * r * math.Asin(math.Sqrt(a))
	return d // km
}

func radians(d float64) float64 {
	return d * math.Pi / 180
}

// places is a collection of the place type that satisfies kdtree.Interface.
type places []place

func (p places) Index(i int) kdtree.Comparable         { return p[i] }
func (p places) Len() int                              { return len(p) }
func (p places) Pivot(d kdtree.Dim) int                { return plane{places: p, Dim: d}.Pivot() }
func (p places) Slice(start, end int) kdtree.Interface { return p[start:end] }

// plane is required to help places.
type plane struct {
	kdtree.Dim
	places
}

func (p plane) Less(i, j int) bool {
	switch p.Dim {
	case 0:
		return p.places[i].lat < p.places[j].lat
	case 1:
		return p.places[i].lon < p.places[j].lon
	default:
		panic("illegal dimension")
	}
}
func (p plane) Pivot() int { return kdtree.Partition(p, kdtree.MedianOfMedians(p)) }
func (p plane) Slice(start, end int) kdtree.SortSlicer {
	p.places = p.places[start:end]
	return p
}
func (p plane) Swap(i, j int) {
	p.places[i], p.places[j] = p.places[j], p.places[i]
}
