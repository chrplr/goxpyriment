// Copyright (2026) Christophe Pallier <christophe@pallier.org>
// Distributed under the GNU General Public License v3.

package stimuli

import (
	"github.com/chrplr/goxpyriment/io"
	"github.com/Zyko0/go-sdl3/sdl"
)

// ThermometerDisplay represents a segmented progress bar with an optional goal.
type ThermometerDisplay struct {
	State           float32 // 0 to 100
	Goal            float32 // 0 to 100
	Size            sdl.FPoint
	NrSegments      int
	Position        sdl.FPoint
	FrameColor      sdl.Color
	ActiveColor     sdl.Color
	InactiveColor   sdl.Color
	GapColor        sdl.Color
	GoalColor       sdl.Color
	SegmentGap      float32
}

// NewThermometerDisplay creates a new ThermometerDisplay.
func NewThermometerDisplay(size sdl.FPoint, nrSegments int, state, goal float32) *ThermometerDisplay {
	return &ThermometerDisplay{
		State:         state,
		Goal:          goal,
		Size:          size,
		NrSegments:    nrSegments,
		Position:      sdl.FPoint{X: 0, Y: 0},
		FrameColor:    sdl.Color{R: 255, G: 255, B: 255, A: 255},
		ActiveColor:   sdl.Color{R: 255, G: 0, B: 0, A: 255},
		InactiveColor: sdl.Color{R: 100, G: 100, B: 100, A: 255},
		GapColor:      sdl.Color{R: 0, G: 0, B: 0, A: 255},
		GoalColor:     sdl.Color{R: 255, G: 255, B: 0, A: 255},
		SegmentGap:    2,
	}
}

func (td *ThermometerDisplay) Draw(screen *io.Screen) error {
	// Draw Frame
	f := NewRectangle(td.Position.X, td.Position.Y, td.Size.X, td.Size.Y, td.FrameColor)
	if err := f.Draw(screen); err != nil {
		return err
	}
	
	// Inner area
	border := float32(2)
	innerW := td.Size.X - 2*border
	innerH := td.Size.Y - 2*border
	
	segmentH := (innerH - float32(td.NrSegments-1)*td.SegmentGap) / float32(td.NrSegments)
	
	for i := 0; i < td.NrSegments; i++ {
		// Calculate percentage of current segment
		segPerc := float32(i) * 100.0 / float32(td.NrSegments)
		
		color := td.InactiveColor
		if td.State > segPerc {
			color = td.ActiveColor
		}
		
		// Position from bottom to top
		yPos := td.Position.Y - td.Size.Y/2 + border + float32(i)*(segmentH+td.SegmentGap) + segmentH/2
		// Wait, Expyriment usually has 0,0 as center. 
		// yPos calculation above:
		// td.Position.Y is center.
		// td.Position.Y - td.Size.Y/2 is bottom edge.
		
		s := NewRectangle(td.Position.X, yPos - td.Size.Y/2 + td.Size.Y/2, innerW, segmentH, color)
		// Let's re-calculate y properly relative to center 0,0
		// Bottom is -Size.Y/2
		segY := -td.Size.Y/2 + border + float32(i)*(segmentH+td.SegmentGap) + segmentH/2
		s.Position = sdl.FPoint{X: td.Position.X, Y: td.Position.Y + segY}
		
		if err := s.Draw(screen); err != nil {
			return err
		}
	}
	
	// Draw Goal
	if td.Goal >= 0 {
		goalY := -td.Size.Y/2 + border + (td.Goal/100.0)*innerH
		gPos := sdl.FPoint{X: td.Position.X, Y: td.Position.Y + goalY}
		
		cX, cY := screen.CenterToSDL(gPos.X, gPos.Y)
		halfGoalSize := float32(5)
		
		if err := screen.Renderer.SetDrawColor(td.GoalColor.R, td.GoalColor.G, td.GoalColor.B, td.GoalColor.A); err != nil {
			return err
		}
		
		// Draw two triangles/diamonds on the sides
		// Left side
		lx := cX - td.Size.X/2 - border
		pts := []sdl.FPoint{
			{X: lx - halfGoalSize, Y: cY},
			{X: lx, Y: cY - halfGoalSize},
			{X: lx + halfGoalSize, Y: cY},
			{X: lx, Y: cY + halfGoalSize},
			{X: lx - halfGoalSize, Y: cY},
		}
		for i := 0; i < 4; i++ {
			screen.Renderer.RenderLine(pts[i].X, pts[i].Y, pts[i+1].X, pts[i+1].Y)
		}
		
		// Right side
		rx := cX + td.Size.X/2 + border
		pts2 := []sdl.FPoint{
			{X: rx - halfGoalSize, Y: cY},
			{X: rx, Y: cY - halfGoalSize},
			{X: rx + halfGoalSize, Y: cY},
			{X: rx, Y: cY + halfGoalSize},
			{X: rx - halfGoalSize, Y: cY},
		}
		for i := 0; i < 4; i++ {
			screen.Renderer.RenderLine(pts2[i].X, pts2[i].Y, pts2[i+1].X, pts2[i+1].Y)
		}
	}
	
	return nil
}

func (td *ThermometerDisplay) Present(screen *io.Screen, clear, update bool) error {
	if clear {
		if err := screen.Clear(); err != nil {
			return err
		}
	}
	if err := td.Draw(screen); err != nil {
		return err
	}
	if update {
		return screen.Update()
	}
	return nil
}

func (td *ThermometerDisplay) GetPosition() sdl.FPoint {
	return td.Position
}

func (td *ThermometerDisplay) SetPosition(pos sdl.FPoint) {
	td.Position = pos
}

func (td *ThermometerDisplay) Preload() error { return nil }
func (td *ThermometerDisplay) Unload() error  { return nil }
