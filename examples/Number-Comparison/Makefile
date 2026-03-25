.PHONY: stimuli build clean

# Generate dot-pattern PNG assets (must be run before building the experiment
# if assets/regular/ or assets/irregular/ are missing or stale).
stimuli:
	go run ./cmd/generate_regular/
	go run ./cmd/generate_irregular/

build: stimuli
	go build -o number-comparison .

clean:
	rm -f number-comparison
	rm -f assets/regular/dot_*.png
	rm -f assets/irregular/dot_*.png
