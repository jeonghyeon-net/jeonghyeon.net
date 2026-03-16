# Goroutine과 Channel

`go` 키워드 하나로 경량 스레드를 만들고, channel로 스레드 간 데이터를 주고받는다. callback도 Promise도 async/await도 없다. 1978년 Tony Hoare의 CSP 이론에서 출발한 이 모델은 "메모리를 공유하지 말고, 통신으로 메모리를 공유하라"는 철학 위에 서 있다.

## 세 편의 논문과 한 편의 강연

1965년, Edsger Dijkstra가 학생 시험 문제로 낸 것이 있다. 원래는 컴퓨터가 테이프 드라이브를 두고 경쟁하는 상황이었는데, 곧이어 Tony Hoare가 이것을 다섯 명의 철학자가 원형 테이블에 앉아 식사하는 문제로 재구성했다. Dining Philosophers Problem. 각 철학자 사이에 포크가 하나씩 있고, 식사하려면 양쪽 포크가 모두 필요하다. 모든 철학자가 동시에 왼쪽 포크를 집으면? 아무도 오른쪽 포크를 집지 못한다. deadlock이다.

1978년, 같은 Hoare가 "Communicating Sequential Processes"라는 논문을 발표한다. 핵심 아이디어는 단순하다. 독립적인 프로세스들이 메모리를 공유하는 대신, 메시지를 주고받으며 협력한다. 입력과 출력을 프로그래밍의 기본 요소로 취급하고, 프로세스 간 통신은 동기적 핸드셰이크로 이루어진다. 보내는 쪽은 받는 쪽이 준비될 때까지 기다리고, 받는 쪽도 보내는 쪽이 준비될 때까지 기다린다. 이 논문은 컴퓨터 과학에서 가장 영향력 있는 논문 중 하나로 꼽힌다. Occam, Limbo, Erlang, 그리고 Go가 이 계보에 있다.

2012년, Rob Pike가 Heroku의 Waza 컨퍼런스에서 "Concurrency is not parallelism"이라는 강연을 한다. 핵심 구분은 이렇다:

- **Concurrency**: 여러 일을 한꺼번에 *다루는* 것. 구조에 관한 이야기다.
- **Parallelism**: 여러 일을 한꺼번에 *실행하는* 것. 실행에 관한 이야기다.

싱글 코어 컴퓨터에서도 concurrency는 가능하다. 여러 goroutine이 번갈아 실행되면서 각자의 일을 처리한다. parallelism은 멀티코어가 있어야 가능하다. 좋은 concurrent 설계는 하드웨어가 허락하면 자동으로 parallel하게 실행된다. Go의 모델이 정확히 이것이다.

## goroutine

goroutine은 Go 런타임이 관리하는 경량 실행 단위다. `go` 키워드를 함수 호출 앞에 붙이면 된다:

```go
func say(s string) {
    fmt.Println(s)
}

func main() {
    go say("hello")
    go say("world")
    time.Sleep(100 * time.Millisecond)
}
```

`go say("hello")`는 새 goroutine에서 `say`를 실행한다. `main` 함수도 goroutine이다(main goroutine). `main`이 끝나면 프로그램이 종료되므로, `time.Sleep`으로 다른 goroutine이 실행될 시간을 확보했다. 물론 이건 임시방편이다. 제대로 된 동기화는 channel이나 `sync.WaitGroup`으로 한다.

익명 함수도 goroutine으로 실행할 수 있다:

```go
func main() {
    go func() {
        fmt.Println("anonymous goroutine")
    }()

    time.Sleep(100 * time.Millisecond)
}
```

### goroutine은 OS 스레드가 아니다

OS 스레드 하나를 만드는 데 1~2MB의 스택 메모리가 필요하다. context switch에 1~2 마이크로초가 걸린다. goroutine은 초기 스택이 2KB에 불과하다. 필요하면 동적으로 늘어나고, GC 시점에 사용량이 적으면 다시 줄어든다. context switch는 50~100 나노초. OS 스레드 대비 10~40배 빠르다.

이 차이가 실질적으로 의미하는 것:

```go
func main() {
    for i := range 100_000 {
        go func() {
            time.Sleep(time.Second)
            fmt.Println(i)
        }()
    }
    time.Sleep(2 * time.Second)
}
```

goroutine 10만 개를 동시에 실행해도 문제없다. 메모리 사용량은 수백 MB 수준이다. OS 스레드 10만 개는 대부분의 시스템에서 불가능하다.

같은 작업을 Node.js로 하면:

```javascript
// Node.js
const promises = [];
for (let i = 0; i < 100_000; i++) {
  promises.push(
    new Promise((resolve) => setTimeout(() => {
      console.log(i);
      resolve();
    }, 1000))
  );
}
await Promise.all(promises);
```

I/O 대기는 이벤트 루프가 효율적으로 처리하지만, CPU를 점유하는 작업이 10만 개라면 싱글 스레드인 Node.js는 하나씩 순서대로 처리할 수밖에 없다. Go는 멀티코어를 활용해서 실제로 병렬 실행한다.

## channel

channel은 goroutine 간 데이터를 주고받는 통로다. CSP의 핵심 아이디어 — 통신으로 동기화한다 — 를 구현한 것이다:

```go
func main() {
    ch := make(chan string)

    go func() {
        ch <- "hello" // channel에 값을 보낸다
    }()

    msg := <-ch // channel에서 값을 받는다
    fmt.Println(msg) // hello
}
```

`make(chan string)`으로 string 타입의 channel을 만든다. `ch <- "hello"`는 값을 보내는 연산, `<-ch`는 값을 받는 연산이다. 이 channel은 unbuffered(버퍼 없음)다. 보내는 쪽은 받는 쪽이 준비될 때까지 블로킹되고, 받는 쪽도 보내는 쪽이 준비될 때까지 블로킹된다. Hoare의 1978년 논문에서 말한 동기적 핸드셰이크가 바로 이것이다.

`time.Sleep` 없이도 동기화가 된다. `<-ch`가 값이 올 때까지 main goroutine을 블로킹하기 때문이다.

### unbuffered vs buffered channel

unbuffered channel은 보내기와 받기가 동시에 일어나야 한다:

```go
func main() {
    ch := make(chan int)

    go func() {
        ch <- 1  // 받는 쪽이 준비될 때까지 여기서 대기
        fmt.Println("sent")
    }()

    time.Sleep(time.Second) // 1초 후에 받기 시작
    fmt.Println(<-ch)       // 이 시점에 보내기와 받기가 동시에 완료
    time.Sleep(100 * time.Millisecond) // "sent" 출력 대기
}
```

buffered channel은 버퍼 크기만큼 값을 미리 보낼 수 있다:

```go
func main() {
    ch := make(chan int, 3) // 버퍼 크기 3

    ch <- 1 // 블로킹되지 않음
    ch <- 2 // 블로킹되지 않음
    ch <- 3 // 블로킹되지 않음
    // ch <- 4 // 여기서 블로킹됨 (버퍼가 가득 참)

    fmt.Println(<-ch) // 1
    fmt.Println(<-ch) // 2
    fmt.Println(<-ch) // 3
}
```

buffered channel은 생산자와 소비자의 속도 차이를 완충한다. 버퍼가 가득 차면 보내기가 블로킹되고, 비어 있으면 받기가 블로킹된다.

언제 어떤 것을 쓰는가:

- **unbuffered**: goroutine 간 확실한 동기화가 필요할 때. 보내는 쪽이 받는 쪽의 수신을 보장받을 수 있다.
- **buffered**: 생산과 소비의 속도가 다를 때. 일시적인 burst를 흡수한다. 하지만 버퍼 크기를 무작정 늘리는 것은 문제를 미루는 것일 뿐이다.

### channel 방향

함수 시그니처에서 channel의 방향을 제한할 수 있다:

```go
// send 전용: 이 함수는 channel에 값을 보내기만 할 수 있다
func produce(ch chan<- int) {
    for i := range 5 {
        ch <- i
    }
    close(ch)
}

// receive 전용: 이 함수는 channel에서 값을 받기만 할 수 있다
func consume(ch <-chan int) {
    for v := range ch {
        fmt.Println(v)
    }
}

func main() {
    ch := make(chan int)
    go produce(ch)
    consume(ch)
}
```

`chan<-`는 send 전용, `<-chan`은 receive 전용이다. 화살표가 channel 쪽을 향하면 보내기, channel에서 나오면 받기. 양방향 channel(`chan int`)은 send 전용이나 receive 전용으로 자동 변환된다.

방향 제한은 컴파일 타임에 검사된다. `consume` 함수 안에서 `ch <- 1`을 쓰면 컴파일 에러가 발생한다. 08편에서 다룬 interface의 최소 권한 원칙과 같은 맥락이다. 함수가 channel을 어떻게 사용하는지 시그니처만 보고 알 수 있다.

### channel 닫기와 range

`close(ch)`로 channel을 닫으면, 더 이상 값이 오지 않는다는 신호를 보낸다. 닫힌 channel에서 받기를 하면 zero value가 즉시 반환된다:

```go
func main() {
    ch := make(chan int, 3)
    ch <- 1
    ch <- 2
    close(ch)

    fmt.Println(<-ch) // 1
    fmt.Println(<-ch) // 2
    fmt.Println(<-ch) // 0 (zero value, channel이 닫힘)

    // 닫혔는지 확인
    v, ok := <-ch
    fmt.Println(v, ok) // 0 false
}
```

`range`로 channel을 순회하면, channel이 닫힐 때까지 값을 받는다:

```go
func fibonacci(n int, ch chan<- int) {
    a, b := 0, 1
    for range n {
        ch <- a
        a, b = b, a+b
    }
    close(ch) // range 루프를 종료시킨다
}

func main() {
    ch := make(chan int)
    go fibonacci(10, ch)

    for v := range ch {
        fmt.Println(v)
    }
    // 출력: 0 1 1 2 3 5 8 13 21 34
}
```

규칙: channel을 닫는 것은 보내는 쪽의 책임이다. 받는 쪽이 닫으면 안 된다. 닫힌 channel에 값을 보내면 panic이 발생한다.

## select

`select`는 여러 channel 연산을 동시에 대기한다. `switch`와 비슷하게 생겼지만, channel 연산에 특화되어 있다:

```go
func main() {
    ch1 := make(chan string)
    ch2 := make(chan string)

    go func() {
        time.Sleep(100 * time.Millisecond)
        ch1 <- "one"
    }()

    go func() {
        time.Sleep(200 * time.Millisecond)
        ch2 <- "two"
    }()

    for range 2 {
        select {
        case msg := <-ch1:
            fmt.Println("received from ch1:", msg)
        case msg := <-ch2:
            fmt.Println("received from ch2:", msg)
        }
    }
    // 출력:
    // received from ch1: one
    // received from ch2: two
}
```

여러 case가 동시에 준비되면, Go 런타임이 무작위로 하나를 선택한다. 이는 의도적인 설계다. 특정 channel이 항상 우선되는 starvation을 방지한다.

### timeout 패턴

`select`와 `time.After`를 조합하면 timeout을 구현할 수 있다:

```go
func main() {
    ch := make(chan string)

    go func() {
        time.Sleep(2 * time.Second)
        ch <- "result"
    }()

    select {
    case msg := <-ch:
        fmt.Println(msg)
    case <-time.After(1 * time.Second):
        fmt.Println("timeout")
    }
    // 출력: timeout
}
```

Node.js에서는 `Promise.race`로 비슷하게 구현한다:

```javascript
// Node.js
const result = await Promise.race([
  fetchData(),
  new Promise((_, reject) =>
    setTimeout(() => reject(new Error("timeout")), 1000)
  ),
]);
```

`Promise.race`와 `select`는 비슷해 보이지만 차이가 있다. `Promise.race`에서 지는 쪽의 Promise는 여전히 실행 중이다. 취소하려면 `AbortController`를 별도로 사용해야 한다. Go의 `select`는 선택되지 않은 case가 단순히 무시된다. 물론 goroutine 자체의 취소에는 `context` 패키지가 필요하다.

### default case

`default`를 넣으면 어떤 channel도 준비되지 않았을 때 즉시 실행된다. non-blocking channel 연산이 된다:

```go
func main() {
    ch := make(chan int)

    select {
    case v := <-ch:
        fmt.Println(v)
    default:
        fmt.Println("no value ready")
    }
    // 출력: no value ready
}
```

## deadlock

deadlock은 goroutine들이 서로를 기다리며 영원히 멈추는 상태다. Go 런타임은 모든 goroutine이 블로킹되면 이를 감지하고 프로그램을 종료한다:

```go
func main() {
    ch := make(chan int)
    ch <- 1 // 받는 goroutine이 없으므로 영원히 블로킹
}
// fatal error: all goroutines are asleep - deadlock!
```

unbuffered channel에 값을 보내려면 받는 쪽이 있어야 한다. `main` goroutine이 `ch <- 1`에서 블로킹되는데, 다른 goroutine이 없으므로 아무도 `<-ch`를 실행할 수 없다. 런타임이 이를 감지한다.

Dining Philosophers Problem을 Go로 재현해 보자:

```go
func main() {
    forks := make([]sync.Mutex, 5)

    for i := range 5 {
        go func() {
            for {
                left := i
                right := (i + 1) % 5

                forks[left].Lock()
                // 모든 철학자가 왼쪽 포크를 집은 상태
                // 이제 오른쪽 포크를 기다린다
                forks[right].Lock()

                // 식사
                fmt.Printf("philosopher %d is eating\n", i)

                forks[right].Unlock()
                forks[left].Unlock()
            }
        }()
    }

    select {} // main goroutine 블로킹
}
```

5명의 철학자가 동시에 왼쪽 포크를 집으면, 모두 오른쪽 포크를 기다리며 멈춘다. 이 deadlock은 Go 런타임이 감지하지 못한다. 모든 goroutine이 sleep 상태가 아니라 mutex를 기다리는 상태이기 때문이다. Go 런타임의 deadlock 감지는 모든 goroutine이 channel 연산이나 select에서 블로킹된 경우에만 작동한다.

CSP 스타일로 해결하면:

```go
func philosopher(id int, leftFork, rightFork chan struct{}) {
    for {
        <-leftFork
        <-rightFork

        fmt.Printf("philosopher %d is eating\n", id)

        leftFork <- struct{}{}
        rightFork <- struct{}{}
    }
}

func main() {
    forks := make([]chan struct{}, 5)
    for i := range 5 {
        forks[i] = make(chan struct{}, 1)
        forks[i] <- struct{}{} // 포크를 테이블에 놓는다
    }

    for i := range 4 {
        go philosopher(i, forks[i], forks[(i+1)%5])
    }
    // 마지막 철학자는 포크 순서를 반대로 집는다
    go philosopher(4, forks[0], forks[4])

    select {}
}
```

마지막 철학자가 포크를 집는 순서를 반대로 함으로써 순환 대기를 깨뜨린다. channel을 포크로 사용했다. buffered channel(크기 1)에 값이 있으면 포크가 테이블 위에 있는 것이고, 비어 있으면 누군가 사용 중인 것이다.

deadlock을 피하는 일반적인 전략:

1. **channel에 timeout을 건다**: `select`와 `time.After` 조합
2. **순서를 정한다**: 리소스를 항상 같은 순서로 획득
3. **buffered channel을 사용한다**: 블로킹 가능성을 줄인다
4. **context로 취소한다**: 무한 대기를 방지

## M:N 스케줄링

Node.js는 하나의 스레드에서 JavaScript 코드를 실행한다. I/O 작업(파일 읽기, 네트워크 요청 등)은 libuv가 백그라운드에서 처리하고, 완료되면 콜백을 이벤트 큐에 넣는다. 이벤트 루프가 큐에서 콜백을 꺼내서 실행한다:

```
[ JavaScript 스레드 ]
        |
   이벤트 루프 반복
        |
   +---------+
   | 콜백 실행 | ← 큐에서 하나씩
   +---------+
        |
   I/O 완료 대기
        |
   (libuv가 OS에 위임)
```

이 모델의 장점은 단순함이다. 한 번에 하나의 콜백만 실행되므로 race condition이 원천적으로 없다. 락도 필요 없다. 단점은 CPU-bound 작업이 이벤트 루프를 막는다는 것이다. 피보나치 계산이 1초 걸리면, 그 1초 동안 서버는 어떤 요청도 처리하지 못한다.

```javascript
// Node.js - CPU-bound 작업이 이벤트 루프를 블로킹
function fibonacci(n) {
  if (n <= 1) return n;
  return fibonacci(n - 1) + fibonacci(n - 2);
}

app.get("/fib", (req, res) => {
  const result = fibonacci(45); // 수 초 소요 - 전체 서버 블로킹
  res.json({ result });
});

app.get("/health", (req, res) => {
  res.json({ ok: true }); // /fib 처리 중에는 응답 불가
});
```

`worker_threads`로 우회할 수 있지만, 데이터 전달이 `postMessage`/`onmessage` 패턴이라 복잡도가 급격히 올라간다.

Go 런타임은 이와 달리 GMP 모델이라 불리는 M:N 스케줄러를 내장하고 있다:

- **G** (Goroutine): 실행할 작업. 경량 스레드.
- **M** (Machine): OS 스레드. 실제로 코드를 실행하는 주체.
- **P** (Processor): 스케줄링 컨텍스트. 로컬 실행 큐와 메모리 캐시를 가지고 있다.

P의 개수는 `GOMAXPROCS`로 결정되며, 기본값은 CPU 코어 수다. 8코어 머신이면 P가 8개, 동시에 8개의 goroutine이 실제로 병렬 실행된다.

```
P0 [G1, G2, G3]  ←→  M0 (OS Thread)
P1 [G4, G5]      ←→  M1 (OS Thread)
P2 [G6]          ←→  M2 (OS Thread)
P3 []            ←→  (유휴)

Global Queue: [G7, G8, ...]
```

각 P는 로컬 큐를 가지고 있다. goroutine을 생성하면 현재 P의 로컬 큐에 들어간다. P의 로컬 큐가 비면 다른 P의 큐에서 goroutine을 훔쳐온다(work stealing). 이 설계 덕분에 부하가 코어 간에 자동으로 분산된다.

goroutine이 시스템 콜(파일 I/O 등)로 블로킹되면, M은 P를 놓고 시스템 콜 완료를 기다린다. 놓인 P는 다른 M이 가져가서 나머지 goroutine을 계속 실행한다. 12편에서 "블로킹 I/O가 goroutine을 블로킹할 뿐 OS 스레드를 블로킹하지 않는다"고 한 것이 정확히 이 메커니즘이다.

```go
// Go - CPU-bound 작업이 다른 요청을 블로킹하지 않는다
func fibonacci(n int) int {
    if n <= 1 {
        return n
    }
    return fibonacci(n-1) + fibonacci(n-2)
}

func main() {
    http.HandleFunc("/fib", func(w http.ResponseWriter, r *http.Request) {
        result := fibonacci(45) // 이 goroutine만 바쁘다
        fmt.Fprintf(w, "%d", result)
    })

    http.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
        fmt.Fprintln(w, "ok") // 다른 goroutine에서 즉시 응답
    })

    http.ListenAndServe(":8080", nil)
}
```

`/fib`가 수 초 걸려도 `/health`는 즉시 응답한다. 각 HTTP 요청이 별도의 goroutine에서 처리되고, Go 런타임이 이들을 여러 OS 스레드에 분배하기 때문이다.

### 정리

| | Node.js | Go |
|---|---|---|
| 스레드 모델 | 싱글 스레드 + 이벤트 루프 | M:N (goroutine:OS 스레드) |
| I/O 처리 | 비동기 콜백/Promise | 블로킹 호출 (goroutine 내에서) |
| CPU-bound 작업 | 이벤트 루프 블로킹 | 해당 goroutine만 점유 |
| 멀티코어 활용 | `cluster`/`worker_threads` 필요 | 기본 내장 (`GOMAXPROCS`) |
| race condition | 원천 차단 (싱글 스레드) | 가능 (공유 메모리 접근 시) |
| 동기화 도구 | 불필요 | channel, sync.Mutex 등 |
| 비동기 문법 | async/await, Promise | `go` 키워드, channel |

싱글 스레드 이벤트 루프는 복잡한 동시성을 단순하게 감추는 전략이다. 대부분의 웹 I/O 작업에서 효과적이지만, CPU-bound 작업이나 실제 병렬 처리가 필요한 순간 한계에 부딪힌다. Go는 동시성을 언어의 기본 요소로 제공한다. goroutine과 channel이 변수나 함수만큼 자연스러운 도구다. 대신 race condition이라는 새로운 종류의 버그를 다뤄야 한다. `go run -race`로 race condition을 감지할 수 있다.
