package game

// Symmetric shadowcasting (Albert Ford). Slopes are exact rationals.

type frac struct{ n, d int } // d > 0

func floorDiv(a, b int) int {
	q := a / b
	if (a%b != 0) && ((a < 0) != (b < 0)) {
		q--
	}
	return q
}

// roundTiesUp(depth*slope) and roundTiesDown(depth*slope)
func (f frac) mulRoundUp(depth int) int {
	return floorDiv(2*depth*f.n+f.d, 2*f.d)
}
func (f frac) mulRoundDown(depth int) int {
	// ceil(x - 0.5)
	return -floorDiv(-(2*depth*f.n - f.d), 2*f.d)
}

// ComputeFOV calls mark for every cell visible from (ox,oy) within radius (aspect-corrected metric).
func ComputeFOV(l *Level, ox, oy int, radius float64, mark func(x, y int)) {
	mark(ox, oy)
	for q := 0; q < 4; q++ {
		maxDepth := int(radius) + 1
		if q == 0 || q == 2 { // north/south: rows are y, which count double
			maxDepth = int(radius/2) + 2
		}
		tr := func(depth, col int) (int, int) {
			switch q {
			case 0:
				return ox + col, oy - depth
			case 1:
				return ox + depth, oy + col
			case 2:
				return ox + col, oy + depth
			}
			return ox - depth, oy + col
		}
		blocked := func(depth, col int) bool {
			x, y := tr(depth, col)
			return Solid(l.At(x, y))
		}
		reveal := func(depth, col int) {
			x, y := tr(depth, col)
			if x < 0 || y < 0 || x >= l.W || y >= l.H {
				return
			}
			if dist(ox, oy, x, y) <= radius {
				mark(x, y)
			}
		}
		var scan func(depth int, start, end frac)
		scan = func(depth int, start, end frac) {
			if depth > maxDepth {
				return
			}
			minCol := start.mulRoundUp(depth)
			maxCol := end.mulRoundDown(depth)
			prevSet := false
			prevWall := false
			for col := minCol; col <= maxCol; col++ {
				wall := blocked(depth, col)
				sym := col*start.d >= depth*start.n && col*end.d <= depth*end.n
				if wall || sym {
					reveal(depth, col)
				}
				if prevSet && prevWall && !wall {
					start = frac{2*col - 1, 2 * depth}
				}
				if prevSet && !prevWall && wall {
					scan(depth+1, start, frac{2*col - 1, 2 * depth})
				}
				prevSet, prevWall = true, wall
			}
			if prevSet && !prevWall {
				scan(depth+1, start, end)
			}
		}
		scan(1, frac{-1, 1}, frac{1, 1})
	}
}

// LineClear reports whether a straight line from a to b is free of opaque tiles (endpoints excluded).
func (l *Level) LineClear(ax, ay, bx, by int) bool {
	pts := Line(ax, ay, bx, by)
	for i := 1; i < len(pts)-1; i++ {
		if Solid(l.At(pts[i].X, pts[i].Y)) {
			return false
		}
	}
	return true
}

// Line returns Bresenham points from a to b inclusive.
func Line(ax, ay, bx, by int) []Pt {
	var out []Pt
	dx, dy := abs(bx-ax), -abs(by-ay)
	sx, sy := 1, 1
	if ax > bx {
		sx = -1
	}
	if ay > by {
		sy = -1
	}
	err := dx + dy
	x, y := ax, ay
	for {
		out = append(out, Pt{x, y})
		if x == bx && y == by {
			break
		}
		e2 := 2 * err
		if e2 >= dy {
			err += dy
			x += sx
		}
		if e2 <= dx {
			err += dx
			y += sy
		}
	}
	return out
}
