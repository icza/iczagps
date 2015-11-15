// Geo location and distance calculations based on GPS coordinates (latitude and longitdue)

package logic

// I intentionally do not import "appengine" just for the GeoPoint type...

// If I ever want to switch to a hexagonal grid, see http://blog.ruslans.com/2011/02/hexagonal-grid-math.html

import (
	"math"
)

// Earth mean radius in meters.
//
// Earth is not a normal sphere, it's a little deformed:
// a = Equatorial radius (6,378.1370 km)
// b = Polar radius (6,356.7523 km)
// Mean radius: r = (2*a + b) / 3
// More: http://en.wikipedia.org/wiki/Earth_radius
// Error derived from approximating with a sphere of the mean radius does not matter in the level of an Area.
const earth_r = 6371009

// Circumference of Earth (K=2*r*PI)
const earth_k = 2 * earth_r * math.Pi // 40,030,230.140,708,91 meters

// distFromEq returns the distance of any point at the specified latitude from the Equator, in meters.
func distFromEq(lat float64) float64 {
	return earth_k / 360 * lat
}

// distFromGr returns the distance of the specified geopoint from Greenwich, in meters.
func distFromGr(lat, lng float64) float64 {
	// Earth Circle radius at the given latitude: cos(phi) = cr / r
	cr := math.Cos(lat*math.Pi/180) * earth_r

	// Circumference of Earch Circle at the given latitude
	crk := cr * 2 * math.Pi

	return crk / 360 * lng
}

// areaIndicesAndDistances returns the distance and indices of the Area the specified geopoint falls in.
// The distance is the distance of the Area's South-West corner from the Equator and Greenwitch in meters.
func areaDistancesAndIndices(areaSize int64, lat, lng float64) (dEq, dGr, cEq, cGr int64) {
	dEq = int64(distFromEq(lat))
	dGr = int64(distFromGr(lat, lng))

	// dEq range ~ -10,000,000..10,000,000 meters
	// dGr range ~ -20,000,000..20,000,000 meters

	// Trim distances to area size:
	// Since both 3/10 and -3/10 are 0, we have to manually decrease if negative:
	cEq = dEq / areaSize
	cGr = dGr / areaSize
	if lat < 0 {
		cEq--
	}
	if lng < 0 {
		cGr--
	}

	// Given areaSize = 2,000 meters, ranges of the area indices would be the following:
	// cEq range ~  -5,000.. 5,000
	// cGr range ~ -10,000..10,000

	return
}

// AreaCodeForGeoPt returns the Area code to be used for searching for GPS records nerby the specified GeoPoint.
//
// The Area code contains the distance from the Equator and Greenwich in an encoded form.
// The Area code identifies a square area in which the geopoint falls.
// The identified square has a size specified by the AreaSize constant.
func AreaCodeForGeoPt(areaSize int64, lat, lng float64) int64 {
	_, _, cEq, cGr := areaDistancesAndIndices(areaSize, lat, lng)

	return packAreaIndices(cEq, cGr)
}

// AreaCodesForGeoPt returns the Area codes to be stored in GPS records for the specified GeoPoint
// to be able to search by location later on.
//
// An Area code contains the distance from the Equator and Greenwich in an encoded form.
// The Area code identifies a square area in which the geopoint falls.
// The identified square has a size specified by the areaSize parameter
// which is the double of the search precision indicated on the user interface.
//
// The returned area codes include the area code in which the specified geopoint falls,
// 2 more from the neightbours depending on the location of the point inside the area,
// and optionally a 4th one which is a "corner" neighbour if the point inside the area is within HalfAreaSize
// radius from the corner.
//
// Using this system and filtering by the area code in which a GeoPoint falls, we can list all records (GeoPoints)
// which are within the radius of HalfAreaSize around the GeoPoint. Also additional records may be included
// in the search results which are within the radius of (0.5 + sqrt(2))*AreaSize which is ~1.914*AreaSize
// (and thus ~3.83*search precision).
//
// To sum it up, filtering/searching:
//     -Includes all records within HalfAreaSize radius
//     -Records are optionally included within radius 1.914*AreaSize
//     -No records are included beyond the radius 1.914*AreaSize
func AreaCodesForGeoPt(areaSize int64, lat, lng float64) []int64 {
	dEq, dGr, cEq, cGr := areaDistancesAndIndices(areaSize, lat, lng)

	areaCodes := make([]int64, 1, 4)
	// Main area code:
	areaCodes[0] = packAreaIndices(cEq, cGr)

	// Location inside an area:
	inEq := dEq - cEq*areaSize
	inGr := dGr - cGr*areaSize

	// Half of the size of the Areas, in meters
	halfAreaSize := areaSize / 2

	// Square of the half of the size of the Areas, in meters2
	halfAreaSize2 := halfAreaSize * halfAreaSize

	// Side neighbours:
	if inEq <= halfAreaSize { // South
		areaCodes = append(areaCodes, packAreaIndices(cEq-1, cGr))
	} else { // North
		areaCodes = append(areaCodes, packAreaIndices(cEq+1, cGr))
	}
	if inGr <= halfAreaSize { // West
		areaCodes = append(areaCodes, packAreaIndices(cEq, cGr-1))
	} else { // East
		areaCodes = append(areaCodes, packAreaIndices(cEq, cGr+1))
	}

	// Corner neighbours:
	if inEq <= halfAreaSize { // South
		y := inEq
		if inGr <= halfAreaSize { // Sount-West
			if x := inGr; (x*x + y*y) < halfAreaSize2 {
				areaCodes = append(areaCodes, packAreaIndices(cEq-1, cGr-1))
			}
		} else { // Sount-East
			if x := halfAreaSize - inGr; (x*x + y*y) < halfAreaSize2 {
				areaCodes = append(areaCodes, packAreaIndices(cEq-1, cGr+1))
			}
		}
	} else { // North
		y := halfAreaSize - inEq
		if inGr <= halfAreaSize { // North-West
			if x := inGr; (x*x + y*y) < halfAreaSize2 {
				areaCodes = append(areaCodes, packAreaIndices(cEq+1, cGr-1))
			}
		} else { // Sount-East
			if x := halfAreaSize - inGr; (x*x + y*y) < halfAreaSize2 {
				areaCodes = append(areaCodes, packAreaIndices(cEq+1, cGr+1))
			}
		}
	}

	return areaCodes
}

// packAreaIndices packs the area indices into one Area code of type int64.
func packAreaIndices(cEq, cGr int64) int64 {
	// Encode in base 10 so distances can be extracted with "eye" without having to look into hexa or binary form.
	// Reserve 5 digits to both in the result.
	// Shift them to be positive:
	cEq += 50 * 1000
	cGr += 50 * 1000

	return cEq*1e5 + cGr
}

// Distance returns the distance between 2 geopoints in meters.
// The returned distance is calculated using the Pythagorean theorem.
// Accurate enough if distance is less than 1,000 km = 1,000,000 meters.
func Distance(lat, lng, lat2, lng2 float64) int64 {
	dEq := distFromEq(lat)
	dGr := distFromGr(lat, lng)

	dEq -= distFromEq(lat2)
	dGr -= distFromGr(lat2, lng2)

	return int64(math.Sqrt(dEq*dEq + dGr*dGr))
}
