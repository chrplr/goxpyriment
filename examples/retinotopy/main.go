// Copyright (2026) Christophe Pallier <christophe@pallier.org>
// Distributed under the GNU General Public License v3.

package main

import (
	"bytes"
	"embed"
	"encoding/csv"
	"flag"
	"fmt"
	"image"
	"image/color"
	_ "image/png"
	"log"
	"os"
	"path/filepath"
	"strconv"

	"github.com/chrplr/goxpyriment/control"
	"github.com/chrplr/goxpyriment/clock"
	"github.com/chrplr/goxpyriment/stimuli"

	"github.com/Zyko0/go-sdl3/sdl"
)

//go:embed assets/stimuli_png/fixationGrid.png
var fixationGridData []byte

//go:embed assets/StimuliOrder/*.csv
var stimuliOrderFS embed.FS

const (
	WindowWidth     = 768
	WindowHeight    = 768
	FrameRate       = 15
	FrameDuration   = 1000 / FrameRate // ms
	DotSize         = 7
	MaxRunDuration  = 300 * 1000 // 300 seconds
)

var (
	BackgroundColor = control.Gray
	FixationColors  = []sdl.Color{
		control.White,
		control.Black,
		control.Red,
	}
)

type Retinotopy struct {
	Exp             *control.Experiment
	Patterns        [][]byte // RGB raw data (768x768x3)
	Masks           [][]byte // Gray raw data (768x768x1)
	FixationGrid    *sdl.Texture
	FixationDots    []*stimuli.Circle
	CombinedTexture *sdl.Texture
	PixelBuffer     []byte // RGBA buffer for CombinedTexture (768x768x4)
	
	MaskOrder       []int
	PatternOrder    []int
	DotOrder        []int
	
	RunLabel        string
	StimulusRect    *sdl.FRect // Calculated centered rect
	Scaling         float64    // Scaling factor
}

func NewRetinotopy(exp *control.Experiment, runLabel string, scaling float64) *Retinotopy {
	return &Retinotopy{
		Exp:      exp,
		RunLabel: runLabel,
		Scaling:  scaling,
	}
}

func (r *Retinotopy) showStatus(msg string) error {
	fmt.Println(msg)
	r.Exp.Screen.Clear()
	txt := stimuli.NewTextLine(msg, 0, 0, control.White)
	if err := txt.Present(r.Exp.Screen, false, true); err != nil {
		return err
	}
	
	// Process events to keep OS happy and allow interruption during loading
	var event sdl.Event
	for sdl.PollEvent(&event) {
		if event.Type == sdl.EVENT_QUIT {
			return sdl.EndLoop
		}
		if event.Type == sdl.EVENT_KEY_DOWN && event.KeyboardEvent().Key == sdl.K_ESCAPE {
			return sdl.EndLoop
		}
	}
	return nil
}

func (r *Retinotopy) LoadStimuli(subjID int, runID int, assetsDir string) error {
	// 1. Load Orders
	if err := r.showStatus("Loading orders..."); err != nil { return err }
	if err := r.loadOrders(subjID, runID); err != nil {
		return err
	}

	// 2. Load Patterns (100)
	if err := r.showStatus("Loading 100 patterns..."); err != nil { return err }
	r.Patterns = make([][]byte, 100)
	for i := 1; i <= 100; i++ {
		path := filepath.Join(assetsDir, "stimuli_png", "patterns", fmt.Sprintf("pattern_%04d.png", i))
		data, err := loadRawRGB(path)
		if err != nil {
			return fmt.Errorf("failed to load pattern %d at %s: %v", i, path, err)
		}
		r.Patterns[i-1] = data
		if i%10 == 0 {
			if err := r.showStatus(fmt.Sprintf("Loading patterns... %d%%", i)); err != nil { return err }
		}
	}

	// 3. Load Masks for current run
	maskDir := ""
	maskPrefix := ""
	numMasks := 0
	switch {
	case r.RunLabel == "RETCCW" || r.RunLabel == "RETCW":
		maskDir = "rotatingWedge"
		maskPrefix = "wedge"
		numMasks = 480
	case r.RunLabel == "RETEXP" || r.RunLabel == "RETCON":
		maskDir = "expendingCircles"
		maskPrefix = "circle"
		numMasks = 420
	case len(r.RunLabel) >= 6 && r.RunLabel[:6] == "RETBAR":
		maskDir = "swippingBars"
		maskPrefix = "bar"
		numMasks = 1680
	default:
		return fmt.Errorf("unknown run label: %s", r.RunLabel)
	}

	if err := r.showStatus(fmt.Sprintf("Loading %d masks from %s...", numMasks, maskDir)); err != nil { return err }
	r.Masks = make([][]byte, numMasks)
	for i := 1; i <= numMasks; i++ {
		path := filepath.Join(assetsDir, "stimuli_png", "masks", maskDir, fmt.Sprintf("%s_%04d.png", maskPrefix, i))
		data, err := loadRawGray(path)
		if err != nil {
			return fmt.Errorf("failed to load mask %d at %s: %v", i, path, err)
		}
		r.Masks[i-1] = data
		if i%50 == 0 {
			if err := r.showStatus(fmt.Sprintf("Loading masks... %d/%d", i, numMasks)); err != nil { return err }
		}
	}

	// 4. Load Fixation Grid
	if err := r.showStatus("Loading fixation grid..."); err != nil { return err }
	gridTex, err := r.loadTextureFromBytes(fixationGridData)
	if err != nil {
		return err
	}
	r.FixationGrid = gridTex

	// 5. Initialize Fixation Dots
	r.FixationDots = make([]*stimuli.Circle, len(FixationColors))
	scaledDotSize := float32(DotSize * r.Scaling)
	for i, c := range FixationColors {
		r.FixationDots[i] = stimuli.NewCircle(scaledDotSize, c)
	}

	// 6. Initialize Combined Texture and Buffer
	tex, err := r.Exp.Screen.Renderer.CreateTexture(sdl.PIXELFORMAT_RGBA32, sdl.TEXTUREACCESS_STREAMING, WindowWidth, WindowHeight)
	if err != nil {
		return err
	}
	r.CombinedTexture = tex
	r.CombinedTexture.SetBlendMode(sdl.BLENDMODE_BLEND)
	r.PixelBuffer = make([]byte, WindowWidth*WindowHeight*4)

	// 7. Calculate centered StimulusRect (768x768 * scaling)
	if err := r.showStatus("Finalizing setup..."); err != nil { return err }
	
	// Stimuli should be (768*scaling)x(768*scaling) and centered
	stimSize := float32(768 * r.Scaling)
	r.StimulusRect = &sdl.FRect{
		X: (float32(r.Exp.WindowWidth) - stimSize) / 2,
		Y: (float32(r.Exp.WindowHeight) - stimSize) / 2,
		W: stimSize,
		H: stimSize,
	}

	return nil
}

func (r *Retinotopy) loadOrders(subjID int, runID int) error {
	// Mask Order
	f, err := stimuliOrderFS.Open("assets/StimuliOrder/maskOrderRetinotopy.csv")
	if err != nil {
		return fmt.Errorf("failed to open embedded mask order: %v", err)
	}
	defer f.Close()
	reader := csv.NewReader(f)
	records, err := reader.ReadAll()
	if err != nil {
		return err
	}
	
	colIdx := -1
	for i, name := range records[0] {
		if name == r.RunLabel {
			colIdx = i
			break
		}
	}
	if colIdx == -1 {
		return fmt.Errorf("column %s not found in maskOrderRetinotopy.csv", r.RunLabel)
	}
	
	r.MaskOrder = make([]int, len(records)-1)
	for i := 1; i < len(records); i++ {
		val, _ := strconv.Atoi(records[i][colIdx])
		r.MaskOrder[i-1] = val
	}

	// Pattern and Dot Order
	f2, err := stimuliOrderFS.Open(fmt.Sprintf("assets/StimuliOrder/sub-%03d_stimuliOrderRetinotopy.csv", subjID))
	if err != nil {
		return fmt.Errorf("failed to open embedded subject order (sub-%03d): %v", subjID, err)
	}
	defer f2.Close()
	reader2 := csv.NewReader(f2)
	records2, err := reader2.ReadAll()
	if err != nil {
		return err
	}

	pCol := fmt.Sprintf("run%d_pattern", runID)
	dCol := fmt.Sprintf("run%d_dotColor", runID)
	pIdx, dIdx := -1, -1
	for i, name := range records2[0] {
		if name == pCol { pIdx = i }
		if name == dCol { dIdx = i }
	}
	if pIdx == -1 || dIdx == -1 {
		return fmt.Errorf("columns %s or %s not found in subject order CSV", pCol, dCol)
	}

	r.PatternOrder = make([]int, len(records2)-1)
	r.DotOrder = make([]int, len(records2)-1)
	for i := 1; i < len(records2); i++ {
		pVal, _ := strconv.Atoi(records2[i][pIdx])
		dVal, _ := strconv.Atoi(records2[i][dIdx])
		r.PatternOrder[i-1] = pVal
		r.DotOrder[i-1] = dVal
	}

	return nil
}

func (r *Retinotopy) loadTextureFromBytes(data []byte) (*sdl.Texture, error) {
	img, _, err := image.Decode(bytes.NewReader(data))
	if err != nil { return nil, err }
	
	bounds := img.Bounds()
	w, h := bounds.Dx(), bounds.Dy()
	rgba := image.NewRGBA(bounds)
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			c := img.At(x, y)
			// Convert to grayscale and use as alpha, with white color
			grayC := color.GrayModel.Convert(c).(color.Gray)
			rgba.Set(x, y, color.RGBA{255, 255, 255, grayC.Y})
		}
	}
	
	surface, err := sdl.CreateSurfaceFrom(w, h, sdl.PIXELFORMAT_RGBA32, rgba.Pix, w*4)
	if err != nil { return nil, err }
	defer surface.Destroy()
	
	return r.Exp.Screen.Renderer.CreateTextureFromSurface(surface)
}

func loadRawRGB(path string) ([]byte, error) {
	f, err := os.Open(path)
	if err != nil { return nil, err }
	defer f.Close()
	img, _, err := image.Decode(f)
	if err != nil { return nil, err }
	
	bounds := img.Bounds()
	w, h := bounds.Dx(), bounds.Dy()
	data := make([]byte, w*h*3)

	switch src := img.(type) {
	case *image.RGBA:
		for i := 0; i < w*h; i++ {
			data[i*3] = src.Pix[i*4]
			data[i*3+1] = src.Pix[i*4+1]
			data[i*3+2] = src.Pix[i*4+2]
		}
	case *image.NRGBA:
		for i := 0; i < w*h; i++ {
			data[i*3] = src.Pix[i*4]
			data[i*3+1] = src.Pix[i*4+1]
			data[i*3+2] = src.Pix[i*4+2]
		}
	default:
		for y := 0; y < h; y++ {
			for x := 0; x < w; x++ {
				r, g, b, _ := img.At(x, y).RGBA()
				idx := (y*w + x) * 3
				data[idx] = byte(r >> 8)
				data[idx+1] = byte(g >> 8)
				data[idx+2] = byte(b >> 8)
			}
		}
	}
	return data, nil
}

func loadRawGray(path string) ([]byte, error) {
	f, err := os.Open(path)
	if err != nil { return nil, err }
	defer f.Close()
	img, _, err := image.Decode(f)
	if err != nil { return nil, err }
	
	bounds := img.Bounds()
	w, h := bounds.Dx(), bounds.Dy()
	data := make([]byte, w*h)

	switch src := img.(type) {
	case *image.Gray:
		copy(data, src.Pix)
	default:
		for y := 0; y < h; y++ {
			for x := 0; x < w; x++ {
				gray := color.GrayModel.Convert(img.At(x, y)).(color.Gray)
				data[y*w+x] = gray.Y
			}
		}
	}
	return data, nil
}

func (r *Retinotopy) Instructions() error {
	msg := "Press the response button as soon as the color of the dot changes\n\nPress any key to start"
	instr := stimuli.NewTextBox(msg, 600, sdl.FPoint{X: 0, Y: 0}, control.White)
	instr.Present(r.Exp.Screen, true, true)

	for {
		key, btn, err := r.Exp.HandleEvents()
		if err != nil {
			return err
		}
		if key != 0 || btn != 0 {
			break
		}
		clock.Wait(10)
	}
	return nil
}

func (r *Retinotopy) Run() error {
	fmt.Printf("Starting run %s...\n", r.RunLabel)
	
	// Recalculate StimulusRect using logical dimensions
	stimSize := float32(768 * r.Scaling)
	r.StimulusRect = &sdl.FRect{
		X: (float32(r.Exp.WindowWidth) - stimSize) / 2,
		Y: (float32(r.Exp.WindowHeight) - stimSize) / 2,
		W: stimSize,
		H: stimSize,
	}

	startTime := clock.GetTime()
	
	r.Exp.Data.AddVariableNames([]string{
		"run_label", "trial_id", "target_time", "start_time", "end_time", 
		"pres_delay", "is_late", "mask_id", "pattern_id", "dot_color_id",
	})

	numFrames := len(r.MaskOrder)
	if len(r.PatternOrder) < numFrames { numFrames = len(r.PatternOrder) }
	
	for i := 0; i < numFrames; i++ {
		frameStartTime := clock.GetTime()
		targetTime := startTime + int64(i * FrameDuration)
		
		maskID := r.MaskOrder[i]
		patternID := r.PatternOrder[i]
		dotColorID := r.DotOrder[i]
		
		// 1. Clear Screen
		r.Exp.Screen.Clear()
		
		// 2. Prepare and draw masked pattern
		if maskID >= 0 {
			r.updateCombinedTexture(patternID, maskID)
			r.Exp.Screen.Renderer.RenderTexture(r.CombinedTexture, nil, r.StimulusRect)
		}
		
		// 3. Draw Fixation Dot
		dot := r.FixationDots[dotColorID]
		dot.Position = sdl.FPoint{X: 0, Y: 0}
		dot.Draw(r.Exp.Screen)
		
		// 4. Draw Fixation Grid
		r.Exp.Screen.Renderer.RenderTexture(r.FixationGrid, nil, r.StimulusRect)
		
		// 5. Update Screen
		r.Exp.Screen.Update()
		
		// 6. Data Logging
		endTime := clock.GetTime()
		isLate := endTime > targetTime + FrameDuration
		r.Exp.Data.Add([]interface{}{
			r.RunLabel, i, targetTime - startTime, frameStartTime - startTime, endTime - startTime,
			endTime - frameStartTime, isLate, maskID, patternID, dotColorID,
		})
		
		// 7. Handle Inputs (Fixation Task)
		// Subject should press a key or mouse button when dot color changes.
		key, btn, err := r.Exp.HandleEvents()
		if err == sdl.EndLoop {
			return sdl.EndLoop
		}
		if key != 0 {
			r.Exp.Data.Add([]interface{}{
				r.RunLabel, "keypress", targetTime - startTime, clock.GetTime() - startTime, 0,
				0, false, 0, 0, key,
			})
		}
		if btn != 0 {
			r.Exp.Data.Add([]interface{}{
				r.RunLabel, "mousepress", targetTime - startTime, clock.GetTime() - startTime, 0,
				0, false, 0, 0, btn,
			})
		}
		
		// 8. Wait for next frame
		waitDur := targetTime + int64(FrameDuration) - clock.GetTime()
		if waitDur > 0 {
			clock.Wait(int(waitDur))
		}
	}
	
	return nil
}

func (r *Retinotopy) updateCombinedTexture(patternID, maskID int) {
	pattern := r.Patterns[patternID]
	mask := r.Masks[maskID]
	
	for i := 0; i < WindowWidth*WindowHeight; i++ {
		r.PixelBuffer[i*4] = pattern[i*3]     // R
		r.PixelBuffer[i*4+1] = pattern[i*3+1] // G
		r.PixelBuffer[i*4+2] = pattern[i*3+2] // B
		r.PixelBuffer[i*4+3] = mask[i]         // A
	}
	
	r.CombinedTexture.Update(nil, r.PixelBuffer, WindowWidth*4)
}

func main() {
	subjID := flag.Int("s", 0, "Subject ID")
	runID := flag.Int("r", 1, "Run ID (1-6)")
	develop := flag.Bool("d", false, "Developer mode (windowed 1024x1024)")
	scaling := flag.Float64("scaling", 1.0, "Scaling factor for stimuli (e.g., 0.5, 1.5)")
	assetsDirFlag := flag.String("assets", "", "Path to assets directory")
	// Keep -F for backward compatibility if needed, but we'll prioritize -d
	fullscreenFlag := flag.Bool("F", false, "Force Fullscreen (redundant if -d is not used)")
	flag.Parse()

	// Assets discovery
	assetsDir := *assetsDirFlag
	if assetsDir == "" {
		// Try common locations
		candidates := []string{
			filepath.Join("examples", "retinotopy", "assets"), // Development from project root
			"assets",                  // Standalone from dist root
			"../assets",               // Standalone from linux_x64/ etc.
			"../../examples/retinotopy/assets", // If run from within some subfolders
		}
		for _, c := range candidates {
			// Check for something that is still external (patterns or masks)
			if _, err := os.Stat(filepath.Join(c, "stimuli_png", "patterns")); err == nil {
				assetsDir = c
				break
			}
		}
	}
	// Fallback to default if not found (will fail later with clear message)
	if assetsDir == "" {
		assetsDir = filepath.Join("examples", "retinotopy", "assets")
	}
	log.Printf("Using assets directory: %s", assetsDir)

	// Determine window mode:
	// - default: exclusive fullscreen at desktop resolution (0,0)
	// - developer mode: 1024x1024 window.
	width, height, fullscreen := 0, 0, true
	if *develop {
		width, height, fullscreen = 1024, 1024, false
	}
	if *fullscreenFlag {
		fullscreen = true
	}

	labels := map[int]string{
		1: "RETBAR1", 2: "RETBAR2", 3: "RETCCW", 4: "RETCW", 5: "RETEXP", 6: "RETCON",
	}
	runLabel, ok := labels[*runID]
	if !ok {
		log.Fatalf("Invalid run ID: %d", *runID)
	}

	exp := control.NewExperiment("Retinotopy", width, height, fullscreen, BackgroundColor, control.White, 32)
	exp.SubjectID = *subjID
	if err := exp.Initialize(); err != nil {
		log.Fatal(err)
	}
	defer exp.End()


	// Wait for fullscreen transition to stabilize
	// if fullscreen {
	//	clock.Wait(2000)
	//}

	// Hide the mouse cursor
	exp.Mouse.ShowCursor(false)

	retino := NewRetinotopy(exp, runLabel, *scaling)
	if err := retino.LoadStimuli(*subjID, *runID, assetsDir); err != nil {
		log.Fatal(err)
	}

	err := exp.Run(func() error {
		if err := retino.Instructions(); err != nil {
			return err
		}
		if err := retino.Run(); err != nil {
			return err
		}
		return sdl.EndLoop
	})

	if err != nil && err != sdl.EndLoop {
		log.Fatal(err)
	}
}
