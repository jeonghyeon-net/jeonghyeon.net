# 동시성 패턴

13편에서 goroutine, channel, select, WaitGroup의 기본을 다뤘다. 이 편에서는 실전에서 반복적으로 등장하는 동시성 패턴을 정리한다. 공유 상태 보호, 에러 처리, goroutine 수명 관리까지 — Go 동시성 코드를 안전하게 작성하기 위해 알아야 할 도구와 관용구를 다룬다.

## sync.Mutex — 공유 상태 보호

여러 goroutine이 같은 변수에 동시에 접근하면 race condition이 발생한다. Node.js의 싱글 스레드 모델에서는 겪을 일이 없는 문제다:

```go
func main() {
    counter := 0
    var wg sync.WaitGroup

    for range 1000 {
        wg.Add(1)
        go func() {
            defer wg.Done()
            counter++ // race condition
        }()
    }

    wg.Wait()
    fmt.Println(counter) // 1000이 아닐 수 있다
}
```

`counter++`는 읽기-수정-쓰기 세 단계로 이루어진다. 두 goroutine이 동시에 같은 값을 읽고, 각각 1을 더하고, 각각 쓰면 증가분 하나가 사라진다. `go run -race`로 실행하면 런타임이 이를 감지한다:

```
==================
WARNING: DATA RACE
Read at 0x00c0000b4010 by goroutine 7:
  main.main.func1()
      main.go:13 +0x5a
...
==================
```

`sync.Mutex`로 해결한다:

```go
func main() {
    counter := 0
    var mu sync.Mutex
    var wg sync.WaitGroup

    for range 1000 {
        wg.Add(1)
        go func() {
            defer wg.Done()
            mu.Lock()
            counter++
            mu.Unlock()
        }()
    }

    wg.Wait()
    fmt.Println(counter) // 항상 1000
}
```

`Lock()`과 `Unlock()` 사이의 코드는 한 번에 하나의 goroutine만 실행한다. 이 구간을 critical section이라 부른다.

`worker_threads`를 쓸 때는 Node.js에서도 비슷한 문제를 다뤄야 한다. `SharedArrayBuffer`와 `Atomics`가 그 도구다:

```javascript
// Node.js worker_threads
const shared = new SharedArrayBuffer(4);
const view = new Int32Array(shared);
Atomics.add(view, 0, 1); // atomic 연산
```

### sync.RWMutex — 읽기가 많을 때

읽기는 여러 goroutine이 동시에 해도 안전하다. 쓰기만 단독으로 실행되면 된다. `sync.RWMutex`는 이 구분을 지원한다:

```go
type Cache struct {
    mu   sync.RWMutex
    data map[string]string
}

func (c *Cache) Get(key string) (string, bool) {
    c.mu.RLock()
    defer c.mu.RUnlock()
    v, ok := c.data[key]
    return v, ok
}

func (c *Cache) Set(key, value string) {
    c.mu.Lock()
    defer c.mu.Unlock()
    c.data[key] = value
}
```

- `RLock()` / `RUnlock()`: 읽기 락. 여러 goroutine이 동시에 획득 가능.
- `Lock()` / `Unlock()`: 쓰기 락. 단독 점유. 읽기 락이 모두 해제될 때까지 대기.

읽기가 쓰기보다 압도적으로 많은 캐시 같은 구조에서 효과적이다. 읽기와 쓰기 비율이 비슷하면 일반 `Mutex`와 성능 차이가 없거나 오히려 RWMutex 쪽이 느릴 수 있다.

## sync.Once — 정확히 한 번만

초기화를 정확히 한 번만 실행해야 할 때 쓴다. 여러 goroutine이 동시에 호출해도 첫 호출만 실행되고, 나머지는 그 완료를 기다린다:

```go
var (
    instance *DB
    once     sync.Once
)

func GetDB() *DB {
    once.Do(func() {
        instance = connectDB() // 한 번만 실행
    })
    return instance
}
```

Node.js에서는 모듈 시스템이 이 문제를 해결한다. 모듈은 처음 import될 때 한 번만 평가된다:

```javascript
// Node.js - db.js
// 모듈이 처음 import될 때 한 번만 실행된다
const db = connectDB();
export default db;
```

Go에서는 `init()` 함수도 비슷한 역할을 하지만, 테스트에서 제어하기 어렵다는 단점이 있다. `sync.Once`는 호출 시점에 초기화를 지연시킬 수 있어 더 유연하다.

Go 1.21부터 `sync.OnceValue`와 `sync.OnceValues`가 추가되어 반환값을 더 깔끔하게 처리할 수 있다:

```go
var getDB = sync.OnceValue(func() *DB {
    return connectDB()
})

func main() {
    db := getDB() // 첫 호출에서 connectDB() 실행, 이후 캐시된 값 반환
    _ = db
}
```

## errgroup

`errgroup`은 `golang.org/x/sync/errgroup` 패키지가 제공하는 도구로, 여러 goroutine을 실행하고 첫 번째 에러를 반환한다. `Promise.all`과 역할이 비슷하지만, context 연동과 동시성 제한까지 지원한다.

### context로 빠른 실패

`errgroup.WithContext`가 반환하는 context는 첫 번째 에러가 발생하면 자동으로 취소된다. 다른 goroutine이 이 context를 확인하면 나머지 작업을 조기 종료할 수 있다:

```go
func fetchURL(ctx context.Context, url string) (string, error) {
    req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
    if err != nil {
        return "", err
    }
    resp, err := http.DefaultClient.Do(req)
    if err != nil {
        return "", err
    }
    defer resp.Body.Close()
    body, err := io.ReadAll(resp.Body)
    return string(body), err
}

func main() {
    g, ctx := errgroup.WithContext(context.Background())
    urls := []string{
        "https://example.com",
        "https://invalid.example", // 실패
        "https://example.org",
    }

    results := make([]string, len(urls))
    for i, url := range urls {
        g.Go(func() error {
            body, err := fetchURL(ctx, url)
            if err != nil {
                return err
            }
            results[i] = body // 인덱스가 다르므로 race 없음
            return nil
        })
    }

    if err := g.Wait(); err != nil {
        fmt.Println("error:", err)
    }
}
```

`https://invalid.example`가 실패하면 ctx가 취소되고, `fetchURL`의 HTTP 요청이 ctx 취소를 감지하여 남은 요청도 중단된다. `Promise.all`도 첫 번째 reject에서 즉시 reject되지만, 나머지 Promise를 취소하지는 않는다. `AbortController`를 직접 연결해야 한다:

```javascript
// Node.js
const controller = new AbortController();
try {
  await Promise.all(
    urls.map((url) =>
      fetch(url, { signal: controller.signal })
    )
  );
} catch (err) {
  controller.abort(); // 수동으로 나머지를 취소해야 한다
}
```

### 동시성 제한

`SetLimit`으로 동시에 실행되는 goroutine 수를 제한할 수 있다:

```go
func main() {
    g, ctx := errgroup.WithContext(context.Background())
    g.SetLimit(3) // 동시에 최대 3개만 실행

    for i := range 100 {
        g.Go(func() error {
            if ctx.Err() != nil {
                return ctx.Err()
            }
            fmt.Println("processing", i)
            time.Sleep(time.Second)
            return nil
        })
    }

    if err := g.Wait(); err != nil {
        fmt.Println("error:", err)
    }
}
```

내부적으로 세마포어를 사용하여, 4번째 goroutine은 앞선 3개 중 하나가 끝날 때까지 대기한다. API rate limit이 있는 외부 서비스를 호출할 때 유용하다.

## 모든 결과 수집

`Promise.allSettled`는 모든 Promise가 settle될 때까지 기다리고, 각각의 성공/실패 결과를 모은다:

```javascript
// Node.js
const results = await Promise.allSettled([
  fetch("/api/a"),
  fetch("/api/b"),
  fetch("/api/c"),
]);
// [{status: "fulfilled", value: ...}, {status: "rejected", reason: ...}, ...]
```

Go에는 대응하는 표준 라이브러리가 없다. `errgroup`은 첫 번째 에러에서 context를 취소하므로, 모든 결과를 모으고 싶다면 직접 구현해야 한다:

```go
type Result struct {
    Value string
    Err   error
}

func fetchAll(urls []string) []Result {
    results := make([]Result, len(urls))
    var wg sync.WaitGroup

    for i, url := range urls {
        wg.Add(1)
        go func() {
            defer wg.Done()
            resp, err := http.Get(url)
            if err != nil {
                results[i] = Result{Err: err}
                return
            }
            defer resp.Body.Close()
            body, _ := io.ReadAll(resp.Body)
            results[i] = Result{Value: string(body)}
        }()
    }

    wg.Wait()
    return results
}
```

각 goroutine이 고유한 인덱스에만 쓰므로 Mutex가 필요 없다. `WaitGroup`으로 모든 goroutine의 완료를 기다리고, 에러가 발생해도 개별 결과에 기록만 한다.

## Worker Pool

fan-out은 여러 goroutine이 하나의 channel에서 작업을 가져가는 것이고, fan-in은 여러 goroutine의 결과를 하나의 channel로 모으는 것이다. 실전에 가까운 worker pool을 구성한다. graceful shutdown과 에러 처리를 포함한다:

```go
type Job struct {
    ID   int
    Data string
}

type JobResult struct {
    JobID int
    Out   string
    Err   error
}

func worker(ctx context.Context, jobs <-chan Job, results chan<- JobResult) {
    for job := range jobs {
        if ctx.Err() != nil {
            results <- JobResult{JobID: job.ID, Err: ctx.Err()}
            continue
        }
        // 작업 수행
        out, err := process(job.Data)
        results <- JobResult{JobID: job.ID, Out: out, Err: err}
    }
}

func process(data string) (string, error) {
    time.Sleep(100 * time.Millisecond) // 작업 시뮬레이션
    return "done: " + data, nil
}

func main() {
    ctx, cancel := context.WithCancel(context.Background())
    defer cancel()

    const numWorkers = 5
    jobs := make(chan Job, 10)
    results := make(chan JobResult, 10)

    // worker 시작
    var wg sync.WaitGroup
    for range numWorkers {
        wg.Add(1)
        go func() {
            defer wg.Done()
            worker(ctx, jobs, results)
        }()
    }

    // 작업 투입
    go func() {
        for i := range 20 {
            jobs <- Job{ID: i, Data: fmt.Sprintf("task-%d", i)}
        }
        close(jobs)
    }()

    // results channel을 닫는 goroutine
    go func() {
        wg.Wait()
        close(results)
    }()

    // 결과 수집
    for r := range results {
        if r.Err != nil {
            fmt.Printf("job %d failed: %v\n", r.JobID, r.Err)
            continue
        }
        fmt.Printf("job %d: %s\n", r.JobID, r.Out)
    }
}
```

핵심 구조:

1. `jobs` channel로 작업을 분배한다.
2. 여러 worker goroutine이 `jobs`에서 경쟁적으로 받아 처리한다(fan-out).
3. 결과를 `results` channel로 모은다(fan-in).
4. `WaitGroup`으로 모든 worker의 종료를 감지한 후 `results`를 닫는다.
5. context로 취소 신호를 전파한다.

`jobs`를 닫으면 모든 worker의 `range jobs` 루프가 종료된다. worker가 모두 끝나면 `wg.Wait()`가 반환되고, `results`가 닫히며, `range results` 루프도 종료된다. 이 흐름이 깔끔하게 연결되는 것이 channel 기반 설계의 장점이다.

Node.js에서는 `worker_threads`를 직접 구성하거나 `p-limit` 같은 라이브러리를 써야 한다:

```javascript
// Node.js (p-limit 사용)
import pLimit from "p-limit";

const limit = pLimit(5);
const tasks = Array.from({ length: 20 }, (_, i) =>
  limit(() => process(`task-${i}`))
);
const results = await Promise.allSettled(tasks);
```

## Goroutine Leak

goroutine은 가비지 컬렉터가 수거하지 않는다. goroutine이 블로킹된 채 남아 있으면 메모리와 리소스가 계속 점유된다. 이것이 goroutine leak이다.

### 흔한 실수 1 — 받는 쪽이 없는 channel

```go
func search(query string) string {
    ch := make(chan string)

    go func() {
        ch <- callAPI(query) // 아무도 받지 않으면 영원히 블로킹
    }()

    select {
    case result := <-ch:
        return result
    case <-time.After(500 * time.Millisecond):
        return "timeout"
    }
    // timeout이 발생하면 goroutine이 ch <- 에서 영원히 대기한다
}
```

timeout이 발생하면 `search` 함수는 반환되지만, goroutine은 `ch <-`에서 블로킹된 채 남는다. 이 함수가 반복 호출되면 goroutine이 계속 누적된다. buffered channel로 해결할 수 있다:

```go
func search(query string) string {
    ch := make(chan string, 1) // 버퍼 1: 받는 쪽이 없어도 보내기 가능

    go func() {
        ch <- callAPI(query) // 버퍼에 넣고 즉시 종료
    }()

    select {
    case result := <-ch:
        return result
    case <-time.After(500 * time.Millisecond):
        return "timeout"
    }
}
```

버퍼 크기가 1이면 받는 쪽이 없어도 보내기가 블로킹되지 않는다. goroutine은 값을 버퍼에 넣고 종료된다.

### 흔한 실수 2 — 닫히지 않는 channel을 range로 읽기

```go
func process() {
    ch := make(chan int)

    go func() {
        for i := range 5 {
            ch <- i
        }
        // close(ch)를 빠뜨림
    }()

    for v := range ch {
        fmt.Println(v) // 5개를 받은 후 영원히 대기
    }
}
```

`range ch`는 channel이 닫힐 때까지 계속 받기를 시도한다. `close(ch)`가 없으면 5개의 값을 받은 후 영원히 블로킹된다.

### 흔한 실수 3 — context를 무시하는 goroutine

```go
// 잘못된 코드
func worker(ctx context.Context, ch <-chan int) {
    for v := range ch {
        process(v) // ctx가 취소되어도 계속 실행
    }
}

// 올바른 코드
func worker(ctx context.Context, ch <-chan int) {
    for {
        select {
        case <-ctx.Done():
            return // 취소 시 즉시 종료
        case v, ok := <-ch:
            if !ok {
                return
            }
            process(v)
        }
    }
}
```

context를 받아놓고 확인하지 않으면, 취소 신호가 와도 goroutine이 멈추지 않는다. `select`로 context의 Done channel과 작업 channel을 동시에 대기해야 한다.

### 감지와 예방

goroutine 수를 모니터링하면 leak을 조기에 발견할 수 있다:

```go
fmt.Println("goroutines:", runtime.NumGoroutine())
```

테스트에서는 `goleak` 패키지를 쓸 수 있다:

```go
func TestNoLeak(t *testing.T) {
    defer goleak.VerifyNone(t)
    // 테스트 코드
}
```

goroutine leak을 방지하는 원칙:

1. **모든 goroutine에 종료 경로를 만든다.** context 취소, channel 닫기, 또는 done channel.
2. **buffered channel을 고려한다.** 받는 쪽이 사라질 가능성이 있으면 버퍼를 1 이상으로 설정한다.
3. **누가 channel을 닫는지 명확히 한다.** 보내는 쪽이 닫는 것이 원칙이다.

## 세마포어 패턴

Mutex는 한 번에 하나의 goroutine만 허용한다. N개를 동시에 허용하려면 세마포어가 필요하다. Go에서는 buffered channel로 구현한다:

```go
func main() {
    sem := make(chan struct{}, 3) // 동시에 최대 3개
    var wg sync.WaitGroup

    for i := range 10 {
        wg.Add(1)
        go func() {
            defer wg.Done()
            sem <- struct{}{} // 슬롯 획득
            defer func() { <-sem }() // 슬롯 반환

            fmt.Printf("worker %d start\n", i)
            time.Sleep(time.Second)
            fmt.Printf("worker %d done\n", i)
        }()
    }

    wg.Wait()
}
```

버퍼가 가득 차면 `sem <- struct{}{}`가 블로킹되어 추가 goroutine의 진입을 막는다. `struct{}{}`는 메모리를 차지하지 않는 빈 구조체다. 토큰 역할만 한다.

`golang.org/x/sync/semaphore` 패키지도 있다. context 연동과 가중치(weighted) 세마포어를 지원한다:

```go
func main() {
    sem := semaphore.NewWeighted(3)
    var wg sync.WaitGroup

    for i := range 10 {
        wg.Add(1)
        go func() {
            defer wg.Done()
            if err := sem.Acquire(context.Background(), 1); err != nil {
                return
            }
            defer sem.Release(1)

            fmt.Printf("worker %d start\n", i)
            time.Sleep(time.Second)
        }()
    }

    wg.Wait()
}
```

## sync.Map — 동시성 안전한 map

Go의 일반 map은 동시 읽기/쓰기에 안전하지 않다. 여러 goroutine이 동시에 map에 접근하면 런타임이 panic을 발생시킨다:

```
fatal error: concurrent map writes
```

`sync.Map`은 이를 해결한다:

```go
func main() {
    var m sync.Map

    // 쓰기
    m.Store("key", "value")

    // 읽기
    v, ok := m.Load("key")
    fmt.Println(v, ok) // value true

    // 없으면 저장, 있으면 기존 값 반환
    actual, loaded := m.LoadOrStore("key2", "default")
    fmt.Println(actual, loaded) // default false
}
```

`sync.Map`이 효과적인 경우:

- 키가 한 번 쓰이고 이후 읽기만 하는 경우 (캐시)
- 여러 goroutine이 서로 겹치지 않는 키에 접근하는 경우

그 외에는 `sync.RWMutex` + 일반 map이 보통 더 빠르다. `sync.Map`은 내부적으로 추가 메모리를 사용하고, 타입 안전성도 없다(값이 `any` 타입).

goroutine이 메모리를 공유하기 때문에 이 패턴들은 Go 동시성 코드에서 필수 도구다.
