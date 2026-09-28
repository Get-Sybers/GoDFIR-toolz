# GoDFIR-toolz — Go modules. Root Makefile: the standard Go target set
# (go-standards.md §13). conform/upgrade are the two scripted mechanisms (§12);
# fmt writes, fmt-check gates. Targets fan out across the go.work modules.
#
# Container-image build/hardening conformance is SEPARATE tooling and is left
# as-is: conform.sh (per-image framework checks) and build-all.sh (standalone
# image build). signatures/goyara needs libyara (CGO); build/test/vet skip it
# where the library is absent — CI with libyara is its gate.

.PHONY: build test vet fmt fmt-check tidy generate conform go-conform upgrade

MODULES = $(shell go work edit -json | sed -n 's/.*"DiskPath": "\(.*\)".*/\1/p')

# skip a module needing a CGO library we lack (goyara/libyara)
define run_go
@set -e; for m in $(MODULES); do \
  case $$m in */goyara) pkg-config --exists yara 2>/dev/null || { echo ">> $$m (skip: libyara absent)"; continue; };; esac; \
  echo ">> $$m"; (cd $$m && $(1)); \
done
endef

build:    ; $(call run_go,go build ./...)
test:     ; $(call run_go,go test ./...)
vet:      ; $(call run_go,go vet ./...)
generate: ; @set -e; for m in $(MODULES); do (cd $$m && go generate ./...); done
tidy:     ; @set -e; for m in $(MODULES); do echo ">> $$m"; (cd $$m && go mod tidy); done

## fmt: gofmt -w every module (writes)
fmt:      ; @gofmt -w $(MODULES)
## fmt-check: fail if any Go file needs gofmt (gates CI)
fmt-check: ; @out=$$(gofmt -l $(MODULES)); if [ -n "$$out" ]; then echo "gofmt needed:"; echo "$$out"; exit 1; fi

## conform: the Go conformance gate (§12) — run by CI and locally
go-conform: ; @scripts/go-conform.sh
conform: go-conform

## upgrade: raise deps + toolchain to the highest allowed, then re-conform (§12)
upgrade:  ; @scripts/go-upgrade.sh
