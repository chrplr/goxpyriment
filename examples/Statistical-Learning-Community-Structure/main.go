// Copyright (2026) Christophe Pallier <christophe@pallier.org>
// Distributed under the GNU General Public License v3.

// Statistical Learning with Community Structure — Schapiro et al. (2013).
//
// Implements the behavioral protocol of Experiment 2:
//
//	Exposure phase (1,400 trials): a continuous random walk on a 15-node graph
//	with 3 densely-connected communities (5 nodes each). Cover task: press F
//	(normal orientation) or J (rotated 90°). Rotation rate ≈ 20 %.
//
//	Parsing phase (600 trials): 40 alternating blocks of 15 stimuli — random
//	walk and fixed Hamiltonian path. The Hamiltonian cycle is entered at the
//	last node of the preceding random-walk block; traversal direction
//	(forward/backward) is chosen randomly for each Hamiltonian block.
//	Participants press SPACE at perceived natural breaking points while
//	continuing the cover task.
//
// Graph: 15 nodes in 3 communities (C0 = 0–4, C1 = 5–9, C2 = 10–14).
// Within each community the induced subgraph is K5 minus the edge between the
// two boundary nodes. Cross-community edges: 4↔5, 9↔10, 14↔0.
// Each node therefore has exactly 4 neighbours.
//
// Stimuli: 15 abstract polygon symbols (regular polygons and star polygons of
// varying point-count and inner/outer radius), similar in spirit to the
// non-verbalizable stimuli used in experiments 2 and 3 of the paper.
//
// Reference:
//
//	Schapiro, A. C., Rogers, T. T., Cordova, N. I., Turk-Browne, N. B., &
//	Botvinick, M. M. (2013). Neural representations of events arise from
//	temporal community structure. Nature Neuroscience, 16(4), 486–492.
//	https://doi.org/10.1038/nn.3331
//
// Usage:
//
//	go run . -w -s 1
package main

import (
	"errors"
	"fmt"
	"log"
	"math"
	"math/rand"

	"github.com/Zyko0/go-sdl3/sdl"
	"github.com/chrplr/goxpyriment/clock"
	"github.com/chrplr/goxpyriment/control"
	"github.com/chrplr/goxpyriment/stimuli"
)

// ── Timing ────────────────────────────────────────────────────────────────────

const (
	stimDurMS    = 1500 // ms per stimulus (exposure and parsing phases)
	rotatedFrac  = 0.20 // fraction of stimuli presented rotated 90°
	symbolRadius = 60.0 // outer bounding radius of each symbol (logical px)
)

// ── Graph ─────────────────────────────────────────────────────────────────────
//
// Three communities of 5 nodes (C0 = 0–4, C1 = 5–9, C2 = 10–14).
// Within each community: K5 minus the boundary-boundary edge.
// Cross edges: 4↔5, 9↔10, 14↔0.

const nNodes = 15

var adjacency = [nNodes][]int{
	{1, 2, 3, 14},    // 0 – C0 left boundary
	{0, 2, 3, 4},     // 1 – C0 internal
	{0, 1, 3, 4},     // 2 – C0 internal
	{0, 1, 2, 4},     // 3 – C0 internal
	{1, 2, 3, 5},     // 4 – C0 right boundary
	{4, 6, 7, 8},     // 5 – C1 left boundary
	{5, 7, 8, 9},     // 6 – C1 internal
	{5, 6, 8, 9},     // 7 – C1 internal
	{5, 6, 7, 9},     // 8 – C1 internal
	{6, 7, 8, 10},    // 9 – C1 right boundary
	{9, 11, 12, 13},  // 10 – C2 left boundary
	{10, 12, 13, 14}, // 11 – C2 internal
	{10, 11, 13, 14}, // 12 – C2 internal
	{10, 11, 12, 14}, // 13 – C2 internal
	{11, 12, 13, 0},  // 14 – C2 right boundary
}

func communityOf(node int) int { return node / 5 }

// ── Hamiltonian cycle ─────────────────────────────────────────────────────────

// hamiltonianCycle is a valid Hamiltonian cycle on the graph.
// Verified edges: 0-1 ✓, 1-2 ✓, 2-3 ✓, 3-4 ✓, 4-5 ✓, 5-6 ✓, 6-7 ✓,
// 7-8 ✓, 8-9 ✓, 9-10 ✓, 10-11 ✓, 11-12 ✓, 12-13 ✓, 13-14 ✓, 14-0 ✓.
var hamiltonianCycle = [nNodes]int{0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14}

// cyclePos maps each node to its index in hamiltonianCycle.
var cyclePos [nNodes]int

func init() {
	for i, n := range hamiltonianCycle {
		cyclePos[n] = i
	}
}

// randomStep returns one randomly chosen neighbour of node.
func randomStep(node int) int {
	nb := adjacency[node]
	return nb[rand.Intn(len(nb))]
}

// hamiltonianBlock returns 15 nodes starting at startNode, traversing the
// fixed cycle in the given direction.
func hamiltonianBlock(startNode int, forward bool) []int {
	seq := make([]int, nNodes)
	pos := cyclePos[startNode]
	for i := range seq {
		seq[i] = hamiltonianCycle[pos]
		if forward {
			pos = (pos + 1) % nNodes
		} else {
			pos = (pos + nNodes - 1) % nNodes
		}
	}
	return seq
}

// ── Symbols ───────────────────────────────────────────────────────────────────

type symbolPair struct{ normal, rotated *stimuli.Shape }

var symbols [nNodes]symbolPair

// regPoly returns the vertices of a regular n-gon of radius r offset by offsetDeg.
func regPoly(n int, r, offsetDeg float64) []sdl.FPoint {
	pts := make([]sdl.FPoint, n)
	for i := range pts {
		a := (float64(i)/float64(n))*2*math.Pi + offsetDeg*math.Pi/180
		pts[i] = sdl.FPoint{X: float32(r * math.Cos(a)), Y: float32(r * math.Sin(a))}
	}
	return pts
}

// starPoly returns the vertices of a star polygon with nPts outer points.
func starPoly(nPts int, outerR, innerR, offsetDeg float64) []sdl.FPoint {
	pts := make([]sdl.FPoint, nPts*2)
	for i := range pts {
		r := outerR
		if i%2 == 1 {
			r = innerR
		}
		a := (float64(i)/float64(nPts*2))*2*math.Pi + offsetDeg*math.Pi/180
		pts[i] = sdl.FPoint{X: float32(r * math.Cos(a)), Y: float32(r * math.Sin(a))}
	}
	return pts
}

func rotatePoints(pts []sdl.FPoint, deg float64) []sdl.FPoint {
	out := make([]sdl.FPoint, len(pts))
	s, c := math.Sin(deg*math.Pi/180), math.Cos(deg*math.Pi/180)
	for i, p := range pts {
		out[i] = sdl.FPoint{
			X: float32(float64(p.X)*c - float64(p.Y)*s),
			Y: float32(float64(p.X)*s + float64(p.Y)*c),
		}
	}
	return out
}

func buildSymbols(color sdl.Color) {
	r := symbolRadius
	in := func(f float64) float64 { return r * f }

	ptsDefs := [nNodes][]sdl.FPoint{
		regPoly(3, r, -90),              // 0: triangle pointing up
		regPoly(4, r, -45),              // 1: square (flat sides)
		regPoly(5, r, -90),              // 2: pentagon pointing up
		regPoly(6, r, 0),                // 3: hexagon flat-top
		regPoly(8, r, 0),                // 4: octagon
		starPoly(3, r, in(0.38), -90),   // 5: 3-point star
		starPoly(4, r, in(0.35), -45),   // 6: 4-point star, narrow
		starPoly(5, r, in(0.38), -90),   // 7: 5-point star, classic
		starPoly(5, r, in(0.62), -90),   // 8: 5-point star, fat
		starPoly(6, r, in(0.45), 0),     // 9: 6-point star, narrow
		starPoly(6, r, in(0.68), 0),     // 10: 6-point star, fat
		starPoly(7, r, in(0.45), 0),     // 11: 7-point star
		starPoly(8, r, in(0.40), -22.5), // 12: 8-point star
		regPoly(3, r, 90),               // 13: triangle pointing down
		regPoly(4, r, 0),                // 14: diamond (square rotated 45°)
	}

	for i, pts := range ptsDefs {
		symbols[i] = symbolPair{
			normal:  stimuli.NewShape(pts, color),
			rotated: stimuli.NewShape(rotatePoints(pts, 90), color),
		}
	}
}

// ── Trial presentation ────────────────────────────────────────────────────────

type trialResult struct {
	nodeIdx      int
	community    int
	isRotated    bool
	response     string // "F", "J", or "none"
	correct      bool
	rtMS         int64
	spacePressed bool
	spaceMS      int64 // ms from stimulus onset; 0 if no space pressed
}

// presentTrial displays the symbol for stimDurMS ms and collects:
//   - first F/J keypress as the rotation-detection response
//   - first SPACE keypress (only when collectSpace=true) as a boundary mark
func presentTrial(exp *control.Experiment, nodeIdx int, isRotated, collectSpace bool) (trialResult, error) {
	sym := symbols[nodeIdx].normal
	if isRotated {
		sym = symbols[nodeIdx].rotated
	}
	if err := exp.Show(sym); err != nil {
		return trialResult{}, err
	}

	startMS := clock.GetTime()
	deadline := startMS + int64(stimDurMS)

	var resp string
	var rt int64
	spacePressed := false
	var spaceMS int64

	for {
		remaining := int(deadline - clock.GetTime())
		if remaining <= 0 {
			break
		}

		keys := make([]control.Keycode, 0, 3)
		if resp == "" {
			keys = append(keys, control.K_F, control.K_J)
		}
		if collectSpace && !spacePressed {
			keys = append(keys, control.K_SPACE)
		}
		if len(keys) == 0 {
			clock.Wait(remaining)
			break
		}

		key, rtMs, err := exp.Keyboard.WaitKeysRT(keys, remaining)
		if err != nil {
			return trialResult{}, err
		}
		switch key {
		case 0: // timeout
			remaining = 0
		case control.K_F:
			if resp == "" {
				resp = "F"
				rt = rtMs
			}
		case control.K_J:
			if resp == "" {
				resp = "J"
				rt = rtMs
			}
		case control.K_SPACE:
			spacePressed = true
			spaceMS = clock.GetTime() - startMS
		}
		if remaining == 0 {
			break
		}
	}

	if resp == "" {
		resp = "none"
	}
	correct := (resp == "F" && !isRotated) || (resp == "J" && isRotated)
	return trialResult{
		nodeIdx:      nodeIdx,
		community:    communityOf(nodeIdx),
		isRotated:    isRotated,
		response:     resp,
		correct:      correct,
		rtMS:         rt,
		spacePressed: spacePressed,
		spaceMS:      spaceMS,
	}, nil
}

// rotationMask builds a boolean slice of length n with exactly floor(n*rotatedFrac)
// randomly placed true values.
func rotationMask(n int) []bool {
	mask := make([]bool, n)
	nRot := int(float64(n) * rotatedFrac)
	for _, i := range rand.Perm(n)[:nRot] {
		mask[i] = true
	}
	return mask
}

// ── Exposure phase ────────────────────────────────────────────────────────────

func runExposurePhase(exp *control.Experiment) (lastNode int, err error) {
	const nTrials = 1400

	exp.ShowInstructions(
		"EXPOSURE PHASE\n\n" +
			"You will see a stream of abstract symbols,\n" +
			"each displayed for 1.5 seconds.\n\n" +
			"Your task: decide whether each symbol appears in its\n" +
			"NORMAL orientation or ROTATED 90°.\n\n" +
			"   F  →  normal orientation\n" +
			"   J  →  rotated 90°\n\n" +
			"Respond on every symbol.  There are 1,400 symbols total.\n" +
			"You will get a rest break every 350 symbols.\n\n" +
			"Press SPACE to begin.")

	rotated := rotationMask(nTrials)

	// Build random walk.
	cur := rand.Intn(nNodes)
	walk := make([]int, nTrials)
	for i := range walk {
		walk[i] = cur
		cur = randomStep(cur)
	}

	for i, node := range walk {
		res, tErr := presentTrial(exp, node, rotated[i], false)
		if tErr != nil {
			return 0, tErr
		}
		exp.Data.Add(
			"exposure", i+1, node, res.community,
			0, "rw", 0, 0,
			res.isRotated, res.response, res.correct, res.rtMS,
			false, 0,
		)

		if (i+1)%350 == 0 && i+1 < nTrials {
			exp.ShowInstructions(fmt.Sprintf(
				"Rest break\n\n%d / %d symbols completed.\n\nPress SPACE to continue.",
				i+1, nTrials))
		}
	}

	return walk[nTrials-1], nil
}

// ── Parsing phase ─────────────────────────────────────────────────────────────

func runParsingPhase(exp *control.Experiment, startNode int) error {
	const (
		nBlocks   = 40                  // 20 random-walk + 20 Hamiltonian, alternating
		blockSize = 15                  // nodes per block
		nTrials   = nBlocks * blockSize // = 600
	)

	exp.ShowInstructions(
		"PARSING PHASE\n\n" +
			"Continue pressing F (normal) or J (rotated) for each symbol.\n\n" +
			"NEW TASK: Also press SPACE whenever you feel that a natural\n" +
			"boundary or 'beginning' occurs in the sequence.\n\n" +
			"Trust your intuition — there are no right or wrong answers.\n\n" +
			"Press SPACE to begin.")

	rotated := rotationMask(nTrials)
	trialIdx := 0
	cur := startNode
	prevNode := -1

	for block := 0; block < nBlocks; block++ {
		var seq []int
		blockType := "rw"

		if block%2 == 0 {
			// Random-walk block starting from cur.
			seq = make([]int, blockSize)
			node := cur
			for i := range seq {
				seq[i] = node
				node = randomStep(node)
			}
		} else {
			// Hamiltonian block entering at cur, random direction.
			blockType = "ham"
			seq = hamiltonianBlock(cur, rand.Intn(2) == 0)
		}

		for pos, node := range seq {
			commTrans := 0
			if prevNode >= 0 && communityOf(node) != communityOf(prevNode) {
				commTrans = 1
			}

			res, tErr := presentTrial(exp, node, rotated[trialIdx], true)
			if tErr != nil {
				return tErr
			}
			exp.Data.Add(
				"parsing", trialIdx+1, node, res.community,
				block+1, blockType, pos+1, commTrans,
				res.isRotated, res.response, res.correct, res.rtMS,
				res.spacePressed, res.spaceMS,
			)
			prevNode = node
			trialIdx++
		}

		// Next block begins at the last node shown in this block.
		cur = seq[blockSize-1]
	}
	return nil
}

// ── Main ──────────────────────────────────────────────────────────────────────

func main() {
	fields := []control.InfoField{
		{Name: "subject_id", Label: "Subject ID", Default: ""},
		control.FullscreenField,
	}

	info, err := control.GetParticipantInfo(
		"Community Structure Statistical Learning — Schapiro et al. (2013)", fields)
	if errors.Is(err, control.ErrCancelled) {
		log.Fatal("Setup cancelled.")
	}
	if err != nil {
		log.Fatalf("GetParticipantInfo: %v", err)
	}

	fullscreen := info["fullscreen"] == "true"
	winW, winH := 0, 0
	if !fullscreen {
		winW, winH = 1024, 768
	}

	exp := control.NewExperiment(
		"Schapiro2013", winW, winH, fullscreen,
		control.Black, control.White, 32)
	if initErr := exp.Initialize(); initErr != nil {
		log.Fatalf("Initialize: %v", initErr)
	}
	defer exp.End()
	exp.Info = info

	if err := exp.SetLogicalSize(1024, 768); err != nil {
		log.Printf("Warning: SetLogicalSize: %v", err)
	}

	buildSymbols(control.White)

	exp.AddDataVariableNames([]string{
		"phase", "trial", "node", "community",
		"block_num", "block_type", "pos_in_block", "community_transition",
		"is_rotated", "response", "correct", "rt_ms",
		"space_pressed", "space_ms",
	})

	runErr := exp.Run(func() error {
		lastNode, err := runExposurePhase(exp)
		if err != nil {
			return err
		}

		exp.ShowInstructions(
			"Exposure phase complete!\n\n" +
				"The parsing phase will begin next.\n\n" +
				"Press SPACE to continue.")

		if err := runParsingPhase(exp, lastNode); err != nil {
			return err
		}

		if err := exp.Data.Save(); err != nil {
			log.Printf("Warning: could not save data: %v", err)
		}

		exp.ShowInstructions(
			"Experiment complete!\n\n" +
				"Thank you for your participation.\n\n" +
				"Press SPACE to exit.")
		return control.EndLoop
	})

	if runErr != nil && !control.IsEndLoop(runErr) {
		exp.Fatal("experiment error: %v", runErr)
	}
}
