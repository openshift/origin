// Copyright ©2019 The Gonum Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package vptree_test

import (
	"fmt"
	"log"
	"math"

	"gonum.org/v1/gonum/spatial/vptree"
)

func Example_accessiblePublicTransport() {
	// Construct a vp tree of train station locations
	// to identify accessible public transport for the
	// elderly.
	t, err := vptree.New(stations, 5, nil)
	if err != nil {
		log.Fatal(err)
	}

	// Residence.
	q := place{lat: 51.501476, lon: -0.140634}

	var keep vptree.Keeper

	// Find all stations within 0.75 of the residence.
	keep = vptree.NewDistKeeper(0.75)
	t.NearestSet(keep, q)

	fmt.Println(`Stations within 750 m of 51.501476N 0.140634W.`)
	for _, c := range keep.(*vptree.DistKeeper).Heap {
		p := c.Comparable.(place)
		fmt.Printf("%s: %0.3f km\n", p.name, p.Distance(q))
	}
	fmt.Println()

	// Find the five closest stations to the residence.
	keep = vptree.NewNKeeper(5)
	t.NearestSet(keep, q)

	fmt.Println(`5 closest stations to 51.501476N 0.140634W.`)
	for _, c := range keep.(*vptree.NKeeper).Heap {
		p := c.Comparable.(place)
		fmt.Printf("%s: %0.3f km\n", p.name, p.Distance(q))
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

// stations is a list of railways stations.
var stations = []vptree.Comparable{
	place{name: "Bond Street", lat: 51.5142, lon: -0.1494},
	place{name: "Charing Cross", lat: 51.508, lon: -0.1247},
	place{name: "Covent Garden", lat: 51.5129, lon: -0.1243},
	place{name: "Embankment", lat: 51.5074, lon: -0.1223},
	place{name: "Green Park", lat: 51.5067, lon: -0.1428},
	place{name: "Hyde Park Corner", lat: 51.5027, lon: -0.1527},
	place{name: "Leicester Square", lat: 51.5113, lon: -0.1281},
	place{name: "Marble Arch", lat: 51.5136, lon: -0.1586},
	place{name: "Oxford Circus", lat: 51.515, lon: -0.1415},
	place{name: "Picadilly Circus", lat: 51.5098, lon: -0.1342},
	place{name: "Pimlico", lat: 51.4893, lon: -0.1334},
	place{name: "Sloane Square", lat: 51.4924, lon: -0.1565},
	place{name: "South Kensington", lat: 51.4941, lon: -0.1738},
	place{name: "St. James's Park", lat: 51.4994, lon: -0.1335},
	place{name: "Temple", lat: 51.5111, lon: -0.1141},
	place{name: "Tottenham Court Road", lat: 51.5165, lon: -0.131},
	place{name: "Vauxhall", lat: 51.4861, lon: -0.1253},
	place{name: "Victoria", lat: 51.4965, lon: -0.1447},
	place{name: "Waterloo", lat: 51.5036, lon: -0.1143},
	place{name: "Westminster", lat: 51.501, lon: -0.1254},
}

// place is a vptree.Comparable implementations.
type place struct {
	name     string
	lat, lon float64
}

// Distance returns the distance between the receiver and c.
func (p place) Distance(c vptree.Comparable) float64 {
	q := c.(place)
	return haversine(p.lat, p.lon, q.lat, q.lon)
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
