#! /usr/bin/env python
# Time-stamp: <2026-08-20 christophe@pallier.org>
"""Generate the consonant-string control for the 4-word sentence condition.

Writes `consonant_strings.txt`: 32 lines of 4 five-letter consonant strings,
one line per sentence, so that each control item matches a sentence in number
of items (4), item length (5), and total character count (20).

The letters are drawn from the consonant frequency of the sentence corpus
itself, so the two conditions differ in lexical status and pronounceability,
not in surface letter statistics.  Vowels are excluded; `y` counts as a
consonant, as in the existing single-item consonant list.

Deterministic: re-running produces the same strings.
"""

import collections
import random

SEED = 20260820
VOWELS = set("aeiou")

# The sentence condition this list controls for (see README.md).
SENTENCES = """girls drink fresh water
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
three teens paint doors""".splitlines()

# The single-item consonant strings already in README.md; kept distinct so the
# two control sets never share an item.
ALREADY_USED = """rbhgt sxclb gnbgr pgqvb vskfz kcdwq vtpzk pwqxq sbgjc gdhrk kghyr
qftfk zrpsw vgbjh gyxjr lymhh vcxqn dvjvw dzfhp shxzg rcbgz nfsfx pymst drmth
dwgvh nbvmd ypfgv gnhhq xxzbn vpwzc kjvcq ybmvy""".split()

WORD_LENGTH = 5
ITEMS_PER_LINE = 4


def consonant_weights(sentences):
    """Letter frequencies of the sentence corpus, restricted to consonants."""
    counts = collections.Counter(
        c for s in sentences for c in s if c.isalpha() and c not in VOWELS)
    letters = sorted(counts)
    return letters, [counts[c] for c in letters]


def acceptable(s):
    """Reject the strings that would read as too word-like or too degenerate."""
    if any(a == b for a, b in zip(s, s[1:])):    # no doubled letter
        return False
    if max(collections.Counter(s).values()) > 2:  # no letter more than twice
        return False
    return True


def main():
    rng = random.Random(SEED)
    letters, weights = consonant_weights(SENTENCES)

    seen = set(ALREADY_USED)
    lines = []
    for sentence in SENTENCES:
        item = []
        while len(item) < ITEMS_PER_LINE:
            s = "".join(rng.choices(letters, weights=weights, k=WORD_LENGTH))
            if s in seen or not acceptable(s):
                continue
            seen.add(s)
            item.append(s)
        lines.append(" ".join(item))

    with open("consonant_strings.txt", "w") as f:
        f.write("\n".join(lines) + "\n")

    for sentence, line in zip(SENTENCES, lines):
        print(f"{line}    # {sentence}")
    n = len(lines) * ITEMS_PER_LINE
    print(f"\nSaved: consonant_strings.txt ({len(lines)} lines, {n} strings)")


if __name__ == "__main__":
    main()
