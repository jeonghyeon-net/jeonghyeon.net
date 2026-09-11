SITE_BUILDER_DIR := $(patsubst %/,%,$(dir $(abspath $(lastword $(MAKEFILE_LIST)))))
REPO_ROOT := $(abspath $(SITE_BUILDER_DIR)/../..)
SITE_BUILDER := $(SITE_BUILDER_DIR)/site-builder
SITE_BUILDER_SOURCES := $(shell find $(SITE_BUILDER_DIR) -type f -name '*.go')
SOURCE_DIR := $(REPO_ROOT)
DIST_DIR := $(REPO_ROOT)/dist
FIND_OPTIMIZABLE_IMAGES = find $(SOURCE_DIR) \( -path '$(SOURCE_DIR)/.git' -o -path '$(SOURCE_DIR)/.github' -o -path '$(DIST_DIR)' \) -prune -o -type f \( -name "*.png" -o -name "*.jpg" -o -name "*.jpeg" -o -name "*.gif" -o -name "*.bmp" -o -name "*.tiff" \)

.PHONY: prepare lint build has-optimizable-images optimize clean serve

$(SITE_BUILDER): $(SITE_BUILDER_SOURCES) $(SITE_BUILDER_DIR)/go.mod $(SITE_BUILDER_DIR)/go.sum
	cd $(SITE_BUILDER_DIR) && go build -o site-builder .

lint: $(SITE_BUILDER)
	$(SITE_BUILDER) lint $(SOURCE_DIR)

prepare: optimize $(SITE_BUILDER)
	$(SITE_BUILDER) lint --fix $(SOURCE_DIR)

build: clean $(SITE_BUILDER)
	$(SITE_BUILDER) index $(SOURCE_DIR)
	$(SITE_BUILDER) render $(SOURCE_DIR) $(DIST_DIR)
	$(SITE_BUILDER) minify $(DIST_DIR)
	$(SITE_BUILDER) check $(DIST_DIR)

MAX_DIM = 700

has-optimizable-images:
	@$(FIND_OPTIMIZABLE_IMAGES) -print -quit | grep -q .

optimize:
	@set -e; \
	if ! $(FIND_OPTIMIZABLE_IMAGES) -print -quit | grep -q .; then \
		echo "optimize complete (no images to convert)"; \
		exit 0; \
	fi; \
	command -v cwebp >/dev/null 2>&1 || { echo "cwebp not found. Install libwebp."; exit 1; }; \
	$(FIND_OPTIMIZABLE_IMAGES) -print | while IFS= read -r img; do \
		webp="$${img%.*}.webp"; \
		if command -v sips >/dev/null 2>&1; then \
			w=$$(sips -g pixelWidth "$$img" | tail -1 | awk '{print $$2}'); \
			h=$$(sips -g pixelHeight "$$img" | tail -1 | awk '{print $$2}'); \
		elif command -v identify >/dev/null 2>&1; then \
			w=$$(identify -format '%w' "$$img"); \
			h=$$(identify -format '%h' "$$img"); \
		else \
			echo "sips or ImageMagick identify is required."; \
			exit 1; \
		fi; \
		if [ "$$w" -ge "$$h" ]; then \
			resize="-resize $(MAX_DIM) 0"; \
		else \
			resize="-resize 0 $(MAX_DIM)"; \
		fi; \
		echo "converting $$img ($${w}x$${h}) -> $$webp (max $(MAX_DIM)px)"; \
		cwebp -q 80 $$resize "$$img" -o "$$webp"; \
		old_basename=$$(basename "$$img"); \
		new_basename=$$(basename "$$webp"); \
		grep -rl --include="*.md" --exclude-dir=.git --exclude-dir=.github --exclude-dir=dist "$$old_basename" $(SOURCE_DIR) | while IFS= read -r markdown; do \
			sed -i.bak "s/$$old_basename/$$new_basename/g" "$$markdown"; \
			rm "$$markdown.bak"; \
		done; \
		rm "$$img"; \
	done; \
	echo "optimize complete"

clean:
	rm -rf $(DIST_DIR)

serve: clean $(SITE_BUILDER)
	@mkdir -p $(DIST_DIR)
	@npx wrangler pages dev $(DIST_DIR) &
	@$(SITE_BUILDER) watch $(SOURCE_DIR) $(DIST_DIR)
