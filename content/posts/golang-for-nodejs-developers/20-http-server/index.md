# HTTP 서버

Go의 HTTP 서버는 `net/http` 패키지 하나로 완결된다. 라우팅, 미들웨어, graceful shutdown까지 표준 라이브러리에 포함되어 있어서 Express 같은 프레임워크가 필요 없다. Go 1.22에서 `ServeMux`의 라우팅이 대폭 개선되면서 서드파티 라우터의 필요성이 더 줄었다.

## 최소한의 서버

Node.js의 `http.createServer`에 해당하는 가장 간단한 형태:

```go
package main

import (
    "fmt"
    "net/http"
)

func main() {
    http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
        fmt.Fprint(w, "Hello, World!")
    })
    http.ListenAndServe(":8080", nil)
}
```

`http.HandleFunc`은 경로와 핸들러 함수를 등록한다. `http.ListenAndServe`는 지정한 주소에서 요청을 받기 시작한다. 두 번째 인자 `nil`은 기본 `ServeMux`를 사용한다는 뜻이다.

핸들러 함수의 인자 순서가 Node.js와 반대다. Go는 `(w, r)` — 응답이 먼저, 요청이 나중이다.

## Handler interface

Go HTTP 서버의 핵심은 `http.Handler` interface다:

```go
type Handler interface {
    ServeHTTP(ResponseWriter, *Request)
}
```

메서드가 단 하나다. 08편에서 다뤘듯 Go의 interface는 작을수록 강력하다. `ServeHTTP` 메서드만 있으면 어떤 타입이든 HTTP 요청을 처리할 수 있다:

```go
type greeting struct {
    message string
}

func (g greeting) ServeHTTP(w http.ResponseWriter, r *http.Request) {
    fmt.Fprint(w, g.message)
}

func main() {
    http.Handle("/hello", greeting{message: "Hello!"})
    http.ListenAndServe(":8080", nil)
}
```

## HandlerFunc — 함수를 Handler로

매번 struct를 정의하는 건 번거롭다. `http.HandlerFunc`은 함수 시그니처를 `Handler` interface로 변환하는 어댑터다:

```go
type HandlerFunc func(ResponseWriter, *Request)

func (f HandlerFunc) ServeHTTP(w ResponseWriter, r *Request) {
    f(w, r)
}
```

`HandlerFunc`은 `func(ResponseWriter, *Request)` 시그니처를 가진 함수에 `ServeHTTP` 메서드를 부여한다. 함수를 타입으로 정의하고 그 타입에 메서드를 붙이는 Go의 특성을 활용한 패턴이다.

`http.HandleFunc`(패키지 수준 함수)는 내부에서 이 변환을 자동으로 수행한다:

```go
// 이 두 줄은 동일하다
http.Handle("/path", http.HandlerFunc(myHandler))
http.HandleFunc("/path", myHandler)
```

Express에서 `app.get('/path', (req, res) => { ... })`로 핸들러 함수를 직접 전달하는 것과 비슷하다. 차이는 Go가 interface라는 타입 시스템 위에서 이 변환을 수행한다는 점이다.

## ServeMux — 라우터

`http.ServeMux`는 Go의 내장 HTTP 라우터다. URL 패턴을 핸들러에 매핑한다. Go 1.22에서 메서드 매칭과 경로 파라미터가 추가되면서 실용성이 크게 올라갔다.

### 기본 라우팅

```go
mux := http.NewServeMux()

mux.HandleFunc("GET /posts", listPosts)
mux.HandleFunc("POST /posts", createPost)
mux.HandleFunc("GET /posts/{id}", getPost)

http.ListenAndServe(":8080", mux)
```

Express의 `app.get("/posts", handler)`와 거의 같은 패턴이다. `"GET /posts"`처럼 HTTP 메서드를 패턴 앞에 붙이는 것이 Go 1.22에서 추가된 문법이다. 이전에는 핸들러 내부에서 `r.Method`를 직접 확인해야 했다.

### 경로 파라미터

`{id}` 같은 와일드카드로 경로 파라미터를 캡처한다:

```go
func getPost(w http.ResponseWriter, r *http.Request) {
    id := r.PathValue("id")
    fmt.Fprintf(w, "Post ID: %s", id)
}
```

`r.PathValue("id")`로 값을 꺼낸다. Express의 `req.params.id`와 같다. Go 1.22 이전에는 이 기능이 없어서 gorilla/mux나 chi 같은 서드파티 라우터가 사실상 필수였다.

`{path...}` 형태로 나머지 경로를 캡처할 수도 있다:

```go
mux.HandleFunc("GET /files/{path...}", func(w http.ResponseWriter, r *http.Request) {
    filePath := r.PathValue("path")
    // /files/images/photo.webp -> filePath = "images/photo.webp"
    fmt.Fprintf(w, "File: %s", filePath)
})
```

### 패턴 우선순위

더 구체적인 패턴이 우선한다:

```go
mux.HandleFunc("GET /posts/{id}", getPost)     // 구체적
mux.HandleFunc("GET /posts/latest", getLatest)  // 더 구체적
```

`/posts/latest` 요청은 `getLatest`가 처리한다. 리터럴 경로가 와일드카드보다 우선하기 때문이다. Express와 달리 등록 순서에 영향을 받지 않고, 구체성만으로 판단한다.

## Request와 ResponseWriter

### http.Request

요청 정보를 담는 struct다. 주요 필드와 메서드:

```go
func handler(w http.ResponseWriter, r *http.Request) {
    // HTTP 메서드
    method := r.Method // "GET", "POST" 등

    // URL 경로
    path := r.URL.Path

    // 쿼리 파라미터
    page := r.URL.Query().Get("page") // ?page=2

    // 헤더
    contentType := r.Header.Get("Content-Type")

    // 요청 본문
    body, err := io.ReadAll(r.Body)
    defer r.Body.Close()
}
```

Express의 `req` 객체와 대응 관계:

| Node.js | Go |
|---|---|
| `req.method` | `r.Method` |
| `req.url` | `r.URL.Path` |
| `req.headers['content-type']` | `r.Header.Get("Content-Type")` |
| `req.query.page` (Express) | `r.URL.Query().Get("page")` |
| `req.params.id` (Express) | `r.PathValue("id")` |

`r.Body`는 `io.ReadCloser`다. 12편에서 다뤘던 `io.Reader`를 구현하므로, `io.ReadAll`이나 `json.NewDecoder` 등 Reader를 받는 모든 함수와 연결된다.

### http.ResponseWriter

응답을 작성하는 interface다:

```go
type ResponseWriter interface {
    Header() http.Header
    Write([]byte) (int, error)
    WriteHeader(statusCode int)
}
```

`io.Writer`를 포함하고 있어서 `fmt.Fprint`, `json.NewEncoder` 등과 자연스럽게 조합된다:

```go
func jsonHandler(w http.ResponseWriter, r *http.Request) {
    w.Header().Set("Content-Type", "application/json")
    w.WriteHeader(http.StatusOK)

    data := map[string]string{"message": "hello"}
    json.NewEncoder(w).Encode(data)
}
```

`WriteHeader`는 `Write`보다 먼저 호출해야 한다. `Write`를 먼저 호출하면 암묵적으로 `200 OK`가 전송된다.

## JSON API 서버 예제

실용적인 수준의 JSON API 서버를 만들어 본다:

```go
package main

import (
    "encoding/json"
    "fmt"
    "net/http"
    "sync"
)

type Todo struct {
    ID   string `json:"id"`
    Text string `json:"text"`
    Done bool   `json:"done"`
}

type TodoStore struct {
    mu    sync.Mutex
    todos map[string]Todo
    seq   int
}

func NewTodoStore() *TodoStore {
    return &TodoStore{todos: make(map[string]Todo)}
}

func (s *TodoStore) handleList(w http.ResponseWriter, r *http.Request) {
    s.mu.Lock()
    defer s.mu.Unlock()

    todos := make([]Todo, 0, len(s.todos))
    for _, t := range s.todos {
        todos = append(todos, t)
    }

    w.Header().Set("Content-Type", "application/json")
    json.NewEncoder(w).Encode(todos)
}

func (s *TodoStore) handleCreate(w http.ResponseWriter, r *http.Request) {
    var todo Todo
    if err := json.NewDecoder(r.Body).Decode(&todo); err != nil {
        http.Error(w, err.Error(), http.StatusBadRequest)
        return
    }

    s.mu.Lock()
    s.seq++
    todo.ID = fmt.Sprintf("%d", s.seq)
    s.todos[todo.ID] = todo
    s.mu.Unlock()

    w.Header().Set("Content-Type", "application/json")
    w.WriteHeader(http.StatusCreated)
    json.NewEncoder(w).Encode(todo)
}

func main() {
    store := NewTodoStore()
    mux := http.NewServeMux()

    mux.HandleFunc("GET /todos", store.handleList)
    mux.HandleFunc("POST /todos", store.handleCreate)

    http.ListenAndServe(":8080", mux)
}
```

이 코드에서 주목할 점:

1. `sync.Mutex`로 동시 접근을 보호한다. Go의 HTTP 서버는 각 요청을 별도 goroutine에서 처리하므로, 공유 상태가 있으면 동기화가 필요하다.
2. `json.NewDecoder`와 `json.NewEncoder`가 `io.Reader`/`io.Writer`를 활용한다. `r.Body`에서 직접 디코딩하고, `w`에 직접 인코딩한다.
3. `http.Error`는 에러 응답을 보내는 편의 함수다.

## 서버 타임아웃 설정

`http.ListenAndServe`는 간편하지만 타임아웃 설정이 없다. 프로덕션에서는 `http.Server`를 직접 구성한다:

```go
srv := &http.Server{
    Addr:         ":8080",
    Handler:      mux,
    ReadTimeout:  5 * time.Second,
    WriteTimeout: 10 * time.Second,
    IdleTimeout:  120 * time.Second,
}

srv.ListenAndServe()
```

각 타임아웃의 의미:

| 타임아웃 | 설명 |
|---|---|
| `ReadTimeout` | 요청 헤더 + 본문을 읽는 데 허용되는 시간 |
| `WriteTimeout` | 응답을 작성하는 데 허용되는 시간 |
| `IdleTimeout` | keep-alive 연결에서 다음 요청까지 대기 시간 |

타임아웃을 설정하지 않으면 느린 클라이언트가 연결을 무한히 점유할 수 있다.

## Graceful Shutdown

서버를 종료할 때 처리 중인 요청을 갑자기 끊으면 안 된다. 진행 중인 요청이 완료될 때까지 기다린 후 종료하는 것이 graceful shutdown이다:

```go
package main

import (
    "context"
    "log"
    "net/http"
    "os"
    "os/signal"
    "syscall"
    "time"
)

func main() {
    mux := http.NewServeMux()
    mux.HandleFunc("GET /", func(w http.ResponseWriter, r *http.Request) {
        time.Sleep(2 * time.Second) // 느린 요청 시뮬레이션
        w.Write([]byte("done"))
    })

    srv := &http.Server{
        Addr:    ":8080",
        Handler: mux,
    }

    // 별도 goroutine에서 서버 시작
    go func() {
        log.Println("서버 시작: :8080")
        if err := srv.ListenAndServe(); err != http.ErrServerClosed {
            log.Fatal(err)
        }
    }()

    // 종료 시그널 대기
    quit := make(chan os.Signal, 1)
    signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
    <-quit
    log.Println("종료 시그널 수신")

    // 5초 내에 graceful shutdown
    ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
    defer cancel()

    if err := srv.Shutdown(ctx); err != nil {
        log.Fatal("강제 종료:", err)
    }
    log.Println("서버 종료 완료")
}
```

`srv.Shutdown`은 새로운 연결 수락을 중단하고, 진행 중인 요청이 완료될 때까지 기다린다. context의 타임아웃 내에 완료되지 않으면 강제 종료한다.

Node.js의 `server.close`와 개념이 동일하지만, Go는 context로 타임아웃을 명시적으로 제어한다는 점이 다르다.

`Handler` interface와 `io.Reader`/`io.Writer`의 조합은 Go 표준 라이브러리의 설계 철학 -- 작은 interface를 합성하여 큰 기능을 만드는 -- 이 HTTP 서버에서 실현된 결과다.
