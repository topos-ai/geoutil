package wkb

const (
	wkbXDR             byte   = 0 // Big-endian
	wkbNDR             byte   = 1 // Little-endian
	wkbPoint           uint32 = 1
	wkbLineString      uint32 = 2
	wkbPolygon         uint32 = 3
	wkbMultiPoint      uint32 = 4
	wkbMultiLineString uint32 = 5
	wkbMultiPolygon    uint32 = 6
)
