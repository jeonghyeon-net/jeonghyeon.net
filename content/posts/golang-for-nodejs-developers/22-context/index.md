# Context

Go에서 context는 요청의 수명을 관리하는 도구다. HTTP 요청이 취소되면 그 요청이 트리거한 DB 쿼리, 외부 API 호출, 파일 처리까지 전부 취소되어야 한다. `context.Context`는 이 취소 신호를 함수 호출 체인 전체에 전파한다.

## 왜 필요한가

HTTP 핸들러가 DB에서 데이터를 가져오고, 외부 API를 호출하고, 결과를 조합하는 상황을 생각해 보자. 클라이언트가 응답을 기다리다 연결을 끊었다. 서버는 이미 DB 쿼리를 실행 중이다. 쿼리 결과를 누구에게 보낼 것인가? 아무에게도 보내지 않는다. 그런데 쿼리는 계속 실행된다. 외부 API 호출도 계속 진행된다. 리소스가 낭비된다.

Node.js에서도 같은 문제가 있다. 하지만 싱글 스레드 모델에서는 진행 중인 비동기 작업을 취소하는 것이 선택 사항에 가깝다. 대부분의 Node.js 코드는 취소를 고려하지 않는다. Go에서는 다르다. goroutine이 수백 개 동시에 실행되므로, 불필요한 작업을 빠르게 정리하지 않으면 리소스가 금방 고갈된다.

## context.Context interface

`context.Context`는 네 개의 메서드를 가진 interface다:

```go
type Context interface {
    Deadline() (deadline time.Time, ok bool)
    Done() <-chan struct{}
    Err() error
    Value(key any) any
}
```

- `Deadline()` — 이 context가 만료되는 시각. 설정되지 않았으면 `ok`가 `false`.
- `Done()` — context가 취소되면 닫히는 channel. `select`에서 사용한다.
- `Err()` — `Done()`이 닫힌 이유. `context.Canceled` 또는 `context.DeadlineExceeded`.
- `Value(key)` — context에 저장된 값 조회. 21편에서 미들웨어에 사용했다.

## r.Context() — 시작점

HTTP 핸들러에서 context의 출발점은 `r.Context()`다. Go의 HTTP 서버는 클라이언트 연결이 끊어지면 이 context를 자동으로 취소한다:

```go
func handler(w http.ResponseWriter, r *http.Request) {
    ctx := r.Context()

    result, err := queryDB(ctx, "SELECT ...")
    if err != nil {
        http.Error(w, err.Error(), http.StatusInternalServerError)
        return
    }

    json.NewEncoder(w).Encode(result)
}
```

`r.Context()`로 얻은 context를 `queryDB`에 전달한다. 클라이언트가 연결을 끊으면 ctx가 취소되고, DB 드라이버가 이를 감지하여 쿼리를 중단한다. `database/sql` 패키지의 `QueryContext`, `ExecContext` 등이 모두 context를 받는다.

Node.js Express에서는 이에 대응하는 메커니즘이 없다. 클라이언트가 연결을 끊어도 핸들러는 끝까지 실행된다:

```javascript
app.get("/data", async (req, res) => {
  // 클라이언트가 끊어도 쿼리는 계속 실행된다
  const result = await db.query("SELECT ...");
  res.json(result);
});
```

`req.on('close', ...)`로 감지할 수는 있지만, 이미 시작된 쿼리를 취소하려면 추가 작업이 필요하다.

## context.WithCancel

수동으로 취소할 수 있는 context를 만든다:

```go
func process(ctx context.Context) error {
    ctx, cancel := context.WithCancel(ctx)
    defer cancel() // 함수 종료 시 반드시 취소

    errCh := make(chan error, 1)
    go func() {
        errCh <- longTask(ctx)
    }()

    select {
    case err := <-errCh:
        return err
    case <-ctx.Done():
        return ctx.Err()
    }
}
```

`cancel()`을 호출하면 `ctx.Done()` channel이 닫힌다. 이 ctx를 받은 모든 하위 함수에 취소 신호가 전파된다.

`defer cancel()`은 습관적으로 작성해야 한다. cancel을 호출하지 않으면 부모 context가 취소될 때까지 내부 리소스가 해제되지 않는다. 이것은 goroutine leak과 다르지만 역시 리소스 누수다.

## context.WithTimeout과 WithDeadline

`WithTimeout`은 지정된 시간이 지나면 자동으로 취소되는 context를 만든다:

```go
func callExternalAPI(ctx context.Context, url string) ([]byte, error) {
    ctx, cancel := context.WithTimeout(ctx, 3*time.Second)
    defer cancel()

    req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
    if err != nil {
        return nil, err
    }

    resp, err := http.DefaultClient.Do(req)
    if err != nil {
        return nil, err // timeout이면 context deadline exceeded
    }
    defer resp.Body.Close()

    return io.ReadAll(resp.Body)
}
```

3초 안에 응답이 오지 않으면 context가 취소되고, `http.DefaultClient.Do`가 에러를 반환한다. `http.NewRequestWithContext`로 context를 HTTP 요청에 연결하는 것이 핵심이다.

`WithDeadline`은 절대 시각을 기준으로 한다:

```go
deadline := time.Now().Add(5 * time.Second)
ctx, cancel := context.WithDeadline(ctx, deadline)
defer cancel()
```

`WithTimeout(ctx, 5*time.Second)`와 `WithDeadline(ctx, time.Now().Add(5*time.Second))`는 동일하다. 실제로 `WithTimeout`은 내부에서 `WithDeadline`을 호출한다.

### timeout 중첩

context는 트리 구조다. 자식 context의 timeout이 부모보다 길면 부모의 timeout이 적용된다:

```go
func handler(w http.ResponseWriter, r *http.Request) {
    // 서버의 WriteTimeout이 10초라고 가정
    ctx := r.Context()

    // 5초 timeout 설정 — 부모(10초)보다 짧으므로 이 timeout이 적용
    ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
    defer cancel()

    // 30초 timeout 설정 — 부모(5초)보다 길므로 5초에 취소됨
    ctx2, cancel2 := context.WithTimeout(ctx, 30*time.Second)
    defer cancel2()

    _ = ctx2 // 실제로는 5초 후 취소
}
```

이 특성 덕분에 timeout이 누적되지 않는다. 외부 API에 30초를 주더라도, 전체 요청 timeout이 5초면 5초 후에 모든 것이 취소된다.

## context.WithValue

21편 미들웨어에서 이미 사용했다. context에 key-value 쌍을 저장한다:

```go
type requestIDKey struct{}

func requestID(next http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        id := uuid.New().String()
        ctx := context.WithValue(r.Context(), requestIDKey{}, id)
        next.ServeHTTP(w, r.WithContext(ctx))
    })
}

func handler(w http.ResponseWriter, r *http.Request) {
    id, _ := r.Context().Value(requestIDKey{}).(string)
    log.Printf("[%s] handling request", id)
}
```

### WithValue 사용 원칙

`WithValue`는 남용하기 쉽다. 몇 가지 원칙이 있다:

1. **요청 범위(request-scoped) 데이터에만 사용한다.** request ID, 인증된 사용자 정보, trace ID 등. 비즈니스 로직의 파라미터를 context에 넣지 않는다.
2. **함수 시그니처로 전달할 수 있으면 시그니처를 쓴다.** `func getUser(ctx context.Context, userID string)` — `userID`를 context에 넣지 않고 명시적으로 전달한다.
3. **key는 unexported 타입을 쓴다.** 21편에서 다뤘듯 `type key struct{}`처럼 빈 struct를 정의하면 패키지 외부에서 접근할 수 없어 충돌이 방지된다.

Node.js에서 비슷한 역할을 하는 것이 `AsyncLocalStorage`다:

```javascript
const { AsyncLocalStorage } = require("async_hooks");
const requestStore = new AsyncLocalStorage();

app.use((req, res, next) => {
  requestStore.run({ requestId: crypto.randomUUID() }, next);
});

// 어디서든 접근 가능
const { requestId } = requestStore.getStore();
```

`AsyncLocalStorage`는 비동기 호출 체인을 따라 자동으로 전파된다. Go의 context는 명시적으로 전달해야 한다. Go 쪽이 더 번거롭지만, 어떤 함수가 어떤 context를 사용하는지 코드에서 바로 보인다.

## Context 전파 관례

Go에서 context를 전달하는 관례는 단순하다. **함수의 첫 번째 인자로 `ctx context.Context`를 받는다:**

```go
func GetUser(ctx context.Context, id string) (*User, error) { ... }
func SendEmail(ctx context.Context, to, subject, body string) error { ... }
func ProcessOrder(ctx context.Context, order *Order) error { ... }
```

이 관례를 따르면 취소 신호가 함수 호출 체인을 따라 자연스럽게 흐른다:

```go
func handler(w http.ResponseWriter, r *http.Request) {
    ctx := r.Context()

    user, err := GetUser(ctx, userID)
    if err != nil { ... }

    orders, err := GetOrders(ctx, user.ID)
    if err != nil { ... }

    if err := SendNotification(ctx, user, orders); err != nil { ... }
}
```

핸들러에서 시작된 context가 `GetUser` -> `GetOrders` -> `SendNotification`으로 전파된다. 클라이언트가 중간에 연결을 끊으면 모든 함수에서 취소를 감지할 수 있다.

context를 struct에 저장하지 않는다. context는 요청의 수명과 같이 짧게 살아야 한다:

```go
// 잘못된 패턴
type Service struct {
    ctx context.Context // context를 struct에 저장하지 않는다
}

// 올바른 패턴
type Service struct{}

func (s *Service) Do(ctx context.Context) error {
    // 메서드 호출 시 context를 받는다
    return nil
}
```

## 실전 예제: HTTP 핸들러에서 context 활용

DB 쿼리와 외부 API 호출을 조합하는 핸들러:

```go
func handleOrder(w http.ResponseWriter, r *http.Request) {
    ctx := r.Context()

    // DB에서 주문 조회 — r.Context()가 취소되면 쿼리도 취소
    orderID := r.PathValue("id")
    order, err := getOrder(ctx, orderID)
    if err != nil {
        http.Error(w, err.Error(), http.StatusInternalServerError)
        return
    }

    // 외부 결제 API 호출 — 별도 timeout 설정
    payCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
    defer cancel()

    status, err := checkPaymentStatus(payCtx, order.PaymentID)
    if err != nil {
        http.Error(w, "payment service unavailable", http.StatusServiceUnavailable)
        return
    }

    order.PaymentStatus = status
    w.Header().Set("Content-Type", "application/json")
    json.NewEncoder(w).Encode(order)
}

func getOrder(ctx context.Context, id string) (*Order, error) {
    row := db.QueryRowContext(ctx, "SELECT id, payment_id FROM orders WHERE id = $1", id)
    var o Order
    err := row.Scan(&o.ID, &o.PaymentID)
    return &o, err
}

func checkPaymentStatus(ctx context.Context, paymentID string) (string, error) {
    url := "https://payment.example.com/status/" + paymentID
    req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
    if err != nil {
        return "", err
    }

    resp, err := http.DefaultClient.Do(req)
    if err != nil {
        return "", err
    }
    defer resp.Body.Close()

    var result struct {
        Status string `json:"status"`
    }
    if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
        return "", err
    }
    return result.Status, nil
}
```

이 코드에서 context는 세 가지 역할을 한다:

1. **클라이언트 연결 끊김 감지** — `r.Context()`가 DB 쿼리까지 전파.
2. **외부 API timeout** — `WithTimeout`으로 결제 API에 2초 제한.
3. **계층적 취소** — 결제 API의 2초 timeout은 요청 전체 context의 하위에 있으므로, 요청 자체가 취소되면 결제 API 호출도 즉시 취소.

## context.AfterFunc

Go 1.21에서 추가된 `context.AfterFunc`는 context가 취소될 때 실행할 콜백을 등록한다:

```go
ctx, cancel := context.WithCancel(context.Background())

stop := context.AfterFunc(ctx, func() {
    log.Println("context cancelled, cleaning up")
})

// 필요 없어지면 콜백 등록 취소
if stop() {
    log.Println("cleanup callback was unregistered")
}
```

`stop()`을 호출하면 콜백 등록을 취소할 수 있다. context가 이미 취소되어 콜백이 실행되었거나 실행 중이면 `false`를 반환한다.

## Node.js의 AbortController

Node.js에서 context와 가장 가까운 것은 `AbortController`다:

```javascript
async function fetchWithTimeout(url, timeoutMs) {
  const controller = new AbortController();
  const timeoutId = setTimeout(() => controller.abort(), timeoutMs);

  try {
    const response = await fetch(url, { signal: controller.signal });
    return await response.json();
  } finally {
    clearTimeout(timeoutId);
  }
}
```

비교해 보면 차이가 명확하다:

| 특성 | Go context | AbortController |
|---|---|---|
| 전파 방식 | 함수 인자로 명시적 전달 | signal 객체를 옵션으로 전달 |
| 계층 구조 | 부모-자식 트리. 부모 취소 시 자식도 취소 | 단일 signal. 계층 없음 |
| timeout 지원 | `WithTimeout`, `WithDeadline` 내장 | `AbortSignal.timeout()` (별도) |
| 값 전달 | `WithValue`로 가능 | 불가. 별도 메커니즘 필요 |
| 관례 | 함수 첫 번째 인자 — 생태계 전체가 준수 | 일부 API만 signal 지원 |

가장 큰 차이는 관례의 체계성이다. Go 표준 라이브러리의 I/O, DB, HTTP, gRPC 관련 함수는 거의 모두 context를 첫 번째 인자로 받는다. Node.js에서 `AbortSignal`을 지원하는 API는 `fetch`, `setTimeout`, `EventTarget` 등 일부에 한정된다. `fs` 모듈의 대부분, `net` 모듈, 그리고 대다수 npm 패키지는 signal을 지원하지 않는다.

## context.Background()와 context.TODO()

모든 context 트리에는 루트가 필요하다:

```go
// 프로그램의 최상위에서 사용
ctx := context.Background()

// 어떤 context를 사용할지 아직 결정하지 못했을 때
ctx := context.TODO()
```

두 함수는 동일한 빈 context를 반환한다. 차이는 의도의 표현이다. `Background()`는 "이것이 루트 context다"를 의미하고, `TODO()`는 "나중에 적절한 context로 교체할 것이다"를 의미한다. 코드 리뷰에서 `TODO()`가 남아 있으면 미완성 코드라는 신호다.

HTTP 핸들러에서는 `r.Context()`가 루트 역할을 하므로 `Background()`를 직접 쓸 일이 드물다. `Background()`는 서버 시작 시점의 초기화나 테스트 코드에서 주로 사용한다.

## 정리

context는 Go의 동시성 모델과 맞물려 동작하는 도구다. goroutine이 쉽게 생성되는 만큼, 그것들의 수명도 체계적으로 관리되어야 한다. HTTP 요청 하나가 여러 goroutine을 생성하고, 각 goroutine이 DB, 캐시, 외부 API를 호출하는 상황에서 context 없이는 정리(cleanup)가 불가능하다.

| 함수 | 용도 |
|---|---|
| `context.Background()` | 루트 context |
| `context.TODO()` | 미결정 context (임시) |
| `context.WithCancel` | 수동 취소 |
| `context.WithTimeout` | 시간 제한 (상대) |
| `context.WithDeadline` | 시간 제한 (절대) |
| `context.WithValue` | 요청 범위 값 전달 |
| `context.AfterFunc` | 취소 시 콜백 등록 |

Node.js에서 Go로 넘어올 때, context를 모든 함수의 첫 번째 인자로 전달하는 것이 처음에는 번거롭게 느껴진다. 하지만 이것이 Go가 수천 개의 동시 요청을 안전하게 처리하는 기반이다. 명시적인 전파는 코드의 취소 경로를 추적 가능하게 만들고, 리소스 누수를 방지한다.
