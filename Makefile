


build-plugin:
	@echo "Building plugins..."
	@mkdir -p ./.claude/hooks
	@mkdir -p ~/.claude/hooks
	@for plugin_dir in plugins/*/; do \
		if [ -d "$$plugin_dir" ]; then \
			plugin_name=$$(basename "$$plugin_dir"); \
			plugin_file="$$plugin_dir$$plugin_name.go"; \
			if [ -f "$$plugin_file" ]; then \
				echo "Building plugin: $$plugin_name"; \
				if go build -buildmode=plugin -o ./.claude/hooks/$$plugin_name.so "$$plugin_file"; then \
					echo "✓ $$plugin_name.so built successfully"; \
				else \
					echo "✗ Failed to build $$plugin_name"; \
					exit 1; \
				fi; \
			else \
				echo "Warning: $$plugin_file not found, skipping"; \
			fi; \
		fi; \
	done
	@if ls ./.claude/hooks/*.so > /dev/null 2>&1; then \
		echo "Installing plugins to ~/.claude/hooks/..."; \
		cp ./.claude/hooks/*.so ~/.claude/hooks/ && echo "✓ Plugins installed successfully"; \
	else \
		echo "No plugins found to install"; \
	fi

build:
	@echo "Building claude-plugin..."
	@mkdir -p ~/.local/bin
	@if go build -o claude-plugin ./main.go; then \
		echo "✓ claude-plugin built successfully"; \
		cp ./claude-plugin ~/.local/bin/ && echo "✓ claude-plugin installed to ~/.local/bin/"; \
	else \
		echo "✗ Failed to build claude-plugin"; \
		exit 1; \
	fi

clean:
	@echo "Cleaning build artifacts..."
	@rm -f claude-plugin
	@rm -rf ./.claude/hooks/*.so
	@echo "✓ Clean completed"

clean-all: clean
	@echo "Removing installed plugins..."
	@rm -rf ~/.claude/hooks/*.so
	@echo "✓ All artifacts removed"

list-plugins:
	@echo "Available plugins:"
	@for plugin_dir in plugins/*/; do \
		if [ -d "$$plugin_dir" ]; then \
			plugin_name=$$(basename "$$plugin_dir"); \
			if [ -f "$$plugin_dir$$plugin_name.go" ]; then \
				echo "  - $$plugin_name"; \
			fi; \
		fi; \
	done

install: build build-plugin
	@echo "✓ Installation completed"

.PHONY: build build-plugin clean clean-all list-plugins install