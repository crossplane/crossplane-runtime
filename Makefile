# ====================================================================================
# Setup Project

PROJECT_NAME := crossplane-runtime
PROJECT_REPO := github.com/crossplane/$(PROJECT_NAME)

PLATFORMS ?= linux_amd64 linux_arm64
# -include will silently skip missing files, which allows us
# to load those files with a target in the Makefile. If only
# "include" was used, the make command would fail and refuse
# to run a target until the include commands succeeded.
-include build/makelib/common.mk

# ====================================================================================
# Setup Images

# even though this repo doesn't build images (note the no-op img.build target below),
# some of the init is needed for the cross build container, e.g. setting BUILD_REGISTRY
-include build/makelib/image.mk
img.build:

# ====================================================================================
# Setup Go

# Set a sane default so that the nprocs calculation below is less noisy on the initial
# loading of this file
NPROCS ?= 1

# each of our test suites starts a kube-apiserver and running many test suites in
# parallel can lead to high CPU utilization. by default we reduce the parallelism
# to half the number of CPU cores.
GO_TEST_PARALLEL := $(shell echo $$(( $(NPROCS) / 2 )))

GO_LDFLAGS += -X $(GO_PROJECT)/pkg/version.Version=$(VERSION)
GO_SUBDIRS += pkg apis
GO111MODULE = on
GO_REQUIRED_VERSION = 1.19
-include build/makelib/golang.mk

# ====================================================================================
# Targets

# run `make help` to see the targets and options

# We want submodules to be set up the first time `make` is run.
# We manage the build/ folder and its Makefiles as a submodule.
# The first time `make` is run, the includes of build/*.mk files will
# all fail, and this target will be run. The next time, the default as defined
# by the includes will be run instead.
fallthrough: submodules
	@echo Initial setup complete. Running make again . . .
	@make

# Generate a coverage report for cobertura applying exclusions on
# - generated file
cobertura:
	@cat $(GO_TEST_OUTPUT)/coverage.txt | \
		grep -v zz_generated.deepcopy | \
		$(GOCOVER_COBERTURA) > $(GO_TEST_OUTPUT)/cobertura-coverage.xml

# Ensure a PR is ready for review.
reviewable: generate lint
	@go mod tidy

# Ensure branch is clean.
check-diff: reviewable
	@$(INFO) checking that branch is clean
	@git diff --quiet || $(FAIL)
	@$(OK) branch is clean

# Update the submodules, such as the common build scripts.
submodules:
	@git submodule sync
	@git submodule update --init --recursive

.PHONY: cobertura reviewable submodules fallthrough

# ====================================================================================
# Special Targets

define CROSSPLANE_RUNTIME_HELP
Crossplane Runtime Targets:
    cobertura          Generate a coverage report for cobertura applying exclusions on generated files.
    reviewable         Ensure a PR is ready for review.
    submodules         Update the submodules, such as the common build scripts.

endef
export CROSSPLANE_RUNTIME_HELP

crossplane-runtime.help:
	@echo "$$CROSSPLANE_RUNTIME_HELP"

help-special: crossplane-runtime.help

.PHONY: crossplane-runtime.help help-special
