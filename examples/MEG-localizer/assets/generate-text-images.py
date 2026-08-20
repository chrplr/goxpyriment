#! /usr/bin/env python
# Time-stamp: <2026-08-20 christophe@pallier.org>
"""Render the equation and word conditions of README.md as images.

    ./generate-text-images.py

Writes one PNG per item, white on black, in Inconsolata at size 40:

  equations/1+1.png ...   the "simple equations" list, rendered as written
  words/admit.png ...     every distinct word of the "Words" and "Sentences"
                          lists, rendered in upper case
  motor-visual/press.png ...
                          the words of the instruction sentences in
                          motor-visual/README.txt, one image per distinct word
  consonant-strings/rbhgt.png ...
                          every distinct string of both consonant-string
                          lists -- the 32 isolated ones and the 128 that match
                          the sentences -- also in upper case, since they are
                          the control for the word and sentence conditions and
                          have to differ from them in lexicality, not in case

The lists are read from README.md rather than copied here, so the images and
the documented stimuli cannot drift apart.

Every item in both conditions is exactly five characters ("1 + 1" as much as
"admit"), and Inconsolata is monospaced, so every string is the same 120 px
wide and one canvas size suits all of them.

Requires Pillow.
"""

import re
from pathlib import Path

from PIL import Image, ImageDraw, ImageFont

HERE = Path(__file__).resolve().parent
README = HERE.parent / "README.md"
FONT = HERE.parent.parent.parent / "assets" / "Inconsolata.ttf"

FONT_SIZE = 80
CANVAS = (320, 128)         # fits the ink of any five-character item at size 80
BACKGROUND = 0              # black
FOREGROUND = 255            # white


def sections(text):
    """Map each '### heading' of the README to its non-blank lines."""
    lines = text.splitlines()
    heads = [(i, l.strip()) for i, l in enumerate(lines) if l.startswith("###")]
    out = {}
    for k, (i, h) in enumerate(heads):
        j = heads[k + 1][0] if k + 1 < len(heads) else len(lines)
        out[h[4:].strip()] = [l.strip() for l in lines[i + 1:j] if l.strip()]
    return out


def render(text, path):
    """Draw `text` centred on the canvas and save it.

    The glyph box is measured and centred rather than trusting anchors: the
    ascent leaves different slack above digits and capitals, and a stimulus
    that shifts vertically by a few pixels between conditions is a confound
    the participant's eye can pick up.
    """
    font = ImageFont.truetype(str(FONT), FONT_SIZE)
    img = Image.new("L", CANVAS, BACKGROUND)
    draw = ImageDraw.Draw(img)
    x0, y0, x1, y1 = draw.textbbox((0, 0), text, font=font)
    draw.text(((CANVAS[0] - (x1 - x0)) / 2 - x0,
               (CANVAS[1] - (y1 - y0)) / 2 - y0), text, font=font, fill=FOREGROUND)
    img.save(path)


def main():
    if not FONT.exists():
        raise SystemExit(f"font not found: {FONT}")
    sec = sections(README.read_text())

    equations = [l for l in sec["simple equations"] if re.fullmatch(r"\d \+ \d", l)]
    words = [l for l in sec["Words"] if re.fullmatch(r"[a-z]+", l)]
    sentences = [l for l in sec["Sentences"]
                 if re.fullmatch(r"([a-z]+ ){3}[a-z]+", l)]
    vocabulary = sorted(set(words) | {w for s in sentences for w in s.split()})

    # Both consonant lists: the isolated items and the four-per-line ones that
    # match the sentences. They are disjoint by construction (the generator
    # excludes the isolated set), but take the union in case that changes.
    isolated = [l for l in sec["Consonant strings"]
                if re.fullmatch(r"[a-z]+", l)]
    matched = [w for l in sec["Consonant strings, matched to the sentences"]
               if re.fullmatch(r"([a-z]+ ){3}[a-z]+", l) for w in l.split()]
    strings = sorted(set(isolated) | set(matched))

    # The motor instructions are sentences to be shown word by word, so they
    # need the same per-word images as the other text conditions. A word that
    # also occurs elsewhere ("three" is in the sentence list) renders to a byte
    # identical file, so the two conditions can share one stimulus file and be
    # told apart by the condition label in the protocol table.
    motor_dir = HERE / "motor-visual"
    motor_words = sorted({w.strip(".,;:!?").lower()
                          for line in (motor_dir / "README.txt").read_text().splitlines()
                          for w in line.split() if w.strip(".,;:!?")})

    eq_dir, wd_dir = HERE / "equations", HERE / "words"
    cs_dir = HERE / "consonant-strings"
    eq_dir.mkdir(exist_ok=True)
    wd_dir.mkdir(exist_ok=True)
    cs_dir.mkdir(exist_ok=True)

    for e in equations:
        render(e, eq_dir / f"{e.replace(' ', '')}.png")
    for w in vocabulary:
        render(w.upper(), wd_dir / f"{w}.png")
    for c in strings:
        render(c.upper(), cs_dir / f"{c}.png")
    for w in motor_words:
        render(w.upper(), motor_dir / f"{w}.png")

    print(f"equations: {len(equations)} files in {eq_dir.name}/")
    print(f"words:     {len(vocabulary)} files in {wd_dir.name}/ "
          f"({len(words)} from the word list, {len(sentences)} sentences, "
          f"{len(set(words) & {w for s in sentences for w in s.split()})} shared)")
    print(f"motor:     {len(motor_words)} files in {motor_dir.name}/ "
          f"({', '.join(motor_words)})")
    print(f"consonants:{len(strings)} files in {cs_dir.name}/ "
          f"({len(isolated)} isolated, {len(matched)} matched to the sentences, "
          f"{len(set(isolated) & set(matched))} shared)")


if __name__ == "__main__":
    main()
