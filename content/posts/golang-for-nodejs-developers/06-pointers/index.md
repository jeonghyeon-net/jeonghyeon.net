# 포인터

JavaScript에서는 object를 다른 함수에 넘기면 알아서 참조로 전달된다. 개발자가 신경 쓸 것이 없다. Go는 이 과정을 명시적으로 드러낸다. 포인터가 그 도구다.

## 포인터가 왜 필요한가

JavaScript에서 원시 타입과 참조 타입의 차이를 떠올려 보자:

```javascript
// JavaScript
// 원시 타입: 값이 복사된다
let a = 10;
let b = a;
b = 20;
console.log(a); // 10 (영향 없음)

// 참조 타입: 참조가 복사된다
const user = { name: "Alice" };
const copy = user;
copy.name = "Bob";
console.log(user.name); // "Bob" (원본도 바뀜)
```

JavaScript 엔진이 이 구분을 자동으로 처리한다. 개발자는 "이 값이 스택에 있는가, 힙에 있는가"를 생각할 필요가 없다.

Go에서는 모든 대입이 값 복사다. struct든 배열이든 전부 복사된다:

```go
type User struct {
    Name string
}

func main() {
    user := User{Name: "Alice"}
    copy := user
    copy.Name = "Bob"
    fmt.Println(user.Name) // "Alice" (원본 그대로)
}
```

JavaScript와 결과가 다르다. Go에서 `copy := user`는 struct 전체를 복사한다. `copy`를 수정해도 `user`에 영향이 없다.

원본을 수정하고 싶거나, 큰 struct의 복사를 피하고 싶을 때 포인터가 필요하다.

## &와 * 연산자

포인터는 메모리 주소를 담는 변수다. 두 가지 연산자가 핵심이다:

- `&` — 변수의 메모리 주소를 얻는다 (address-of)
- `*` — 포인터가 가리키는 값에 접근한다 (dereference)

```go
func main() {
    x := 42
    p := &x          // p는 x의 메모리 주소를 담고 있다
    fmt.Println(p)   // 0xc0000b6010 (주소)
    fmt.Println(*p)  // 42 (주소가 가리키는 값)

    *p = 100         // 포인터를 통해 원본 수정
    fmt.Println(x)   // 100
}
```

`p`의 타입은 `*int`다. "int를 가리키는 포인터"라는 뜻이다. `*`이 두 가지 맥락에서 쓰인다는 점에 주의한다:

- 타입 선언에서 `*int` — "int 포인터" 타입
- 표현식에서 `*p` — 포인터가 가리키는 값에 접근 (dereference)

```go
var p *int      // int 포인터 타입 선언. 아직 아무것도 가리키지 않는다 (nil)
x := 42
p = &x          // x의 주소를 p에 대입
fmt.Println(*p) // 42. p가 가리키는 값을 읽는다
```

## 값 전달 vs 포인터 전달

Go에서 함수에 인자를 넘기면 항상 값이 복사된다. 포인터도 예외가 아니다 — 포인터 자체가 복사된다. 하지만 복사된 포인터는 같은 메모리 주소를 가리키므로 원본을 수정할 수 있다.

### 값 전달 (value semantics)

```go
func double(n int) {
    n *= 2
}

func main() {
    x := 10
    double(x)
    fmt.Println(x) // 10 — 원본 불변
}
```

`n`은 `x`의 복사본이다. 함수 안에서 `n`을 수정해도 `x`에 영향이 없다.

### 포인터 전달 (pointer semantics)

```go
func double(n *int) {
    *n *= 2
}

func main() {
    x := 10
    double(&x)
    fmt.Println(x) // 20 — 원본 수정됨
}
```

`&x`로 주소를 넘기고, 함수 안에서 `*n`으로 해당 주소의 값을 수정한다. JavaScript에서 object를 넘기면 자동으로 일어나는 일을, Go에서는 `&`와 `*`로 명시한다.

### struct에서의 차이

```go
type Config struct {
    Port    int
    Debug   bool
    Workers int
}

// 값으로 받으면: 복사본을 수정. 호출자에게 영향 없음
func disableDebug(c Config) {
    c.Debug = false
}

// 포인터로 받으면: 원본을 수정
func disableDebugPtr(c *Config) {
    c.Debug = false
}

func main() {
    cfg := Config{Port: 8080, Debug: true, Workers: 4}

    disableDebug(cfg)
    fmt.Println(cfg.Debug) // true — 안 바뀜

    disableDebugPtr(&cfg)
    fmt.Println(cfg.Debug) // false — 바뀜
}
```

포인터를 통해 struct의 필드에 접근할 때 `(*c).Debug` 대신 `c.Debug`로 쓸 수 있다. Go 컴파일러가 자동으로 dereference한다. C에서 `->` 연산자가 필요했던 부분을 Go는 `.`으로 통일했다.

## 스택과 힙

JavaScript에서는 V8 엔진이 메모리 할당을 전부 관리한다. 원시 타입은 스택에, object는 힙에 할당된다는 것이 일반적 설명이지만, 실제로는 엔진의 최적화에 따라 달라진다. 개발자가 신경 쓸 일이 아니다.

Go에서도 개발자가 직접 스택이나 힙을 지정하지 않는다. 하지만 컴파일러의 결정 기준을 알면 성능을 이해하는 데 도움이 된다.

### escape analysis

Go 컴파일러는 컴파일 타임에 각 변수가 스택에 남을 수 있는지 분석한다. 이것이 escape analysis다.

```go
func createOnStack() int {
    x := 42
    return x // 값을 복사해서 반환. x는 스택에 남는다
}

func createOnHeap() *int {
    x := 42
    return &x // x의 주소를 반환. x는 함수가 끝나도 살아 있어야 한다
}
```

`createOnStack`에서 `x`는 함수가 끝나면 사라져도 된다. 값이 복사되어 반환되기 때문이다. 스택에 할당된다.

`createOnHeap`에서 `x`의 주소가 반환된다. 함수가 끝난 후에도 그 주소에 접근할 수 있어야 하므로 `x`는 힙으로 "탈출(escape)"한다. Go 컴파일러가 이를 감지하고 자동으로 힙에 할당한다.

C에서는 이런 코드가 버그다. 함수의 지역 변수 주소를 반환하면 dangling pointer가 된다. Go는 escape analysis 덕분에 안전하다.

`-gcflags="-m"`으로 컴파일러의 escape analysis 결과를 확인할 수 있다:

```bash
go build -gcflags="-m" main.go
```

```
./main.go:10:2: moved to heap: x
```

스택 할당이 힙 할당보다 빠르다. 스택은 함수가 반환될 때 자동으로 정리되지만, 힙에 할당된 메모리는 garbage collector가 수거해야 한다. 성능이 중요한 코드에서 불필요한 힙 탈출을 줄이면 GC 부담이 줄어든다.

### 힙 탈출이 발생하는 일반적인 경우

- 포인터를 반환하는 경우 (`return &x`)
- 클로저가 지역 변수를 캡처하는 경우
- interface 타입에 값을 저장하는 경우
- slice나 map에 포인터를 넣는 경우

Go로 고성능 서비스를 작성하게 되면 "왜 이 변수가 힙에 갔는가"를 추적하는 순간이 온다.

## nil 포인터

JavaScript에는 "값이 없음"을 나타내는 것이 `null`과 `undefined` 두 가지다. Go에는 `nil`이 있다. 포인터의 zero value가 `nil`이다.

```go
var p *int          // nil
fmt.Println(p)      // <nil>
fmt.Println(p == nil) // true
```

nil 포인터를 dereference하면 프로그램이 panic으로 죽는다:

```go
var p *int
fmt.Println(*p) // panic: runtime error: invalid memory address or nil pointer dereference
```

JavaScript에서 `null.property`에 접근하면 TypeError가 발생하는 것과 비슷하다:

```javascript
// JavaScript
const obj = null;
console.log(obj.name); // TypeError: Cannot read properties of null
```

JavaScript의 TypeError는 try-catch로 잡을 수 있다. Go의 panic도 recover로 잡을 수 있지만, 실무에서는 사전에 nil을 체크하는 것이 관례다:

```go
func printName(u *User) {
    if u == nil {
        fmt.Println("no user")
        return
    }
    fmt.Println(u.Name)
}
```

JavaScript에서 optional chaining(`?.`)으로 null을 우회하는 것과 비슷한 방어적 패턴이다:

```javascript
// JavaScript
console.log(user?.name ?? "no user");
```

Go에는 optional chaining이 없다. 명시적인 nil 체크가 필요하다.

## 포인터를 쓸 때와 쓰지 않을 때

### 포인터를 쓰는 경우

**함수에서 값을 수정해야 할 때:**

```go
func reset(c *Config) {
    c.Port = 3000
    c.Debug = false
}
```

**struct가 클 때 (복사 비용 절감):**

```go
// 필드가 많은 큰 struct
type Response struct {
    Headers map[string]string
    Body    []byte
    Status  int
    // ... 수십 개의 필드
}

func process(r *Response) {
    // 포인터로 받아 복사를 피한다
}
```

**nil로 "값 없음"을 표현해야 할 때:**

```go
func findUser(id int) *User {
    // 찾지 못하면 nil 반환
    return nil
}
```

03편에서 다룬 zero value 문제를 상기하면 된다. `User{}`는 빈 User인지, 의도적으로 비운 것인지 구분할 수 없다. 포인터를 쓰면 `nil`이 "없음"을 명확히 표현한다.

### 포인터를 쓰지 않는 경우

**작은 struct나 기본 타입:**

```go
type Point struct {
    X, Y float64
}

// 16바이트짜리 struct는 복사해도 비용이 거의 없다
func distance(a, b Point) float64 {
    dx := a.X - b.X
    dy := a.Y - b.Y
    return math.Sqrt(dx*dx + dy*dy)
}
```

**불변성이 중요할 때:**

값으로 전달하면 함수가 원본을 수정할 수 없다. 의도적으로 불변성을 보장하는 설계다.

**map, slice, channel:**

이 타입들은 내부적으로 이미 포인터를 포함하고 있다. 함수에 넘겨도 데이터가 통째로 복사되지 않는다. 포인터로 감쌀 필요가 없다:

```go
func appendItem(s []string, item string) []string {
    return append(s, item)
}

// []string을 값으로 받아도 된다
// slice header만 복사된다 (24바이트)
// 내부 배열은 공유된다
```

### 판단 기준 정리

| 상황 | 포인터 | 값 |
|---|---|---|
| 원본 수정 필요 | O | |
| 큰 struct (수백 바이트 이상) | O | |
| nil로 "없음" 표현 | O | |
| 작은 struct, 기본 타입 | | O |
| 불변성 보장 | | O |
| map, slice, channel | | O (이미 참조 의미) |

확신이 없으면 값으로 시작하고, 필요할 때 포인터로 바꾼다.

## Go가 포인터 산술을 제거한 이유

C에서 포인터는 산술 연산이 가능하다. 주소에 정수를 더하거나 빼서 메모리를 직접 탐색할 수 있다:

```c
// C
int arr[5] = {10, 20, 30, 40, 50};
int *p = arr;
printf("%d\n", *(p + 2)); // 30 — 세 번째 원소
p++;                       // 다음 원소로 이동
```

이 기능은 강력하지만, 역사적으로 가장 심각한 보안 사고들의 원인이 되었다.

1988년, 코넬 대학의 대학원생 Robert Tappan Morris가 만든 웜이 인터넷에 퍼졌다. 당시 인터넷에 연결된 약 60,000대의 컴퓨터 중 약 6,000대가 감염되었다. 웜이 이용한 취약점 중 하나가 BSD Unix `fingerd` 데몬의 buffer overflow였다. `gets()` 함수가 입력 크기를 검사하지 않아 512바이트 버퍼를 넘겨 스택을 덮어쓸 수 있었다. 공격자는 함수의 return address를 조작해 임의의 코드를 실행했다. 포인터와 메모리를 직접 조작할 수 있는 C의 특성이 만든 사고였다.

2014년에는 Heartbleed가 터졌다. OpenSSL의 heartbeat 구현에서 `memcpy()` 호출 전에 길이를 검증하지 않아, 서버 메모리의 내용을 64KB씩 읽어올 수 있었다. 비밀키, 세션 쿠키, 비밀번호가 유출될 수 있는 취약점이었다. 당시 인터넷 보안 서버의 약 17%가 영향을 받았다.

이런 사고는 지금도 계속된다. Microsoft와 Google 모두 자사 제품의 심각한 보안 취약점 중 약 70%가 메모리 안전성 문제에서 비롯된다고 밝혔다. 대부분 C와 C++의 포인터 산술, buffer overflow, use-after-free 같은 문제다.

Go의 설계자들은 이 역사를 알고 있었다. Rob Pike는 "기계가 할 수 있다고 해서 프로그래머에게 허용해야 하는 것은 아니다"라는 입장을 취했다. Go에서 포인터 산술이 빠진 것은 실수가 아니라 의도적 결정이다. 포인터를 통한 값의 참조와 수정은 허용하되, 메모리 주소를 직접 계산하는 것은 허용하지 않는다. 배열의 경계를 넘는 접근은 런타임에 panic이 발생한다:

```go
arr := [5]int{10, 20, 30, 40, 50}
fmt.Println(arr[5]) // panic: runtime error: index out of range [5] with length 5
```

C에서는 `arr[5]`가 undefined behavior다. 프로그램이 죽을 수도, 엉뚱한 값을 반환할 수도, 보안 취약점이 될 수도 있다. Go에서는 즉시 panic이 발생한다. 예측할 수 없는 동작보다 명확한 실패가 낫다.

표준 라이브러리의 `unsafe` package를 사용하면 포인터 산술이 가능하다. 하지만 이름이 말해주듯 "안전하지 않다." `unsafe`를 사용하는 코드는 Go의 메모리 안전성 보장을 포기하는 것이며, Go 버전 업그레이드 시 호환성이 보장되지 않는다. CGo 연동이나 극단적인 성능 최적화가 아닌 이상 쓸 일이 없다.

Go의 포인터는 C의 포인터가 아니다. 산술이 없고, 경계 검사가 있고, escape analysis가 메모리 관리를 돕는다. 위험한 부분은 제거하고 유용한 부분만 남긴 설계다. `&`로 주소를 넘기고, `*`로 값에 접근한다 -- 이 두 연산자에 익숙해지면 된다.
