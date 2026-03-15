# 테스트

Node.js에서 테스트를 작성하려면 Jest, Mocha, Vitest 같은 프레임워크를 선택하고 설치해야 한다. Go는 테스트 도구가 언어에 내장되어 있다. `go test` 한 줄이면 된다. 별도의 프레임워크도, 설정 파일도, assert 라이브러리도 필요 없다. 테스트 파일 관례부터 fuzzing까지 Go의 테스트 시스템 전체를 다룬다.

## _test.go 파일 관례

Go에서 테스트 파일은 `_test.go`로 끝나야 한다. 이것은 관례가 아니라 도구가 강제하는 규칙이다. `go build`는 `_test.go` 파일을 무시하고, `go test`만 이 파일을 포함한다:

```
math/
  calc.go          # 프로덕션 코드
  calc_test.go     # 테스트 코드
```

같은 패키지 안에 테스트 파일을 함께 둔다. Node.js처럼 `__tests__/` 디렉토리를 따로 만들 필요가 없다. 테스트 대상과 테스트 코드가 같은 디렉토리에 있으므로 탐색이 쉽다.

```javascript
// Node.js 프로젝트 구조 (프레임워크마다 다름)
// src/math/calc.js
// src/math/__tests__/calc.test.js
// 또는
// src/math/calc.test.js
```

Go는 `_test.go` 하나로 통일된다. 프레임워크마다 다른 파일 패턴을 외울 필요가 없다.

## testing.T 기본

테스트 함수는 `Test`로 시작하고, `*testing.T`를 인자로 받는다:

```go
// calc.go
package math

func Add(a, b int) int {
    return a + b
}
```

```go
// calc_test.go
package math

import "testing"

func TestAdd(t *testing.T) {
    got := Add(2, 3)
    want := 5
    if got != want {
        t.Errorf("Add(2, 3) = %d, want %d", got, want)
    }
}
```

`go test`로 실행한다:

```
$ go test
PASS
ok      example.com/math    0.001s
```

Jest의 `expect(x).toBe(y)`에 해당하는 것이 `if`와 `t.Errorf`다:

```javascript
// Jest
test("add", () => {
  expect(add(2, 3)).toBe(5);
});
```

Go에는 `assert` 함수가 없다. 의도적인 설계다. `if`문이면 충분하고, 실패 메시지를 직접 작성하면 디버깅할 때 더 유용한 정보를 담을 수 있다.

`t.Errorf`는 실패를 기록하되 테스트를 계속 진행한다. 즉시 중단하려면 `t.Fatalf`를 쓴다:

```go
func TestDivide(t *testing.T) {
    result, err := Divide(10, 0)
    if err == nil {
        t.Fatal("expected error for division by zero")
    }
    // t.Fatal 이후 코드는 실행되지 않는다

    result, err = Divide(10, 2)
    if err != nil {
        t.Fatalf("unexpected error: %v", err)
    }
    if result != 5 {
        t.Errorf("Divide(10, 2) = %d, want 5", result)
    }
}
```

## 테이블 드리븐 테스트

Go에서 가장 흔한 테스트 패턴이다. 여러 입력과 기대값을 슬라이스로 정의하고, 반복문으로 순회한다:

```go
func TestAdd(t *testing.T) {
    tests := []struct {
        name string
        a, b int
        want int
    }{
        {"positive", 2, 3, 5},
        {"zero", 0, 0, 0},
        {"negative", -1, -2, -3},
        {"mixed", -1, 5, 4},
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            got := Add(tt.a, tt.b)
            if got != tt.want {
                t.Errorf("Add(%d, %d) = %d, want %d", tt.a, tt.b, got, tt.want)
            }
        })
    }
}
```

Jest의 `describe`/`it` 구조와 비교하면:

```javascript
// Jest
describe("add", () => {
  it.each([
    [2, 3, 5],
    [0, 0, 0],
    [-1, -2, -3],
    [-1, 5, 4],
  ])("add(%i, %i) = %i", (a, b, expected) => {
    expect(add(a, b)).toBe(expected);
  });
});
```

테이블 드리븐 테스트의 장점:

1. **케이스 추가가 쉽다.** 슬라이스에 한 줄만 추가하면 된다.
2. **반복 코드가 없다.** 검증 로직을 한 번만 작성한다.
3. **실패 시 어떤 케이스인지 명확하다.** `t.Run`의 이름이 출력된다.

실행하면 각 subtest가 독립적으로 보고된다:

```
$ go test -v
=== RUN   TestAdd
=== RUN   TestAdd/positive
=== RUN   TestAdd/zero
=== RUN   TestAdd/negative
=== RUN   TestAdd/mixed
--- PASS: TestAdd (0.00s)
    --- PASS: TestAdd/positive (0.00s)
    --- PASS: TestAdd/zero (0.00s)
    --- PASS: TestAdd/negative (0.00s)
    --- PASS: TestAdd/mixed (0.00s)
PASS
```

## Subtests — t.Run

위에서 이미 `t.Run`을 사용했다. `t.Run`은 테이블 드리븐 테스트뿐 아니라 테스트를 논리적으로 그룹화할 때도 쓴다:

```go
func TestUser(t *testing.T) {
    t.Run("Create", func(t *testing.T) {
        // 사용자 생성 테스트
    })

    t.Run("Update", func(t *testing.T) {
        // 사용자 수정 테스트
    })

    t.Run("Delete", func(t *testing.T) {
        // 사용자 삭제 테스트
    })
}
```

특정 subtest만 실행할 수도 있다:

```
$ go test -run TestUser/Create
```

Jest의 `describe` 중첩과 비슷한 역할이다. 하지만 Go에서는 깊은 중첩보다 평탄한 구조를 선호한다.

## 테스트 헬퍼 — t.Helper()

테스트 유틸리티 함수에서 `t.Helper()`를 호출하면, 실패 시 헬퍼 함수가 아닌 호출한 쪽의 파일명과 줄 번호가 출력된다:

```go
func assertEqual(t *testing.T, got, want int) {
    t.Helper()
    if got != want {
        t.Errorf("got %d, want %d", got, want)
    }
}

func TestAdd(t *testing.T) {
    assertEqual(t, Add(2, 3), 5)  // 실패 시 이 줄이 보고된다
    assertEqual(t, Add(0, 0), 0)
}
```

`t.Helper()` 없이 실행하면 실패 위치가 `assertEqual` 함수 내부로 표시된다. 디버깅할 때 실제 테스트 코드가 어디서 실패했는지 찾기 어려워진다. 테스트 헬퍼를 작성할 때는 항상 `t.Helper()`를 첫 줄에 넣는다.

## Benchmark

`testing.B`를 사용하여 성능을 측정한다. 함수 이름이 `Benchmark`로 시작해야 한다:

```go
func BenchmarkAdd(b *testing.B) {
    for b.Loop() {
        Add(2, 3)
    }
}
```

`b.Loop()`은 Go 1.24에서 추가된 방식이다. 프레임워크가 반복 횟수를 자동으로 조절하여 안정적인 측정값을 만든다. 이전 버전에서는 `for i := 0; i < b.N; i++` 패턴을 사용했다.

```
$ go test -bench=.
goos: darwin
goarch: arm64
pkg: example.com/math
cpu: Apple M1
BenchmarkAdd-8    1000000000    0.2500 ns/op
PASS
```

`-bench=.`은 모든 벤치마크를 실행한다. `-benchmem`을 추가하면 메모리 할당 정보도 출력된다:

```
$ go test -bench=. -benchmem
BenchmarkAdd-8    1000000000    0.2500 ns/op    0 B/op    0 allocs/op
```

Node.js에서 벤치마크를 하려면 `benchmark.js` 같은 외부 패키지를 설치해야 한다. Go는 표준 도구에 포함되어 있다.

## Fuzzing

Go 1.18부터 fuzz testing이 내장되었다. 무작위 입력을 자동 생성하여 예상치 못한 엣지 케이스를 찾는다:

```go
func Reverse(s string) string {
    runes := []rune(s)
    for i, j := 0, len(runes)-1; i < j; i, j = i+1, j-1 {
        runes[i], runes[j] = runes[j], runes[i]
    }
    return string(runes)
}
```

```go
func FuzzReverse(f *testing.F) {
    // seed corpus: 초기 입력값
    f.Add("hello")
    f.Add("world")
    f.Add("")

    f.Fuzz(func(t *testing.T, s string) {
        rev := Reverse(s)
        doubleRev := Reverse(rev)
        if s != doubleRev {
            t.Errorf("double reverse of %q = %q", s, doubleRev)
        }
    })
}
```

일반 테스트처럼 실행하면 seed corpus만 검사한다. `-fuzz` 플래그를 주면 무작위 입력을 계속 생성한다:

```
$ go test -fuzz=FuzzReverse
fuzz: elapsed: 0s, gathering baseline coverage: 0/3 completed
fuzz: elapsed: 0s, gathering baseline coverage: 3/3 completed, now fuzzing
fuzz: elapsed: 3s, execs: 325017 (108336/sec), new interesting: 0
^C
```

크래시를 발견하면 `testdata/fuzz/` 디렉토리에 실패 입력을 저장한다. 이후 일반 `go test`에서도 이 입력이 자동으로 포함된다.

Node.js에는 내장 fuzzer가 없다. `fast-check` 같은 property-based testing 라이브러리가 비슷한 역할을 한다.

## testdata 디렉토리

테스트에 필요한 파일(JSON, 텍스트, 바이너리 등)은 `testdata/` 디렉토리에 둔다:

```
parser/
  parser.go
  parser_test.go
  testdata/
    input.json
    expected.json
```

`testdata`는 Go 도구가 인식하는 특별한 이름이다. `go build`와 `go test`가 이 디렉토리를 패키지로 취급하지 않는다. 테스트 코드에서 상대 경로로 접근한다:

```go
func TestParse(t *testing.T) {
    input, err := os.ReadFile("testdata/input.json")
    if err != nil {
        t.Fatal(err)
    }

    expected, err := os.ReadFile("testdata/expected.json")
    if err != nil {
        t.Fatal(err)
    }

    got := Parse(input)
    if string(got) != string(expected) {
        t.Errorf("output mismatch")
    }
}
```

`go test`는 테스트 파일이 있는 디렉토리를 working directory로 설정하므로 `"testdata/..."`처럼 상대 경로를 쓸 수 있다.

앞서 다룬 fuzzing의 실패 입력도 `testdata/fuzz/` 아래에 저장된다. `testdata`는 테스트 관련 파일의 표준 위치다.

## go test 명령어

기본 사용법:

```
$ go test              # 현재 패키지 테스트
$ go test ./...        # 모든 하위 패키지 테스트
$ go test -v           # 각 테스트 함수명과 결과 출력
$ go test -run TestAdd # 이름이 매칭되는 테스트만 실행
$ go test -count=1     # 캐시 무시하고 강제 실행
```

`go test ./...`는 프로젝트 전체를 테스트하는 관용적 명령이다. Node.js에서 `npm test`가 하는 역할과 같다.

`-race` 플래그는 race condition detector를 활성화한다. 14편에서 다뤘던 동시성 버그를 테스트 단계에서 잡을 수 있다:

```
$ go test -race ./...
```

CI에서 `-race`를 기본으로 켜두는 것이 권장된다.

## 정리

| 개념 | Node.js (Jest/Vitest) | Go |
|---|---|---|
| 테스트 프레임워크 | Jest, Mocha, Vitest 등 선택 | `go test` (내장) |
| 테스트 파일 | `*.test.js`, `*.spec.js` | `*_test.go` |
| assertion | `expect(x).toBe(y)` | `if got != want { t.Errorf(...) }` |
| 테스트 그룹 | `describe` / `it` | 테이블 드리븐 + `t.Run` |
| setup/teardown | `beforeEach` / `afterEach` | `t.Cleanup`, 직접 호출 |
| 벤치마크 | `benchmark.js` 등 외부 | `testing.B` (내장) |
| fuzzing | `fast-check` 등 외부 | `testing.F` (내장, Go 1.18+) |
| 테스트 데이터 | 자유 배치 | `testdata/` 관례 |
| 전체 실행 | `npm test` | `go test ./...` |
| race 감지 | 해당 없음 (싱글 스레드) | `go test -race` |

Go의 테스트 도구는 "하나의 표준 방법"이라는 Go의 철학을 그대로 반영한다. 프레임워크 선택, 설정 파일, 플러그인 조합 같은 의사결정이 필요 없다. `_test.go` 파일을 만들고, `Test`로 시작하는 함수를 작성하고, `go test`를 실행한다. 이 단순함이 Go 테스트의 핵심이다.
