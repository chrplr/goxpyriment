.PHONY: docs pdfs serve deploy clean-docs

# Generate PDF versions of the documentation.
# Requires: pandoc, xelatex  (sudo apt install pandoc texlive-xetex)
pdfs:
	bash docs/make_pdfs.sh

# Build the MkDocs HTML site locally (output → site/).
docs:
	mkdocs build

# Live-reload preview at http://127.0.0.1:8000
serve:
	mkdocs serve

# Generate PDFs, then build and push to GitHub Pages.
deploy: pdfs
	mkdocs gh-deploy

# Remove the generated site/ directory.
clean-docs:
	rm -rf site/
