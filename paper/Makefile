MAIN = goxpyriment_paper

.PHONY: all clean

all: $(MAIN).pdf

$(MAIN).pdf: $(MAIN).tex biblio.bib
	latexmk -pdf -bibtex $(MAIN).tex

clean:
	latexmk -C $(MAIN).tex
