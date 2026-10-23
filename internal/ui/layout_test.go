package ui

import "testing"

func TestLayoutPortraitUsesSingleFocusedPanel(t *testing.T) {
	layout := LayoutFor(40, 12)
	if layout.Main.Width != 40 || layout.Main.Height < 1 { t.Fatalf("main = %#v", layout.Main) }
	if layout.Side.Width != 0 { t.Fatalf("portrait side panel should be hidden: %#v", layout.Side) }
}

func TestLayoutLandscapeShowsSidePanel(t *testing.T) {
	layout := LayoutFor(120, 35)
	if layout.Main.Width == 0 || layout.Side.Width == 0 { t.Fatalf("expected split layout: %#v", layout) }
}
