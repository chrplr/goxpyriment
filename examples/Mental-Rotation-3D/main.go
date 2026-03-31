// Copyright (2026) Christophe Pallier <christophe@pallier.org>
// Distributed under the GNU General Public License v3.

package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"image/png"
	"log"
	"math"
	"math/rand"
	"os"
	"sort"

	"github.com/chrplr/goxpyriment/clock"
	"github.com/chrplr/goxpyriment/control"
	"github.com/chrplr/goxpyriment/design"
	"github.com/chrplr/goxpyriment/stimuli"
)

// --- 3D Generator & Renderer ---

type Vector3 struct{ X, Y, Z int }
type Vector3f struct{ X, Y, Z float64 }
type Point2D struct{ X, Y int }
type Face struct {
	Vertices    [4]Vector3f
	Center      Vector3f
	Normal      Vector3f
	LocalNormal Vector3f
	CubeIdx     int
}

type Metadata struct {
	TrialID         string     `json:"trial_id"`
	IsSame          bool       `json:"is_same"`
	RotationAngle   float64    `json:"rotation_angle"`
	RotationAxis    string     `json:"rotation_axis"`
	CubeCoordinates [][3]int   `json:"cube_coordinates"`
}

func min(a, b int) int { if a < b { return a }; return b }
func max(a, b int) int { if a > b { return a }; return b }
func abs(x int) int    { if x < 0 { return -x }; return x }

func canonicalize(pts []Vector3) []Vector3 {
	if len(pts) == 0 {
		return pts
	}
	minX, minY, minZ := pts[0].X, pts[0].Y, pts[0].Z
	for _, p := range pts {
		if p.X < minX { minX = p.X }
		if p.Y < minY { minY = p.Y }
		if p.Z < minZ { minZ = p.Z }
	}
	res := make([]Vector3, len(pts))
	for i, p := range pts {
		res[i] = Vector3{p.X - minX, p.Y - minY, p.Z - minZ}
	}
	sort.Slice(res, func(i, j int) bool {
		if res[i].X != res[j].X { return res[i].X < res[j].X }
		if res[i].Y != res[j].Y { return res[i].Y < res[j].Y }
		return res[i].Z < res[j].Z
	})
	return res
}

func isChiral(pts []Vector3) bool {
	orig := canonicalize(pts)
	mirrored := make([]Vector3, len(pts))
	for i, p := range pts {
		mirrored[i] = Vector3{-p.X, p.Y, p.Z}
	}

	rotate := func(p Vector3, i int) Vector3 {
		x, y, z := p.X, p.Y, p.Z
		switch i {
		case 0: return Vector3{x, y, z}
		case 1: return Vector3{x, -z, y}
		case 2: return Vector3{x, -y, -z}
		case 3: return Vector3{x, z, -y}
		case 4: return Vector3{-x, -y, z}
		case 5: return Vector3{-x, -z, -y}
		case 6: return Vector3{-x, y, -z}
		case 7: return Vector3{-x, z, y}
		case 8: return Vector3{-y, x, z}
		case 9: return Vector3{-y, -z, x}
		case 10: return Vector3{-y, -x, -z}
		case 11: return Vector3{-y, z, -x}
		case 12: return Vector3{y, -x, z}
		case 13: return Vector3{y, -z, -x}
		case 14: return Vector3{y, x, -z}
		case 15: return Vector3{y, z, x}
		case 16: return Vector3{-z, y, x}
		case 17: return Vector3{-z, -x, y}
		case 18: return Vector3{-z, -y, -x}
		case 19: return Vector3{-z, x, -y}
		case 20: return Vector3{z, y, -x}
		case 21: return Vector3{z, -x, -y}
		case 22: return Vector3{z, -y, x}
		case 23: return Vector3{z, x, y}
		}
		return p
	}

	for i := 0; i < 24; i++ {
		rotatedMirrored := make([]Vector3, len(pts))
		for j, p := range mirrored {
			rotatedMirrored[j] = rotate(p, i)
		}
		cand := canonicalize(rotatedMirrored)
		match := true
		for j := range orig {
			if orig[j] != cand[j] {
				match = false
				break
			}
		}
		if match { return false }
	}
	return true
}

func generateBaseShape(numCubes int) []Vector3 {
	for {
		pts := []Vector3{{0, 0, 0}}
		dirs := []Vector3{
			{1, 0, 0}, {-1, 0, 0}, {0, 1, 0}, {0, -1, 0}, {0, 0, 1}, {0, 0, -1},
		}

		dirIdx := rand.Intn(len(dirs))
		d1 := dirs[dirIdx]

		var orthos []Vector3
		for _, d := range dirs {
			if d.X*d1.X+d.Y*d1.Y+d.Z*d1.Z == 0 {
				orthos = append(orthos, d)
			}
		}
		d2 := orthos[rand.Intn(len(orthos))]

		var orthos2 []Vector3
		for _, d := range dirs {
			if d.X*d2.X+d.Y*d2.Y+d.Z*d2.Z == 0 {
				// Avoid returning to the same plane immediately for small numCubes
				if d.X == -d1.X && d.Y == -d1.Y && d.Z == -d1.Z {
					continue
				}
				orthos2 = append(orthos2, d)
			}
		}
		if len(orthos2) == 0 { continue }
		d3 := orthos2[rand.Intn(len(orthos2))]

		var orthos3 []Vector3
		for _, d := range dirs {
			if d.X*d3.X+d.Y*d3.Y+d.Z*d3.Z == 0 {
				if d.X == -d2.X && d.Y == -d2.Y && d.Z == -d2.Z {
					continue
				}
				orthos3 = append(orthos3, d)
			}
		}
		if len(orthos3) == 0 { continue }
		d4 := orthos3[rand.Intn(len(orthos3))]

		// Distribute remaining cubes (numCubes - 1) among 4 segments, each at least 1 cube.
		remaining := numCubes - 1
		lens := []int{1, 1, 1, 1}
		for i := 0; i < remaining-4; i++ {
			lens[rand.Intn(4)]++
		}
		rand.Shuffle(len(lens), func(i, j int) { lens[i], lens[j] = lens[j], lens[i] })

		curr := Vector3{0, 0, 0}
		valid := true
		visited := map[Vector3]bool{curr: true}

		segments := []Vector3{d1, d2, d3, d4}
		for i, d := range segments {
			for step := 0; step < lens[i]; step++ {
				curr.X += d.X
				curr.Y += d.Y
				curr.Z += d.Z
				if visited[curr] {
					valid = false
					break
				}
				visited[curr] = true
				pts = append(pts, curr)
			}
			if !valid { break }
		}

		if len(pts) == numCubes && valid && isChiral(pts) {
			return pts
		}
	}
}

func centerShape(pts []Vector3) []Vector3f {
	var sum Vector3
	for _, p := range pts {
		sum.X += p.X
		sum.Y += p.Y
		sum.Z += p.Z
	}
	cx := float64(sum.X) / float64(len(pts))
	cy := float64(sum.Y) / float64(len(pts))
	cz := float64(sum.Z) / float64(len(pts))

	var res []Vector3f
	for _, p := range pts {
		res = append(res, Vector3f{float64(p.X) - cx, float64(p.Y) - cy, float64(p.Z) - cz})
	}
	return res
}

func getCubeFaces(c Vector3f, idx int) [6]Face {
	s := 0.5
	f0 := [4]Vector3f{{-s, -s, s}, {s, -s, s}, {s, s, s}, {-s, s, s}} // Front
	f1 := [4]Vector3f{{-s, -s, -s}, {-s, s, -s}, {s, s, -s}, {s, -s, -s}} // Back
	f2 := [4]Vector3f{{-s, s, s}, {s, s, s}, {s, s, -s}, {-s, s, -s}} // Top
	f3 := [4]Vector3f{{-s, -s, s}, {-s, -s, -s}, {s, -s, -s}, {s, -s, s}} // Bottom
	f4 := [4]Vector3f{{s, -s, s}, {s, -s, -s}, {s, s, -s}, {s, s, s}} // Right
	f5 := [4]Vector3f{{-s, -s, s}, {-s, s, s}, {-s, s, -s}, {-s, -s, -s}} // Left

	normals := []Vector3f{
		{0, 0, 1}, {0, 0, -1}, {0, 1, 0}, {0, -1, 0}, {1, 0, 0}, {-1, 0, 0},
	}

	var res [6]Face
	for i, f := range [][4]Vector3f{f0, f1, f2, f3, f4, f5} {
		for j := 0; j < 4; j++ {
			res[i].Vertices[j] = Vector3f{c.X + f[j].X, c.Y + f[j].Y, c.Z + f[j].Z}
		}
		res[i].LocalNormal = normals[i]
		res[i].CubeIdx = idx
	}
	return res
}

func rotateZ(p Vector3f, deg float64) Vector3f {
	rad := deg * math.Pi / 180.0
	c, s := math.Cos(rad), math.Sin(rad)
	return Vector3f{p.X*c - p.Y*s, p.X*s + p.Y*c, p.Z}
}
func rotateY(p Vector3f, deg float64) Vector3f {
	rad := deg * math.Pi / 180.0
	c, s := math.Cos(rad), math.Sin(rad)
	return Vector3f{p.X*c + p.Z*s, p.Y, -p.X*s + p.Z*c}
}
func rotateX(p Vector3f, deg float64) Vector3f {
	rad := deg * math.Pi / 180.0
	c, s := math.Cos(rad), math.Sin(rad)
	return Vector3f{p.X, p.Y*c - p.Z*s, p.Y*s + p.Z*c}
}

func drawLine(img *image.RGBA, p0, p1 Point2D, col color.RGBA) {
	dx := abs(p1.X - p0.X)
	sx := -1
	if p0.X < p1.X {
		sx = 1
	}
	dy := -abs(p1.Y - p0.Y)
	sy := -1
	if p0.Y < p1.Y {
		sy = 1
	}
	err := dx + dy

	x, y := p0.X, p0.Y
	for {
		if x >= 0 && x < img.Bounds().Dx() && y >= 0 && y < img.Bounds().Dy() {
			img.SetRGBA(x, y, col)
		}
		if x == p1.X && y == p1.Y {
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
}

func fillTriangleWithID(img *image.RGBA, p1, p2, p3 Point2D, col color.RGBA, buffer []int, width int, id int) {
	minX := min(p1.X, min(p2.X, p3.X))
	maxX := max(p1.X, max(p2.X, p3.X))
	minY := min(p1.Y, min(p2.Y, p3.Y))
	maxY := max(p1.Y, max(p2.Y, p3.Y))

	b := img.Bounds()
	if minX < b.Min.X { minX = b.Min.X }
	if maxX >= b.Max.X { maxX = b.Max.X - 1 }
	if minY < b.Min.Y { minY = b.Min.Y }
	if maxY >= b.Max.Y { maxY = b.Max.Y - 1 }

	edgeFunc := func(a, b, c Point2D) int {
		return (c.X-a.X)*(b.Y-a.Y) - (c.Y-a.Y)*(b.X-a.X)
	}

	area := edgeFunc(p1, p2, p3)
	if area < 0 {
		p1, p2 = p2, p1
		area = -area
	}
	if area == 0 {
		return
	}

	for y := minY; y <= maxY; y++ {
		for x := minX; x <= maxX; x++ {
			p := Point2D{X: x, Y: y}
			w0 := edgeFunc(p2, p3, p)
			w1 := edgeFunc(p3, p1, p)
			w2 := edgeFunc(p1, p2, p)
			if w0 >= 0 && w1 >= 0 && w2 >= 0 {
				img.SetRGBA(x, y, col)
				buffer[y*width+x] = id
			}
		}
	}
}

func renderCondition(faces []Face, mirror bool, taskAxis string, taskDeg float64, scaling float64, filepath string, numCubes int) bool {
	size := int(300 * scaling)
	img := image.NewRGBA(image.Rect(0, 0, size, size))
	draw.Draw(img, img.Bounds(), &image.Uniform{color.RGBA{255, 255, 255, 255}}, image.Point{}, draw.Src) // White background

	// Buffer to track which cube is visible at each pixel
	cubeBuffer := make([]int, size*size)
	for i := range cubeBuffer { cubeBuffer[i] = -1 }

	var transformed []Face
	for _, f := range faces {
		var tf Face
		tf.CubeIdx = f.CubeIdx
		tf.LocalNormal = f.LocalNormal
		for i, v := range f.Vertices {
			if mirror {
				v.X = -v.X
			}
			v = rotateY(v, 45)
			v = rotateX(v, 35.264)
			if taskAxis == "Z" {
				v = rotateZ(v, taskDeg)
			}
			if taskAxis == "Y" {
				v = rotateY(v, taskDeg)
			}
			if taskAxis == "X" {
				v = rotateX(v, taskDeg)
			}
			tf.Vertices[i] = v
		}

		if mirror {
			tf.Vertices[1], tf.Vertices[3] = tf.Vertices[3], tf.Vertices[1]
		}

		u := Vector3f{tf.Vertices[1].X - tf.Vertices[0].X, tf.Vertices[1].Y - tf.Vertices[0].Y, tf.Vertices[1].Z - tf.Vertices[0].Z}
		v := Vector3f{tf.Vertices[3].X - tf.Vertices[0].X, tf.Vertices[3].Y - tf.Vertices[0].Y, tf.Vertices[3].Z - tf.Vertices[0].Z}
		tf.Normal = Vector3f{u.Y*v.Z - u.Z*v.Y, u.Z*v.X - u.X*v.Z, u.X*v.Y - u.Y*v.X}

		tf.Center = Vector3f{
			(tf.Vertices[0].X + tf.Vertices[1].X + tf.Vertices[2].X + tf.Vertices[3].X) / 4,
			(tf.Vertices[0].Y + tf.Vertices[1].Y + tf.Vertices[2].Y + tf.Vertices[3].Y) / 4,
			(tf.Vertices[0].Z + tf.Vertices[1].Z + tf.Vertices[2].Z + tf.Vertices[3].Z) / 4,
		}

		if tf.Normal.Z > 0 {
			transformed = append(transformed, tf)
		}
	}

	sort.Slice(transformed, func(i, j int) bool { return transformed[i].Center.Z < transformed[j].Center.Z })

	for _, tf := range transformed {
		var col color.RGBA
		// Use fixed shades based on LocalNormal for a clean Labvanced look
		if tf.LocalNormal.Z != 0 { // Front/Back in local space
			col = color.RGBA{220, 220, 220, 255} // Lightest
		} else if tf.LocalNormal.Y != 0 { // Top/Bottom in local space
			col = color.RGBA{160, 160, 160, 255} // Middle
		} else { // Right/Left in local space
			col = color.RGBA{100, 100, 100, 255} // Darkest
		}

		var p [4]Point2D
		offset := float64(size) / 2.0
		cubeScale := 25.0 * scaling
		for i := 0; i < 4; i++ {
			p[i] = Point2D{
				X: int(tf.Vertices[i].X*cubeScale + offset),
				Y: int(-tf.Vertices[i].Y*cubeScale + offset),
			}
		}

		// Use CubeIdx in buffer for visibility check
		fillTriangleWithID(img, p[0], p[1], p[2], col, cubeBuffer, size, tf.CubeIdx)
		fillTriangleWithID(img, p[0], p[2], p[3], col, cubeBuffer, size, tf.CubeIdx)

		// Black outlines
		outline := color.RGBA{0, 0, 0, 255}
		drawLine(img, p[0], p[1], outline)
		drawLine(img, p[1], p[2], outline)
		drawLine(img, p[2], p[3], outline)
		drawLine(img, p[3], p[0], outline)
	}

	// Visibility check
	visible := make(map[int]bool)
	for _, id := range cubeBuffer {
		if id != -1 {
			visible[id] = true
		}
	}

	if filepath != "" {
		f, _ := os.Create(filepath)
		defer f.Close()
		png.Encode(f, img)
	}
	return len(visible) == numCubes
}

func generateStimuli(numCubes int, scaling float64) {
	os.MkdirAll("stimuli", 0755)
	if _, err := os.Stat("stimuli/left.png"); err == nil {
		fmt.Println("Stimuli already generated.")
		return
	}

	angles := []int{0, 20, 40, 60, 80, 100, 120, 140, 160, 180}
	conditions := []string{"same", "mirrored"}

	for {
		fmt.Printf("Searching for a valid 3D shape with %d cubes...\n", numCubes)
		pts := generateBaseShape(numCubes)
		centered := centerShape(pts)

		var faces []Face
		for i, c := range centered {
			for _, f := range getCubeFaces(c, i) {
				faces = append(faces, f)
			}
		}

		// Verify visibility for all experimental conditions
		allVisible := true
		// Check Reference (left)
		if !renderCondition(faces, false, "Z", 0, scaling, "", numCubes) {
			allVisible = false
		}
		if allVisible {
			for _, angle := range angles {
				for _, cond := range conditions {
					isSame := cond == "same"
					if !renderCondition(faces, !isSame, "Y", float64(angle), scaling, "", numCubes) {
						allVisible = false
						break
					}
				}
				if !allVisible { break }
			}
		}

		if allVisible {
			fmt.Println("Found valid shape. Rendering...")
			renderCondition(faces, false, "Z", 0, scaling, "stimuli/left.png", numCubes)
			for _, angle := range angles {
				for _, cond := range conditions {
					isSame := cond == "same"
					filename := fmt.Sprintf("stimuli/right_%s_%d.png", cond, angle)
					renderCondition(faces, !isSame, "Y", float64(angle), scaling, filename, numCubes)

					var coords [][3]int
					for _, p := range pts {
						coords = append(coords, [3]int{p.X, p.Y, p.Z})
					}

					meta := Metadata{
						TrialID:         fmt.Sprintf("%s_%d", cond, angle),
						IsSame:          isSame,
						RotationAngle:   float64(angle),
						RotationAxis:    "Y",
						CubeCoordinates: coords,
					}
					b, _ := json.MarshalIndent(meta, "", "  ")
					os.WriteFile(fmt.Sprintf("stimuli/meta_%s_%d.json", cond, angle), b, 0644)
				}
			}
			return
		}
	}
}

func showInstructions(exp *control.Experiment) error {
	text := "Mental Rotation Task (3D)\n\n" +
		"Two 3D shapes will appear on the screen.\n" +
		"Determine if they are the SAME shape (just rotated)\n" +
		"or if they are MIRROR images of each other.\n\n" +
		"Press 'S' if they are the SAME.\n" +
		"Press 'D' if they are DIFFERENT (mirrored).\n\n" +
		"Try to be as fast and accurate as possible.\n\n" +
		"Press any key to begin."

	instrBox := stimuli.NewTextBox(text, 600, control.FPoint{X: 0, Y: 0}, control.Black)
	if err := exp.Show(instrBox); err != nil {
		return err
	}
	_, err := exp.Keyboard.Wait()
	return err
}

func main() {
	numCubes := flag.Int("nc", 5, "Number of cubes to generate the 3D shapes")
	scaling := flag.Float64("scaling", 1.0, "Scaling factor for the stimulus size")

	exp := control.NewExperimentFromFlags("Mental-Rotation-3D", control.Color{R: 220, G: 220, B: 220, A: 255}, control.Black, 32)
	defer exp.End()

	// Generate 3D pairs before the experiment starts
	generateStimuli(*numCubes, *scaling)

	exp.HideCursor()

	exp.AddDataVariableNames([]string{"trial_idx", "num_cubes", "scaling", "angle", "condition", "response", "is_correct", "rt"})

	if err := showInstructions(exp); err != nil {
		if control.IsEndLoop(err) {
			return
		}
		log.Fatalf("instruction error: %v", err)
	}

	block := design.NewBlock("Main Block")
	angles := []int{0, 20, 40, 60, 80, 100, 120, 140, 160, 180}
	conditions := []string{"same", "mirrored"}

	for _, angle := range angles {
		for _, cond := range conditions {
			trial := design.NewTrial()
			trial.SetFactor("angle", angle)
			trial.SetFactor("condition", cond)
			block.AddTrial(trial, 4, true)
		}
	}
	block.ShuffleTrials()

	fixation := stimuli.NewFixCross(20, 3, control.Black)

	for i, trial := range block.Trials {
		angle := trial.GetFactor("angle").(int)
		condition := trial.GetFactor("condition").(string)

		leftShape := stimuli.NewPicture("stimuli/left.png", float32(300**scaling), float32(300**scaling))
		leftShape.SetPosition(control.FPoint{X: float32(-160 * *scaling), Y: 0})

		rightFile := fmt.Sprintf("stimuli/right_%s_%d.png", condition, angle)
		rightShape := stimuli.NewPicture(rightFile, float32(300**scaling), float32(300**scaling))
		rightShape.SetPosition(control.FPoint{X: float32(160 * *scaling), Y: 0})

		exp.Screen.Clear()
		fixation.Draw(exp.Screen)
		exp.Screen.Update()
		clock.Wait(500)

		exp.Screen.Clear()
		fixation.Draw(exp.Screen)
		leftShape.Draw(exp.Screen)
		rightShape.Draw(exp.Screen)
		exp.Screen.Update()

		startTime := clock.GetTime()

		var key control.Keycode
		var err error
		for {
			key, err = exp.Keyboard.WaitKeys([]control.Keycode{control.K_S, control.K_D, control.K_ESCAPE}, -1)
			if err != nil {
				if control.IsEndLoop(err) {
					return
				}
				log.Fatalf("keyboard error: %v", err)
			}
			if key != 0 {
				break
			}
		}

		rt := clock.GetTime() - startTime

		response := ""
		isCorrect := false
		if key == control.K_S {
			response = "same"
			isCorrect = (condition == "same")
		} else if key == control.K_D {
			response = "mirrored"
			isCorrect = (condition == "mirrored")
		} else if key == control.K_ESCAPE {
			return
		}

		if !isCorrect {
			stimuli.PlayBuzzer(exp.AudioDevice)
		}

		exp.Data.Add(i+1, *numCubes, *scaling, angle, condition, response, isCorrect, rt)

		exp.Screen.Clear()
		fixation.Draw(exp.Screen)
		exp.Screen.Update()
		clock.Wait(500)
	}
}
