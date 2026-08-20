MEG-localizer
=============

A multimodal localizer for MEG: the participant is shown, and played, a fast
stream of stimuli from many categories, so that one short run exercises the
visual, language, number, auditory and motor systems.

The program is table-driven. Nothing about the paradigm is compiled in: a run
is a tab-separated table of absolute onsets under `protocols/`, and the stimuli
are files under `stimuli/`. The presentation engine is the one from
[`examples/RSVP-multimodal`](../RSVP-multimodal), which ports the Pinel fMRI
localizer; this example changes the stimuli, the schedule, and adds a fixation
crosshair drawn over every frame.


Running it
----------

From this directory:

    go run . -p demo              # fullscreen, the way a session is run
    go run . -p demo -w           # windowed, for looking at the stimuli
    go run . -p demo -s 3         # subject 3
    go run . -p demo -dir .       # read protocols/ and stimuli/ from disk
    go run . -p demo -skip-wait   # start at once, no instruction screen

The run shows an instruction screen, waits for SPACE, then shows a green
fixation cross and waits for **T** to start. The clock starts at that keypress
and every onset is measured from it. ESC, or closing the window, aborts.

| Flag | Meaning |
|---|---|
| `-p NAME` | protocol table to run, from `protocols/NAME.tsv` |
| `-s ID` | subject identifier, recorded in the data file |
| `-w` | windowed instead of fullscreen |
| `-d N` | display index |
| `-dir DIR` | read `protocols/` and `stimuli/` from `DIR` instead of the copies embedded in the binary |
| `-no-crosshair` | do not superimpose the fixation crosshair on the stimuli |
| `-skip-wait` | no instruction screen and no start key |

**Record in fullscreen, never windowed.** A compositing desktop may throttle an
unfocused window: measured here, a windowed run reported a refresh of 5690 Hz
against a nominal 60 Hz, meaning presents were not blocking on VSYNC at all.
Onsets survived that, because the schedule re-anchors on every row, but
stimulus durations did not. `-w` is for looking at stimuli, not for data.


The conditions
--------------

`protocols/demo.tsv` runs 17 conditions on a 2 s grid. A trial takes one slot
unless the participant has something to do after the stimulus — press a button,
solve a sum — in which case it takes two.

| Condition | What a trial is | Slots |
|---|---|---|
| `faces`, `houses` | 4 pictures, RSVP | 1 |
| `sentences` | the 4 words of one sentence, in order | 1 |
| `consonants` | the 4 strings matched to one sentence | 1 |
| `equations` | one written sum, held 1200 ms | 1 |
| `disks`, `wedges`, `rings` | 4 retinotopic frames, RSVP | 1 |
| `sounds` | one natural sound | 1 |
| `tones_regular` | 4 evenly spaced tones, rising or falling | 1 |
| `tones_random` | the same 4 tones, shuffled | 1 |
| `sentences_audio` | one spoken sentence | 1 |
| `sentences_reversed` | the same recording played backwards | 1 |
| `equations_audio` | one spoken sum, then time to solve it | 2 |
| `motor_visual` | "press LEFT/RIGHT three times", word by word | 2 |
| `motor_audio` | the same instruction, spoken | 2 |
| `rest` | fixation cross only | 2 |

The RSVP conditions show each item for 350 ms followed by 50 ms of blank — an
SOA of 400 ms, so four items fill 1600 ms and the rest of the slot is fixation.

There is no single-word condition: `sentences` replaced it. The word images in
`assets/words/` are still what the sentences are built from, but the 13 words
that appear in no sentence, and the 32 isolated consonant strings that were
their control, are generated and embedded without being shown.

Three pairings are deliberate. `consonants` draws the four strings that were
*generated to match* a specific sentence in item count, length and letter
frequency, so the contrast with `sentences` is lexicality rather than surface
form. `tones_random` uses the same four tones as a `tones_regular` trial, so
the contrast is sequence structure rather than pitch content.
`sentences_reversed` is its sentence reversed, which preserves duration and
long-term spectrum exactly while destroying intelligibility.


The protocol table
------------------

    onset_time	duration	type	cond	stimuli
    0	350	IMAGE_STREAM	sentences	poets.png:350:50~learn.png:350:50~these.png:350:50~words.png:350:50
    2000	1231	SOUND	sentences_audio	sentence_01.wav
    4000	1200	IMAGE	equations	6+2.png

`onset_time` and `duration` are in ms; `onset_time` is measured from the start
key. Types are `TEXT`, `BOX`, `IMAGE`, `SOUND`, and the `~`-separated
`TEXT_STREAM`, `IMAGE_STREAM`, `SOUND_STREAM`. In a stream row an element may
override the row default as `name:duration` or `name:duration:gap`.

Rows play in order and must not overlap; the loader rejects a table where they
do. A fixation cross fills every gap, and the crosshair is drawn on top of
everything, stimuli included.


Regenerating the stimuli
------------------------

`make-protocol.py` builds both `stimuli/` and a protocol table. It copies the
files it needs out of `assets/` — which is the working directory, holding the
sources and one generator per category — and writes the schedule:

    ./make-protocol.py                       # protocols/demo.tsv, ~222 s
    ./make-protocol.py --blocks 102 --seed 7 --name run1

Conditions are ordered in shuffled rounds, so counts stay balanced and the same
condition never repeats back to back. Items are drawn without replacement
within a condition.

Under `assets/`:

| Script | Produces |
|---|---|
| `generate-text-images.py` | `equations/`, `words/`, `consonant-strings/`, `motor-visual/` — Inconsolata 80, white on black |
| `disks/generate-disks.py` | eight disks on the wedge/ring canvas |
| `wedges-rings/generate-wedges-rings.py` | eight wedges and eight rings |
| `split-sheets.py` | cuts the AI contact sheets into `faces_kept/`, `houses_kept/` |
| `tones/generate-tones.py` | nine 300 ms tones, log-spaced 220–440 Hz |
| `natural_sounds/make-sound-clips.py` | 1.6 s clips with 50 ms ramps |
| `spoken-sentences/`, `equations-audio/` | text-to-speech, one script each |
| `trim-speech.py` | strips the synthesiser's ~250 ms of padding → `*-trimmed/` |
| `reversed-sentences/make-reversed-sentences.py` | the reversed-speech control |

The word, sentence, consonant-string and equation lists live in the **Stimuli**
sections at the end of this file, and both `generate-text-images.py` and
`make-protocol.py` parse them from here. Edit a list, rerun the generators, and
the images and the schedule follow. Do not rename those headings.


The data file
-------------

Two files per session under the data directory: a CSV with one row per event
and a `-info.txt` with the session metadata (display, refresh rate, drivers).
Columns are `subject_id`, `intended_ms`, `actual_ms`, `event`, `cond`,
`stimuli` — so the scheduled and the achieved onset are both recorded, per
event, on the same clock as any response.

Events are `IMAGE_ONSET`/`OFFSET`, `IMAGE_STREAM_ONSET`/`OFFSET`, `SOUND_ONSET`,
`SOUND_STREAM_ONSET` and `RESPONSE`. Fullscreen runs here give a mean onset
error of about 5 ms and a worst case under 20 ms.


Not yet done
------------

- **Hardware triggers.** A MEG run needs a TTL at each stimulus onset. Nothing
  emits one yet. The hook exists: `stimuli.PresentStreamOfStimuliHooks` takes an
  `OnsetCallback` fired immediately after the flip, and the `triggers/` package
  has parallel-port, DLP-IO8, FT232H, LabJack and GPIO backends. Note that
  `triggers/` is desktop-only and excluded from the browser build.
- **Responses.** The motor trials ask for three button presses; no key mapping
  is defined and nothing counts them. `RESPONSE` events are already timestamped
  on the run clock, so only the definition is missing.
- **Only 10 of the 32 sentences** have been synthesised, so `sentences_audio`
  and `sentences_reversed` draw from a tenth of what `sentences` uses.
- The crosshair sits inside the inner hole of the wedge and ring stimuli, where
  the checks crowd it.


Stimulus lists
--------------

Parsed by the generators — do not rename these headings.

### Disks
see [generate-disks.py]

### Checkboards
see [generate-wedges-rings.py]

### simple equations 

1 + 1
1 + 2
1 + 3
1 + 4
1 + 5
1 + 6
2 + 1
2 + 2
2 + 3
2 + 4
2 + 5
2 + 7
3 + 2
3 + 3
3 + 4
3 + 5
3 + 6
4 + 1
4 + 2
4 + 3
4 + 4
4 + 5
5 + 1
5 + 2
5 + 3
5 + 4
6 + 1
6 + 2
6 + 3
7 + 1
7 + 2
8 + 1


### Words

admit
allow
agree
bring
build
drink
enter
fetch
occur
react
learn
imply
block
spend
teach
write
apple
beach
chair
clock
earth
glass
heart
house
lemon
music
paper
plant
river
table
train
water

### Consonant strings

rbhgt
sxclb
gnbgr
pgqvb
vskfz
kcdwq
vtpzk
pwqxq
sbgjc
gdhrk
kghyr
qftfk
zrpsw
vgbjh
gyxjr
lymhh
vcxqn
dvjvw
dzfhp
shxzg
rcbgz
nfsfx
pymst
drmth
dwgvh
nbvmd
ypfgv
gnhhq
xxzbn
vpwzc
kjvcq
ybmvy


### Sentences

Four words each, every word exactly five letters, so a sentence spans 4 x 5
characters -- visually matched to the word and consonant-string conditions.

girls drink fresh water
birds build small nests
cooks fetch fresh lemon
poets write short poems
women teach young girls
kings enter their house
chefs bring clean plate
teams learn these moves
hosts allow every guest
twins react quite badly
girls spend those coins
monks plant these trees
crews build large boats
chefs slice fresh bread
women write short notes
birds drink river water
kings build stone walls
teams train every night
clans share their bread
poets learn these words
cooks clean their table
girls carry heavy boxes
twins climb steep hills
hosts serve sweet cakes
these girls enjoy music
those kings order bread
every child needs sleep
young birds leave nests
these women study maths
those poets adore music
seven boats cross river
three teens paint doors


### Consonant strings, matched to the sentences

Generated by [generate-consonant-strings.py], which also writes
[consonant_strings.txt].  One line per sentence: 4 strings of 5 letters, so an
item matches a sentence in item count, item length and total characters.  The
letters are sampled from the consonant frequency of the sentence corpus itself.

trswn nrpws nrvsr stlnh
rsbns yrtmr gsnkt nrksr
ntchc mtrhs shlys ctscs
trbrt lmhls msnsh tygmt
ndwdr lghcr tnrhn snmvn
nsrst rdhns nmrhd ktblr
srtfv lrypt ntcrs stsrh
sdbgs rskls srlsf dspnh
krdlp ylbth rlwsr nhytc
rghpl rsltr dstbv ptrcr
htkgw brshj lbrld tsbdh
tcnlw wspst wgsnv tcvtr
trlhd tltsk snsdt pnsts
lkrsp thmlc nlsrf ghsls
hnhsw tysnr fsmsk nkbts
bmtcp bprnw lwdws bnwlq
ntrsl trlgr sghrd ptrtk
yspcb hrnsc srphc vrgls
tslnf jprst btvht crpmr
whtnb rmrct ndkdv ctkmp
cpsnt wfnlr rgkbc wtlhc
lsdkl nktps mnpys sbhsr
fdsnd pwmsn rbgnm srslb
msklh nbrns crtmy vcrgs
shlsr slrht rqtsm srmnf
rnrns stylm dsrdj flsdn
qhypt tghmw srtmc rhvmv
dmdhr tbshn cltgw tbhsh
snhkr tgmts dngns tcnht
ckrst vlthg ybdbr shtrl
stsvl rstbs pmnsj scgtc
lhchd txvst nlrsh lstnh

Faces

See [assets/faces*.png]


