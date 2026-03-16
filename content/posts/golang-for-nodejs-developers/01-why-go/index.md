# 왜 Go인가

Go가 해결하는 문제는 런타임 모델, 배포 방식, 타입 시스템, 컴파일 사이클 네 가지로 요약된다. 시리즈 전체에서 비교 대상은 TypeScript + Node.js 환경이다.

## 45분짜리 빌드가 낳은 언어

2007년 9월, Google 본사 Building 43. Rob Pike는 자신이 작업하던 대규모 C++ 프로그램의 빌드가 끝나기를 기다리고 있었다. 분산 컴파일 클러스터를 동원해도 빌드 한 번에 45분이 걸렸다. 코드를 한 줄 고치면 다시 45분. 그 사이에 할 수 있는 건 기다리는 것뿐이었다.

Pike는 옆자리에 앉아 있던 Robert Griesemer에게 의자를 돌려 말했다. "이 상황을 뭔가 해봐야 하지 않겠나." 옆 사무실에 있던 Ken Thompson을 불러왔고, 세 사람은 화이트보드 앞에 섰다. 2007년 9월 21일, 금요일 오후. Building 43의 Yaounde 회의실에서 새 언어의 목표를 스케치하기 시작했다.

세 사람의 이력은 이 언어의 방향을 설명한다. Thompson은 Unix와 C를 만든 사람이고, Pike는 Plan 9과 UTF-8을 Thompson과 함께 설계했다. Griesemer는 Java HotSpot VM의 컴파일러를 작업한 경험이 있었다. 시스템 프로그래밍, 동시성, 컴파일러 최적화 — 세 사람의 전문 분야가 하나의 언어에 수렴했다.

이들이 불만을 느낀 건 빌드 시간만이 아니었다. C++의 header include 모델은 4.2MB짜리 소스 파일 세트를 컴파일할 때 전처리 과정에서 8GB 이상의 데이터를 읽어 들였다. 약 2,000배의 팽창. 멀티코어 프로세서가 보편화되고 있었지만 C++과 Java에서 thread를 다루는 건 여전히 고통이었다. 구글 규모의 소프트웨어를 작성하기에 기존 언어는 부적합했다.

9월 25일, Pike가 퇴근길에 이메일을 보냈다. 언어 이름으로 "go"를 제안했다. 짧고, 타이핑하기 쉽고, `go.lang`처럼 확장할 수 있다는 이유였다. 처음에는 20% 프로젝트(근무 시간의 20%를 자유 프로젝트에 쓰는 Google의 정책)로 시작했고, 2008년 2월에 첫 Go 프로그램이 작성되었다. 2009년 11월에 오픈소스로 공개, 2012년 3월에 1.0이 릴리스되었다.

## 런타임 모델: 이벤트 루프 vs goroutine

Node.js의 핵심은 V8 엔진 위에서 돌아가는 single-threaded 이벤트 루프다. 비동기 I/O를 libuv에 위임하고, callback이나 Promise로 결과를 받는다. CPU-bound 작업이 이벤트 루프를 점유하면 전체 서버가 멈춘다. 이를 우회하려면 worker thread나 child process를 써야 한다.

```javascript
// Node.js: 비동기 I/O
const data = await fs.readFile("config.json", "utf-8");
const parsed = JSON.parse(data);
```

Go는 다르다. 컴파일된 네이티브 바이너리가 OS 위에서 직접 실행된다. V8도, libuv도, 이벤트 루프도 없다. 대신 goroutine이 있다.

```go
// Go: goroutine으로 동시 실행
go func() {
    data, err := os.ReadFile("config.json")
    if err != nil {
        log.Fatal(err)
    }
    fmt.Println(string(data))
}()
```

goroutine은 Go 런타임이 관리하는 경량 실행 단위다. OS thread가 아니다. 초기 스택 크기가 수 KB에 불과하고, 필요에 따라 동적으로 늘어난다. 수십만 개를 동시에 실행해도 문제없다. Go 런타임의 스케줄러가 goroutine을 OS thread에 매핑하여 멀티코어를 활용한다.

Node.js에서 동시성은 "하나의 thread가 I/O 대기 시간을 효율적으로 활용하는 것"이다. Go에서 동시성은 "여러 goroutine이 실제로 병렬 실행되는 것"이다. 멘탈 모델이 근본적으로 다르다.

## 배포: node_modules vs 단일 바이너리

Node.js 프로젝트를 배포하려면 다음이 필요하다:

- Node.js runtime
- package.json + package-lock.json
- node_modules 디렉토리 (또는 배포 시 `npm install`)
- 소스 코드 전체

Docker 이미지로 만들면 `node:slim` 기반으로도 200MB 이상이다. Alpine 기반으로 줄여도 180MB 근처. runtime과 소스 코드, 의존성을 모두 포함해야 하기 때문이다.

Go는 `go build`로 단일 바이너리를 만든다. 의존성이 모두 바이너리에 포함된다. 배포에 필요한 건 그 파일 하나뿐이다.

```dockerfile
# Go: multi-stage build
FROM golang:1.24 AS build
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -o server .

FROM scratch
COPY --from=build /app/server /server
ENTRYPOINT ["/server"]
```

`scratch`는 빈 이미지다. OS도 shell도 없다. 바이너리만 들어간다. 결과물은 10~15MB. Node.js 이미지의 1/20 수준이다.

cross-compilation도 간단하다. macOS에서 Linux 바이너리를 만들 수 있다:

```bash
GOOS=linux GOARCH=amd64 go build -o server
```

Node.js에서는 불가능한 일이다. native addon이 있으면 대상 OS에서 빌드해야 하고, 아키텍처마다 별도 환경이 필요하다.

## 타입 시스템: TypeScript의 선택 vs Go의 기본값

TypeScript는 선택적이다. `any`를 남용할 수 있고, `@ts-ignore`로 검사를 끌 수 있다. 런타임에는 타입 정보가 사라진다. JavaScript로 트랜스파일된 뒤 실행되기 때문이다.

```typescript
// TypeScript: 컴파일 타임에만 존재하는 타입
interface User {
  id: number;
  name: string;
}

function greet(user: User): string {
  return `Hello, ${user.name}`;
}

// 런타임에 user가 User인지 검증하지 않는다
```

Go의 타입 시스템은 언어에 내장되어 있다. 별도 도구 없이 `go build`만으로 타입 검사가 이루어진다. `any`에 해당하는 `interface{}`가 있지만, 사용하면 타입 단언(type assertion)을 강제하므로 남용하기 어렵다.

```go
type User struct {
    ID   int
    Name string
}

func greet(user User) string {
    return "Hello, " + user.Name
}
```

TypeScript의 타입이 "문서화 도구에 가까운 것"이라면, Go의 타입은 "컴파일러가 강제하는 계약"이다. 타입이 맞지 않으면 바이너리가 만들어지지 않는다.

구조적 타이핑(structural typing)은 양쪽 모두 지원한다. TypeScript에서 같은 shape이면 호환되듯, Go에서도 interface를 구현하는 데 `implements` 키워드가 필요 없다. 메서드 시그니처가 일치하면 자동으로 만족한다.

```go
type Reader interface {
    Read(p []byte) (n int, err error)
}

// os.File은 Read 메서드가 있으므로 Reader를 만족한다
// 명시적 선언 없이.
```

다만 Go의 타입 시스템은 TypeScript보다 표현력이 낮다. union type, mapped type, conditional type 같은 건 없다. Go는 단순함을 선택했다. 이 트레이드오프는 시리즈 전체에 걸쳐 반복적으로 등장할 주제다.

## 컴파일: guardrail로서의 컴파일러

순수 JavaScript의 개발 사이클은 edit-run이다. 파일을 수정하고 `node app.js`를 실행한다. 틀린 코드도 해당 경로가 실행되기 전까지는 문제가 드러나지 않는다. TypeScript를 쓰면 edit-compile-run에 가까워지지만, 타입 에러가 있어도 JavaScript로 emit되는 경우가 많고(`noEmitOnError` 설정에 의존), 런타임 검사는 여전히 없다.

Go의 개발 사이클은 edit-compile-run이다. `go run main.go`를 실행하면 내부적으로 컴파일이 먼저 일어난다. 컴파일이 실패하면 실행되지 않는다.

**사용하지 않는 import는 컴파일 에러다:**

```go
import "fmt" // fmt를 사용하지 않으면 컴파일 에러

func main() {
}
```

```
imported and not used: "fmt"
```

**사용하지 않는 변수도 컴파일 에러다:**

```go
func main() {
    x := 42 // x를 사용하지 않으면 컴파일 에러
}
```

```
x declared and not used
```

ESLint rule로 잡던 것들이 컴파일러 레벨에서 강제된다. TypeScript의 `noUnusedLocals`, `noUnusedParameters`가 비슷한 역할을 하지만, Go는 이것이 끌 수 없는 기본값이다.

REPL은 없다. `node`를 입력하고 한 줄씩 실행하는 환경이 Go에는 기본 제공되지 않는다. 대신 Go Playground(https://go.dev/play/)가 있고, 로컬에서는 짧은 코드를 `main.go`에 작성하고 `go run main.go`로 확인하는 것이 일반적이다.

컴파일 속도는 빠르다. Go가 해결하려 했던 문제가 C++의 느린 빌드였으니 당연하다. import 모델이 C++의 header include와 근본적으로 다르다. Go 컴파일러는 각 package를 한 번만 컴파일하고, 의존성 정보를 object 파일 앞부분에 배치하여 필요한 부분만 읽는다. C++에서 2,000배로 팽창하던 의존성 처리가 Go에서는 약 40배 수준으로 억제된다.

중간 규모 프로젝트라면 `go build`가 1~2초 안에 끝난다. edit-compile-run 사이클이 edit-run과 체감상 크게 다르지 않다.

Go는 "더 적은 것으로 더 많은 일을 한다"는 철학을 일관되게 따르는 언어다. 기능이 적고, 문법이 단순하고, 선택지가 제한된다. 이 제약이 처음에는 답답하게 느껴질 수 있지만, 그 제약이 바로 Go의 강점이다.
