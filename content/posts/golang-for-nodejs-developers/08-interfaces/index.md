# 인터페이스

Go 타입 시스템에서 가장 독특한 부분이 interface다. `implements` 키워드 없이 메서드만 맞으면 구현이 완성되는 implicit satisfaction, 작은 interface를 합성하는 관례, 그리고 "Accept interfaces, return structs" 원칙까지 살펴본다.

## duck typing

"If it walks like a duck and quacks like a duck, then it must be a duck."

이 문구의 원형은 19세기 미국 시인 James Whitcomb Riley에게서 나왔다. "오리처럼 걷고, 오리처럼 헤엄치고, 오리처럼 꽥꽥거리는 새를 보면 그 새를 오리라 부르겠다." 정체가 무엇인지(what it is)가 아니라 무엇을 하는지(what it does)로 판단한다는 뜻이다.

프로그래밍 용어로 "duck typing"을 처음 쓴 건 2000년 7월 Alex Martelli가 comp.lang.python 메일링 리스트에 보낸 메시지다. Python처럼 동적 타입 언어에서 객체의 클래스가 아니라 메서드와 속성의 존재 여부로 호환성을 판단하는 방식을 가리킨다.

Go의 interface는 이 아이디어를 정적 타입 시스템 안에서 구현한다. 런타임이 아니라 컴파일 타임에, 타입 이름이 아니라 메서드 시그니처로 호환성을 검증한다. 정적 타입의 안전성과 duck typing의 유연함을 동시에 취한 설계다.

## implicit satisfaction

TypeScript에서는 `implements`를 명시하는 것이 일반적이다:

```typescript
interface Speaker {
  speak(): string;
}

class Dog implements Speaker {
  speak(): string {
    return "Woof!";
  }
}
```

그런데 사실 TypeScript도 구조적 타이핑이라 `implements` 없이도 동작한다:

```typescript
class Cat {
  speak(): string {
    return "Meow!";
  }
}

const s: Speaker = new Cat(); // implements 없이도 OK — shape이 일치하므로
```

`implements`는 구현하는 쪽에서 "이 interface를 만족하는지 컴파일러가 확인해달라"는 자기 선언이지, 할당 가능성의 필수 조건은 아니다.

Go는 이 방향을 더 철저하게 밀고 나간다:

```go
type Speaker interface {
    Speak() string
}

type Dog struct {
    Name string
}

func (d Dog) Speak() string {
    return "Woof!"
}

func main() {
    var s Speaker = Dog{Name: "Rex"}
    fmt.Println(s.Speak()) // Woof!
}
```

`Dog`는 어디에서도 `Speaker`를 구현하겠다고 선언하지 않았다. `Speak() string` 메서드가 있으니 자동으로 `Speaker`를 만족한다. 이것이 implicit satisfaction이다.

차이는 두 가지다. 첫째, 관례의 차이. TypeScript에서는 `implements`를 쓰는 것이 강한 관례이고, 대부분의 코드베이스에서 class가 interface를 구현할 때 명시한다. Go에는 `implements` 자체가 존재하지 않으므로 모든 구현이 암묵적이다. 둘째, 범위의 차이. TypeScript의 구조적 타이핑은 property와 method를 모두 포함하지만, Go의 interface는 메서드만 검사한다.

실무에서 진짜 차이가 드러나는 건 패키지 경계다. 외부 라이브러리가 노출하는 타입이 프로젝트 코드의 interface를 만족하면, Go에서는 해당 라이브러리를 수정하지 않고도 바로 사용할 수 있다. TypeScript에서도 구조적 타이핑 덕분에 이론적으로 가능하지만, class 기반 라이브러리에서 private 필드가 하나라도 있으면 구조적 호환이 깨진다. Go에서는 exported 메서드 시그니처만 맞으면 된다.

## interface 정의와 사용

interface는 메서드 시그니처의 집합이다. 필드가 아니라 동작을 정의한다:

```go
type Writer interface {
    Write(data []byte) (int, error)
}
```

Go의 interface는 메서드만 가진다. 데이터(필드)를 interface에 넣을 수 없다.

interface를 매개변수 타입으로 사용하면 다형성이 만들어진다:

```go
type Speaker interface {
    Speak() string
}

type Dog struct{}
type Cat struct{}

func (d Dog) Speak() string { return "Woof!" }
func (c Cat) Speak() string { return "Meow!" }

func greet(s Speaker) {
    fmt.Println(s.Speak())
}

func main() {
    greet(Dog{}) // Woof!
    greet(Cat{}) // Meow!
}
```

07편에서 embedding으로는 불가능했던 다형성이 interface로 완성된다. `Dog`와 `Cat`은 아무 관계가 없지만, 둘 다 `Speaker`를 만족하므로 `greet` 함수에 전달할 수 있다.

## interface composition

Go 표준 라이브러리의 `io` package가 interface 설계의 교과서다:

```go
type Reader interface {
    Read(p []byte) (n int, err error)
}

type Writer interface {
    Write(p []byte) (n int, err error)
}

type ReadWriter interface {
    Reader
    Writer
}
```

`ReadWriter`는 `Reader`와 `Writer`를 embedding한다. 07편에서 struct embedding을 봤는데, interface도 같은 방식으로 합성된다. 작은 interface를 조합해서 큰 interface를 만든다.

TypeScript의 `extends`와 형태가 비슷하지만, Go 표준 라이브러리는 이 합성을 훨씬 적극적으로 활용한다. 1-2개 메서드짜리 interface가 기본 단위다:

| interface | 메서드 | 용도 |
|---|---|---|
| `io.Reader` | `Read` | 데이터를 읽는 모든 것 |
| `io.Writer` | `Write` | 데이터를 쓰는 모든 것 |
| `io.Closer` | `Close` | 리소스를 닫는 모든 것 |
| `fmt.Stringer` | `String` | 문자열 표현 (JS의 `toString`) |
| `error` | `Error` | 에러 (내장 interface) |

`os.File`, `net.Conn`, `bytes.Buffer` 등 수십 개의 타입이 `io.Reader`를 만족한다. 파일에서 읽든 네트워크에서 읽든 메모리 버퍼에서 읽든, 소비하는 쪽은 `io.Reader` 하나만 알면 된다.

## empty interface와 any

Go 1.18 이전에는 모든 타입을 받을 수 있는 타입이 `interface{}`였다. 메서드가 0개인 interface이므로 모든 타입이 만족한다:

```go
func printAnything(v interface{}) {
    fmt.Println(v)
}

printAnything(42)
printAnything("hello")
printAnything(true)
```

Go 1.18부터 `interface{}`의 별칭으로 `any`가 추가되었다:

```go
func printAnything(v any) {
    fmt.Println(v)
}
```

TypeScript의 `any`와 이름은 같지만 성격이 다르다. TypeScript의 `any`는 타입 검사를 완전히 끈다. 어떤 메서드를 호출해도, 어떤 연산을 해도 컴파일러가 허용한다. Go의 `any`는 타입 검사를 유지한다. `any` 타입 변수에 메서드를 호출하거나 연산을 하려면 반드시 type assertion으로 원래 타입을 복원해야 한다:

```typescript
// TypeScript: any는 무엇이든 허용
let x: any = "hello";
x.toUpperCase(); // OK - 타입 검사 없음
x * 2;           // OK - 런타임에 NaN
```

```go
// Go: any는 타입 검사를 유지
var x any = "hello"
// x.ToUpper()   // 컴파일 에러: any에 ToUpper 메서드 없음
// x * 2         // 컴파일 에러: any에 * 연산 불가

s := x.(string) // type assertion으로 string 복원
fmt.Println(strings.ToUpper(s)) // HELLO
```

TypeScript에서 `any`를 쓰면 타입 시스템의 보호를 포기하는 것이다. Go에서 `any`를 쓰면 타입 정보를 잠시 감추는 것이다. 다시 꺼내려면 type assertion이 필요하고, 틀리면 panic이 발생한다.

## type assertion

`any`나 interface 타입 변수에서 원래 타입을 복원하는 방법이다:

```go
var s Speaker = Dog{Name: "Rex"}

// type assertion
d := s.(Dog)
fmt.Println(d.Name) // Rex

// 타입이 틀리면 panic
// c := s.(Cat) // panic: interface conversion
```

안전한 방식은 두 번째 반환값으로 성공 여부를 확인하는 것이다. 04편에서 다룬 comma ok 패턴과 동일하다:

```go
d, ok := s.(Dog)
if ok {
    fmt.Println(d.Name)
} else {
    fmt.Println("Dog가 아니다")
}
```

TypeScript의 type guard와 같은 역할이지만, 별도 함수를 정의할 필요 없이 `s.(Dog)`으로 끝난다.

## type switch

여러 타입을 분기해야 할 때 type switch를 쓴다:

```go
func describe(v any) string {
    switch t := v.(type) {
    case int:
        return fmt.Sprintf("정수: %d", t)
    case string:
        return fmt.Sprintf("문자열: %s", t)
    case bool:
        if t {
            return "참"
        }
        return "거짓"
    default:
        return fmt.Sprintf("알 수 없는 타입: %T", t)
    }
}

func main() {
    fmt.Println(describe(42))      // 정수: 42
    fmt.Println(describe("hello")) // 문자열: hello
    fmt.Println(describe(true))    // 참
}
```

`v.(type)`은 type switch 전용 문법이다. 일반 type assertion과 달리 `switch` 안에서만 사용할 수 있다. 각 `case` 블록 안에서 `t`는 해당 타입으로 자동 변환된다. `case int` 블록에서 `t`는 `int`다.

TypeScript의 discriminated union이 tag 필드(`kind`)로 분기하는 것과 달리, Go는 타입 자체로 분기한다. 접근이 다르지만 각 분기에서 컴파일러가 정확한 타입 정보를 제공한다는 점은 같다.

## 작은 interface를 선호하는 문화

Go 커뮤니티에서 반복되는 조언이 있다. interface는 작게 만들어라. 메서드 1-2개가 이상적이다.

이유는 implicit satisfaction과 맞닿아 있다. interface가 작을수록 더 많은 타입이 자연스럽게 만족한다. `io.Reader`는 메서드가 하나뿐이라서 파일, 네트워크 연결, HTTP 응답 본문, 압축 스트림, 암호화 스트림 등 수십 개의 타입이 모두 `Reader`다. 만약 `Reader`에 `Close`와 `Seek`까지 넣었다면 이 보편성은 사라진다.

Go에서는 interface가 클수록 암묵적으로 만족하는 타입이 줄어들고, interface의 장점이 희석된다.

```go
// 나쁜 예: 메서드가 너무 많다
type Repository interface {
    Find(id string) (*User, error)
    FindAll() ([]*User, error)
    Create(user *User) error
    Update(user *User) error
    Delete(id string) error
    Count() (int, error)
}

// 좋은 예: 필요한 동작만 정의
type UserFinder interface {
    Find(id string) (*User, error)
}

type UserCreator interface {
    Create(user *User) error
}
```

큰 interface가 필요하면 작은 interface를 합성한다. 소비하는 쪽에서 필요한 만큼만 요구하면 테스트에서 mock을 만들기도 쉬워진다.

## Accept interfaces, return structs

Go에서 자주 인용되는 설계 원칙이다. 함수의 매개변수는 interface로, 반환값은 구체적인 struct로 정의하라.

```go
// 매개변수: interface (유연함)
// 반환값: 구체적 struct (명확함)
func NewServer(logger Logger) *Server {
    return &Server{logger: logger}
}
```

매개변수를 interface로 받으면 호출하는 쪽이 어떤 구현체든 전달할 수 있다. 테스트에서는 mock을, 프로덕션에서는 실제 구현을 넣을 수 있다. 반환값을 struct로 돌려주면 호출하는 쪽이 구체적인 타입 정보를 알 수 있다.

반대로 하면 어떤 문제가 생기는가:

```go
// 반환값이 interface면
func NewServer(logger Logger) ServerInterface {
    return &Server{logger: logger}
}
```

호출하는 쪽이 `ServerInterface`에 정의된 메서드만 사용할 수 있다. 나중에 `Server`에 메서드를 추가해도 interface를 수정하지 않으면 접근할 수 없다. interface가 불필요한 추상화 계층이 된다.

dependency injection 프레임워크가 하는 역할을 Go에서는 언어 자체의 interface만으로 달성한다.

## interface 만족 여부 컴파일 타임 검증

implicit satisfaction은 편리하지만, 메서드 하나를 빠뜨려도 컴파일 에러가 의도한 위치에서 발생하지 않을 수 있다. 이를 방지하는 관용 패턴이 있다:

```go
// 컴파일 타임에 Dog가 Speaker를 만족하는지 검증
var _ Speaker = Dog{}
var _ Speaker = (*Dog)(nil)
```

`_`에 할당하므로 변수는 사용되지 않지만, 컴파일러가 `Dog`를 `Speaker`에 할당할 수 있는지 확인한다. 메서드가 빠져 있으면 이 줄에서 컴파일 에러가 발생한다. 표준 라이브러리와 주요 오픈소스 프로젝트에서 흔히 볼 수 있는 패턴이다.

Go의 interface는 "무엇인가"가 아니라 "무엇을 하는가"로 타입을 분류한다. 이 철학이 implicit satisfaction, 작은 interface, interface composition으로 이어진다. 07편의 struct와 embedding, 그리고 이번 편의 interface가 결합되면 상속 없이도 유연한 다형성을 구현하는 Go의 타입 시스템이 완성된다.
