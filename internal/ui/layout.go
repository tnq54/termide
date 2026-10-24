package ui

type Rect struct{ X, Y, Width, Height int }
type Layout struct{ Main, Side, Status, Footer Rect }

func LayoutFor(width, height int) Layout {
	if width < 80 {
		return Layout{Main: Rect{Width: width, Height: max(1, height-2)}, Status: Rect{Y: height - 2, Width: width, Height: 1}, Footer: Rect{Y: height - 1, Width: width, Height: 1}}
	}
	sideWidth := width / 3
	return Layout{Main: Rect{Width: width - sideWidth, Height: max(1, height-2)}, Side: Rect{X: width - sideWidth, Width: sideWidth, Height: max(1, height-2)}, Status: Rect{Y: height - 2, Width: width, Height: 1}, Footer: Rect{Y: height - 1, Width: width, Height: 1}}
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
