# 입출력

Go의 I/O는 두 개의 interface 위에 서 있다. `io.Reader`와 `io.Writer`. 각각 메서드가 하나뿐이다. 08편에서 작은 interface의 위력을 다뤘는데, 그 철학이 가장 극적으로 드러나는 곳이 `io` 패키지다. Node.js의 Stream과 비교하면 설계 방향의 차이가 선명해진다.

## io.Reader와 io.Writer

08편에서 이미 시그니처를 봤다:

```go
type Reader interface {
    Read(p []byte) (n int, err error)
}

type Writer interface {
    Write(p []byte) (n int, err error)
}
```

`Reader`는 `Read` 하나, `Writer`는 `Write` 하나. 이것이 전부다.

`Read`의 동작 방식을 보면, 호출하는 쪽이 byte slice를 할당해서 넘긴다. `Read`는 그 slice에 데이터를 채우고, 몇 바이트를 읽었는지(`n`)와 에러를 반환한다. 데이터 끝에 도달하면 `io.EOF`를 반환한다:

```go
func main() {
    r := strings.NewReader("hello, world")
    buf := make([]byte, 4)

    for {
        n, err := r.Read(buf)
        if n > 0 {
            fmt.Print(string(buf[:n]))
        }
        if err == io.EOF {
            break
        }
        if err != nil {
            log.Fatal(err)
        }
    }
    // 출력: hello, world
}
```

버퍼 크기가 4이므로 `Read`가 여러 번 호출된다. "hell", "o, w", "orld" 순서로 읽힌다. 호출하는 쪽이 버퍼를 제공하고 루프를 돌린다. 이것이 pull 기반이다.

`Write`는 방향이 반대다. byte slice를 받아서 어딘가에 쓴다:

```go
func main() {
    f, err := os.Create("output.txt")
    if err != nil {
        log.Fatal(err)
    }
    defer f.Close()

    n, err := f.Write([]byte("hello, world"))
    if err != nil {
        log.Fatal(err)
    }
    fmt.Printf("%d bytes written\n", n) // 12 bytes written
}
```

## 왜 이 두 interface가 강력한가

`io.Reader`를 구현하는 타입이 표준 라이브러리에만 수십 개다:

- `os.File` — 파일
- `net.Conn` — 네트워크 연결
- `http.Request.Body` — HTTP 요청 본문
- `bytes.Buffer` — 메모리 버퍼
- `strings.Reader` — 문자열
- `gzip.Reader` — gzip 압축 해제 스트림
- `crypto/aes` — 암호화 스트림

이들은 서로 아무 관계가 없다. 파일과 네트워크 연결과 gzip 스트림은 완전히 다른 것이다. 하지만 모두 `Read(p []byte) (n int, err error)`를 구현한다. 그래서 `io.Reader`를 받는 함수 하나로 이 모든 데이터 소스를 처리할 수 있다.

08편에서 "interface는 작을수록 더 많은 타입이 만족한다"고 했다. `io.Reader`가 메서드 하나짜리라서 이 보편성이 가능하다. `Read` 하나에 `Seek`, `Close`, `ReadAt`까지 넣었다면, 이 중 일부만 지원하는 타입은 탈락한다.

필요하면 합성한다:

```go
type ReadCloser interface {
    Reader
    Closer
}

type ReadWriteSeeker interface {
    Reader
    Writer
    Seeker
}
```

작은 단위를 조합해서 필요한 만큼만 요구한다. 08편의 interface composition이 실전에서 작동하는 모습이다.

## io.Copy — Reader에서 Writer로

가장 자주 쓰이는 유틸리티 함수다. `Reader`에서 읽어서 `Writer`로 쓴다:

```go
func main() {
    r := strings.NewReader("hello, world")
    n, err := io.Copy(os.Stdout, r)
    if err != nil {
        log.Fatal(err)
    }
    fmt.Printf("\n%d bytes copied\n", n)
    // 출력:
    // hello, world
    // 12 bytes copied
}
```

시그니처가 `func Copy(dst Writer, src Reader) (written int64, err error)`다. 어떤 `Reader`든 어떤 `Writer`로든 복사할 수 있다. 파일을 네트워크로, 네트워크를 파일로, HTTP 응답을 파일로. 조합이 자유롭다.

파일 복사가 이렇게 간결해진다:

```go
func copyFile(src, dst string) error {
    in, err := os.Open(src)
    if err != nil {
        return err
    }
    defer in.Close()

    out, err := os.Create(dst)
    if err != nil {
        return err
    }
    defer out.Close()

    _, err = io.Copy(out, in)
    return err
}
```

## io.TeeReader — 읽으면서 복사

`io.TeeReader`는 Unix의 `tee` 명령과 같다. `Reader`에서 읽은 데이터를 `Writer`에도 동시에 쓴다:

```go
func main() {
    r := strings.NewReader("hello, world")
    var buf bytes.Buffer

    tee := io.TeeReader(r, &buf)

    // tee에서 읽으면 buf에도 기록된다
    data, err := io.ReadAll(tee)
    if err != nil {
        log.Fatal(err)
    }

    fmt.Println("read:", string(data))      // read: hello, world
    fmt.Println("buffered:", buf.String())   // buffered: hello, world
}
```

HTTP 응답 본문을 처리하면서 동시에 로그에 남기는 등의 상황에서 유용하다. 데이터를 메모리에 전부 올리지 않고 스트리밍으로 처리할 수 있다.

## io.MultiWriter — 여러 곳에 동시에 쓰기

하나의 `Write` 호출이 여러 `Writer`에 동시에 전달된다:

```go
func main() {
    var buf1, buf2 bytes.Buffer
    multi := io.MultiWriter(&buf1, &buf2)

    fmt.Fprintln(multi, "hello")

    fmt.Println("buf1:", buf1.String()) // buf1: hello
    fmt.Println("buf2:", buf2.String()) // buf2: hello
}
```

파일과 stdout에 동시에 쓰거나, 여러 로그 목적지에 동시에 출력하는 패턴에 자연스럽게 적용된다.

## bufio — 버퍼링된 I/O

`io.Reader`와 `io.Writer`는 호출할 때마다 시스템 콜이 발생할 수 있다. 1바이트씩 읽으면 1바이트마다 시스템 콜이다. `bufio`는 내부 버퍼를 두어 시스템 콜 횟수를 줄인다.

```go
func main() {
    f, err := os.Open("data.txt")
    if err != nil {
        log.Fatal(err)
    }
    defer f.Close()

    scanner := bufio.NewScanner(f)
    for scanner.Scan() {
        fmt.Println(scanner.Text()) // 한 줄씩 읽기
    }
    if err := scanner.Err(); err != nil {
        log.Fatal(err)
    }
}
```

`bufio.Scanner`는 줄 단위 읽기에 최적화되어 있다. Node.js의 `readline` 모듈과 비슷한 역할이다:

```javascript
// Node.js
const rl = readline.createInterface({ input: fs.createReadStream("data.txt") });
for await (const line of rl) {
  console.log(line);
}
```

`bufio.NewReader`와 `bufio.NewWriter`도 있다. 기존 `Reader`나 `Writer`를 감싸서 버퍼링을 추가한다:

```go
func main() {
    f, err := os.Create("output.txt")
    if err != nil {
        log.Fatal(err)
    }
    defer f.Close()

    w := bufio.NewWriter(f)
    fmt.Fprintln(w, "line 1")
    fmt.Fprintln(w, "line 2")
    w.Flush() // 버퍼에 남은 데이터를 Writer에 쓴다
}
```

`bufio.NewWriter`를 쓸 때는 반드시 `Flush`를 호출해야 한다. 버퍼에 남아 있는 데이터가 실제 `Writer`에 기록되지 않고 유실될 수 있다.

## 파일 읽기/쓰기

Go에서 파일을 다루는 기본 함수는 `os.Open`과 `os.Create`다:

```go
// 읽기 전용으로 열기
f, err := os.Open("config.json")

// 쓰기용으로 생성 (파일이 있으면 덮어쓴다)
f, err := os.Create("output.txt")

// 세밀한 제어가 필요하면 os.OpenFile
f, err := os.OpenFile("app.log", os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
```

`os.File`은 `io.Reader`, `io.Writer`, `io.Closer`를 모두 만족한다. 그래서 위에서 본 `io.Copy`, `bufio.Scanner` 등과 자연스럽게 결합된다.

파일 전체를 한 번에 읽거나 쓰는 간편 함수도 있다:

```go
// 파일 전체 읽기
data, err := os.ReadFile("config.json")

// 파일 전체 쓰기
err := os.WriteFile("output.txt", []byte("hello"), 0644)
```

Node.js에서 같은 작업:

```javascript
// Node.js
const data = await fs.promises.readFile("config.json");
await fs.promises.writeFile("output.txt", "hello");
```

`os.ReadFile`과 `os.WriteFile`은 편리하지만 파일 전체를 메모리에 올린다. 큰 파일은 `os.Open`과 `io.Copy` 조합으로 스트리밍 처리하는 것이 맞다.

## strings.NewReader와 bytes.Buffer

테스트에서 빛나는 타입들이다.

`strings.NewReader`는 문자열을 `io.Reader`로 변환한다:

```go
func process(r io.Reader) error {
    data, err := io.ReadAll(r)
    if err != nil {
        return err
    }
    fmt.Println(string(data))
    return nil
}

func main() {
    // 프로덕션: 파일에서 읽기
    f, _ := os.Open("data.txt")
    process(f)
    f.Close()

    // 테스트: 문자열에서 읽기
    process(strings.NewReader("test data"))
}
```

함수가 `io.Reader`를 받으므로, 프로덕션에서는 파일을 넘기고 테스트에서는 `strings.NewReader`를 넘긴다. mock 라이브러리가 필요 없다. 작은 interface의 실용적 효과다.

`bytes.Buffer`는 `io.Reader`와 `io.Writer`를 모두 만족한다:

```go
func main() {
    var buf bytes.Buffer

    // Writer로 사용
    fmt.Fprintln(&buf, "hello")
    buf.WriteString("world\n")

    // Reader로 사용
    data, _ := io.ReadAll(&buf)
    fmt.Print(string(data))
    // 출력:
    // hello
    // world
}
```

`bytes.Buffer`는 `io.Writer`를 받는 함수의 출력을 캡처할 때 특히 유용하다:

```go
func render(w io.Writer, name string) {
    fmt.Fprintf(w, "Hello, %s!", name)
}

func TestRender(t *testing.T) {
    var buf bytes.Buffer
    render(&buf, "Alice")
    if buf.String() != "Hello, Alice!" {
        t.Errorf("got %q", buf.String())
    }
}
```

## Node.js Stream과의 비교

Node.js의 Stream은 네 가지 타입이 있다:

```javascript
// Node.js Stream 타입
Readable   // 데이터를 읽는 소스
Writable   // 데이터를 쓰는 목적지
Duplex     // 읽기 + 쓰기 (TCP 소켓)
Transform  // 읽기 + 쓰기 + 변환 (gzip)
```

Go에는 `io.Reader`와 `io.Writer` 두 가지뿐이다. Duplex는 `io.ReadWriter`(Reader + Writer embedding), Transform은 `io.Reader`를 감싸서 새 `io.Reader`를 반환하는 패턴으로 처리한다.

### 이벤트 기반 vs pull 기반

Node.js Stream은 이벤트 기반이다:

```javascript
// Node.js - 이벤트 기반
const readable = fs.createReadStream("large.txt");

readable.on("data", (chunk) => {
  console.log("received:", chunk.length);
});

readable.on("end", () => {
  console.log("done");
});

readable.on("error", (err) => {
  console.error(err);
});
```

데이터가 준비되면 `data` 이벤트가 발생한다. 끝나면 `end`, 에러가 나면 `error`. 이벤트 핸들러를 등록하고 기다린다. push 모델이다 — 데이터가 밀려온다.

Go의 `io.Reader`는 pull 기반이다:

```go
// Go - pull 기반
f, err := os.Open("large.txt")
if err != nil {
    log.Fatal(err)
}
defer f.Close()

buf := make([]byte, 1024)
for {
    n, err := f.Read(buf)
    if n > 0 {
        fmt.Printf("received: %d\n", n)
    }
    if err == io.EOF {
        fmt.Println("done")
        break
    }
    if err != nil {
        log.Fatal(err)
    }
}
```

호출하는 쪽이 `Read`를 호출해서 데이터를 당겨온다. 준비될 때까지 블로킹된다. 이벤트 루프도, 콜백도, 이벤트 이름도 없다. 일반적인 for 루프와 if 분기만으로 흐름을 제어한다.

### backpressure

Node.js Stream에서 backpressure는 복잡한 주제다. Writable의 내부 버퍼가 가득 차면 `write()`가 `false`를 반환하고, `drain` 이벤트를 기다려야 한다. 이걸 제대로 처리하지 않으면 메모리 사용량이 폭증한다:

```javascript
// Node.js - backpressure 처리
readable.on("data", (chunk) => {
  const ok = writable.write(chunk);
  if (!ok) {
    readable.pause();
    writable.once("drain", () => readable.resume());
  }
});
```

`pipeline`이나 `pipe`를 쓰면 자동으로 처리되지만, 직접 스트림을 다루면 실수하기 쉽다.

Go에서는 backpressure가 자연스럽게 해결된다. `Read`와 `Write`가 블로킹 호출이기 때문이다. Writer가 느리면 `Write`가 느리게 반환되고, 그동안 `Read`가 호출되지 않는다. 별도의 메커니즘이 필요 없다:

```go
// Go - backpressure가 자동으로 처리된다
io.Copy(dst, src)
```

`io.Copy`는 내부적으로 고정 크기 버퍼를 사용해서 `src.Read` → `dst.Write`를 반복한다. `Write`가 완료될 때까지 다음 `Read`를 하지 않으므로, 메모리 사용량이 버퍼 크기를 넘지 않는다.

### pipe

Node.js에서 스트림을 연결하는 방법:

```javascript
// Node.js
const { pipeline } = require("stream/promises");
await pipeline(
  fs.createReadStream("input.txt"),
  zlib.createGzip(),
  fs.createWriteStream("output.gz")
);
```

Go에서 같은 작업:

```go
func main() {
    in, err := os.Open("input.txt")
    if err != nil {
        log.Fatal(err)
    }
    defer in.Close()

    out, err := os.Create("output.gz")
    if err != nil {
        log.Fatal(err)
    }
    defer out.Close()

    gw := gzip.NewWriter(out)
    defer gw.Close()

    _, err = io.Copy(gw, in)
    if err != nil {
        log.Fatal(err)
    }
}
```

`gzip.NewWriter`는 `io.Writer`를 받아서 새 `io.Writer`를 반환한다. 쓰기를 하면 gzip 압축 후 원래 `Writer`에 전달된다. Node.js의 Transform 스트림과 같은 역할이지만, 별도의 스트림 타입이 아니라 `io.Writer`를 감싸는 패턴이다.

반대로 `gzip.NewReader`는 `io.Reader`를 받아서 새 `io.Reader`를 반환한다:

```go
func main() {
    f, err := os.Open("data.gz")
    if err != nil {
        log.Fatal(err)
    }
    defer f.Close()

    gr, err := gzip.NewReader(f)
    if err != nil {
        log.Fatal(err)
    }
    defer gr.Close()

    _, err = io.Copy(os.Stdout, gr)
    if err != nil {
        log.Fatal(err)
    }
}
```

Reader를 감싸서 Reader를 반환하고, Writer를 감싸서 Writer를 반환한다. 이 패턴이 decorator처럼 겹겹이 쌓인다. 암호화, 압축, 버퍼링, 로깅 등을 각각 독립적으로 구현하고, `io.Copy` 하나로 연결한다.

## 정리

| 개념 | Node.js | Go |
|---|---|---|
| 핵심 추상화 | Stream (Readable, Writable, Duplex, Transform) | `io.Reader`, `io.Writer` |
| 데이터 흐름 | push 기반 (이벤트) | pull 기반 (블로킹 호출) |
| backpressure | `drain` 이벤트, `pause`/`resume` | 블로킹 호출로 자동 해결 |
| 스트림 연결 | `pipe`, `pipeline` | `io.Copy`, Writer/Reader wrapping |
| 변환 | Transform 스트림 | Reader/Writer를 감싸는 패턴 |
| 버퍼링 | Stream 내장 버퍼 | `bufio` 패키지 |
| 파일 전체 읽기 | `fs.readFile` | `os.ReadFile` |
| 줄 단위 읽기 | `readline` 모듈 | `bufio.Scanner` |
| 테스트용 소스 | 커스텀 Readable 구현 | `strings.NewReader` |
| 테스트용 싱크 | 배열에 push | `bytes.Buffer` |

Node.js의 Stream은 기능이 풍부하다. 네 가지 타입, 이벤트 시스템, 자동 backpressure(`pipeline`), object mode 등 다양한 기능을 제공한다. 반면 학습 곡선이 가파르고, `data`/`end`/`error`/`drain`/`close`/`finish` 등 이벤트 조합을 정확히 이해해야 올바르게 사용할 수 있다.

Go의 I/O는 메서드 하나짜리 interface 두 개가 전부다. 이벤트 루프도 콜백도 없다. for 루프와 if 분기로 데이터를 읽고 쓴다. 이 단순함이 가능한 이유는 Go가 동시성을 goroutine으로 처리하기 때문이다. 블로킹 I/O가 goroutine을 블로킹할 뿐 OS 스레드를 블로킹하지 않으므로, 비동기 I/O의 복잡성 없이도 높은 동시성을 달성할 수 있다. 이 부분은 concurrency 편에서 자세히 다룬다.
