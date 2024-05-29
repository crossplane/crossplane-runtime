# See https://docs.earthly.dev/docs/earthfile/features
VERSION --try --raw-output 0.8

PROJECT crossplane/crossplane-runtime

ARG --global GO_VERSION=1.22.3

# reviewable checks that a branch is ready for review. Run it before opening a
# pull request. It will catch a lot of the things our CI workflow will catch.
reviewable:
  WAIT
    BUILD +generate
  END
  BUILD +lint
  BUILD +test

# test runs unit tests.
test:
  BUILD +go-test

# lint runs linters.
lint:
  BUILD +go-lint

# build builds Crossplane for your native OS and architecture.
build:
  BUILD +go-build

# multiplatform-build builds Crossplane for all supported OS and architectures.
multiplatform-build:
  BUILD +go-multiplatform-build

# generate runs code generation. To keep builds fast, it doesn't run as part of
# the build target. It's important to run it explicitly when code needs to be
# generated, for example when you update an API type.
generate:
  BUILD +go-modules-tidy
  BUILD +go-generate

# go-modules downloads Crossplane's go modules. It's the base target of most Go
# related target (go-build, etc).
go-modules:
  ARG NATIVEPLATFORM
  FROM --platform=${NATIVEPLATFORM} golang:${GO_VERSION}
  WORKDIR /crossplane
  CACHE --id go-build --sharing shared /root/.cache/go-build
  COPY go.mod go.sum ./
  RUN go mod download
  SAVE ARTIFACT go.mod AS LOCAL go.mod
  SAVE ARTIFACT go.sum AS LOCAL go.sum

# go-modules-tidy tidies and verifies go.mod and go.sum.
go-modules-tidy:
  FROM +go-modules
  CACHE --id go-build --sharing shared /root/.cache/go-build
  COPY --dir apis/ pkg/ .
  RUN go mod tidy
  RUN go mod verify
  SAVE ARTIFACT go.mod AS LOCAL go.mod
  SAVE ARTIFACT go.sum AS LOCAL go.sum

# go-generate runs Go code generation.
go-generate:
  FROM +go-modules
  CACHE --id go-build --sharing shared /root/.cache/go-build
  COPY --dir apis/ hack/ .
  RUN go generate -tags 'generate' ./apis/...
  SAVE ARTIFACT apis/ AS LOCAL apis

# go-build builds Crossplane binaries for your native OS and architecture.
go-build:
  ARG TARGETARCH
  ARG TARGETOS
  ARG GOARCH=${TARGETARCH}
  ARG GOOS=${TARGETOS}
  ARG CGO_ENABLED=0
  FROM +go-modules
  CACHE --id go-build --sharing shared /root/.cache/go-build
  COPY --dir apis/ pkg/ .
  RUN go build ./...

# go-multiplatform-build builds Crossplane binaries for all supported OS
# and architectures.
go-multiplatform-build:
  BUILD \
    --platform=linux/amd64 \
    --platform=linux/arm64 \
    --platform=linux/arm \
    --platform=linux/ppc64le \
    --platform=darwin/arm64 \
    --platform=darwin/amd64 \
    --platform=windows/amd64 \
    +go-build

# go-test runs Go unit tests.
go-test:
  FROM +go-modules
  CACHE --id go-build --sharing shared /root/.cache/go-build
  COPY --dir apis/ pkg/ .
  RUN go test -covermode=count -coverprofile=coverage.txt ./...
  SAVE ARTIFACT coverage.txt AS LOCAL _output/tests/coverage.txt

# go-lint lints Go code.
go-lint:
  ARG GOLANGCI_LINT_VERSION=v1.59.0
  FROM +go-modules
  # This cache is private because golangci-lint doesn't support concurrent runs.
  CACHE --id go-lint --sharing private /root/.cache/golangci-lint
  CACHE --id go-build --sharing shared /root/.cache/go-build
  RUN curl -sSfL https://raw.githubusercontent.com/golangci/golangci-lint/master/install.sh | sh -s -- -b $(go env GOPATH)/bin ${GOLANGCI_LINT_VERSION}
  COPY .golangci.yml .
  COPY --dir apis/ pkg/ .
  RUN golangci-lint run --fix
  SAVE ARTIFACT apis AS LOCAL apis
  SAVE ARTIFACT pkg AS LOCAL pkg

# Targets below this point are intended only for use in GitHub Actions CI. They
# may not work outside of that environment. For example they may depend on
# secrets that are only availble in the CI environment. Targets below this point
# must be prefixed with ci-.

# TODO(negz): Is there a better way to determine the Crossplane version?
# This versioning approach maintains compatibility with the build submodule. See
# https://github.com/crossplane/build/blob/231258/makelib/common.mk#L205. This
# approach is problematic in Earthly because computing it inside a containerized
# target requires copying the entire git repository into the container. Doing so
# would invalidate all dependent target caches any time any file in git changed.

# ci-codeql-setup sets up CodeQL for the ci-codeql target.
ci-codeql-setup:
  ARG CODEQL_VERSION=v2.17.3
  FROM curlimages/curl:8.8.0
  RUN curl -fsSL https://github.com/github/codeql-action/releases/download/codeql-bundle-${CODEQL_VERSION}/codeql-bundle-linux64.tar.gz|tar zx
  SAVE ARTIFACT codeql

# ci-codeql is used by CI to build Crossplane with CodeQL scanning enabled.
ci-codeql:
  ARG CGO_ENABLED=0
  ARG TARGETOS
  ARG TARGETARCH
  # Using a static CROSSPLANE_VERSION allows Earthly to cache E2E runs as long
  # as no code changed. If the version contains a git commit (the default) the
  # build layer cache is invalidated on every commit.
  FROM +go-modules --CROSSPLANE_VERSION=v0.0.0-codeql
  IF [ "${TARGETARCH}" = "arm64" ] && [ "${TARGETOS}" = "linux" ]
    RUN --no-cache echo "CodeQL doesn't support Linux on Apple Silicon" && false
  END
  COPY --dir +ci-codeql-setup/codeql /codeql
  CACHE --id go-build --sharing shared /root/.cache/go-build
  COPY --dir apis/ pkg/ .
  RUN /codeql/codeql database create /codeqldb --language=go
  RUN /codeql/codeql database analyze /codeqldb --threads=0 --format=sarif-latest --output=go.sarif --sarif-add-baseline-file-info
  SAVE ARTIFACT go.sarif AS LOCAL _output/codeql/go.sarif
