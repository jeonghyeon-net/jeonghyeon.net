# 로깅

Node.js에서 로깅은 `console.log`로 시작해서 pino나 winston 같은 구조화 로깅 라이브러리로 넘어가는 경로를 밟는다. Go도 비슷하다. `log` 패키지로 시작해서 Go 1.21부터 표준 라이브러리에 포함된 `log/slog`로 넘어간다. 차이점은 Go에서는 구조화 로깅이 서드파티가 아니라 표준이라는 것이다.

## log 패키지 — 기본 로깅

`log` 패키지는 Go의 가장 기본적인 로깅 도구다:

```go
package main

import "log"

func main() {
    log.Println("server started")
    log.Printf("listening on port %d", 8080)
}
```

출력:

```
2026/03/15 10:30:00 server started
2026/03/15 10:30:00 listening on port 8080
```

`log.Println`은 타임스탬프를 자동으로 붙인다. `fmt.Println`과의 차이가 이것이다. `fmt`는 순수 출력이고, `log`는 타임스탬프 + stderr 출력이다.

Node.js 대응:

```javascript
console.log("server started");
console.log(`listening on port ${8080}`);
```

`console.log`는 타임스탬프를 붙이지 않는다. 프로덕션에서 타임스탬프가 필요하면 pino 같은 라이브러리를 쓰거나 런타임 환경(Docker, CloudWatch)이 대신 붙여준다.

### log.Fatal과 log.Panic

`log.Fatal`은 로그를 출력한 뒤 `os.Exit(1)`을 호출한다. `log.Panic`은 로그를 출력한 뒤 `panic`을 호출한다:

```go
log.Fatal("database connection failed")
// 로그 출력 후 os.Exit(1) — defer도 실행되지 않는다

log.Panic("unexpected state")
// 로그 출력 후 panic — defer는 실행된다
```

`log.Fatal`은 `main` 함수의 초기화 단계에서만 사용하는 것이 관례다. 서버가 시작 전에 실패하는 경우(DB 연결 불가, 설정 파일 누락 등)에 적합하다. 요청 처리 중에는 사용하지 않는다.

### 커스텀 Logger

`log.New`로 출력 대상과 prefix를 지정한 Logger를 만들 수 있다:

```go
package main

import (
    "log"
    "os"
)

func main() {
    errorLog := log.New(os.Stderr, "ERROR: ", log.Ldate|log.Ltime|log.Lshortfile)
    errorLog.Println("something went wrong")
    // ERROR: 2026/03/15 10:30:00 main.go:11: something went wrong
}
```

`log.Lshortfile`은 파일명과 줄 번호를 추가한다. 하지만 이것이 `log` 패키지의 한계이기도 하다. 로그 레벨 구분이 없고, 구조화된 필드를 추가할 수 없다. prefix로 "ERROR: "를 붙이는 것은 문자열 조작이지 레벨 시스템이 아니다.

## log/slog — 구조화 로깅

Go 1.21에서 `log/slog` 패키지가 표준 라이브러리에 추가되었다. Node.js 생태계에서 pino가 하는 역할을 Go 표준이 담당한다.

```go
package main

import "log/slog"

func main() {
    slog.Info("server started", "port", 8080, "env", "production")
}
```

출력:

```
2026/03/15 10:30:00 INFO server started port=8080 env=production
```

`slog.Info`의 첫 번째 인자는 메시지, 나머지는 key-value 쌍이다. 이것이 구조화 로깅의 핵심이다. 메시지와 데이터가 분리되어 있어 로그 수집 시스템에서 파싱하기 쉽다.

Node.js pino 대응:

```javascript
const pino = require("pino");
const logger = pino();

logger.info({ port: 8080, env: "production" }, "server started");
```

pino는 key-value를 객체로, slog는 가변 인자로 전달한다. 형태는 다르지만 구조화 로깅이라는 개념은 동일하다.

## 로그 레벨

slog는 네 가지 레벨을 제공한다:

```go
slog.Debug("cache miss", "key", "user:123")
slog.Info("request received", "method", "GET", "path", "/api/users")
slog.Warn("slow query", "duration", "2.3s", "query", "SELECT ...")
slog.Error("failed to connect", "err", err, "host", "db.example.com")
```

기본 레벨은 `Info`다. `Debug` 레벨 로그는 기본적으로 출력되지 않는다. 레벨을 변경하려면 Handler를 설정해야 한다:

```go
handler := slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
    Level: slog.LevelDebug,
})
logger := slog.New(handler)
logger.Debug("this is now visible")
```

Node.js pino도 동일한 레벨 체계를 가진다:

```javascript
const logger = pino({ level: "debug" });
logger.debug("this is now visible");
```

### 런타임 레벨 변경

`slog.LevelVar`를 사용하면 프로그램 실행 중에 레벨을 변경할 수 있다:

```go
var logLevel slog.LevelVar
logLevel.Set(slog.LevelInfo)

handler := slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
    Level: &logLevel,
})
slog.SetDefault(slog.New(handler))

slog.Debug("not visible")

logLevel.Set(slog.LevelDebug) // 런타임에 변경
slog.Debug("now visible")
```

디버깅이 필요할 때 서버를 재시작하지 않고 로그 레벨을 변경할 수 있다. HTTP 엔드포인트로 노출하면 운영 중에도 제어 가능하다.

## Handler: TextHandler와 JSONHandler

slog의 출력 형식은 Handler가 결정한다. 두 가지 내장 Handler가 있다.

### TextHandler

```go
handler := slog.NewTextHandler(os.Stdout, nil)
logger := slog.New(handler)
logger.Info("user login", "user_id", 42, "ip", "192.168.1.1")
```

출력:

```
time=2026-03-15T10:30:00.000+09:00 level=INFO msg="user login" user_id=42 ip=192.168.1.1
```

key=value 형식이다. 사람이 읽기 쉽다. 개발 환경에 적합하다.

### JSONHandler

```go
handler := slog.NewJSONHandler(os.Stdout, nil)
logger := slog.New(handler)
logger.Info("user login", "user_id", 42, "ip", "192.168.1.1")
```

출력:

```json
{"time":"2026-03-15T10:30:00.000+09:00","level":"INFO","msg":"user login","user_id":42,"ip":"192.168.1.1"}
```

JSON 형식이다. 로그 수집 시스템(Datadog, Elasticsearch, CloudWatch)이 파싱하기 좋다. 프로덕션 환경에 적합하다.

Node.js pino는 기본 출력이 JSON이고, 개발 환경에서 사람이 읽기 쉬운 형식으로 바꾸려면 `pino-pretty`를 파이프한다. Go slog는 반대로 기본이 텍스트이고, 프로덕션에서 JSONHandler로 전환한다.

## slog.With — child logger

pino에서 `child()`로 기본 필드를 추가한 로거를 만드는 것처럼, slog에서는 `With()`를 사용한다:

```go
logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))

// request 처리용 로거 — 모든 로그에 request_id가 포함된다
reqLogger := logger.With("request_id", "abc-123", "method", "POST")
reqLogger.Info("processing request", "path", "/api/users")
reqLogger.Info("request completed", "status", 200)
```

출력:

```json
{"time":"...","level":"INFO","msg":"processing request","request_id":"abc-123","method":"POST","path":"/api/users"}
{"time":"...","level":"INFO","msg":"request completed","request_id":"abc-123","method":"POST","status":200}
```

`With()`는 새 Logger를 반환한다. 원본은 변경되지 않는다. pino의 child logger와 동일한 패턴이다:

```javascript
const reqLogger = logger.child({ requestId: "abc-123", method: "POST" });
reqLogger.info({ path: "/api/users" }, "processing request");
reqLogger.info({ status: 200 }, "request completed");
```

### slog.Group — 필드 그룹화

관련 필드를 그룹으로 묶을 수 있다:

```go
logger.Info("request",
    slog.Group("user",
        slog.String("id", "u-123"),
        slog.String("role", "admin"),
    ),
    slog.Group("request",
        slog.String("method", "GET"),
        slog.String("path", "/api/data"),
    ),
)
```

JSONHandler 출력:

```json
{"time":"...","level":"INFO","msg":"request","user":{"id":"u-123","role":"admin"},"request":{"method":"GET","path":"/api/data"}}
```

중첩된 JSON 구조가 만들어진다. 로그 분석 시 필드 충돌을 방지하고, 관련 데이터를 논리적으로 묶는다.

## 타입 안전한 속성

key-value를 가변 인자로 전달하면 key와 value의 타입이 보장되지 않는다. `slog.Attr`를 사용하면 타입 안전하게 속성을 지정할 수 있다:

```go
slog.Info("user created",
    slog.String("name", "Alice"),
    slog.Int("age", 30),
    slog.Bool("verified", true),
    slog.Duration("latency", 150*time.Millisecond),
    slog.Time("created_at", time.Now()),
)
```

`slog.String`, `slog.Int` 등은 `slog.Attr`를 반환한다. 가변 인자 방식(`"key", value`)보다 IDE 지원이 좋고, key 없이 value만 전달하는 실수를 방지한다.

## LogAttrs — 성능 최적화

`slog.Info` 등의 편의 함수는 내부에서 `any` 타입 변환이 발생한다. 고성능이 필요한 경로에서는 `LogAttrs`를 사용한다:

```go
logger.LogAttrs(ctx, slog.LevelInfo, "request handled",
    slog.String("method", r.Method),
    slog.String("path", r.URL.Path),
    slog.Int("status", statusCode),
    slog.Duration("duration", elapsed),
)
```

`LogAttrs`는 모든 인자가 `slog.Attr` 타입이므로 `any`로의 boxing이 발생하지 않는다. 대부분의 경우 차이는 미미하지만, 초당 수만 건의 로그를 출력하는 hot path에서는 의미 있다.

## context에서 로거 전달

22편에서 다룬 context를 활용하여 요청 범위의 로거를 전달하는 패턴이다. 미들웨어에서 request ID 등의 필드를 추가한 로거를 context에 저장하고, 하위 함수에서 꺼내 쓴다:

```go
type loggerKey struct{}

func WithLogger(ctx context.Context, logger *slog.Logger) context.Context {
    return context.WithValue(ctx, loggerKey{}, logger)
}

func FromContext(ctx context.Context) *slog.Logger {
    if logger, ok := ctx.Value(loggerKey{}).(*slog.Logger); ok {
        return logger
    }
    return slog.Default()
}
```

미들웨어에서 로거를 context에 주입한다:

```go
func loggingMiddleware(next http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        logger := slog.Default().With(
            "request_id", r.Header.Get("X-Request-ID"),
            "method", r.Method,
            "path", r.URL.Path,
        )
        ctx := WithLogger(r.Context(), logger)
        next.ServeHTTP(w, r.WithContext(ctx))
    })
}
```

핸들러와 서비스 함수에서 context로부터 로거를 꺼낸다:

```go
func handler(w http.ResponseWriter, r *http.Request) {
    logger := FromContext(r.Context())
    logger.Info("handling request")

    result, err := processOrder(r.Context(), "order-123")
    if err != nil {
        logger.Error("failed to process order", "err", err)
        http.Error(w, err.Error(), http.StatusInternalServerError)
        return
    }

    logger.Info("order processed", "result", result)
}

func processOrder(ctx context.Context, orderID string) (string, error) {
    logger := FromContext(ctx)
    logger.Info("processing order", "order_id", orderID)
    // 비즈니스 로직
    return "done", nil
}
```

`processOrder`는 로거를 인자로 받지 않지만, context에서 꺼내면 request_id, method, path가 이미 포함된 로거를 사용할 수 있다. 모든 로그가 동일한 request_id를 가지므로 요청 단위로 로그를 추적할 수 있다.

이 패턴은 pino의 child logger를 req 객체에 붙이는 것과 유사하다:

```javascript
app.use((req, res, next) => {
  req.log = logger.child({ requestId: req.headers["x-request-id"] });
  next();
});

app.get("/orders/:id", (req, res) => {
  req.log.info("handling request");
});
```

## 커스텀 Handler 작성

`slog.Handler` interface를 구현하면 로그 출력을 완전히 제어할 수 있다:

```go
type Handler interface {
    Enabled(context.Context, Level) bool
    Handle(context.Context, Record) error
    WithAttrs(attrs []Attr) Handler
    WithGroup(name string) Handler
}
```

개발 환경에서 사람이 읽기 쉬운 컬러 출력을 만드는 예시:

```go
type PrettyHandler struct {
    w     io.Writer
    level slog.Level
    attrs []slog.Attr
}

func NewPrettyHandler(w io.Writer, level slog.Level) *PrettyHandler {
    return &PrettyHandler{w: w, level: level}
}

func (h *PrettyHandler) Enabled(_ context.Context, level slog.Level) bool {
    return level >= h.level
}

func (h *PrettyHandler) Handle(_ context.Context, r slog.Record) error {
    levelStr := r.Level.String()
    switch r.Level {
    case slog.LevelDebug:
        levelStr = "\033[36mDEBUG\033[0m" // cyan
    case slog.LevelInfo:
        levelStr = "\033[32mINFO\033[0m" // green
    case slog.LevelWarn:
        levelStr = "\033[33mWARN\033[0m" // yellow
    case slog.LevelError:
        levelStr = "\033[31mERROR\033[0m" // red
    }

    fmt.Fprintf(h.w, "%s [%s] %s", r.Time.Format("15:04:05"), levelStr, r.Message)

    r.Attrs(func(a slog.Attr) bool {
        fmt.Fprintf(h.w, " %s=%v", a.Key, a.Value)
        return true
    })
    for _, a := range h.attrs {
        fmt.Fprintf(h.w, " %s=%v", a.Key, a.Value)
    }
    fmt.Fprintln(h.w)
    return nil
}

func (h *PrettyHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
    return &PrettyHandler{w: h.w, level: h.level, attrs: append(h.attrs, attrs...)}
}

func (h *PrettyHandler) WithGroup(name string) slog.Handler {
    // 단순화를 위해 그룹은 무시
    return h
}
```

사용:

```go
func main() {
    handler := NewPrettyHandler(os.Stdout, slog.LevelDebug)
    slog.SetDefault(slog.New(handler))

    slog.Info("server started", "port", 8080)
    slog.Debug("cache initialized", "size", 1024)
    slog.Warn("deprecated API called", "endpoint", "/v1/users")
    slog.Error("connection lost", "host", "db.example.com")
}
```

출력 (터미널에서 컬러로 표시):

```
10:30:00 [INFO] server started port=8080
10:30:00 [DEBUG] cache initialized size=1024
10:30:00 [WARN] deprecated API called endpoint=/v1/users
10:30:00 [ERROR] connection lost host=db.example.com
```

실제 프로덕션에서는 직접 Handler를 작성하기보다 기존 Handler를 래핑하는 방식이 더 일반적이다. 기본 JSONHandler로 충분하고, 출력 대상만 바꾸면 되는 경우가 대부분이다.

## 정리

| 개념 | Node.js | Go |
|---|---|---|
| 기본 로깅 | `console.log` | `log.Println` |
| 구조화 로깅 | pino, winston (서드파티) | `log/slog` (표준) |
| JSON 출력 | pino 기본 출력 | `slog.NewJSONHandler` |
| 텍스트 출력 | `pino-pretty` | `slog.NewTextHandler` |
| 로그 레벨 | `logger.level = "debug"` | `slog.HandlerOptions{Level: ...}` |
| child logger | `logger.child({key: val})` | `logger.With("key", val)` |
| 요청별 로거 | `req.log` | context에 로거 저장 |
| 커스텀 포맷 | custom transport | `slog.Handler` interface 구현 |

Node.js에서 Go로 넘어올 때 가장 큰 차이는 구조화 로깅이 표준이라는 점이다. pino나 winston을 고르고, 설정을 맞추고, transport를 구성하는 과정이 Go에서는 `slog.NewJSONHandler` 한 줄로 끝난다. 서드파티 의존성 없이 프로덕션 수준의 구조화 로깅을 시작할 수 있다.
