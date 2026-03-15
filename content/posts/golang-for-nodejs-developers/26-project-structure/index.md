# 프로젝트 구조와 관례

Go 프로젝트가 커지면 디렉토리를 어떻게 나눌지 고민이 시작된다. Node.js는 정해진 규칙 없이 팀마다 다른 구조를 쓰지만, Go는 커뮤니티가 수렴한 관례가 있다. 코드 포매팅과 린팅도 마찬가지다. 논쟁 대신 도구가 결정한다.

## 표준 디렉토리 레이아웃

02편에서 간단한 프로젝트 구조를 봤다. 프로젝트 규모가 커지면 다음 관례적 레이아웃으로 발전한다:

```
myapp/
  cmd/
    server/
      main.go       // 서버 바이너리 진입점
    cli/
      main.go       // CLI 도구 진입점
  internal/
    auth/
      token.go
    user/
      repository.go
      service.go
  pkg/
    validate/
      email.go
  go.mod
  go.sum
```

각 디렉토리의 역할:

- **cmd/** -- 실행 가능한 바이너리의 진입점. 각 하위 디렉토리가 하나의 `main` 패키지다. 비즈니스 로직은 여기에 두지 않는다. 의존성 조립과 실행만 담당한다.
- **internal/** -- 프로젝트 내부에서만 사용하는 패키지. 외부 모듈에서 import할 수 없다.
- **pkg/** -- 외부에서 import해도 되는 라이브러리 코드. 다른 프로젝트에서 재사용할 수 있다.

Node.js 프로젝트와 비교하면:

| Node.js | Go | 역할 |
|---|---|---|
| `src/index.ts` | `cmd/server/main.go` | 진입점 |
| `src/lib/`, `src/utils/` | `pkg/` | 재사용 가능한 코드 |
| (해당 없음) | `internal/` | 외부 접근 차단 |
| `src/modules/`, `src/services/` | `internal/` | 비즈니스 로직 |

Node.js에서는 `src/` 아래에 자유롭게 디렉토리를 만든다. `controllers/`, `services/`, `models/`, `utils/` 등 팀마다 다른 구조를 쓰고, 강제하는 메커니즘은 없다. Go에서도 이 레이아웃이 강제는 아니지만, 대부분의 오픈소스 프로젝트가 따르고 있어 코드를 처음 보는 사람도 구조를 바로 파악할 수 있다.

### cmd/ -- 여러 바이너리 관리

하나의 모듈에서 여러 실행 파일을 만들 수 있다:

```go
// cmd/server/main.go
package main

import (
    "fmt"
    "myapp/internal/user"
)

func main() {
    svc := user.NewService()
    fmt.Println("starting server with", svc)
}
```

```go
// cmd/cli/main.go
package main

import (
    "fmt"
    "myapp/internal/user"
)

func main() {
    svc := user.NewService()
    fmt.Println("running cli with", svc)
}
```

빌드는 경로를 지정한다:

```bash
go build -o server ./cmd/server
go build -o cli ./cmd/cli
```

Node.js에서는 `package.json`의 `bin` 필드로 여러 명령을 등록하지만, 결국 같은 런타임에서 분기한다. Go에서는 각 `cmd/` 하위 디렉토리가 독립적인 바이너리를 생성한다.

### pkg/ 사용 여부

`pkg/` 디렉토리는 논란이 있다. Go 팀 공식 권장이 아니며, "외부에 공개할 코드"라는 의도를 표현하는 관례일 뿐이다. 실제로 프로젝트가 라이브러리가 아닌 애플리케이션이라면 `pkg/`가 불필요할 수 있다. `internal/`만으로 충분한 경우가 많다.

소규모 프로젝트에서는 `cmd/`, `internal/`, `pkg/`를 모두 만들 필요가 없다. 파일 몇 개로 구성된 프로젝트에 이 구조를 적용하면 오히려 과도한 추상화다. Go 공식 가이드도 "필요해지면 나눠라"라고 말한다.

## internal 패키지 -- 접근 제한 메커니즘

11편에서 `internal` 패키지를 간략히 다뤘다. 여기서 더 자세히 살펴본다.

`internal` 디렉토리 아래의 패키지는 `internal`의 부모 디렉토리를 루트로 하는 트리 안에서만 import할 수 있다. 컴파일러가 강제한다:

```
myapp/
  internal/
    auth/
      token.go      // package auth
  handler/
    user.go          // import "myapp/internal/auth" -- OK
  main.go            // import "myapp/internal/auth" -- OK
```

같은 모듈 안에서는 자유롭게 import한다. 하지만 다른 모듈에서 `import "myapp/internal/auth"`를 시도하면:

```
use of internal package myapp/internal/auth not allowed
```

컴파일 에러다. 이 제한은 `internal`이라는 디렉토리 이름만으로 활성화된다. 별도의 설정이 필요 없다.

`internal`은 중첩할 수도 있다:

```
myapp/
  service/
    internal/
      cache/        // service/ 하위에서만 import 가능
    handler.go      // import "myapp/service/internal/cache" -- OK
  main.go           // import "myapp/service/internal/cache" -- 에러
```

`service/internal/cache`는 `service/` 패키지와 그 하위에서만 접근 가능하다. 프로젝트 루트의 `main.go`에서도 접근할 수 없다.

Node.js에는 이에 대응하는 메커니즘이 약하다. `package.json`의 `exports` 필드로 모듈의 공개 API를 정의할 수 있지만, 파일 경로를 직접 import하면 우회된다. TypeScript의 `paths` 설정이나 ESLint 규칙으로 제한할 수는 있으나, 런타임이나 컴파일러 수준의 강제는 아니다.

## naming convention -- 간결한 이름

Go는 이름을 짧게 짓는 문화가 있다. Node.js 생태계에서 흔한 `utils/`, `helpers/`, `common/` 같은 이름은 Go에서 권장되지 않는다.

### 패키지 이름

```go
// 나쁜 예
package string_utils
package httpHelpers
package common

// 좋은 예
package strings
package http
package auth
```

규칙:

- 소문자, 한 단어. 밑줄이나 camelCase를 쓰지 않는다.
- 패키지가 하는 일을 설명하는 명사를 쓴다.
- `util`, `helper`, `common`, `misc` 같은 포괄적 이름을 피한다. 무엇이든 들어갈 수 있는 이름은 결국 모든 것이 들어간다.

Node.js 프로젝트에서 `utils/` 디렉토리가 비대해지는 현상은 흔하다. `utils/string.ts`, `utils/date.ts`, `utils/validation.ts`가 쌓이면 결국 성격이 다른 코드가 한 곳에 섞인다. Go에서는 처음부터 `strings`, `time`, `validate` 등 구체적인 패키지로 나눈다.

### 함수와 변수 이름

패키지명이 컨텍스트를 제공하므로 함수명에서 패키지명을 반복하지 않는다:

```go
// 나쁜 예
package user

func UserCreate() {}  // user.UserCreate -- user가 반복된다
func GetUser() {}     // user.GetUser -- Get 접두사 불필요

// 좋은 예
package user

func Create() {}      // user.Create
func ByID(id int) {}  // user.ByID
```

Node.js에서는 파일 단위로 import하므로 함수명에 충분한 컨텍스트가 필요하다:

```javascript
// Node.js에서 흔한 패턴
import { createUser, getUserById } from "./user.js";
```

Go에서는 패키지명이 접두사 역할을 하므로 함수명이 짧아진다. `http.Get`, `json.Marshal`, `fmt.Println` 등 표준 라이브러리가 이 패턴을 따른다.

## gofmt와 goimports -- 포매팅은 논쟁 대상이 아니다

Node.js 프로젝트에서 Prettier를 설정할 때 탭이냐 스페이스냐, 세미콜론 유무, 줄 길이, trailing comma 등을 논의한다. Go에서 이런 논의는 없다.

`gofmt`는 Go에 내장된 코드 포매터다. 설정 옵션이 없다. 모든 Go 코드가 동일한 스타일로 포매팅된다:

```bash
gofmt -w .
```

`goimports`는 `gofmt`의 상위 호환이다. 포매팅에 더해 import 구문을 자동 정리한다. 사용하지 않는 import를 제거하고, 필요한 import를 추가한다:

```bash
go install golang.org/x/tools/cmd/goimports@latest
goimports -w .
```

VS Code의 Go extension은 저장 시 자동으로 `goimports`를 실행한다. 대부분의 Go 개발자가 이 설정을 쓰므로, 포매팅은 의식하지 않아도 되는 영역이다.

| 도구 | 포매팅 | import 정리 | 설정 |
|---|---|---|---|
| gofmt | O | X | 없음 |
| goimports | O | O | 없음 |
| Prettier | O | X | 다수 |

"gofmt의 스타일이 마음에 들지 않더라도, gofmt의 스타일이 모두의 스타일이다"는 Go 커뮤니티의 격언이다.

## go vet -- 내장 정적 분석

`go vet`은 Go 툴체인에 내장된 정적 분석 도구다. 컴파일러가 잡지 않는 의심스러운 코드를 검출한다:

```go
package main

import "fmt"

func main() {
    fmt.Printf("%d", "hello") // 포맷 인자 불일치
}
```

```bash
go vet ./...
# ./main.go:6:2: fmt.Printf format %d has arg "hello" of wrong type string
```

`go vet`이 잡는 대표적인 문제:

- `Printf` 포맷 문자열과 인자 타입 불일치
- 도달할 수 없는 코드
- 구조체 복사 시 lock 복사
- 잘못된 struct tag 문법
- 테스트 함수명 규칙 위반

`go vet`은 `go test`를 실행할 때 자동으로 함께 실행된다. 별도로 실행하지 않아도 테스트를 돌리면 vet 검사가 수행된다.

Node.js 대응은 ESLint의 기본 규칙이다. 차이점은 `go vet`이 별도 설치 없이 Go에 포함되어 있다는 것이다.

## staticcheck -- 사실상 표준 린터

`staticcheck`은 Go 커뮤니티에서 가장 널리 사용되는 서드파티 정적 분석 도구다. `go vet`보다 더 많은 규칙을 제공한다:

```bash
go install honnef.co/go/tools/cmd/staticcheck@latest
staticcheck ./...
```

검출 예시:

```go
func process(items []string) {
    for i, _ := range items { // SA4006: _ 불필요
        fmt.Println(i)
    }

    if err := doSomething(); err != nil {
        return
    }
    // S1023: 함수 마지막의 불필요한 return
    return
}
```

`staticcheck`의 규칙은 카테고리로 분류된다:

- **SA** -- 정적 분석. 버그 가능성이 높은 코드.
- **S** -- 단순화. 더 간결하게 쓸 수 있는 코드.
- **ST** -- 스타일. 관례에 맞지 않는 코드.
- **QF** -- 빠른 수정. 자동 수정 가능한 항목.

## golangci-lint -- 린터 통합 도구

여러 린터를 하나씩 실행하는 것은 번거롭다. `golangci-lint`는 수십 개의 린터를 통합 실행하는 도구다. Node.js에서 ESLint가 여러 플러그인을 하나로 묶어 실행하는 것과 같은 위치다:

```bash
go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest
golangci-lint run
```

기본 설정으로도 `go vet`, `staticcheck`, `errcheck` 등 핵심 린터가 활성화된다. `.golangci.yml` 파일로 설정을 조정한다:

```yaml
linters:
  enable:
    - govet
    - staticcheck
    - errcheck
    - gosimple
    - ineffassign
    - unused
    - gocritic

linters-settings:
  errcheck:
    check-type-assertions: true

issues:
  exclude-use-default: false
```

Node.js의 `.eslintrc`에 대응하는 설정 파일이다. 차이점은 Go의 린터 설정이 훨씬 단순하다는 것이다. ESLint에서 parser, plugin, extends, rules를 조합하는 복잡함이 없다. 린터 목록에서 활성화할 항목을 고르면 된다.

| Node.js | Go | 역할 |
|---|---|---|
| ESLint | golangci-lint | 린터 통합 실행 |
| ESLint 기본 규칙 | go vet | 내장 정적 분석 |
| eslint-plugin-* | staticcheck 등 | 추가 린터 |
| Prettier | gofmt | 코드 포매팅 |
| Prettier + ESLint | goimports + golangci-lint | 포매팅 + 린팅 |

Node.js에서는 ESLint와 Prettier의 충돌을 해결하기 위해 `eslint-config-prettier`를 설치하거나 `eslint-plugin-prettier`로 통합한다. Go에서는 포매팅(gofmt)과 린팅(golangci-lint)이 처음부터 독립적이라 충돌이 없다.

## CI에 린팅 통합

CI에서 린팅을 실행하여 코드 품질을 자동으로 검증할 수 있다. GitHub Actions 예시:

```yaml
name: lint
on: [push, pull_request]

jobs:
  lint:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-go@v5
        with:
          go-version: "1.24"
      - uses: golangci/golangci-lint-action@v6
        with:
          version: latest
```

Node.js 프로젝트에서 CI에 `npx eslint .`을 추가하는 것과 동일한 위치다. `golangci-lint`는 공식 GitHub Action을 제공하므로 설정이 간단하다.

디렉토리 구조, 코드 스타일, 포매팅 도구 모두 커뮤니티가 수렴한 답이 있다. 프로젝트마다 컨벤션을 새로 정하고 문서화하는 비용이 사라지고, 새로운 Go 프로젝트에 합류했을 때 구조를 파악하는 데 걸리는 시간이 짧다.
