# 디버깅

Node.js에서는 Chrome DevTools나 `--inspect` 플래그로 디버깅한다. 브라우저 기반 DevTools에 익숙하다면 Go의 디버깅 방식이 낯설 수 있다. Go에는 delve라는 전용 디버거가 있다. breakpoint, 변수 검사, step 실행은 물론 goroutine 상태까지 확인할 수 있다. Node.js 디버깅 경험과 비교하며 Go의 디버깅 도구를 다룬다.

## delve 설치

delve는 Go 전용 디버거다. GDB도 Go를 지원하지만, goroutine이나 Go 런타임을 제대로 이해하지 못한다. delve는 Go를 위해 만들어졌다:

```
$ go install github.com/go-delve/delve/cmd/dlv@latest
$ dlv version
Delve Debugger
Version: 1.24.1
```

Node.js는 V8 inspector가 내장되어 있지만, Go는 delve를 별도로 설치해야 한다.

## dlv debug — 기본 사용법

간단한 프로그램으로 시작한다:

```go
// main.go
package main

import "fmt"

func fibonacci(n int) int {
    if n <= 1 {
        return n
    }
    return fibonacci(n-1) + fibonacci(n-2)
}

func main() {
    for i := 0; i < 10; i++ {
        fmt.Println(fibonacci(i))
    }
}
```

`dlv debug`로 디버깅 세션을 시작한다:

```
$ dlv debug main.go
Type 'help' for list of commands.
(dlv)
```

Node.js의 `node --inspect-brk main.js`에 해당한다. `--inspect-brk`는 첫 줄에서 멈추고, `dlv debug`는 프로그램을 컴파일한 뒤 진입점에서 대기한다.

기본 명령어:

```
(dlv) break main.fibonacci    # breakpoint 설정
Breakpoint 1 set at 0x104a2e0 for main.fibonacci() ./main.go:6

(dlv) continue                # breakpoint까지 실행
> main.fibonacci() ./main.go:6 (hits goroutine(1):1 total:1)
     5:
=>   6: func fibonacci(n int) int {
     7:     if n <= 1 {

(dlv) print n                 # 변수 값 확인
0

(dlv) next                    # 다음 줄로 이동 (step over)
(dlv) step                    # 함수 내부로 진입 (step in)
(dlv) stepout                 # 현재 함수에서 빠져나옴 (step out)
(dlv) continue                # 다음 breakpoint까지 실행
(dlv) quit                    # 디버거 종료
```

Chrome DevTools의 버튼과 대응시키면:

| Chrome DevTools | delve | 단축키 |
|---|---|---|
| Resume (F8) | `continue` | `c` |
| Step over (F10) | `next` | `n` |
| Step into (F11) | `step` | `s` |
| Step out (Shift+F11) | `stepout` | `so` |

## breakpoint 다루기

줄 번호로 breakpoint를 설정할 수 있다:

```
(dlv) break main.go:8         # 파일명:줄번호
Breakpoint 1 set at 0x104a2f0 for main.fibonacci() ./main.go:8

(dlv) breakpoints             # 설정된 breakpoint 목록
Breakpoint 1 at 0x104a2f0 for main.fibonacci() ./main.go:8

(dlv) clear 1                 # breakpoint 제거
Breakpoint 1 cleared at 0x104a2f0 for main.fibonacci() ./main.go:8

(dlv) clearall                # 모든 breakpoint 제거
```

### conditional breakpoint

특정 조건에서만 멈추도록 설정할 수 있다:

```
(dlv) break main.go:8
Breakpoint 1 set at 0x104a2f0 for main.fibonacci() ./main.go:8

(dlv) condition 1 n == 5      # n이 5일 때만 멈춤
(dlv) continue
> main.fibonacci() ./main.go:8 (hits goroutine(1):1 total:1)

(dlv) print n
5
```

Node.js에서 Chrome DevTools의 "Edit breakpoint" > "Add conditional breakpoint"와 같은 기능이다. 반복문이나 재귀 함수에서 특정 조건을 추적할 때 유용하다.

## 변수 검사

`print`(줄여서 `p`)로 변수를 확인한다:

```
(dlv) print n                 # 변수 값
5

(dlv) print n * 2             # 표현식 평가
10

(dlv) locals                  # 현재 스코프의 모든 지역 변수
n = 5

(dlv) args                    # 현재 함수의 인자
n = 5

(dlv) whatis n                # 변수 타입
int
```

struct나 slice도 확인할 수 있다:

```
(dlv) print user
main.User {
    ID:   1,
    Name: "Alice",
    Tags: []string len: 2, cap: 2, ["admin","editor"],
}

(dlv) print user.Name
"Alice"

(dlv) print user.Tags[0]
"admin"
```

Chrome DevTools의 Scope 패널에서 변수를 펼쳐보는 것과 같다. 차이점은 delve에서는 CLI로 직접 타이핑해야 한다는 것이다. IDE를 사용하면 이 차이가 사라진다.

## IDE 연동

실제 개발에서는 IDE 연동이 훨씬 편하다.

### VS Code

Go 확장(공식 `golang.go`)을 설치하면 delve가 자동으로 연동된다. `.vscode/launch.json` 설정:

```json
{
  "version": "0.2.0",
  "configurations": [
    {
      "name": "Launch",
      "type": "go",
      "request": "launch",
      "mode": "debug",
      "program": "${workspaceFolder}"
    }
  ]
}
```

이후 사용법은 Node.js 디버깅과 거의 동일하다:

1. 에디터 왼쪽 여백을 클릭하여 breakpoint 설정
2. F5로 디버깅 시작
3. 변수 패널에서 값 확인
4. call stack 패널에서 호출 스택 확인
5. F10(step over), F11(step in), Shift+F11(step out)

launch.json의 `type`이 `"node"`에서 `"go"`로 바뀌는 것 정도가 차이다.

### GoLand

GoLand(JetBrains)는 delve를 내장하고 있어 별도 설정이 필요 없다. 함수 옆의 실행 버튼에서 "Debug"를 선택하면 된다. breakpoint, 변수 검사, evaluate expression 등 모든 기능이 GUI로 제공된다.

### 테스트 디버깅

VS Code에서 테스트 함수 위에 나타나는 "debug test" 링크를 클릭하면 해당 테스트만 디버깅 모드로 실행된다. CLI에서는:

```
$ dlv test -- -run TestFibonacci
```

`dlv test`는 `go test`와 같은 방식으로 테스트를 컴파일하되, 디버거를 붙여서 실행한다. Node.js에서 Jest를 `--inspect`로 실행하는 것에 해당한다.

## goroutine 디버깅

Go 디버깅에서 Node.js와 가장 크게 다른 부분이다. Node.js는 싱글 스레드이므로 한 시점에 하나의 실행 흐름만 추적하면 된다. Go는 수십, 수백 개의 goroutine이 동시에 실행될 수 있다.

```go
package main

import (
    "fmt"
    "sync"
)

func worker(id int, wg *sync.WaitGroup) {
    defer wg.Done()
    fmt.Printf("worker %d start\n", id)
    // 작업 수행
    fmt.Printf("worker %d done\n", id)
}

func main() {
    var wg sync.WaitGroup
    for i := 0; i < 5; i++ {
        wg.Add(1)
        go worker(i, &wg)
    }
    wg.Wait()
}
```

delve에서 goroutine 관련 명령:

```
(dlv) goroutines               # 모든 goroutine 목록
* Goroutine 1 - User: ./main.go:18 main.main (0x104a3a0)
  Goroutine 2 - User: runtime/proc.go:400 runtime.gopark (0x103e5c0)
  Goroutine 3 - User: ./main.go:10 main.worker (0x104a2e0)
  Goroutine 4 - User: ./main.go:10 main.worker (0x104a2e0)
  Goroutine 5 - User: ./main.go:10 main.worker (0x104a2e0)

(dlv) goroutine 3              # goroutine 3으로 전환
Switched from 1 to 3 (thread 12345)

(dlv) bt                       # 현재 goroutine의 스택 트레이스
0  0x104a2e0 in main.worker at ./main.go:10
1  0x103e700 in runtime.goexit at runtime/asm_arm64.s:1222

(dlv) goroutine 1              # 다시 goroutine 1로 전환
```

goroutine 간에 자유롭게 전환하면서 각각의 상태, 지역 변수, 스택 트레이스를 확인할 수 있다. VS Code에서도 CALL STACK 패널에 goroutine이 각각 표시된다.

### deadlock 디버깅

goroutine이 서로를 기다리며 멈추는 deadlock은 Go에서 흔한 버그다. Go 런타임은 모든 goroutine이 블록되면 자동으로 감지한다:

```
fatal error: all goroutines are asleep - deadlock!

goroutine 1 [chan receive]:
main.main()
    /path/to/main.go:15 +0x68

goroutine 18 [chan send]:
main.worker()
    /path/to/main.go:9 +0x30
```

하지만 일부 goroutine만 deadlock에 빠진 경우(나머지는 정상 동작)에는 런타임이 감지하지 못한다. 이때 delve로 프로그램을 일시 중단하고 `goroutines`로 각 goroutine의 상태를 확인하면 어떤 goroutine이 어디서 블록되어 있는지 파악할 수 있다.

## fmt.Println 디버깅

현실적으로 가장 많이 쓰이는 디버깅 방법이다. Go 개발자들 사이에서도 `fmt.Println`을 코드 곳곳에 넣어 값을 확인하는 방식이 여전히 흔하다. Node.js에서 `console.log`를 쓰는 것과 같다:

```go
func processOrder(order Order) error {
    fmt.Printf("DEBUG order: %+v\n", order)

    total := calculateTotal(order)
    fmt.Printf("DEBUG total: %d\n", total)

    err := validateOrder(order)
    fmt.Printf("DEBUG validate err: %v\n", err)

    return err
}
```

`%+v`는 struct의 필드명을 포함하여 출력한다. `%v`만 쓰면 필드명 없이 값만 나온다. 디버깅 시 `%+v`가 유용하다.

이 방식이 통하는 이유:

1. **설정이 필요 없다.** 디버거 설정, launch.json, 확장 설치 없이 바로 쓸 수 있다.
2. **빠르다.** breakpoint를 설정하고 step 실행하는 것보다 코드에 한 줄 넣고 실행하는 것이 빠를 때가 많다.
3. **로그로 남는다.** 출력이 터미널에 시간 순서대로 나열되므로 실행 흐름을 파악하기 쉽다.

하지만 한계도 분명하다:

1. **커밋 전에 반드시 제거해야 한다.** 디버그 출력이 프로덕션에 남으면 문제가 된다.
2. **동시성 버그에 약하다.** 출력문을 추가하면 타이밍이 바뀌어서 race condition이 사라질 수 있다.
3. **복잡한 상태에 부적합하다.** 중첩된 struct나 긴 slice를 출력하면 읽기 어렵다.

디버거가 필요한 상황은 명확하다. 동시성 버그, 재현이 어려운 간헐적 문제, 복잡한 상태 추적. 그 외에는 `fmt.Println`이 충분할 때가 많다. 다만 `log` 패키지를 쓰면 타임스탬프가 함께 출력되어 조금 더 유용하다:

```go
log.Printf("DEBUG order: %+v", order)
// 2025/01/15 10:30:45 DEBUG order: {ID:1 Items:[...] Total:5000}
```

## 원격 디버깅

컨테이너나 원격 서버에서 실행 중인 프로그램을 디버깅할 때 사용한다:

```
# 원격 서버에서 headless 모드로 delve 실행
$ dlv debug --headless --listen=:2345 --api-version=2

# 로컬에서 접속
$ dlv connect localhost:2345
```

VS Code에서는 launch.json에 remote 설정을 추가한다:

```json
{
  "name": "Remote",
  "type": "go",
  "request": "attach",
  "mode": "remote",
  "remotePath": "/app",
  "port": 2345,
  "host": "127.0.0.1"
}
```

Node.js에서 `node --inspect=0.0.0.0:9229`로 원격 디버깅을 여는 것과 같은 패턴이다.

Node.js와 가장 큰 차이는 goroutine이다. 여러 goroutine이 동시에 실행되므로 각각의 상태를 개별적으로 추적해야 한다. delve는 이를 위해 만들어진 도구다. IDE와 연동하면 Node.js에서 VS Code로 디버깅하던 경험을 거의 그대로 가져올 수 있다.
