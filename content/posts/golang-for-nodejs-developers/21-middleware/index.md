# 미들웨어와 요청 처리

20편에서 미들웨어의 기본 형태를 소개했다. `func(http.Handler) http.Handler` — Handler를 받아서 Handler를 반환하는 함수. 이 단순한 시그니처로 로깅, 인증, CORS, panic recovery까지 구현할 수 있다. Express의 `app.use(middleware)`와 같은 역할이지만, 동작 방식은 근본적으로 다르다.

## Express 미들웨어 vs Go 미들웨어

Express 미들웨어는 `next` callback으로 제어를 넘긴다:

```javascript
function logging(req, res, next) {
  console.log(`${req.method} ${req.url}`);
  next();
  // next() 이후 코드도 실행된다
  console.log("응답 완료");
}

app.use(logging);
```

Go 미들웨어는 Handler를 감싸는 함수 합성이다:

```go
func logging(next http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        log.Printf("%s %s", r.Method, r.URL.Path)
        next.ServeHTTP(w, r)
        // next.ServeHTTP 이후 코드도 실행된다
        log.Println("응답 완료")
    })
}
```

핵심 차이는 제어 흐름의 명시성이다. Express의 `next()`는 callback이므로 호출 시점이 자유롭다. 비동기 작업 후에 호출하거나, 아예 호출하지 않을 수 있다. Go의 `next.ServeHTTP(w, r)`은 일반 함수 호출이다. 호출하면 다음 핸들러가 실행되고, 반환되면 실행이 끝난 것이다. callback chain이 아닌 call stack이다.

## 실행 순서

미들웨어를 체이닝하면 실행 순서가 중요해진다:

```go
func first(next http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        log.Println("first: 요청 진입")
        next.ServeHTTP(w, r)
        log.Println("first: 응답 완료")
    })
}

func second(next http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        log.Println("second: 요청 진입")
        next.ServeHTTP(w, r)
        log.Println("second: 응답 완료")
    })
}
```

이 두 미들웨어를 적용한다:

```go
handler := first(second(mux))
```

출력은 다음과 같다:

```
first: 요청 진입
second: 요청 진입
(핸들러 실행)
second: 응답 완료
first: 응답 완료
```

함수 호출이 중첩되므로 요청은 바깥에서 안으로, 응답은 안에서 바깥으로 흐른다. `first`가 `second`를 감싸고, `second`가 실제 핸들러를 감싼다. 이 구조는 Express의 미들웨어 스택과 결과는 같지만, Express는 `next()` callback을 통한 순차 실행이고 Go는 함수 호출의 중첩이다.

`chain` 함수를 만들면 읽기 순서와 실행 순서가 일치한다:

```go
func chain(h http.Handler, middlewares ...func(http.Handler) http.Handler) http.Handler {
    for i := len(middlewares) - 1; i >= 0; i-- {
        h = middlewares[i](h)
    }
    return h
}

handler := chain(mux, first, second)
// first -> second -> mux 순서로 실행
```

## 응답을 가로채는 ResponseWriter wrapper

미들웨어에서 응답 상태 코드나 본문 크기를 알고 싶을 때가 있다. `http.ResponseWriter`는 한 번 쓰면 읽을 수 없다. wrapper를 만들어 해결한다:

```go
type statusRecorder struct {
    http.ResponseWriter
    statusCode int
}

func (r *statusRecorder) WriteHeader(code int) {
    r.statusCode = code
    r.ResponseWriter.WriteHeader(code)
}
```

`http.ResponseWriter`를 embed하면 `Header()`와 `Write()` 메서드는 그대로 위임된다. `WriteHeader`만 오버라이드하여 상태 코드를 기록한다.

## 로깅 미들웨어

실용적인 로깅 미들웨어는 응답 상태 코드와 처리 시간도 기록해야 한다:

```go
func logging(next http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        start := time.Now()

        rec := &statusRecorder{ResponseWriter: w, statusCode: http.StatusOK}
        next.ServeHTTP(rec, r)

        log.Printf("%s %s %d %s",
            r.Method, r.URL.Path, rec.statusCode, time.Since(start))
    })
}
```

Express에서 같은 일을 하려면 `morgan` 같은 라이브러리를 쓰거나 `res.on('finish', ...)`로 응답 완료 이벤트를 잡아야 한다. Go에서는 `next.ServeHTTP` 호출이 동기적이므로, 그 이후에 바로 처리 시간을 계산하면 된다.

## 인증 미들웨어

인증 미들웨어는 요청을 검증하고, 실패하면 다음 핸들러를 호출하지 않는다:

```go
func auth(next http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        token := r.Header.Get("Authorization")
        if token == "" {
            http.Error(w, "unauthorized", http.StatusUnauthorized)
            return // next.ServeHTTP를 호출하지 않는다
        }

        userID, err := validateToken(token)
        if err != nil {
            http.Error(w, "invalid token", http.StatusUnauthorized)
            return
        }

        // 인증된 사용자 정보를 context에 저장
        ctx := context.WithValue(r.Context(), userIDKey{}, userID)
        next.ServeHTTP(w, r.WithContext(ctx))
    })
}
```

Express에서 `next()`를 호출하지 않으면 요청이 거기서 멈추는 것처럼, Go에서는 `next.ServeHTTP`를 호출하지 않고 `return`하면 된다. 차이는 명확하다 — Express는 `next()`를 빠뜨리면 실수인지 의도인지 구분하기 어렵지만, Go는 `return`으로 의도를 명시한다.

### context로 값 전달

Express에서 `req.user = decoded`처럼 요청 객체에 값을 직접 넣는 패턴이 흔하다. Go에서는 `context.WithValue`를 사용한다:

```go
// key 타입 정의 — 패키지 간 충돌 방지
type userIDKey struct{}

// 값 저장
ctx := context.WithValue(r.Context(), userIDKey{}, "user-123")
r = r.WithContext(ctx)

// 값 꺼내기
userID, ok := r.Context().Value(userIDKey{}).(string)
```

key에 빈 struct를 쓰는 이유는 타입 자체가 고유한 식별자가 되기 때문이다. 문자열 key를 쓰면 서로 다른 패키지에서 같은 문자열을 사용했을 때 충돌한다. Express의 `req.user`도 다른 미들웨어가 같은 속성명을 쓰면 덮어쓰는 문제가 있다. Go의 방식이 더 안전하다.

## CORS 미들웨어

CORS 미들웨어는 응답 헤더를 설정하고, preflight 요청(OPTIONS)을 처리한다:

```go
func cors(next http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        w.Header().Set("Access-Control-Allow-Origin", "*")
        w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE")
        w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")

        if r.Method == http.MethodOptions {
            w.WriteHeader(http.StatusNoContent)
            return
        }

        next.ServeHTTP(w, r)
    })
}
```

Express에서 `cors` 패키지가 하는 일과 동일하다. preflight 요청은 헤더만 설정하고 `204 No Content`로 응답한 뒤, 핸들러를 호출하지 않고 끝낸다.

## Recovery 미들웨어

Go의 HTTP 서버는 핸들러에서 panic이 발생하면 해당 goroutine이 종료된다. 서버 자체는 죽지 않지만, 클라이언트는 빈 응답을 받는다. recovery 미들웨어로 panic을 잡아서 500 응답을 반환한다:

```go
func recovery(next http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        defer func() {
            if err := recover(); err != nil {
                log.Printf("panic recovered: %v\n%s", err, debug.Stack())
                http.Error(w, "internal server error", http.StatusInternalServerError)
            }
        }()

        next.ServeHTTP(w, r)
    })
}
```

`defer`와 `recover`의 조합이다. 10편에서 다뤘듯 `recover`는 `defer` 내부에서만 동작한다. `next.ServeHTTP` 실행 중 panic이 발생하면 `defer` 함수가 실행되고, `recover`가 panic 값을 잡는다. `debug.Stack()`으로 스택 트레이스도 기록한다.

Express에서 같은 역할을 하는 에러 핸들러:

```javascript
app.use((err, req, res, next) => {
  console.error(err.stack);
  res.status(500).send("internal server error");
});
```

Express의 에러 핸들러는 인자가 4개인 미들웨어로 구분한다. Go는 `defer`/`recover`라는 언어 수준의 메커니즘을 사용한다.

## 미들웨어 조합

지금까지 만든 미들웨어를 조합하여 서버를 구성한다:

```go
func main() {
    mux := http.NewServeMux()

    // 공개 엔드포인트
    mux.HandleFunc("GET /health", func(w http.ResponseWriter, r *http.Request) {
        w.Write([]byte("ok"))
    })

    // 인증이 필요한 엔드포인트
    mux.Handle("GET /me", auth(http.HandlerFunc(handleMe)))
    mux.Handle("POST /posts", auth(http.HandlerFunc(handleCreatePost)))

    // 전역 미들웨어 적용
    handler := chain(mux, recovery, logging, cors)

    http.ListenAndServe(":8080", handler)
}
```

두 가지 수준에서 미들웨어를 적용했다:

1. **전역 미들웨어** — `chain`으로 모든 요청에 적용. recovery, logging, CORS.
2. **라우트별 미들웨어** — `auth(handler)` 형태로 특정 핸들러에만 적용.

Express에서도 같은 구분이 있다:

```javascript
// 전역
app.use(morgan("dev"));
app.use(corsMiddleware());

// 라우트별
app.get("/me", authMiddleware, handleMe);
```

Go에서 라우트별 미들웨어가 `mux.Handle`을 사용하는 것에 주의한다. `mux.HandleFunc`은 함수를 받지만, 미들웨어가 반환하는 것은 `http.Handler`이므로 `mux.Handle`을 써야 한다.

## 요청 lifecycle 정리

요청이 서버에 도착해서 응답이 나가기까지의 전체 흐름:

```
클라이언트 요청
  -> recovery (defer 설정)
    -> logging (시작 시간 기록)
      -> cors (헤더 설정, OPTIONS이면 여기서 반환)
        -> ServeMux (경로 매칭)
          -> auth (토큰 검증, 실패하면 여기서 반환)
            -> 핸들러 (비즈니스 로직, 응답 작성)
          <- auth
        <- ServeMux
      <- cors
    <- logging (처리 시간 계산, 로그 출력)
  <- recovery (panic이 있었으면 여기서 처리)
클라이언트 응답
```

이 흐름은 함수 call stack 그 자체다. 미들웨어마다 `next.ServeHTTP` 호출 전에 요청 전처리를, 호출 후에 응답 후처리를 수행한다.

## 서드파티 미들웨어와의 호환

`func(http.Handler) http.Handler` 시그니처는 Go 생태계의 사실상 표준이다. 서드파티 라이브러리도 이 시그니처를 따른다:

```go
import "github.com/rs/cors"

// rs/cors 라이브러리의 Handler 메서드는 func(http.Handler) http.Handler와 같은 역할
c := cors.New(cors.Options{
    AllowedOrigins: []string{"https://example.com"},
    AllowedMethods: []string{"GET", "POST"},
})

handler := c.Handler(mux)
```

chi, gorilla, alice 등 대부분의 라우터와 미들웨어 라이브러리가 `http.Handler`를 기반으로 동작한다. Express 생태계에서 미들웨어마다 `(req, res, next)` 시그니처를 따르는 것과 같은 관례다. 다만 Go는 이것이 표준 라이브러리의 interface에서 비롯된 것이므로 호환성이 더 강하다.
