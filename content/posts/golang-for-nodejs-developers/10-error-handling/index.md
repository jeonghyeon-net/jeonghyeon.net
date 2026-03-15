# 에러 처리

Go에서 에러는 값이다. 던지고 잡는 것이 아니라 반환하고 검사한다. Node.js의 try/catch와 Promise rejection에 익숙한 상태에서 이 전환은 단순한 문법 변화가 아니라 멘탈 모델의 전환이다.

## billion dollar mistake

2009년 QCon London에서 Tony Hoare가 청중 앞에 서서 사과했다. "I call it my billion-dollar mistake." 1965년 ALGOL W의 타입 시스템을 설계하면서 null reference를 도입한 것에 대한 고백이었다. 구현이 너무 쉬웠기 때문에 유혹을 뿌리치지 못했다고 했다. 그 결과 수십 년간 셀 수 없는 에러, 취약점, 시스템 크래시가 발생했다.

null의 문제는 타입 시스템에 구멍을 낸다는 것이다. 어떤 참조 타입이든 null이 될 수 있으므로, 데이터를 사용할 때마다 null인지 확인해야 한다. 대부분의 프로그래머는 이 확인을 빠뜨리거나 잊는다.

JavaScript도 이 문제에서 자유롭지 않다. `null`과 `undefined` 두 가지가 있어서 상황이 더 복잡하다:

```javascript
function getUser(id) {
  // 반환값이 User일 수도, null일 수도, undefined일 수도 있다
  return db.find(id);
}

const user = getUser(123);
user.name; // TypeError: Cannot read properties of null
```

Go는 이 문제를 다른 방식으로 접근한다. 에러가 발생할 수 있는 함수는 반드시 error를 반환하도록 하고, 호출하는 쪽이 명시적으로 처리하게 만든다. 에러가 "보이지 않는 경로"로 전파되지 않는다.

## error는 interface다

Go의 `error`는 특별한 키워드가 아니다. 표준 라이브러리에 정의된 interface다:

```go
type error interface {
    Error() string
}
```

`Error() string` 메서드 하나만 있으면 error다. 08편에서 다룬 implicit satisfaction이 그대로 적용된다. 어떤 타입이든 `Error()` 메서드를 구현하면 error로 사용할 수 있다.

가장 간단한 error 생성 방법:

```go
import "errors"

err := errors.New("something went wrong")
fmt.Println(err.Error()) // something went wrong
fmt.Println(err)         // something went wrong (fmt.Println이 Error()를 호출)
```

`fmt.Errorf`로 포맷팅된 error를 만들 수도 있다:

```go
name := "config.json"
err := fmt.Errorf("file not found: %s", name)
fmt.Println(err) // file not found: config.json
```

## if err != nil

Go 코드에서 가장 자주 보이는 패턴이다:

```go
f, err := os.Open("config.json")
if err != nil {
    return fmt.Errorf("failed to open config: %w", err)
}
defer f.Close()

data, err := io.ReadAll(f)
if err != nil {
    return fmt.Errorf("failed to read config: %w", err)
}
```

Node.js에서 같은 작업:

```javascript
try {
  const f = await fs.promises.open("config.json");
  try {
    const data = await f.readFile();
    // data 사용
  } finally {
    await f.close();
  }
} catch (err) {
  throw new Error(`failed to handle config: ${err.message}`);
}
```

Go 코드가 더 길다. `if err != nil`이 반복된다. 이것이 Go 에러 처리에 대한 가장 흔한 비판이다.

Go 팀은 이 장황함을 의도적인 선택이라고 설명한다. 공식 FAQ의 표현을 빌리면:

> We believe that coupling exceptions to a control structure, as in the try-catch-finally idiom, results in convoluted code. It also tends to encourage programmers to label too many ordinary errors, such as failing to open a file, as exceptional.

파일을 여는 것이 실패하는 건 "예외적인 상황"이 아니다. 파일이 없을 수 있고, 권한이 없을 수 있고, 디스크가 가득 찰 수 있다. 이런 일상적인 실패를 exception으로 처리하면, 정말 예외적인 상황(메모리 부족, 스택 오버플로우)과 구분이 흐려진다.

### 장점

`if err != nil`의 장점은 에러 경로가 코드에 명시적으로 드러난다는 것이다:

```go
user, err := db.FindUser(id)
if err != nil {
    return nil, fmt.Errorf("find user %d: %w", id, err)
}

orders, err := db.FindOrders(user.ID)
if err != nil {
    return nil, fmt.Errorf("find orders for user %d: %w", user.ID, err)
}
```

이 코드를 읽으면 어떤 함수가 실패할 수 있는지, 실패하면 어떤 메시지와 함께 반환되는지가 한눈에 보인다. try/catch에서는 try 블록 안의 어떤 줄이든 throw할 수 있고, 어떤 에러가 catch에 도달하는지 추적해야 한다.

### 단점

반복이 많다. 함수 하나에 `if err != nil`이 다섯 번, 열 번 나오기도 한다. 이 반복이 진짜 로직을 가리는 경우가 있다. Go 커뮤니티에서도 이에 대한 논의가 지속적으로 있었고, error handling 문법 개선 제안이 여러 차례 나왔지만 아직 채택된 것은 없다.

## error wrapping

Go 1.13에서 도입된 error wrapping은 에러에 맥락을 추가하면서 원본 에러를 보존하는 메커니즘이다.

### fmt.Errorf와 %w

```go
func readConfig(path string) ([]byte, error) {
    data, err := os.ReadFile(path)
    if err != nil {
        return nil, fmt.Errorf("read config %s: %w", path, err)
    }
    return data, nil
}
```

`%w` verb가 핵심이다. `%v`나 `%s`를 쓰면 에러 메시지만 문자열로 포함되지만, `%w`를 쓰면 원본 에러가 wrapping된다. wrapping된 에러는 나중에 `errors.Is`와 `errors.As`로 검사할 수 있다.

Node.js에서 비슷한 패턴:

```javascript
try {
  const data = await fs.promises.readFile(path);
  return data;
} catch (err) {
  const wrapped = new Error(`read config ${path}: ${err.message}`);
  wrapped.cause = err; // ES2022 Error cause
  throw wrapped;
}
```

ES2022에서 `Error.cause`가 도입되기 전까지 JavaScript에는 에러 wrapping의 표준 방법이 없었다. Go는 1.13(2019년)부터 언어 레벨에서 지원했다.

### errors.Is

wrapping된 에러 체인에서 특정 에러를 찾는다:

```go
var ErrNotExist = errors.New("file does not exist")

func readFile(path string) ([]byte, error) {
    data, err := os.ReadFile(path)
    if err != nil {
        return nil, fmt.Errorf("readFile: %w", err)
    }
    return data, nil
}

func main() {
    _, err := readFile("missing.txt")
    if errors.Is(err, os.ErrNotExist) {
        fmt.Println("file not found")
        return
    }
    if err != nil {
        log.Fatal(err)
    }
}
```

`errors.Is`는 에러 체인을 따라가면서 비교한다. `err`가 `os.ErrNotExist`를 wrapping하고 있으므로 `true`를 반환한다. 단순히 `err == os.ErrNotExist`로 비교하면 wrapping된 에러를 찾지 못한다.

Node.js에서 에러 종류를 구분하는 방법과 비교하면:

```javascript
try {
  await fs.promises.readFile("missing.txt");
} catch (err) {
  if (err.code === "ENOENT") {
    console.log("file not found");
  } else {
    throw err;
  }
}
```

JavaScript는 에러 객체의 property(`.code`, `.name`)로 구분한다. Go는 에러 값 자체를 비교한다.

### errors.As

에러 체인에서 특정 타입의 에러를 찾아 변환한다:

```go
var pathErr *os.PathError
if errors.As(err, &pathErr) {
    fmt.Println("failed path:", pathErr.Path)
}
```

`errors.Is`가 값 비교라면, `errors.As`는 타입 비교다. 08편에서 다룬 type assertion과 비슷하지만, wrapping된 에러 체인 전체를 탐색한다는 점이 다르다.

## sentinel error

package 레벨에서 미리 정의해둔 error 값을 sentinel error라고 부른다. 표준 라이브러리에 많이 있다:

```go
// io package
var EOF = errors.New("EOF")

// sql package
var ErrNoRows = errors.New("sql: no rows in result set")
```

`io.EOF`가 대표적이다. 파일이나 스트림의 끝에 도달했을 때 반환된다:

```go
reader := strings.NewReader("hello")
buf := make([]byte, 10)

for {
    n, err := reader.Read(buf)
    if errors.Is(err, io.EOF) {
        break
    }
    if err != nil {
        log.Fatal(err)
    }
    fmt.Print(string(buf[:n]))
}
```

sentinel error의 네이밍 관례: `Err`로 시작한다. `ErrNotFound`, `ErrTimeout`, `ErrInvalidInput` 등. `io.EOF`는 이 관례의 예외인데, 워낙 오래전에 만들어졌기 때문이다.

JavaScript에서 비슷한 패턴을 찾자면 Node.js의 에러 코드다:

```javascript
// Node.js 에러 코드
if (err.code === "ECONNREFUSED") { /* ... */ }
if (err.code === "ENOENT") { /* ... */ }
```

문자열 비교 대신 값 비교를 한다는 점에서 Go의 sentinel error가 더 타입 안전하다.

## custom error 타입

`Error() string` 메서드만 구현하면 어떤 타입이든 error가 된다:

```go
type ValidationError struct {
    Field   string
    Message string
}

func (e *ValidationError) Error() string {
    return fmt.Sprintf("validation failed on %s: %s", e.Field, e.Message)
}

func validateAge(age int) error {
    if age < 0 {
        return &ValidationError{
            Field:   "age",
            Message: "must be non-negative",
        }
    }
    return nil
}

func main() {
    err := validateAge(-1)
    if err != nil {
        var ve *ValidationError
        if errors.As(err, &ve) {
            fmt.Println("field:", ve.Field)     // field: age
            fmt.Println("message:", ve.Message) // message: must be non-negative
        }
    }
}
```

custom error 타입은 에러에 구조화된 정보를 담을 때 유용하다. HTTP 상태 코드, 실패한 필드 이름, 재시도 가능 여부 등을 에러 자체에 포함할 수 있다.

Node.js에서도 비슷한 패턴을 쓴다:

```javascript
class ValidationError extends Error {
  constructor(field, message) {
    super(`validation failed on ${field}: ${message}`);
    this.field = field;
  }
}
```

차이점은 JavaScript에서 `instanceof`로 검사하는 것과 Go에서 `errors.As`로 검사하는 것이다. Go의 `errors.As`는 wrapping된 에러 체인을 따라가므로, 에러가 여러 번 wrapping되어도 원본 타입을 찾을 수 있다.

## panic과 recover

Go에도 프로그램을 즉시 중단시키는 메커니즘이 있다. `panic`과 `recover`다.

```go
func mustParseInt(s string) int {
    n, err := strconv.Atoi(s)
    if err != nil {
        panic(fmt.Sprintf("invalid integer: %s", s))
    }
    return n
}
```

`panic`이 호출되면 현재 함수의 실행이 즉시 중단되고, defer된 함수들이 실행된 후 호출자에게 전파된다. 이 과정이 goroutine의 call stack 최상단까지 올라가면 프로그램이 크래시한다.

`recover`는 defer 안에서 panic을 잡는다:

```go
func safeDiv(a, b int) (result int, err error) {
    defer func() {
        if r := recover(); r != nil {
            err = fmt.Errorf("recovered: %v", r)
        }
    }()
    return a / b, nil // b가 0이면 panic
}

func main() {
    result, err := safeDiv(10, 0)
    if err != nil {
        fmt.Println(err) // recovered: runtime error: integer divide by zero
        return
    }
    fmt.Println(result)
}
```

형태만 보면 try/catch와 비슷하다. 하지만 용도가 근본적으로 다르다.

### panic을 써도 되는 경우

거의 없다. Go 커뮤니티의 관례는 명확하다:

- 프로그램 초기화 시 필수 설정이 없는 경우 (`log.Fatal`이 더 일반적)
- 프로그래머의 실수를 나타내는 경우 (잘못된 정규표현식 패턴 등)
- 표준 라이브러리의 `Must` 접두사 함수들 (`regexp.MustCompile`, `template.Must`)

```go
// Must 패턴: 컴파일 타임에 확정되는 값에만 사용
var emailRegex = regexp.MustCompile(`^[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\.[a-zA-Z]{2,}$`)
```

`MustCompile`은 내부적으로 컴파일에 실패하면 panic한다. 정규표현식 패턴이 코드에 하드코딩되어 있으므로 컴파일 실패는 프로그래머의 실수다. 런타임에 동적으로 생성되는 패턴이라면 `regexp.Compile`을 쓰고 error를 처리해야 한다.

### panic을 쓰면 안 되는 경우

네트워크 요청 실패, 파일 읽기 실패, 잘못된 사용자 입력 등 런타임에 충분히 일어날 수 있는 상황에서는 panic 대신 error를 반환한다:

```go
// 나쁜 예: 런타임 실패에 panic
func getUser(id int) *User {
    user, err := db.Find(id)
    if err != nil {
        panic(err) // 서버가 크래시한다
    }
    return user
}

// 좋은 예: error를 반환
func getUser(id int) (*User, error) {
    user, err := db.Find(id)
    if err != nil {
        return nil, fmt.Errorf("get user %d: %w", id, err)
    }
    return user, nil
}
```

Express에서 에러가 발생하면 error middleware가 잡아서 500 응답을 보낸다. 서버는 계속 실행된다. Go에서 panic이 발생하면 (recover하지 않는 한) 프로세스 자체가 종료된다. 이것이 panic을 일상적인 에러 처리에 쓰면 안 되는 이유다.

## 에러 전파 방식의 차이

Node.js(Express)에서 에러가 전파되는 경로:

```javascript
// Express: 에러가 middleware 체인을 따라 올라간다
app.get("/users/:id", async (req, res, next) => {
  try {
    const user = await findUser(req.params.id);
    const orders = await findOrders(user.id);
    res.json({ user, orders });
  } catch (err) {
    next(err); // error middleware로 전달
  }
});

// error middleware (모든 에러가 여기로 온다)
app.use((err, req, res, next) => {
  console.error(err.stack);
  res.status(500).json({ error: "Internal Server Error" });
});
```

이 구조에서는 `findUser`와 `findOrders` 중 어디에서 에러가 발생했는지 catch 블록만 보고는 알 수 없다. 모든 에러가 같은 catch로 흘러든다.

Go에서는 에러가 call stack을 따라 명시적으로 올라간다:

```go
func handleGetUser(w http.ResponseWriter, r *http.Request) {
    user, err := findUser(r.PathValue("id"))
    if err != nil {
        http.Error(w, "user not found", http.StatusNotFound)
        return
    }

    orders, err := findOrders(user.ID)
    if err != nil {
        http.Error(w, "failed to load orders", http.StatusInternalServerError)
        return
    }

    json.NewEncoder(w).Encode(map[string]any{"user": user, "orders": orders})
}
```

각 함수 호출 직후에 에러를 처리한다. `findUser` 실패와 `findOrders` 실패에 대해 다른 응답을 보낼 수 있다. 에러가 "보이지 않는 경로"로 전파되지 않으므로, 코드를 읽는 것만으로 에러 흐름을 완전히 파악할 수 있다.

## try/catch를 배제한 이유

Go 팀이 exception을 도입하지 않은 것은 기술적 한계가 아니라 의도적인 설계 결정이다. exception의 비용이 비싸서가 아니라, exception이 만드는 코드 구조가 장기 유지보수에 해롭다고 판단한 것이다.

try/catch의 근본적인 문제는 에러 경로를 숨긴다는 것이다. try 블록 안에 열 줄의 코드가 있으면 어떤 줄이 throw할 수 있는지, 어떤 타입의 exception이 날아올 수 있는지 코드만 보고는 알 수 없다. Java는 checked exception으로 이 문제를 해결하려 했지만, 결과적으로 개발자들이 `catch (Exception e)`로 모든 것을 잡거나 `throws`를 기계적으로 전파하는 상황을 만들었다.

Go는 다른 방향을 선택했다. 에러가 발생할 수 있는 함수는 반환 타입에 `error`를 포함하고, 호출하는 쪽은 그 `error`를 검사한다. 에러는 값이므로 변수에 저장하고, 비교하고, wrapping하고, 로그에 남기는 등 어떤 프로그래밍이든 가능하다.

Rob Pike는 이 철학을 한 문장으로 요약했다: "Errors are values." 에러를 특별한 제어 흐름이 아니라 평범한 값으로 다루면, 프로그래머가 에러를 프로그래밍할 수 있다.

## 정리

| 개념 | Node.js | Go |
|---|---|---|
| 에러 표현 | Error 객체, throw | error interface, 반환값 |
| 에러 처리 | try/catch | `if err != nil` |
| 에러 wrapping | `Error.cause` (ES2022) | `fmt.Errorf("%w", err)` |
| 에러 비교 | `.code`, `instanceof` | `errors.Is`, `errors.As` |
| 미리 정의된 에러 | 에러 코드 문자열 | sentinel error 값 |
| 치명적 에러 | `process.exit`, uncaught exception | `panic` |
| 에러 복구 | error middleware, `process.on("uncaughtException")` | `recover` (defer 안에서) |
| 에러 전파 | call stack을 자동으로 타고 올라감 | 반환값으로 명시적 전달 |

Go의 에러 처리는 장황하다. `if err != nil`이 끝없이 반복된다. 하지만 이 장황함의 대가로 얻는 것이 있다. 에러 경로가 코드에 명시적으로 드러나고, 어떤 함수가 실패할 수 있는지 반환 타입만 보면 알 수 있고, 에러를 값으로 다루므로 유연한 처리가 가능하다. Tony Hoare가 "보이지 않는 null"의 위험을 경고했듯이, Go는 "보이지 않는 에러 경로"를 제거하는 쪽을 택했다.
