# 프로파일링

성능 문제를 해결하려면 먼저 병목이 어디인지 알아야 한다. Node.js에서는 `node --prof`나 clinic.js로 프로파일링한다. Go는 pprof라는 프로파일링 도구가 런타임에 내장되어 있다. CPU, 메모리, goroutine 프로파일을 수집하고, flame graph로 시각화하고, `go tool trace`로 실행 흐름을 추적하는 방법을 다룬다.

## pprof 기본 개념

pprof는 Go 런타임에 내장된 프로파일러다. 별도 설치가 필요 없다. 프로그램 실행 중 일정 간격으로 샘플을 수집하여 어떤 함수가 CPU 시간을 얼마나 쓰는지, 메모리를 얼마나 할당하는지 기록한다.

Node.js에서는 V8 profiler가 내장되어 있지만 직접 사용하기보다 clinic.js 같은 래퍼를 쓰는 경우가 많다. Go는 표준 라이브러리만으로 프로파일링이 완결된다.

수집 가능한 프로파일 종류:

| 프로파일 | 설명 |
|---|---|
| cpu | 함수별 CPU 사용 시간 |
| heap | 현재 메모리 할당 상태 |
| allocs | 누적 메모리 할당 횟수 |
| goroutine | 현재 goroutine 상태 |
| block | 동기화 프리미티브 대기 시간 |
| mutex | mutex 경합 시간 |

## 벤치마크에서 프로파일 수집

가장 간단한 프로파일링 방법은 벤치마크와 연계하는 것이다. 16편에서 다뤘던 `testing.B`를 그대로 활용한다:

```go
// concat.go
package concat

import "strings"

func ConcatLoop(strs []string) string {
    result := ""
    for _, s := range strs {
        result += s
    }
    return result
}

func ConcatBuilder(strs []string) string {
    var b strings.Builder
    for _, s := range strs {
        b.WriteString(s)
    }
    return b.String()
}
```

```go
// concat_test.go
package concat

import "testing"

func makeStrings(n int) []string {
    strs := make([]string, n)
    for i := range strs {
        strs[i] = "hello"
    }
    return strs
}

func BenchmarkConcatLoop(b *testing.B) {
    strs := makeStrings(1000)
    b.ResetTimer()
    for b.Loop() {
        ConcatLoop(strs)
    }
}

func BenchmarkConcatBuilder(b *testing.B) {
    strs := makeStrings(1000)
    b.ResetTimer()
    for b.Loop() {
        ConcatBuilder(strs)
    }
}
```

`-cpuprofile` 플래그로 CPU 프로파일을 파일에 저장한다:

```
$ go test -bench=. -cpuprofile=cpu.prof
BenchmarkConcatLoop-8        2404    497810 ns/op
BenchmarkConcatBuilder-8   218498      5765 ns/op
PASS
```

메모리 프로파일도 같은 방식이다:

```
$ go test -bench=. -memprofile=mem.prof
```

Node.js에서 벤치마크 결과를 프로파일링하려면 별도 도구를 조합해야 한다. Go는 테스트 도구에 프로파일 수집이 통합되어 있다.

## go tool pprof

수집한 프로파일을 분석한다. `go tool pprof`는 대화형 셸을 제공한다:

```
$ go tool pprof cpu.prof
Type: cpu
Duration: 1.2s, Total samples = 1.1s (91.67%)
Entering interactive mode (type "help" for commands)
(pprof)
```

### top — 함수별 소비 시간

```
(pprof) top
Showing nodes accounting for 1.06s, 96.36% of 1.10s total
Showing top 10 nodes out of 30
      flat  flat%   sum%        cum   cum%
     0.52s 47.27% 47.27%      0.52s 47.27%  runtime.memmove
     0.22s 20.00% 67.27%      0.97s 88.18%  concat.ConcatLoop
     0.12s 10.91% 78.18%      0.12s 10.91%  runtime.mallocgc
```

`flat`은 해당 함수 자체에서 소비한 시간, `cum`(cumulative)은 해당 함수가 호출한 하위 함수 시간을 포함한 값이다. `ConcatLoop`이 `runtime.memmove`를 유발하고 있음을 알 수 있다. 문자열을 반복 연결할 때마다 새로운 메모리를 할당하고 복사하기 때문이다.

### list — 소스 코드 수준 분석

```
(pprof) list ConcatLoop
Total: 1.10s
ROUTINE ======================== concat.ConcatLoop
     0.22s      0.97s (flat, cum) 88.18% of Total
         .          .      5:func ConcatLoop(strs []string) string {
         .          .      6:    result := ""
         .          .      7:    for _, s := range strs {
     0.22s      0.97s      8:        result += s
         .          .      9:    }
         .          .     10:    return result
         .          .     11:}
```

8번째 줄, `result += s`에서 거의 모든 시간이 소비된다. Node.js의 `--prof` 출력이 V8 내부 함수 이름으로 가득한 것과 달리, Go의 pprof는 소스 코드에 직접 매핑된다.

### web — flame graph 시각화

```
(pprof) web
```

기본 브라우저에 SVG 형태의 call graph가 열린다. graphviz가 설치되어 있어야 한다. flame graph를 보려면:

```
$ go tool pprof -http=:8080 cpu.prof
```

브라우저에서 `http://localhost:8080`에 접속하면 flame graph, call graph, source view 등 다양한 시각화를 볼 수 있다.

## flame graph 읽는 법

flame graph는 call stack을 시각화한 것이다. 가로축은 시간 비율(넓을수록 오래 걸림), 세로축은 call stack 깊이다.

읽는 규칙:

1. **아래에서 위로 읽는다.** 맨 아래가 진입점(main), 위로 갈수록 깊은 호출이다.
2. **넓은 블록이 병목이다.** 가로폭이 전체 대비 넓은 함수가 CPU 시간을 많이 쓰는 것이다.
3. **색상은 의미가 없다.** 구분용이다. 블록 너비만 본다.
4. **좌우 순서도 의미가 없다.** 알파벳순 정렬일 뿐 호출 순서가 아니다.

Node.js의 clinic.js flame graph와 동일한 개념이다. 차이점은 Go의 flame graph에 goroutine별 스택이 포함될 수 있다는 것이다.

## 메모리 프로파일 분석

메모리 프로파일에는 두 가지 관점이 있다:

```
$ go tool pprof mem.prof
(pprof) top
```

기본값은 `inuse_space`로, 현재 사용 중인 메모리를 보여준다. 다른 관점으로 전환할 수 있다:

| 옵션 | 설명 |
|---|---|
| `-inuse_space` | 현재 힙에 남아있는 메모리 크기 |
| `-inuse_objects` | 현재 힙에 남아있는 객체 수 |
| `-alloc_space` | 누적 할당된 메모리 크기 |
| `-alloc_objects` | 누적 할당된 객체 수 |

```
$ go tool pprof -alloc_space mem.prof
(pprof) top
Showing nodes accounting for 4.88GB, 99.9% of 4.88GB total
      flat  flat%   sum%        cum   cum%
    4.88GB   100%   100%     4.88GB   100%  concat.ConcatLoop
```

`ConcatLoop`이 총 4.88GB를 할당했다. 문자열을 반복 연결할 때마다 새 문자열을 할당하기 때문이다. `ConcatBuilder`는 내부 버퍼를 재사용하므로 할당이 훨씬 적다.

Node.js에서 V8 heap snapshot을 Chrome DevTools에서 분석하는 것에 해당한다. V8 heap snapshot은 객체 그래프를 시각화하는 데 강하고, Go의 pprof는 함수별 할당량을 추적하는 데 강하다.

## HTTP 서버에 pprof 엔드포인트 추가

장시간 실행되는 서버의 프로파일을 수집하려면 HTTP 엔드포인트를 노출한다. `net/http/pprof` 패키지를 import하면 된다:

```go
package main

import (
    "fmt"
    "net/http"
    _ "net/http/pprof"
)

func heavyHandler(w http.ResponseWriter, r *http.Request) {
    // CPU를 많이 쓰는 작업 시뮬레이션
    sum := 0
    for i := 0; i < 10_000_000; i++ {
        sum += i
    }
    fmt.Fprintf(w, "sum = %d", sum)
}

func main() {
    http.HandleFunc("/heavy", heavyHandler)
    http.ListenAndServe(":8080", nil)
}
```

`_ "net/http/pprof"`는 blank import다. 패키지의 `init()` 함수만 실행하여 `/debug/pprof/` 경로에 핸들러를 등록한다. 코드에서 직접 사용하는 것이 없으므로 `_`를 붙인다.

서버를 실행한 뒤 프로파일을 수집한다:

```
$ go run main.go &

$ go tool pprof http://localhost:8080/debug/pprof/profile?seconds=10
Fetching profile over HTTP from http://localhost:8080/debug/pprof/profile?seconds=10
Saved profile in /home/user/pprof/pprof.samples.cpu.001.pb.gz
(pprof) top
```

10초간 CPU 프로파일을 수집한다. 수집하는 동안 서버에 요청을 보내야 의미 있는 데이터가 쌓인다.

사용 가능한 엔드포인트:

| 경로 | 설명 |
|---|---|
| `/debug/pprof/profile` | CPU 프로파일 (기본 30초) |
| `/debug/pprof/heap` | 힙 메모리 프로파일 |
| `/debug/pprof/allocs` | 누적 메모리 할당 프로파일 |
| `/debug/pprof/goroutine` | goroutine 덤프 |
| `/debug/pprof/block` | 블로킹 프로파일 |
| `/debug/pprof/mutex` | mutex 경합 프로파일 |
| `/debug/pprof/trace` | 실행 추적 (go tool trace용) |

goroutine 프로파일로 goroutine 누수를 감지할 수 있다:

```
$ go tool pprof http://localhost:8080/debug/pprof/goroutine
(pprof) top
```

goroutine 수가 시간이 지나면서 계속 증가한다면 누수를 의심한다. 어떤 함수에서 goroutine이 생성되는지 `list` 명령으로 확인한다.

주의: **프로덕션 서버에서 `/debug/pprof/`를 외부에 노출하면 안 된다.** 내부 네트워크에서만 접근 가능하도록 별도 포트로 분리하는 것이 일반적이다:

```go
go func() {
    http.ListenAndServe("localhost:6060", nil)
}()
```

## runtime/pprof — 프로그램 내부에서 프로파일 수집

HTTP 서버가 아닌 CLI 도구나 배치 프로그램에서는 `runtime/pprof`를 직접 사용한다:

```go
package main

import (
    "os"
    "runtime/pprof"
)

func main() {
    f, _ := os.Create("cpu.prof")
    pprof.StartCPUProfile(f)
    defer pprof.StopCPUProfile()

    // 프로파일링 대상 코드
    doWork()
}
```

Node.js에서 `v8.writeHeapSnapshot()`으로 특정 시점의 힙 스냅샷을 저장하는 것과 비슷한 패턴이다.

## go tool trace

pprof가 "어떤 함수가 시간을 많이 썼는가"에 답한다면, trace는 "시간 순서대로 무슨 일이 일어났는가"에 답한다. goroutine 스케줄링, GC 이벤트, 시스템 콜, 네트워크 대기 등을 타임라인으로 보여준다.

trace를 수집하는 방법:

```
$ go test -bench=. -trace=trace.out
```

또는 HTTP 서버에서:

```
$ curl -o trace.out http://localhost:8080/debug/pprof/trace?seconds=5
```

수집한 trace를 분석한다:

```
$ go tool trace trace.out
```

브라우저가 열리면서 타임라인 뷰를 보여준다. 확인할 수 있는 정보:

1. **goroutine 스케줄링** — 각 goroutine이 언제 실행되고, 언제 대기하는지
2. **프로세서 사용률** — 각 CPU 코어가 어떤 goroutine을 실행하는지
3. **GC 이벤트** — GC가 언제, 얼마나 오래 실행되는지
4. **네트워크/시스템 콜 대기** — I/O 바운드 병목 식별

Node.js의 clinic.js bubbleprof가 이벤트 루프와 비동기 작업의 관계를 시각화하는 것과 유사하다. Go의 trace는 goroutine과 프로세서 수준의 스케줄링을 시각화한다.

pprof와 trace의 차이:

| | pprof | trace |
|---|---|---|
| 방식 | 샘플링 (통계적) | 이벤트 기록 (정확) |
| 질문 | "어디서 시간을 쓰는가?" | "언제 무슨 일이 일어나는가?" |
| 오버헤드 | 낮음 | 높음 |
| 출력 | 함수별 요약 | 타임라인 |
| 용도 | CPU/메모리 병목 찾기 | 스케줄링/지연 문제 분석 |

trace는 오버헤드가 크므로 프로덕션에서 장시간 수집하는 것은 권장하지 않는다. 짧은 구간(5~10초)만 수집한다.

## Node.js 프로파일링과 비교

Node.js와 Go의 프로파일링 도구를 대응시키면:

| 작업 | Node.js | Go |
|---|---|---|
| CPU 프로파일링 | `node --prof` | `go tool pprof` |
| 프로파일 분석 | `node --prof-process` | `go tool pprof` (대화형) |
| 시각화 도구 | clinic.js, Chrome DevTools | `go tool pprof -http` |
| 힙 분석 | V8 heap snapshot | `go tool pprof -alloc_space` |
| 실행 추적 | clinic.js bubbleprof | `go tool trace` |
| 서버 프로파일링 | `--inspect` + DevTools | `net/http/pprof` |
| 벤치마크 연계 | 없음 (별도 조합) | `-cpuprofile`, `-memprofile` |

가장 큰 차이는 통합도다. Node.js는 `node --prof`로 수집하고, `node --prof-process`로 변환하고, Chrome DevTools나 clinic.js로 시각화하는 식으로 여러 도구를 조합한다. Go는 `go tool pprof` 하나로 수집, 분석, 시각화를 모두 처리한다.

## 정리

프로파일링 워크플로우를 요약한다:

1. **벤치마크 작성** — `testing.B`로 대상 함수를 벤치마크한다.
2. **프로파일 수집** — `-cpuprofile`, `-memprofile`로 프로파일을 저장한다.
3. **병목 식별** — `go tool pprof`에서 `top`, `list`로 핫스팟을 찾는다.
4. **시각화** — `-http` 플래그로 flame graph를 확인한다.
5. **최적화** — 병목 코드를 수정한다.
6. **검증** — 벤치마크를 다시 실행하여 개선을 확인한다.

서버 애플리케이션에서는 `net/http/pprof`를 미리 넣어두고, 문제가 발생했을 때 프로파일을 수집한다. goroutine 스케줄링이나 GC 관련 문제는 `go tool trace`로 타임라인을 확인한다. 모두 표준 도구이므로 별도 설치가 필요 없다.
