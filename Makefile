TRANSFORMER = ./transformer/transformer
CONTENT_DIR = content
DIST_DIR = dist
SITE_URL = https://jeonghyeon.net

.PHONY: lint build optimize clean hooks

$(TRANSFORMER): $(wildcard transformer/*.go transformer/**/*.go) transformer/go.mod
	cd transformer && go build -o transformer .

lint: $(TRANSFORMER)
	$(TRANSFORMER) lint $(CONTENT_DIR)

build: $(TRANSFORMER)
	$(TRANSFORMER) render $(CONTENT_DIR) $(DIST_DIR) $(SITE_URL)
	$(TRANSFORMER) minify $(DIST_DIR)

optimize:
	@command -v cwebp >/dev/null 2>&1 || { echo "cwebp not found. Install libwebp."; exit 1; }
	@find $(CONTENT_DIR) -type f \( -name "*.png" -o -name "*.jpg" -o -name "*.jpeg" -o -name "*.gif" -o -name "*.bmp" -o -name "*.tiff" \) | while read img; do \
		webp="$${img%.*}.webp"; \
		echo "converting $$img -> $$webp"; \
		cwebp -q 80 "$$img" -o "$$webp"; \
		old_basename=$$(basename "$$img"); \
		new_basename=$$(basename "$$webp"); \
		grep -rl "$$old_basename" $(CONTENT_DIR) --include="*.md" | xargs sed -i.bak "s/$$old_basename/$$new_basename/g"; \
		find $(CONTENT_DIR) -name "*.md.bak" -delete; \
		rm "$$img"; \
	done
	@echo "optimize complete"

clean:
	rm -rf $(DIST_DIR)

hooks:
	git config core.hooksPath hooks
	@echo "git hooks configured: hooks/"
