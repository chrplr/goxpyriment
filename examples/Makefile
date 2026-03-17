# Makefile for goxpyriment examples

# Go compiler
GO := go

# Get list of all subdirectories containing a main.go file
# These are the examples that can be built
EXAMPLES := $(shell find . -maxdepth 2 -name "main.go" | cut -d/ -f2 | sort)

# Default goal
.DEFAULT_GOAL := help

.PHONY: all clean help $(EXAMPLES)

# Build all examples
all: $(EXAMPLES)

# Target to build a specific example
$(EXAMPLES):
	@echo "Building example: $@"
	@cd $@ && $(GO) build .

# Clean up generated binaries
clean:
	@echo "Cleaning up binaries..."
	@for dir in $(EXAMPLES); do \
		if [ -f "$$dir/$$dir" ]; then \
			rm "$$dir/$$dir"; \
			echo "Removed $$dir/$$dir"; \
		fi; \
		if [ -f "$$dir/$$dir.exe" ]; then \
			rm "$$dir/$$dir.exe"; \
			echo "Removed $$dir/$$dir.exe"; \
		fi; \
	done

# Show help information
help:
	@echo "Makefile for goxpyriment examples"
	@echo ""
	@echo "Usage:"
	@echo "  make all          - Build all available examples"
	@echo "  make clean        - Remove all compiled binaries"
	@echo "  make <example>    - Build a specific example"
	@echo "  make help         - Show this help message"
	@echo ""
	@echo "Available examples:"
	@for dir in $(EXAMPLES); do \
		echo "  - $$dir"; \
	done
