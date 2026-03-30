// Copyright (2026) Christophe Pallier <christophe@pallier.org>
// Distributed under the GNU General Public License v3.
//
// Number Comparison — replication of Buckley & Gillman (1974).
//
// Participants are shown two stimuli side-by-side and press F (left) or J
// (right) to indicate which is numerically larger. Four between-subjects
// groups differ in stimulus format.
//
// Usage:
//
//	go run . -group digits   -d -s 1
//	go run . -group regular  -d -s 2
//	go run . -group irregular -d -s 3
//	go run . -group random   -d -s 4
//
// Before building the 'regular' and 'irregular' groups, generate the stimulus
// assets:
//
//	go run ./cmd/generate_regular/
//	go run ./cmd/generate_irregular/
//
// or simply: make stimuli

package main

import (
	"embed"
	"flag"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	_ "image/png" // register PNG decoder
	"log"
	"math"
	"math/rand"
	"strconv"

	"github.com/chrplr/goxpyriment/assets_embed"
	"github.com/chrplr/goxpyriment/control"
	"github.com/chrplr/goxpyriment/design"
	gxio "github.com/chrplr/goxpyriment/apparatus"
	"github.com/chrplr/goxpyriment/stimuli"
)

//go:embed assets/regular
var regularFS embed.FS

//go:embed assets/irregular
var irregularFS embed.FS

// ── constants ────────────────────────────────────────────────────────────────

const (
	numBlocks  = 11   // total blocks; block 0 is practice
	itiMs      = 500  // inter-trial interval (ms)
	fixMs      = 500  // fixation-only period before stimulus (ms)
	maxRTms    = 5000 // maximum response window (ms)
	stimOffset = 225  // centre-to-stimulus distance (px, centre-based coords)
	stimPx     = 300  // displayed width and height of dot-pattern images (px)
	digitPt    = 96   // font size for digit stimuli
)

var (
	respKeys = []control.Keycode{control.K_F, control.K_J}
)

// ── trial structure ───────────────────────────────────────────────────────────

type trial struct {
	nLeft  int // value shown on the left  (1–9)
	nRight int // value shown on the right (1–9)
}

// buildBaseTrials returns the 72-trial list for one block: all C(9,2)=36
// pairs, each shown twice with left/right positions counterbalanced.
func buildBaseTrials() []trial {
	trials := make([]trial, 0, 72)
	for n1 := 1; n1 <= 8; n1++ {
		for n2 := n1 + 1; n2 <= 9; n2++ {
			trials = append(trials, trial{n1, n2}, trial{n2, n1})
		}
	}
	return trials // 72 entries
}

// ── random dot generation ─────────────────────────────────────────────────────

const (
	rdDotR   = 18
	rdMinGap = 6
	rdMargin = 28
)

func fillCircle(img *image.RGBA, cx, cy, r int, c color.RGBA) {
	for dy := -r; dy <= r; dy++ {
		for dx := -r; dx <= r; dx++ {
			if dx*dx+dy*dy <= r*r {
				img.SetRGBA(cx+dx, cy+dy, c)
			}
		}
	}
}

// generateDotImage creates a 300×300 RGBA dot array with n dots placed via
// rejection sampling. Uses the provided RNG so the caller controls
// reproducibility.
func generateDotImage(n int, rng *rand.Rand) *image.RGBA {
	const size = stimPx
	img := image.NewRGBA(image.Rect(0, 0, size, size))
	draw.Draw(img, img.Bounds(), image.NewUniform(color.RGBA{128, 128, 128, 255}), image.Point{}, draw.Src)

	type pt struct{ x, y int }
	var placed []pt
	minDist := float64(2*rdDotR + rdMinGap)
	lo := rdMargin + rdDotR
	hi := size - rdMargin - rdDotR

	for len(placed) < n {
		for attempt := 0; attempt < 20000; attempt++ {
			x := lo + rng.Intn(hi-lo+1)
			y := lo + rng.Intn(hi-lo+1)
			ok := true
			for _, p := range placed {
				dx, dy := float64(x-p.x), float64(y-p.y)
				if math.Sqrt(dx*dx+dy*dy) < minDist {
					ok = false
					break
				}
			}
			if ok {
				placed = append(placed, pt{x, y})
				fillCircle(img, x, y, rdDotR, color.RGBA{0, 0, 0, 255})
				break
			}
		}
	}
	return img
}

// rgbaToTexture uploads an *image.RGBA to a GPU texture via the SDL surface API.
// The caller is responsible for calling Destroy() on the returned texture.
func rgbaToTexture(screen *gxio.Screen, img *image.RGBA) (*gxio.Texture, error) {
	b := img.Bounds()
	surface, err := gxio.CreateSurfaceFrom(b.Dx(), b.Dy(), gxio.PIXELFORMAT_RGBA32, img.Pix, b.Dx()*4)
	if err != nil {
		return nil, err
	}
	defer surface.Destroy()
	return screen.Renderer.CreateTextureFromSurface(surface)
}

// renderTexCentered renders tex centred at (cx,cy) in centre-based coordinates.
func renderTexCentered(screen *gxio.Screen, tex *gxio.Texture, cx, cy, w, h float32) {
	sdlX, sdlY := screen.CenterToSDL(cx, cy)
	dst := &control.FRect{X: sdlX - w/2, Y: sdlY - h/2, W: w, H: h}
	screen.Renderer.RenderTexture(tex, nil, dst)
}

// ── instructions ─────────────────────────────────────────────────────────────

const practiceInstr = `PRACTICE BLOCK

You will see two stimuli on the left and right of a central cross.
Press F if the LEFT stimulus is numerically LARGER.
Press J if the RIGHT stimulus is numerically LARGER.
Respond as quickly and accurately as possible.

Press SPACE to begin the practice block.`

const startInstr = `Practice complete!

The experiment will now begin.
Remember:
  F → LEFT is larger
  J → RIGHT is larger

Press SPACE to start.`

const restInstr = `Rest break.

Take a short break, then press SPACE when you are ready to continue.`

const endMsg = `The experiment is complete. Thank you!

Press any key to exit.`

// ── keyLabel ──────────────────────────────────────────────────────────────────

func keyLabel(k control.Keycode) string {
	switch k {
	case control.K_F:
		return "F"
	case control.K_J:
		return "J"
	default:
		return "timeout"
	}
}

// ── main ──────────────────────────────────────────────────────────────────────

func main() {
	cliGroup := flag.String("group", "digits", "default stimulus group shown in the UI")
	flag.Parse()

	groups := []string{"digits", "regular", "irregular", "random"}
	defaultGroup := "digits"
	for _, g := range groups {
		if g == *cliGroup {
			defaultGroup = *cliGroup
			break
		}
	}

	fields := []control.InfoField{
		{Name: "subject_id", Label: "Subject ID", Default: ""},
		{
			Name:    "group",
			Label:   "Stimulus group",
			Default: defaultGroup,
			Type:    control.FieldSelect,
			Options: groups,
		},
		control.FullscreenField,
		control.DisplayField,
	}
	info, err := control.GetParticipantInfo("Number Comparison", fields)
	if err != nil {
		log.Fatalf("dialog: %v", err)
	}

	subjectID, _ := strconv.Atoi(info["subject_id"])
	group := info["group"]
	fullscreen := info["fullscreen"] == "true"
	width, height := 0, 0
	if !fullscreen {
		width, height = 1024, 768
	}

	exp := control.NewExperiment("Number Comparison", width, height, fullscreen, control.Gray, control.Black, 32)
	exp.SubjectID = subjectID
	exp.ScreenNumber = control.DisplayIDFromInfo(info)
	exp.Info = info
	if err := exp.Initialize(); err != nil {
		log.Fatal(err)
	}
	defer exp.End()

	if err := exp.SetLogicalSize(1368, 1024); err != nil {
		log.Printf("warning: SetLogicalSize: %v", err)
	}

	exp.AddDataVariableNames([]string{
		"block", "is_practice", "group",
		"n_left", "n_right", "response", "rt_ms", "correct",
	})

	// ── Load group-specific stimuli ───────────────────────────────────────────

	var digitStims [10]*stimuli.TextLine  // indices 1–9
	var dotPics [10]*stimuli.Picture      // indices 1–9; used for regular/irregular

	switch group {
	case "digits":
		digitFont, err := control.FontFromMemory(assets_embed.InconsolataFont, digitPt)
		if err != nil {
			log.Fatalf("load digit font: %v", err)
		}
		defer digitFont.Close()
		for n := 1; n <= 9; n++ {
			t := stimuli.NewTextLine(fmt.Sprintf("%d", n), 0, 0, control.Black)
			t.Font = digitFont
			digitStims[n] = t
		}

	case "regular", "irregular":
		fs := regularFS
		prefix := "assets/regular"
		if group == "irregular" {
			fs = irregularFS
			prefix = "assets/irregular"
		}
		for n := 1; n <= 9; n++ {
			data, err := fs.ReadFile(fmt.Sprintf("%s/dot_%d.png", prefix, n))
			if err != nil {
				log.Fatalf("load %s dot %d: %v", group, n, err)
			}
			p := stimuli.NewPictureFromMemory(data, 0, 0)
			p.Width = stimPx
			p.Height = stimPx
			dotPics[n] = p
		}

	case "random":
		// Dot arrays are generated on-the-fly each trial; no pre-loading.
	}

	// ── Fixed stimuli ─────────────────────────────────────────────────────────

	fixCross := stimuli.NewFixCross(30, 4, control.Black)

	// ── Trial list ────────────────────────────────────────────────────────────

	baseTrials := buildBaseTrials()
	rng := rand.New(rand.NewSource(int64(exp.SubjectID)*1234567 + 42))

	// ── Experiment run loop ───────────────────────────────────────────────────

	runErr := exp.Run(func() error {
		for block := 0; block < numBlocks; block++ {
			isPractice := block == 0

			// Show block-start instructions.
			switch {
			case isPractice:
				if err := exp.ShowInstructions(practiceInstr); err != nil {
					return err
				}
			case block == 1:
				if err := exp.ShowInstructions(startInstr); err != nil {
					return err
				}
			default:
				msg := fmt.Sprintf("%s\n\nBlock %d of %d", restInstr, block, numBlocks-1)
				if err := exp.ShowInstructions(msg); err != nil {
					return err
				}
			}

			// Build and shuffle this block's trial list.
			blockTrials := make([]trial, len(baseTrials))
			copy(blockTrials, baseTrials)
			design.ShuffleList(blockTrials)

			for _, t := range blockTrials {
				// Generate random textures during the first part of the ITI so
				// they are ready when the stimulus frame begins.
				var leftTex, rightTex *gxio.Texture
				if group == "random" {
					leftImg := generateDotImage(t.nLeft, rng)
					rightImg := generateDotImage(t.nRight, rng)
					var err error
					leftTex, err = rgbaToTexture(exp.Screen, leftImg)
					if err != nil {
						return fmt.Errorf("create left texture: %w", err)
					}
					rightTex, err = rgbaToTexture(exp.Screen, rightImg)
					if err != nil {
						leftTex.Destroy()
						return fmt.Errorf("create right texture: %w", err)
					}
				}

				// ITI blank
				if err := exp.Blank(itiMs); err != nil {
					if leftTex != nil {
						leftTex.Destroy()
					}
					if rightTex != nil {
						rightTex.Destroy()
					}
					return err
				}

				// Fixation-only period
				if err := exp.Show(fixCross); err != nil {
					return err
				}
				if err := exp.Wait(fixMs); err != nil {
					return err
				}

				// Draw both stimuli + fixation cross, then flip and timestamp.
				exp.Screen.Clear()
				switch group {
				case "digits":
					digitStims[t.nLeft].SetPosition(control.FPoint{X: -stimOffset, Y: 0})
					if err := digitStims[t.nLeft].Draw(exp.Screen); err != nil {
						return err
					}
					digitStims[t.nRight].SetPosition(control.FPoint{X: stimOffset, Y: 0})
					if err := digitStims[t.nRight].Draw(exp.Screen); err != nil {
						return err
					}
				case "regular", "irregular":
					dotPics[t.nLeft].SetPosition(control.FPoint{X: -stimOffset, Y: 0})
					if err := dotPics[t.nLeft].Draw(exp.Screen); err != nil {
						return err
					}
					dotPics[t.nRight].SetPosition(control.FPoint{X: stimOffset, Y: 0})
					if err := dotPics[t.nRight].Draw(exp.Screen); err != nil {
						return err
					}
				case "random":
					renderTexCentered(exp.Screen, leftTex, -stimOffset, 0, stimPx, stimPx)
					renderTexCentered(exp.Screen, rightTex, stimOffset, 0, stimPx, stimPx)
				}
				if err := fixCross.Draw(exp.Screen); err != nil {
					return err
				}
				onsetNS, err := exp.Screen.FlipNS()
				if err != nil {
					return err
				}

				// Wait for response (F or J) with timeout.
				key, eventTS, respErr := exp.Keyboard.WaitKeysEventRT(respKeys, maxRTms)

				// Free random textures immediately after response.
				if leftTex != nil {
					leftTex.Destroy()
				}
				if rightTex != nil {
					rightTex.Destroy()
				}

				if control.IsEndLoop(respErr) {
					return control.EndLoop
				}

				// Compute reaction time and correctness.
				var rtMs int64
				if key != 0 && eventTS >= onsetNS {
					rtMs = int64(eventTS-onsetNS) / 1_000_000
				}
				larger := t.nLeft > t.nRight
				correct := (key == control.K_F && larger) || (key == control.K_J && !larger)

				exp.Data.Add(
					block, isPractice, group,
					t.nLeft, t.nRight,
					keyLabel(key), rtMs, correct,
				)
			}
		}

		// End screen
		end := stimuli.NewTextBox(endMsg, 800, control.FPoint{}, control.Black)
		if err := exp.Show(end); err != nil {
			return err
		}
		if _, err := exp.Keyboard.Wait(); err != nil {
			return err
		}
		return control.EndLoop
	})

	if runErr != nil && !control.IsEndLoop(runErr) {
		log.Fatal(runErr)
	}
}
