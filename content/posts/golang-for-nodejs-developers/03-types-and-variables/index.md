# 타입과 변수

Go의 기본 타입과 변수 선언 방식을 살펴본다. TypeScript에 익숙한 개발자가 Go 타입 시스템으로 전환할 때 필요한 멘탈 모델의 차이에 집중한다.

## 변수 선언

Node.js에는 `var`, `let`, `const` 세 가지 변수 선언 키워드가 있다. Go에는 `var`와 `:=` 두 가지가 있다.

```go
// var 키워드: 타입을 명시하거나 초기값으로 추론
var name string = "Go"
var name2 = "Go" // string으로 추론

// 짧은 선언 (:=): 함수 안에서만 사용 가능
count := 42       // int로 추론
message := "hello" // string으로 추론
```

`:=`는 선언과 대입을 동시에 한다. 함수 밖(package level)에서는 사용할 수 없다. package level 변수는 반드시 `var`로 선언해야 한다.

```go
var globalConfig = "production" // package level: var 필수

func main() {
    localConfig := "debug" // 함수 안: := 사용 가능
    fmt.Println(globalConfig, localConfig)
}
```

Node.js와 매핑하면:

| Node.js | Go | 비고 |
|---|---|---|
| `let x = 1` | `x := 1` | 함수 안에서의 일반적 선언 |
| `const x = 1` | `x := 1` | Go에서 `:=`는 재대입 가능. 불변이 아니다 |
| `var x = 1` | `var x = 1` | Go의 `var`는 hoisting 없음 |

Go에는 `let`과 `const`의 구분이 변수 레벨에서 없다. `:=`로 선언한 변수는 언제든 재대입할 수 있다. 불변성이 필요하면 `const`를 쓰되, `const`는 컴파일 타임 상수만 허용한다(후술).

여러 변수를 한 번에 선언할 수도 있다:

```go
var (
    host = "localhost"
    port = 8080
    debug = false
)
```

## 기본 타입

Go의 기본 타입은 TypeScript보다 세분화되어 있다.

### 숫자

TypeScript에는 `number` 하나뿐이다. 정수든 소수든 전부 64-bit floating point(IEEE 754)다. Go는 정수와 부동소수점을 구분하고, 크기별로 타입이 나뉜다.

```go
var i int     = 42      // 플랫폼에 따라 32 또는 64-bit
var i8 int8   = 127     // -128 ~ 127
var i16 int16 = 32767
var i32 int32 = 2147483647
var i64 int64 = 9223372036854775807

var u uint    = 42      // 부호 없는 정수
var u8 uint8  = 255     // 0 ~ 255

var f32 float32 = 3.14
var f64 float64 = 3.141592653589793
```

실무에서는 대부분 `int`와 `float64`만 쓴다. 특별한 이유가 없으면 `int8`이나 `int32`를 직접 지정할 필요 없다. `:=`로 정수 literal을 대입하면 `int`로, 소수점이 있으면 `float64`로 추론된다.

```go
x := 42    // int
y := 3.14  // float64
```

### bool

TypeScript와 동일하다. `true` 또는 `false`.

```go
done := true
verbose := false
```

### string

Go의 `string`은 immutable byte sequence다. TypeScript의 `string`이 UTF-16 code unit의 sequence인 것과 다르다.

```go
greeting := "Hello, 世界"
fmt.Println(len(greeting)) // 13 (바이트 수, 글자 수가 아니다)
```

`len("Hello, 世界")`가 13인 이유: "Hello, "가 7바이트, "世"가 3바이트, "界"가 3바이트. Go의 `len`은 바이트 수를 반환한다. JavaScript의 `"Hello, 世界".length`는 9를 반환한다(UTF-16 code unit 수).

### byte와 rune

`byte`는 `uint8`의 별칭이다. `rune`은 `int32`의 별칭이며 Unicode code point 하나를 나타낸다.

```go
var b byte = 'A'  // 65
var r rune = '世' // 19990
```

string을 글자 단위로 순회하려면 `range`를 쓴다:

```go
s := "Hello, 世界"

// 바이트 단위 순회
for i := 0; i < len(s); i++ {
    fmt.Printf("%d: %x\n", i, s[i]) // 바이트 값
}

// 글자(rune) 단위 순회
for i, r := range s {
    fmt.Printf("%d: %c\n", i, r) // Unicode 문자
}
```

`range`는 UTF-8을 디코딩하면서 순회하므로 index가 연속적이지 않다. "世"는 index 7에서 시작하고, "界"는 index 10에서 시작한다.

## string, []byte, rune의 관계

JavaScript의 `string`은 UTF-16이고, 인덱싱하면 code unit 하나를 얻는다. emoji 같은 문자에서 문제가 생기는 이유다.

```javascript
// JavaScript
"😀".length          // 2 (surrogate pair)
"😀"[0]              // "\uD83D" (의미 없는 반쪽)
[..."😀"].length     // 1 (iterator는 code point 단위)
```

Go의 `string`은 UTF-8 byte sequence다. 인덱싱하면 바이트 하나를 얻는다.

```go
s := "😀"
fmt.Println(len(s))    // 4 (UTF-8로 4바이트)
fmt.Println(s[0])      // 240 (0xF0, 첫 바이트)

// rune slice로 변환하면 code point 단위
runes := []rune(s)
fmt.Println(len(runes)) // 1
fmt.Println(runes[0])   // 128512 (U+1F600)
```

세 타입 간의 변환은 명시적이다:

```go
s := "hello"
b := []byte(s)   // string → byte slice
s2 := string(b)  // byte slice → string
r := []rune(s)   // string → rune slice
s3 := string(r)  // rune slice → string
```

HTTP body를 다루거나 파일을 읽을 때는 `[]byte`로 작업하고, 사용자에게 보여줄 때는 `string`으로 변환하는 패턴이 일반적이다.

## zero value

Go에서 가장 중요한 개념 중 하나다. 모든 타입에는 기본값(zero value)이 있다. 변수를 선언만 하고 초기화하지 않으면 zero value가 할당된다.

```go
var i int       // 0
var f float64   // 0.0
var b bool      // false
var s string    // "" (빈 문자열)
var p *int      // nil (포인터)
```

JavaScript와 비교하면:

```javascript
// JavaScript
let x;           // undefined
let y = null;    // null
console.log(x);  // undefined
```

JavaScript에는 `undefined`와 `null`이라는 두 가지 "없음"이 있다. TypeScript의 `strictNullChecks`를 켜면 `null`과 `undefined`를 명시적으로 처리해야 하므로 상황이 나아지지만, 이는 컴파일 타임 검사일 뿐이고 런타임에는 여전히 `null`/`undefined`가 존재한다.

Go에는 `undefined`가 없다. 선언된 변수는 항상 유효한 값을 가진다. `int`의 zero value는 `0`이지 "값이 없음"이 아니다.

이 차이가 실무에서 의미하는 것:

```go
type Config struct {
    Port    int
    Debug   bool
    Timeout int
}

cfg := Config{}
fmt.Println(cfg.Port)    // 0 — 설정하지 않았지만 유효한 값
fmt.Println(cfg.Debug)   // false
fmt.Println(cfg.Timeout) // 0
```

"Port를 설정하지 않은 것"과 "Port를 0으로 설정한 것"을 구분할 수 없다. 이 문제를 해결하려면 pointer를 사용하거나(`*int`이면 nil이 "미설정"을 의미), sentinel value를 두는 방법이 있다.

## 타입 변환

JavaScript는 암묵적 형변환(implicit coercion)이 만연하다:

```javascript
// JavaScript
"5" + 3        // "53" (string)
"5" - 3        // 2 (number)
true + 1       // 2
"" == false    // true
```

TypeScript가 이 중 일부를 잡아준다. `"5" - 3`은 컴파일 에러다. 하지만 `"5" + 3`은 TypeScript에서도 허용된다 — `string + number`는 string 결합으로 취급하기 때문이다.

Go에서는 암묵적 형변환이 없다. 타입이 다르면 명시적으로 변환해야 한다. 그렇지 않으면 컴파일 에러다.

```go
var i int = 42
var f float64 = float64(i)  // int → float64: 명시적 변환 필수
var u uint = uint(i)        // int → uint: 명시적 변환 필수

// i + f  // 컴파일 에러: mismatched types int and float64
result := float64(i) + f    // 한쪽을 맞춰야 한다
```

string과 숫자 간 변환은 타입 캐스팅이 아니라 `strconv` package를 사용한다:

```go
import "strconv"

s := strconv.Itoa(42)         // int → string: "42"
n, err := strconv.Atoi("42")  // string → int: 42, nil

// string(42)는 "42"가 아니다
fmt.Println(string(42))       // "*" (Unicode code point 42)
```

`string(42)`가 `"42"`를 반환하지 않는 것은 Go 입문자가 자주 실수하는 부분이다. `string()` 변환은 숫자를 Unicode code point로 해석한다.

## 상수

Go의 `const`는 JavaScript의 `const`와 다르다. JavaScript의 `const`는 "재대입 불가"일 뿐 값 자체가 불변이 아닌 경우도 있다(객체의 속성은 변경 가능). Go의 `const`는 컴파일 타임에 값이 확정되는 진짜 상수다.

```go
const pi = 3.14159
const greeting = "hello"
const maxRetry = 3

// const timestamp = time.Now() // 컴파일 에러: 런타임 값은 const 불가
```

`const`에는 함수 호출 결과를 넣을 수 없다. 컴파일 시점에 확정 가능한 값만 허용된다.

### iota

`iota`는 Go의 상수 열거 생성기다. TypeScript의 enum과 유사한 역할을 한다.

```go
type Weekday int

const (
    Sunday Weekday = iota // 0
    Monday                // 1
    Tuesday               // 2
    Wednesday             // 3
    Thursday              // 4
    Friday                // 5
    Saturday              // 6
)
```

`iota`는 `const` 블록 안에서 0부터 시작해 줄마다 1씩 증가한다. 첫 줄에만 표현식을 지정하면 나머지는 같은 패턴을 반복한다.

비트 플래그 패턴에도 유용하다:

```go
type Permission int

const (
    Read    Permission = 1 << iota // 1
    Write                          // 2
    Execute                        // 4
)

// 조합
userPerm := Read | Write // 3
```

TypeScript의 enum과 비교:

```typescript
// TypeScript
enum Weekday {
  Sunday,    // 0
  Monday,    // 1
  Tuesday,   // 2
}
```

기능은 비슷하지만, Go의 `iota`는 `const` 블록의 문법 요소일 뿐 별도의 `enum` 타입이 아니다. `Weekday` 타입은 그냥 `int`의 별칭이므로, 아무 정수나 대입할 수 있다. TypeScript의 enum이 제공하는 exhaustiveness check 같은 안전 장치는 없다.

## 구조적 타입 vs 명목적 타입

TypeScript는 구조적 타이핑(structural typing)을 따른다. 구조가 같으면 같은 타입이다:

```typescript
// TypeScript
interface Point { x: number; y: number }
interface Coordinate { x: number; y: number }

const p: Point = { x: 1, y: 2 };
const c: Coordinate = p; // 구조가 같으므로 호환
```

Go는 명목적 타이핑(nominal typing)을 따른다. 이름이 다르면 다른 타입이다:

```go
type Celsius float64
type Fahrenheit float64

var c Celsius = 100
// var f Fahrenheit = c  // 컴파일 에러: 다른 타입
var f Fahrenheit = Fahrenheit(c) // 명시적 변환 필요
```

`Celsius`와 `Fahrenheit`는 내부적으로 둘 다 `float64`이지만 다른 타입이다. 섭씨를 화씨 변수에 실수로 대입하는 버그를 컴파일러가 잡아준다.

단, Go의 interface는 구조적 타이핑이다. 메서드 시그니처가 일치하면 명시적 선언 없이 interface를 만족한다.

## typeof가 없는 세상

JavaScript에서 `typeof`는 런타임에 타입을 확인하는 기본 도구다:

```javascript
typeof 42         // "number"
typeof "hello"    // "string"
typeof undefined  // "undefined"
```

Go에서는 `typeof` 연산자가 없다. 모든 변수의 타입이 컴파일 타임에 확정되므로 런타임에 확인할 필요가 없다.

디버깅 목적으로 타입을 출력하고 싶다면 `fmt.Printf`의 `%T` verb를 쓴다:

```go
x := 42
fmt.Printf("%T\n", x) // int

y := 3.14
fmt.Printf("%T\n", y) // float64

s := "hello"
fmt.Printf("%T\n", s) // string
```

`%T`는 디버깅용이다. 프로덕션 코드에서 타입에 따라 분기해야 한다면 type switch를 쓴다.

Go의 타입 시스템은 TypeScript보다 표현력이 낮지만 명확하다. 암묵적으로 일어나는 것이 없고, 컴파일러가 강제하는 것이 많다.
