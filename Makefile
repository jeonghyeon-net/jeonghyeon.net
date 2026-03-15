TRANSFORMER = ./transformer/transformer
CONTENT_DIR = content
DIST_DIR = dist

.PHONY: lint build optimize clean hooks serve

$(TRANSFORMER): $(wildcard transformer/*.go transformer/**/*.go) transformer/go.mod
	cd transformer && go build -o transformer .

lint: $(TRANSFORMER)
	$(TRANSFORMER) lint $(CONTENT_DIR)

build: clean $(TRANSFORMER)
	$(TRANSFORMER) index $(CONTENT_DIR)
	$(TRANSFORMER) render $(CONTENT_DIR) $(DIST_DIR)
	$(TRANSFORMER) minify $(DIST_DIR)
	$(TRANSFORMER) check $(DIST_DIR)

MAX_DIM = 700

optimize:
	@command -v cwebp >/dev/null 2>&1 || { echo "cwebp not found. Install libwebp."; exit 1; }
	@command -v sips >/dev/null 2>&1 || { echo "sips not found."; exit 1; }
	@find $(CONTENT_DIR) -type f \( -name "*.png" -o -name "*.jpg" -o -name "*.jpeg" -o -name "*.gif" -o -name "*.bmp" -o -name "*.tiff" \) | while read img; do \
		webp="$${img%.*}.webp"; \
		w=$$(sips -g pixelWidth "$$img" | tail -1 | awk '{print $$2}'); \
		h=$$(sips -g pixelHeight "$$img" | tail -1 | awk '{print $$2}'); \
		if [ "$$w" -ge "$$h" ]; then \
			resize="-resize $(MAX_DIM) 0"; \
		else \
			resize="-resize 0 $(MAX_DIM)"; \
		fi; \
		echo "converting $$img ($${w}x$${h}) -> $$webp (max $(MAX_DIM)px)"; \
		cwebp -q 80 $$resize "$$img" -o "$$webp"; \
		old_basename=$$(basename "$$img"); \
		new_basename=$$(basename "$$webp"); \
		grep -rl "$$old_basename" $(CONTENT_DIR) --include="*.md" | xargs sed -i.bak "s/$$old_basename/$$new_basename/g"; \
		find $(CONTENT_DIR) -name "*.md.bak" -delete; \
		rm "$$img"; \
	done
	@echo "optimize complete"

clean:
	rm -rf $(DIST_DIR)

serve: clean $(TRANSFORMER)
	@mkdir -p $(DIST_DIR)
	@npx wrangler pages dev $(DIST_DIR) &
	@$(TRANSFORMER) watch $(CONTENT_DIR) $(DIST_DIR)

hooks:
	git config core.hooksPath hooks
	@echo "git hooks configured: hooks/"
