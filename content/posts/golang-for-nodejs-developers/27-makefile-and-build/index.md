# Makefile과 빌드

Go는 번들러도, 트랜스파일러도 필요 없다. `go build` 하나면 바이너리가 나온다. 하지만 빌드 옵션, 테스트, 린팅, 크로스 컴파일을 매번 타이핑하는 것은 비효율적이다. `package.json` scripts와 같은 역할을 Makefile이 한다.

## 왜 Makefile인가

`go.mod`는 의존성만 관리하고 스크립트 기능이 없다. `npm run build` 같은 단축 명령이 필요하면 Makefile을 쓴다:

```makefile
build:
	go build -o bin/server ./cmd/server

test:
	go test ./...

lint:
	golangci-lint run

fmt:
	gofmt -w .
```

`make build`, `make test`, `make lint`. `npm run`과 같은 역할이다.

Makefile은 Go 전용 도구가 아니다. C/C++ 시대부터 존재한 범용 빌드 도구다. Go 커뮤니티가 Makefile을 선호하는 이유는 단순하다. 의존성 없이 대부분의 시스템에 이미 설치되어 있고, 셸 명령을 그대로 쓸 수 있으며, 파일 의존성 기반 증분 빌드를 지원한다.

## Makefile 기본 문법

Makefile은 target, dependency, recipe 세 요소로 구성된다:

```makefile
target: dependency1 dependency2
	recipe command
```

- **target** -- 만들려는 것의 이름. 파일명이거나 추상적인 작업명이다.
- **dependency** -- target을 만들기 전에 먼저 실행해야 하는 다른 target.
- **recipe** -- 실행할 셸 명령. 반드시 탭 문자로 들여쓰기해야 한다. 스페이스는 안 된다.

탭 문자 규칙은 Makefile 초심자가 가장 많이 겪는 함정이다. 에디터가 탭을 스페이스로 변환하면 `make`가 실패한다.

파일을 생성하지 않는 target은 `.PHONY`로 선언한다:

```makefile
.PHONY: build test lint fmt clean

build:
	go build -o bin/server ./cmd/server

clean:
	rm -rf bin/
```

`.PHONY`가 없으면 `build`라는 파일이 디렉토리에 존재할 때 `make build`가 "이미 최신"이라며 아무것도 하지 않는다. Go 프로젝트에서 대부분의 target은 파일이 아닌 작업이므로 `.PHONY`를 습관적으로 선언한다.

변수를 정의하고 참조할 수 있다:

```makefile
BINARY=bin/server
CMD=./cmd/server

build:
	go build -o $(BINARY) $(CMD)
```

## 실전 Makefile 구성

Go 프로젝트에서 자주 사용하는 target을 모아보면:

```makefile
.PHONY: build test lint fmt run clean vet

BINARY := bin/server
CMD := ./cmd/server

build:
	go build -o $(BINARY) $(CMD)

run: build
	./$(BINARY)

test:
	go test -v -race ./...

vet:
	go vet ./...

lint: vet
	golangci-lint run

fmt:
	gofmt -w .
	goimports -w .

clean:
	rm -rf bin/
	go clean -cache
```

`run`은 `build`를 dependency로 가진다. `make run`을 실행하면 먼저 `build`가 실행되고, 성공하면 바이너리를 실행한다. `lint`는 `vet`에 의존하므로 `make lint`를 실행하면 `go vet`이 먼저 돌고, 이어서 `golangci-lint`가 실행된다.

webpack, esbuild, tsc 같은 번들러나 트랜스파일러 조합이 `go build` 하나로 대체된다. 컴파일러가 의존성 분석, 데드코드 제거, 단일 바이너리 생성을 모두 처리한다.

## go build 옵션

`go build`의 주요 옵션:

```bash
# 출력 파일명 지정
go build -o bin/myapp ./cmd/server

# 모든 패키지 빌드 (바이너리 생성 없이 컴파일만 확인)
go build ./...

# 경쟁 상태 감지기 포함 빌드
go build -race -o bin/myapp ./cmd/server

# 캐시 무시하고 전체 재빌드
go build -a ./cmd/server

# 빌드 과정 상세 출력
go build -v ./cmd/server

# 컴파일러/링커 명령 확인
go build -x ./cmd/server
```

`-race` 플래그는 13편에서 다룬 경쟁 상태 감지기를 활성화한다. 개발/테스트 빌드에서 사용하고, 프로덕션 빌드에서는 성능 오버헤드 때문에 제외하는 것이 일반적이다.

## 크로스 컴파일

Go는 네이티브 바이너리를 생성하므로 대상 OS와 아키텍처에 맞게 빌드해야 한다. 런타임이 알아서 플랫폼 차이를 흡수해주지 않는다.

Go의 크로스 컴파일은 환경변수 두 개면 된다:

```bash
# Linux AMD64용 빌드 (macOS에서 실행)
GOOS=linux GOARCH=amd64 go build -o bin/server-linux ./cmd/server

# Windows용 빌드
GOOS=windows GOARCH=amd64 go build -o bin/server.exe ./cmd/server

# Linux ARM64용 빌드 (AWS Graviton, Raspberry Pi 등)
GOOS=linux GOARCH=arm64 go build -o bin/server-arm64 ./cmd/server
```

별도의 크로스 컴파일 툴체인 설치가 필요 없다. Go 컴파일러 자체가 모든 대상 플랫폼의 코드를 생성할 수 있다. C/C++에서 크로스 컴파일 환경을 구축하는 고통과 비교하면 극적으로 간단하다.

Makefile에 크로스 컴파일 target을 추가하면:

```makefile
.PHONY: build-all

PLATFORMS := linux/amd64 linux/arm64 darwin/amd64 darwin/arm64

build-all:
	@for platform in $(PLATFORMS); do \
		os=$${platform%/*}; \
		arch=$${platform#*/}; \
		output=bin/server-$${os}-$${arch}; \
		echo "Building $$output"; \
		GOOS=$$os GOARCH=$$arch go build -o $$output ./cmd/server; \
	done
```

`make build-all`로 네 개의 플랫폼에 대한 바이너리를 한 번에 생성한다.

지원하는 `GOOS`/`GOARCH` 조합 목록은 다음 명령으로 확인한다:

```bash
go tool dist list
```

## ldflags — 빌드 시 변수 주입

`process.env.VERSION`처럼 런타임에 환경변수를 읽는 대신, 빌드 시점에 변수 값을 바이너리에 직접 주입할 수 있다. `-ldflags`(linker flags)를 사용한다:

```go
// main.go
package main

import "fmt"

var (
    version = "dev"
    commit  = "unknown"
    date    = "unknown"
)

func main() {
    fmt.Printf("version: %s\ncommit: %s\ndate: %s\n", version, commit, date)
}
```

```bash
go build -ldflags "-X main.version=1.2.3 -X main.commit=abc123 -X main.date=2026-03-15" -o bin/server ./cmd/server
```

`-X` 플래그는 `패키지경로.변수명=값` 형태로 문자열 변수의 값을 덮어쓴다. 빌드 시점에 결정되어 바이너리에 포함되므로 런타임에 환경변수를 읽는 것과 달리 변경할 수 없다.

Makefile에서 git 정보를 자동으로 주입하는 패턴:

```makefile
VERSION := $(shell git describe --tags --always --dirty)
COMMIT := $(shell git rev-parse --short HEAD)
DATE := $(shell date -u +%Y-%m-%dT%H:%M:%SZ)
LDFLAGS := -X main.version=$(VERSION) -X main.commit=$(COMMIT) -X main.date=$(DATE)

build:
	go build -ldflags "$(LDFLAGS)" -o bin/server ./cmd/server
```

`make build`를 실행하면 바이너리에 현재 git tag, commit hash, 빌드 시각이 포함된다. CI/CD 파이프라인에서 특히 유용하다. 배포된 바이너리가 어떤 커밋에서 빌드되었는지 바이너리 자체에서 확인할 수 있다.

바이너리 크기를 줄이려면 디버그 정보를 제거하는 ldflags를 추가한다:

```bash
go build -ldflags "-s -w" -o bin/server ./cmd/server
```

`-s`는 심볼 테이블을, `-w`는 DWARF 디버그 정보를 제거한다. 바이너리 크기가 20-30% 줄어든다. 디버깅이 필요 없는 프로덕션 배포에 적합하다.

## build tag — 조건부 컴파일

특정 조건에서만 포함할 코드를 지정할 수 있다. 파일 상단에 `//go:build` 지시자를 추가한다:

```go
//go:build linux

package platform

func DataDir() string {
    return "/var/lib/myapp"
}
```

```go
//go:build darwin

package platform

func DataDir() string {
    return "/Library/Application Support/myapp"
}
```

`GOOS=linux`로 빌드하면 첫 번째 파일만, `GOOS=darwin`으로 빌드하면 두 번째 파일만 포함된다. 같은 함수 `DataDir()`이 두 파일에 정의되어 있지만, 한 번에 하나만 컴파일되므로 중복 정의 에러가 발생하지 않는다.

OS/아키텍처 외에 커스텀 tag도 사용할 수 있다:

```go
//go:build integration

package user_test

import "testing"

func TestDatabaseIntegration(t *testing.T) {
    // 실제 데이터베이스에 연결하는 느린 테스트
}
```

```bash
# 일반 테스트만 실행 (integration tag가 없는 파일)
go test ./...

# integration 테스트 포함
go test -tags=integration ./...
```

Makefile에 반영하면:

```makefile
test:
	go test -v -race ./...

test-integration:
	go test -v -race -tags=integration ./...
```

`make test`는 빠른 단위 테스트만, `make test-integration`은 통합 테스트를 포함한다. CI 파이프라인에서 단계를 나눌 때 유용하다.

build tag의 논리 연산도 지원한다:

```go
//go:build linux && amd64
//go:build !windows
//go:build integration || e2e
```

`&&`는 AND, `||`는 OR, `!`는 NOT이다.

`NODE_ENV`에 따른 조건부 import와 달리, build tag는 컴파일 타임에 파일을 제외한다. 프로덕션 바이너리에 테스트 코드가 포함되는 일이 없다.

## 완성된 Makefile 예시

지금까지 다룬 내용을 하나의 Makefile로 종합하면:

```makefile
.PHONY: build run test test-integration lint fmt vet clean build-all

BINARY := bin/server
CMD := ./cmd/server

VERSION := $(shell git describe --tags --always --dirty)
COMMIT := $(shell git rev-parse --short HEAD)
DATE := $(shell date -u +%Y-%m-%dT%H:%M:%SZ)
LDFLAGS := -s -w -X main.version=$(VERSION) -X main.commit=$(COMMIT) -X main.date=$(DATE)

build:
	go build -ldflags "$(LDFLAGS)" -o $(BINARY) $(CMD)

run: build
	./$(BINARY)

test:
	go test -v -race ./...

test-integration:
	go test -v -race -tags=integration ./...

vet:
	go vet ./...

lint: vet
	golangci-lint run

fmt:
	gofmt -w .
	goimports -w .

clean:
	rm -rf bin/
	go clean -cache

build-all:
	@for platform in linux/amd64 linux/arm64 darwin/amd64 darwin/arm64; do \
		os=$${platform%/*}; \
		arch=$${platform#*/}; \
		GOOS=$$os GOARCH=$$arch go build -ldflags "$(LDFLAGS)" \
			-o bin/server-$$os-$$arch $(CMD); \
	done
```

`package.json` scripts 대비 장점은 dependency 체인, 변수 치환, 셸 명령의 자유로운 조합이 가능하다는 것이다. 단점은 문법이 직관적이지 않고, 탭/스페이스 구분 같은 함정이 있다는 것이다. Makefile은 단순한 빌드 명령들을 조직화하는 얇은 레이어일 뿐이다.
