# 개발 환경 설정

Go 설치부터 Hello, World 실행까지. Node.js 개발자가 익숙한 도구와 1:1로 매핑하며 빠르게 환경을 갖춘다.

## Go 설치

[go.dev/dl](https://go.dev/dl/)에서 OS에 맞는 installer를 받아 실행하면 된다. macOS는 `.pkg`, Windows는 `.msi`, Linux는 tarball을 제공한다.

설치 후 확인:

```bash
go version
# go version go1.24.1 darwin/arm64
```

### 버전 관리

Node.js에 `nvm`이 있다면 Go에는 여러 선택지가 있다. 대표적으로 두 가지:

**go install 방식** -- 추가 도구 없이 Go 자체 기능으로 여러 버전을 관리한다:

```bash
go install golang.org/dl/go1.23.0@latest
go1.23.0 download
go1.23.0 version
```

**goenv** -- `nvm`과 유사한 인터페이스:

```bash
goenv install 1.24.1
goenv global 1.24.1
```

실무에서는 팀 전체가 동일 버전을 쓰는 경우가 많고, `go.mod`에 Go 버전이 명시되므로 `nvm`만큼 빈번하게 버전을 전환할 일은 드물다.

## 프로젝트 초기화: go mod init

Node.js에서 새 프로젝트를 시작할 때 `npm init`을 실행하듯, Go에서는 `go mod init`을 실행한다.

```bash
mkdir myapp && cd myapp
go mod init github.com/username/myapp
```

이 명령은 `go.mod` 파일을 생성한다. `package.json`에 대응하는 파일이다.

```
module github.com/username/myapp

go 1.24
```

`go.mod`는 모듈 경로와 Go 버전, 그리고 의존성 목록을 담는다. `package.json`과 비교하면:

| package.json | go.mod |
|---|---|
| `name` | `module` (모듈 경로) |
| `engines.node` | `go` (Go 버전) |
| `dependencies` | `require` (의존성) |

의존성을 추가하면 `go.sum`이 생긴다. `package-lock.json`과 같은 역할이다. 의존성의 정확한 버전과 checksum을 기록하여 재현 가능한 빌드를 보장한다.

```bash
# Node.js
npm install express

# Go
go get github.com/gin-gonic/gin
```

## GOPATH에서 모듈로

Go의 의존성 관리 역사를 간략히 알아둘 필요가 있다. 지금은 모듈 모드가 기본이지만, 과거에는 GOPATH라는 방식을 썼다.

**GOPATH 시대 (Go 1.0 ~ 1.10):**
모든 Go 코드가 하나의 디렉토리(`$GOPATH/src`) 아래에 있어야 했다. 프로젝트를 원하는 위치에 둘 수 없었다. 의존성도 같은 디렉토리에 들어갔고, 버전 개념이 없었다. `go get`은 항상 최신 코드를 가져왔다.

Node.js로 비유하면, 모든 프로젝트와 node_modules가 하나의 전역 디렉토리에 섞여 있고, `npm install`이 항상 latest를 설치하는 상황이다.

**모듈 모드 (Go 1.11~):**
Go 1.11에서 모듈이 실험적으로 도입되었고, Go 1.16부터 기본값이 되었다. `go.mod`로 프로젝트별 의존성을 관리하고, semantic versioning을 지원하며, 프로젝트를 어느 디렉토리에든 둘 수 있게 되었다.

지금 Go를 시작한다면 GOPATH를 직접 다룰 일은 없다. 다만 오래된 문서나 블로그에서 `$GOPATH/src/...` 경로가 등장하면, 그것이 과거 방식이라는 것만 알면 된다.

## IDE 설정

### VS Code

1. [Go extension](https://marketplace.visualstudio.com/items?itemName=golang.go)을 설치한다.
2. extension이 `gopls`(Go language server) 설치를 제안하면 수락한다.

이것으로 끝이다. 자동 완성, 정의로 이동, 에러 표시, 저장 시 자동 포맷팅이 동작한다.

`gopls`는 TypeScript의 `tsserver`에 대응한다. 차이점은 Go의 formatter(`gofmt`)가 언어에 내장되어 있다는 것이다. Prettier 설정을 두고 팀원과 논쟁할 필요가 없다. 탭이냐 스페이스냐의 논쟁은 Go에서 끝났다. 탭이다.

### GoLand

JetBrains의 Go 전용 IDE다. IntelliJ나 WebStorm을 쓰고 있다면 익숙한 환경에서 바로 시작할 수 있다. 별도 설정 없이 설치만 하면 된다.

## Hello, World

프로젝트를 만들고 첫 프로그램을 작성한다.

```bash
mkdir hello && cd hello
go mod init example.com/hello
```

`main.go` 파일을 생성한다:

```go
package main

import "fmt"

func main() {
    fmt.Println("Hello, World")
}
```

세 가지를 짚는다:

- `package main` -- 실행 가능한 프로그램의 진입점 package다. 라이브러리가 아닌 실행 파일을 만들려면 반드시 `main` package여야 한다.
- `import "fmt"` -- 표준 라이브러리의 formatting package를 가져온다. `require`나 `import`와 같다.
- `func main()` -- 프로그램의 시작 함수다. Node.js에는 명시적 진입점 함수가 없지만, Go는 `main` package의 `main` 함수에서 실행이 시작된다.

실행:

```bash
go run main.go
# Hello, World
```

`go run`은 내부적으로 컴파일 후 실행한다. `node main.js`처럼 쓸 수 있지만, 실제로는 컴파일이 먼저 일어난다.

바이너리를 만들려면:

```bash
go build -o hello
./hello
# Hello, World
```

| Node.js | Go | 설명 |
|---|---|---|
| `node index.js` | `go run main.go` | 소스에서 바로 실행 |
| `npm run build` (번들러) | `go build` | 배포용 결과물 생성 |
| `npx` | `go run` (패키지 경로) | 도구 실행 |

## 프로젝트 디렉토리 구조

Go 프로젝트의 기본 구조는 단순하다. 최소한의 구조만 보면:

```
myapp/
  go.mod
  go.sum
  main.go
```

프로젝트가 커지면 package 단위로 디렉토리를 나눈다:

```
myapp/
  go.mod
  go.sum
  main.go
  handler/
    user.go
    product.go
  model/
    user.go
  repository/
    user.go
```

Node.js와 비교하면:

| Node.js | Go |
|---|---|
| `src/` 아래 자유 구조 | 프로젝트 루트부터 package 단위로 구성 |
| `index.js`가 모듈 진입점 | 디렉토리 = package. 진입점 파일 개념 없음 |
| `require`/`import`로 파일 지정 | import 경로가 package(디렉토리) 단위 |

Go에서 디렉토리 하나가 package 하나다. 같은 디렉토리의 `.go` 파일은 모두 같은 package에 속한다. Node.js처럼 파일 단위로 import하지 않고 package 단위로 import한다.
