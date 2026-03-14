To provide a programming agent with the necessary information to reconstruct or simulate these stimuli, the environment and the "Language of Geometry" must be defined. This formal language describes movements on a regular octagon based on atomic primitives and recursive rules.

---

## 1. The Octagon Environment

The workspace consists of **8 locations** arranged as a regular octagon.

* **Indexing**: Points should be indexed **0 to 7**.
* **Default Orientation**: Point **0** is the top-most vertex.
* 
**Distance**: All transitions occur between these 8 discrete coordinates.



---

## 2. Primitive Instruction Set (Lexicon)

Each primitive represents a transition from the current location to a new one.

| Primitive | Description | Programmatic Logic |
| --- | --- | --- |
| **0** | Stay | <br>`index = current` 

 |
| **+1** | Next Clockwise | <br>`index = (current + 1) % 8` 

 |
| **+2** | Second Clockwise | <br>`index = (current + 2) % 8` 

 |
| **-1** | Next Counter-clockwise | <br>`index = (current - 1) % 8` 

 |
| **-2** | Second Counter-clockwise | <br>`index = (current - 2) % 8` 

 |
| **H** | Horizontal Symmetry | Mirror across the vertical axis 

 |
| **V** | Vertical Symmetry | Mirror across the horizontal axis 

 |
| **P** | Rotational Symmetry | Point symmetry (equivalent to **+4**) 

 |
| **A, B** | Diagonal Symmetries | Mirror across the two oblique axes 

 |

---

## 3. Combinatorial Rules (Syntax)

The language allows these primitives to be combined into hierarchically organized expressions.

* 
**Concatenation**: Primitives are executed in sequence (e.g., `+2+2` moves the cursor twice).


* 
**Repetition**: An operation or block can be repeated $n$ times using the notation `[Instruction]^n`.


* 
**Nesting**: Sequences can contain "repetitions of repetitions".


* 
**Variation (Offset)**: A repeated block can include a starting point shift denoted by `<offset>` (e.g., `[[+2]^4]^2<+1>` draws a square, then repeats it starting from the next dot).



---

## 4. Stimuli Sequence Definitions

The following sequences were used in the study, defined by their shortest expression (Minimal Description Length):

* 
**Repeat (+1 or -1)**: A simple progression clockwise or counter-clockwise.


* 
*Logic*: `[+1]^8`.




* 
**Alternate**: Alternating steps in opposite directions.


* 
*Logic*: `[+2-1]^4`.




* 
**2squares**: A square repeated with a rotated starting point.


* 
*Logic*: `[[+2]^4]^2<+1>`.




* 
**2arcs**: An arc of four points mirrored by axial symmetry.


* 
*Logic*: Three applications of `+1`, then a global symmetry flip.




* 
**4segments**: An axial symmetry applied to a segment, translated four times.


* 
*Logic*: `[[Axial Symmetry]^2]^4<shift>`.




* 
**4diagonals**: Rotational symmetry applied to four starting points.


* 
*Logic*: `[[P]^2]^4<+1>`.




* 
**2rectangles**: An initial segment transformed by axial symmetry, then transposed by `+2`.


* 
**2crosses**: A rotational symmetry transformed by axial symmetry, then transposed by `+2`.


* 
**Irregular**: A list of 8 locations with no compression or repetition (Complexity $K=16$).



---

