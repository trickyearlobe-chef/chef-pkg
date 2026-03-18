BINARY     := chef-pkg
MODULE     := github.com/trickyearlobe-chef/chef-pkg
GO         := go
GOFLAGS    :=
LDFLAGS    := -s -w
BUILD_DIR  := bin
HOOKS_DIR  := .githooks

# Secret patterns to scan for in staged files.
# Add additional patterns here — one extended-regex per line.
SECRET_PATTERNS := \
	'(?i)(password|passwd|secret|token|api.?key)\s*[:=]\s*["\x27][^\s"'\'']{8,}' \
	'(?i)license.?id\s*[:=]\s*["\x27][0-9a-f-]{36}' \
	'AKIA[0-9A-Z]{16}' \
	'-----BEGIN (RSA |EC |DSA )?PRIVATE KEY-----'

.PHONY: help all build install test test-verbose lint fmt vet clean tidy \
        setup-hooks remove-hooks

help: ## Show this help (default)
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | \
		awk 'BEGIN {FS = ":.*?## "}; {printf "  \033[36m%-18s\033[0m %s\n", $$1, $$2}'

all: fmt vet test build ## Format, vet, test, then build

build: ## Build the binary
	@mkdir -p $(BUILD_DIR)
	$(GO) build $(GOFLAGS) -ldflags '$(LDFLAGS)' -o $(BUILD_DIR)/$(BINARY) .

install: ## Install the binary into GOBIN or GOPATH/bin
	$(GO) install $(GOFLAGS) -ldflags '$(LDFLAGS)' .

test: ## Run tests
	$(GO) test $(GOFLAGS) ./...

test-verbose: ## Run tests with verbose output
	$(GO) test $(GOFLAGS) -v ./...

test-cover: ## Run tests with coverage report
	$(GO) test $(GOFLAGS) -coverprofile=$(BUILD_DIR)/coverage.out ./...
	$(GO) tool cover -html=$(BUILD_DIR)/coverage.out -o $(BUILD_DIR)/coverage.html
	@echo "Coverage report: $(BUILD_DIR)/coverage.html"

lint: vet ## Run linters (requires golangci-lint)
	@command -v golangci-lint >/dev/null 2>&1 || { \
		echo "golangci-lint not found — install from https://golangci-lint.run/"; exit 1; }
	golangci-lint run ./...

fmt: ## Format source code
	$(GO) fmt ./...

vet: ## Run go vet
	$(GO) vet ./...

tidy: ## Tidy go.mod / go.sum
	$(GO) mod tidy

clean: ## Remove build artifacts
	rm -rf $(BUILD_DIR)

# ---------------------------------------------------------------------------
# Git hooks
# ---------------------------------------------------------------------------

setup-hooks: $(HOOKS_DIR)/pre-commit ## Install git hooks
	git config core.hooksPath $(HOOKS_DIR)
	@echo "Git hooks installed from $(HOOKS_DIR)/"

remove-hooks: ## Remove custom git hooks path
	git config --unset core.hooksPath || true
	@echo "Git hooks path reset to default"

$(HOOKS_DIR)/pre-commit:
	@mkdir -p $(HOOKS_DIR)
	@echo '#!/usr/bin/env bash'                                          >  $@
	@echo 'set -euo pipefail'                                           >> $@
	@echo ''                                                             >> $@
	@echo '# ---- Secret scanning pre-commit hook ----'                 >> $@
	@echo '# Scans staged files for common secret patterns.'            >> $@
	@echo '# Add exceptions to .secret-scan-ignore (one regex per line).' >> $@
	@echo ''                                                             >> $@
	@echo 'IGNORE_FILE=".secret-scan-ignore"'                           >> $@
	@echo ''                                                             >> $@
	@echo 'PATTERNS=('                                                   >> $@
	@for pat in $(SECRET_PATTERNS); do \
		echo "  $$pat" >> $@; \
	done
	@echo ')'                                                            >> $@
	@echo ''                                                             >> $@
	@echo 'FILES=$$(git diff --cached --name-only --diff-filter=ACMR)'  >> $@
	@echo '[ -z "$$FILES" ] && exit 0'                                   >> $@
	@echo ''                                                             >> $@
	@echo 'GREP_OPTS=(-n --with-filename)'                              >> $@
	@echo 'if [ -f "$$IGNORE_FILE" ]; then'                             >> $@
	@echo '  while IFS= read -r line; do'                               >> $@
	@echo '    [[ -z "$$line" || "$$line" == \#* ]] && continue'        >> $@
	@echo '    GREP_OPTS+=(--exclude="$$line")'                         >> $@
	@echo '  done < "$$IGNORE_FILE"'                                    >> $@
	@echo 'fi'                                                          >> $@
	@echo ''                                                             >> $@
	@echo 'FOUND=0'                                                     >> $@
	@echo 'for pattern in "$${PATTERNS[@]}"; do'                        >> $@
	@echo '  if echo "$$FILES" | xargs grep -P -H -n "$$pattern" 2>/dev/null; then' >> $@
	@echo '    FOUND=1'                                                  >> $@
	@echo '  fi'                                                         >> $@
	@echo 'done'                                                         >> $@
	@echo ''                                                             >> $@
	@echo 'if [ "$$FOUND" -eq 1 ]; then'                                >> $@
	@echo '  echo ""'                                                    >> $@
	@echo '  echo "ERROR: Potential secrets detected in staged files."'  >> $@
	@echo '  echo "If these are false positives, add exclusion patterns to .secret-scan-ignore"' >> $@
	@echo '  echo "or commit with --no-verify to bypass this check."'   >> $@
	@echo '  exit 1'                                                     >> $@
	@echo 'fi'                                                          >> $@
	@chmod +x $@
	@echo "Created $@"
