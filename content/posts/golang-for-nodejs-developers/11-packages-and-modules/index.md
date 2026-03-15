# 패키지와 모듈

Go의 코드 구성과 배포 방식을 살펴본다. npm과 근본적으로 다르다. 중앙 레지스트리가 없고, VCS 경로가 곧 패키지명이며, 표준 라이브러리를 먼저 쓰는 문화가 지배적이다.

## 패키지 = 디렉토리

Node.js에서 모듈은 파일 단위다. 파일 하나가 모듈 하나다:

```javascript
// utils.js
export function add(a, b) { return a + b; }

// main.js
import { add } from "./utils.js";
```

Go에서 패키지는 디렉토리 단위다. 같은 디렉토리에 있는 모든 `.go` 파일은 같은 패키지에 속한다:

```
myapp/
  math/
    add.go      // package math
    multiply.go // package math
  main.go       // package main
```

`add.go`와 `multiply.go`는 같은 `math` 패키지다. 서로의 함수나 변수를 import 없이 바로 쓸 수 있다. 파일 경계가 의미 없다. 파일을 어떻게 나누든 컴파일러 입장에서는 하나의 패키지다.

```go
// math/add.go
package math

func Add(a, b int) int {
    return a + b
}

// math/multiply.go
package math

func Multiply(a, b int) int {
    return a * b
}

// main.go
package main

import "myapp/math"

func main() {
    fmt.Println(math.Add(1, 2))
    fmt.Println(math.Multiply(3, 4))
}
```

디렉토리 안의 모든 `.go` 파일은 동일한 `package` 선언을 가져야 한다. `add.go`가 `package math`인데 `multiply.go`가 `package util`이면 컴파일 에러다.

## visibility: 대문자와 소문자

Node.js에서 모듈 밖으로 내보내려면 `export` 키워드를 쓴다:

```javascript
export function publicFunc() { /* ... */ }
function privateFunc() { /* ... */ }
```

Go에는 `export` 키워드가 없다. 대신 이름의 첫 글자가 대문자면 exported(공개), 소문자면 unexported(비공개)다:

```go
package user

func Validate(name string) bool { // 대문자 V: exported
    return len(name) > 0
}

func sanitize(name string) string { // 소문자 s: unexported
    return strings.TrimSpace(name)
}

type User struct {     // 대문자 U: exported
    Name  string       // 대문자 N: exported
    email string       // 소문자 e: unexported
}
```

다른 패키지에서 `user.Validate()`는 호출할 수 있지만 `user.sanitize()`는 컴파일 에러다. `User` struct의 `Name` 필드는 접근 가능하지만 `email` 필드는 접근 불가다.

이 규칙은 함수, 타입, 변수, 상수, struct 필드, 메서드 등 모든 식별자에 동일하게 적용된다. 코드를 읽는 것만으로 visibility를 즉시 파악할 수 있다는 점이 장점이다. 별도의 선언을 찾아볼 필요가 없다.

## go.mod와 go.sum

Node.js 프로젝트는 `package.json`과 `package-lock.json`으로 의존성을 관리한다. Go는 `go.mod`와 `go.sum`을 쓴다.

```
go mod init github.com/user/myapp
```

이 명령이 `go.mod`를 생성한다:

```
module github.com/user/myapp

go 1.24
```

`package.json`과 비교하면:

```javascript
// package.json (참고)
{
  "name": "myapp",
  "dependencies": {
    "express": "^4.18.0"
  }
}
```

```
// go.mod
module github.com/user/myapp

go 1.24

require (
    github.com/gin-gonic/gin v1.9.1
)
```

차이점이 있다. `package.json`의 `name`은 아무 문자열이나 가능하지만, `go.mod`의 module 경로는 실제 VCS 저장소 경로다. `github.com/user/myapp`이라는 모듈명은 곧 이 코드가 GitHub의 해당 경로에 호스팅된다는 의미다.

`go.sum`은 `package-lock.json`과 비슷한 역할이다. 다운로드한 모듈의 cryptographic hash를 저장하여 무결성을 검증한다. `go.sum`은 직접 편집하지 않는다.

## 의존성 관리

### 패키지 추가

```
npm install express
```

Go에서는:

```
go get github.com/gin-gonic/gin
```

`go get`은 모듈을 다운로드하고 `go.mod`에 추가한다. npm과 달리 `node_modules` 같은 로컬 디렉토리에 설치하지 않는다. 모듈 캐시(`$GOPATH/pkg/mod`)에 전역으로 저장되고, 모든 프로젝트가 공유한다.

### 사용하지 않는 의존성 정리

```
npm prune
```

Go에서는:

```
go mod tidy
```

`go mod tidy`는 코드에서 실제로 import하는 모듈만 `go.mod`에 남기고 나머지를 제거한다. 반대로, 코드에서 import하고 있지만 `go.mod`에 없는 모듈은 자동으로 추가한다. Go 프로젝트에서 가장 자주 실행하는 명령 중 하나다.

Go 컴파일러는 사용하지 않는 import를 컴파일 에러로 처리한다. JavaScript에서 `import express from "express"`를 써놓고 사용하지 않아도 아무 문제 없지만, Go에서 `import "fmt"`를 써놓고 fmt를 사용하지 않으면 빌드가 실패한다. 이 엄격함이 `go mod tidy`와 함께 dead dependency를 원천 차단한다.

## VCS 기반 모듈 시스템

npm에는 npmjs.com이라는 중앙 레지스트리가 있다. 패키지를 publish하면 레지스트리에 등록되고, 다른 사람이 `npm install`로 가져간다.

Go에는 중앙 레지스트리가 없다. 모듈의 import 경로가 곧 소스 코드의 위치다:

```go
import "github.com/gorilla/mux"
```

`go get`을 실행하면 Go 도구 체인이 `github.com/gorilla/mux`로 직접 가서 소스 코드를 가져온다. GitHub, GitLab, Bitbucket 등 어떤 VCS 호스팅이든 가능하다. 별도의 publish 절차가 없다. git tag를 push하면 그것이 곧 릴리스다.

```
git tag v1.2.0
git push origin v1.2.0
```

이것만으로 `v1.2.0` 버전이 배포된다. `npm publish` 같은 별도의 명령이 필요 없다.

### 버전 규칙

Go 모듈은 semantic versioning을 따른다. 특이한 규칙이 하나 있는데, major version이 2 이상이면 import 경로에 major version이 포함된다:

```go
import "github.com/user/lib"    // v0.x.x 또는 v1.x.x
import "github.com/user/lib/v2" // v2.x.x
import "github.com/user/lib/v3" // v3.x.x
```

major version이 다르면 다른 패키지로 취급된다. 같은 프로젝트에서 v1과 v2를 동시에 import하는 것도 가능하다. npm에서 major version 충돌로 고생하는 문제가 구조적으로 해결된다.

## pkg.go.dev와 proxy.golang.org

중앙 레지스트리는 없지만 검색과 문서화를 위한 인프라는 있다.

**pkg.go.dev**는 Go 패키지의 문서 검색 사이트다. npmjs.com에서 패키지를 검색하듯이 pkg.go.dev에서 Go 패키지를 검색한다. 소스 코드의 주석에서 자동으로 문서를 생성한다.

**proxy.golang.org**는 Go 모듈 프록시다. `go get`은 기본적으로 소스 저장소에 직접 가지 않고 이 프록시를 거친다. 프록시가 하는 일:

- 모듈을 캐싱하여 원본 저장소가 다운되어도 가져올 수 있다
- 한번 publish된 버전이 삭제되지 않도록 보장한다
- 다운로드 속도를 개선한다

npm에서 패키지 maintainer가 패키지를 unpublish해서 전 세계 빌드가 깨지는 사고가 있었다. left-pad 사건이 대표적이다. Go 모듈 프록시는 이런 문제를 구조적으로 방지한다. 원본 저장소에서 tag를 삭제해도 프록시에 캐싱된 버전은 계속 사용 가능하다.

**sum.golang.org**는 체크섬 데이터베이스다. 모듈의 hash를 투명하게 기록하여 중간자 공격이나 모듈 변조를 탐지한다.

## go install — npx 대응

npm에서 CLI 도구를 실행하는 방법:

```
npx create-react-app my-app
```

Go에서:

```
go install golang.org/x/tools/cmd/goimports@latest
goimports -w .
```

`go install`은 Go로 작성된 CLI 도구를 빌드하고 `$GOPATH/bin`(기본값 `~/go/bin`)에 설치한다. npx가 매번 다운로드하고 실행하는 것과 달리, `go install`은 바이너리를 설치하므로 이후 실행이 빠르다.

`@latest`로 최신 버전을, `@v1.2.0`으로 특정 버전을 지정할 수 있다:

```
go install github.com/golangci/golangci-lint/cmd/golangci-lint@v1.62.0
```

## vendoring

Go 모듈은 기본적으로 모듈 캐시에 저장되지만, 프로젝트 안에 의존성을 복사해둘 수도 있다:

```
go mod vendor
```

이 명령은 `vendor/` 디렉토리를 생성하고 모든 의존성의 소스 코드를 복사한다. Node.js의 `node_modules`를 git에 커밋하는 것과 비슷한 개념이다(물론 Node.js에서는 거의 하지 않는다).

vendoring이 유용한 상황:

- 빌드 재현성을 완벽하게 보장해야 할 때
- CI 환경에서 외부 네트워크 접근 없이 빌드해야 할 때
- 의존성 코드를 감사(audit)해야 할 때

proxy.golang.org가 가용성을 보장하므로 대부분의 프로젝트에서 vendoring은 불필요하다. 하지만 엔터프라이즈 환경이나 보안 요구사항이 높은 프로젝트에서는 여전히 사용된다.

## stdlib-first 문화

Go 커뮤니티와 npm 생태계의 가장 큰 문화적 차이다.

npm 생태계에서는 작은 기능 하나에도 패키지를 만든다. 문자열 왼쪽에 패딩을 추가하는 11줄짜리 `left-pad`가 주간 수백만 다운로드를 기록했다. 2016년 maintainer가 이 패키지를 unpublish했을 때, React와 Babel을 포함한 수천 개의 프로젝트 빌드가 깨졌다.

Go는 표준 라이브러리를 먼저 쓰는 문화다. 표준 라이브러리가 넓고 실용적이기 때문이다:

| 영역 | Node.js | Go 표준 라이브러리 |
|---|---|---|
| HTTP 서버 | express, fastify 등 | `net/http` |
| JSON 처리 | 내장 | `encoding/json` |
| 테스트 | jest, mocha 등 | `testing` |
| 암호화 | crypto (내장) | `crypto` |
| 템플릿 | ejs, handlebars 등 | `html/template` |
| CLI 플래그 | commander, yargs 등 | `flag` |
| 로깅 | winston, pino 등 | `log/slog` |

Node.js에서 HTTP 서버를 만들 때 express 없이 시작하는 사람은 드물다. Go에서 HTTP 서버를 만들 때 표준 라이브러리의 `net/http`만으로 시작하는 사람은 많다. Go 1.22에서 `net/http`의 라우팅 기능이 강화되면서 단순한 API 서버라면 외부 라우터 없이도 충분해졌다.

이 차이의 배경은 Go 표준 라이브러리가 Go 릴리스 주기에 맞춰 업데이트되고, Go 팀이 하위 호환성을 강하게 보장하기 때문이다. Go 1.0에서 작성한 코드는 Go 1.24에서도 컴파일된다. 이런 안정성이 있으므로 표준 라이브러리에 의존하는 것이 외부 패키지에 의존하는 것보다 위험이 낮다.

물론 Go에서도 외부 패키지를 쓴다. 데이터베이스 드라이버(`pgx`), 웹 프레임워크(`gin`, `echo`), ORM(`ent`, `sqlc`) 등은 표준 라이브러리로 대체하기 어렵다. 핵심은 "외부 패키지를 추가하기 전에 표준 라이브러리로 해결할 수 있는지 먼저 확인한다"는 접근 태도다.

## 내부 패키지

Go에는 `internal` 패키지라는 접근 제한 메커니즘이 있다. 디렉토리 이름이 `internal`이면 해당 패키지는 부모 디렉토리의 하위에서만 import할 수 있다:

```
myapp/
  internal/
    auth/       // myapp 내부에서만 import 가능
      token.go
  api/
    handler.go  // import "myapp/internal/auth" OK
  cmd/
    main.go     // import "myapp/internal/auth" OK
```

다른 모듈에서 `import "myapp/internal/auth"`를 시도하면 컴파일 에러다. npm에는 이런 메커니즘이 없다. `package.json`의 `exports` 필드로 진입점을 제한할 수 있지만, Node.js 런타임이 강제하지는 않는다.

`internal` 패키지는 라이브러리를 만들 때 특히 유용하다. 공개 API와 내부 구현을 명확하게 분리할 수 있다.

Go의 모듈 시스템은 npm보다 단순하다. 중앙 레지스트리 없이 VCS 경로만으로 동작하고, 버전 관리는 git tag로 해결한다. 이 단순함 위에 프록시와 체크섬 데이터베이스가 가용성과 보안을 보장하고, stdlib-first 문화가 의존성 트리를 얕게 유지한다.
