# 구조체와 메서드

JavaScript에서 데이터와 동작을 묶는 도구는 class다. Go에는 class가 없다. 대신 struct로 데이터를 정의하고, method로 동작을 붙인다. 상속도 없다. embedding이라는 합성 메커니즘이 그 자리를 대신한다.

## struct 정의

JavaScript의 class와 Go의 struct를 나란히 놓으면:

```javascript
// JavaScript
class User {
  constructor(name, age) {
    this.name = name;
    this.age = age;
  }
}

const user = new User("Alice", 30);
```

```go
type User struct {
    Name string
    Age  int
}

func main() {
    user := User{Name: "Alice", Age: 30}
    fmt.Println(user.Name) // Alice
    fmt.Println(user.Age)  // 30
}
```

`type User struct`가 타입을 정의한다. `class`와 달리 constructor가 없고, 메서드도 struct 안에 들어가지 않는다. struct는 순수하게 데이터의 구조만 정의한다.

## struct 초기화

struct를 만드는 방법이 여러 가지다:

```go
// 필드 이름을 지정 (권장)
u1 := User{Name: "Alice", Age: 30}

// 필드 순서대로 (비권장 — 필드가 추가되면 깨진다)
u2 := User{"Bob", 25}

// zero value로 초기화
var u3 User          // Name: "", Age: 0
u4 := User{}         // 동일

// 특정 필드만 지정 (나머지는 zero value)
u5 := User{Name: "Charlie"} // Age: 0
```

필드 이름을 지정하는 방식이 실무 표준이다. 순서 기반 초기화는 struct에 필드가 추가되면 컴파일 에러가 발생한다. JavaScript의 object literal과 비슷하게 필드 이름을 명시하는 것이 안전하다.

03편에서 다룬 zero value가 여기서도 적용된다. struct를 선언만 하면 모든 필드가 zero value로 초기화된다.

## struct 리터럴과 익명 struct

이름 없는 struct를 즉석에서 만들 수 있다. JavaScript에서 object literal을 자유롭게 쓰는 것과 비슷하다:

```go
// 익명 struct
point := struct {
    X, Y float64
}{X: 1.5, Y: 2.3}

fmt.Println(point.X) // 1.5
```

테스트 코드에서 테이블 기반 테스트를 작성할 때 자주 쓰인다:

```go
tests := []struct {
    input    string
    expected int
}{
    {"hello", 5},
    {"world", 5},
    {"Go", 2},
}

for _, tt := range tests {
    if got := len(tt.input); got != tt.expected {
        fmt.Printf("len(%q) = %d, want %d\n", tt.input, got, tt.expected)
    }
}
```

JavaScript에서 테스트 데이터를 object 배열로 만드는 것과 같은 패턴이다. Go에서는 익명 struct의 slice로 표현한다.

## 메서드

JavaScript에서 메서드는 class 안에 정의한다:

```javascript
// JavaScript
class User {
  constructor(name, age) {
    this.name = name;
    this.age = age;
  }

  greet() {
    return `Hi, I'm ${this.name}`;
  }
}
```

Go에서 메서드는 struct 밖에 정의한다. `func`와 함수 이름 사이에 receiver를 넣는다:

```go
type User struct {
    Name string
    Age  int
}

func (u User) Greet() string {
    return fmt.Sprintf("Hi, I'm %s", u.Name)
}

func main() {
    user := User{Name: "Alice", Age: 30}
    fmt.Println(user.Greet()) // Hi, I'm Alice
}
```

`(u User)`가 receiver다. JavaScript의 `this`에 해당한다. 차이점은 Go에서 receiver의 이름을 직접 정한다는 것이다. `this`처럼 암묵적이지 않다.

receiver 이름은 관례적으로 타입의 첫 글자를 소문자로 쓴다. `User`면 `u`, `Config`면 `c`. `self`나 `this`를 쓰지 않는다.

## value receiver vs pointer receiver

06편에서 다룬 값 전달과 포인터 전달의 구분이 메서드에서도 그대로 적용된다.

### value receiver

```go
func (u User) Greet() string {
    return fmt.Sprintf("Hi, I'm %s", u.Name)
}
```

`u`는 호출 시점의 복사본이다. 메서드 안에서 `u.Name`을 수정해도 원본에 영향이 없다.

### pointer receiver

```go
func (u *User) SetName(name string) {
    u.Name = name
}

func main() {
    user := User{Name: "Alice"}
    user.SetName("Bob")
    fmt.Println(user.Name) // Bob
}
```

`(u *User)`는 포인터를 받는다. 원본을 수정할 수 있다. JavaScript의 메서드가 `this`를 통해 항상 원본을 수정하는 것과 같다.

Go 컴파일러는 호출 방식을 자동으로 맞춰준다. `user.SetName("Bob")`에서 `user`는 포인터가 아니지만, 컴파일러가 자동으로 `(&user).SetName("Bob")`으로 변환한다. 반대 방향도 마찬가지다. 포인터 변수에서 value receiver 메서드를 호출하면 자동으로 dereference한다.

### 어떤 receiver를 쓸 것인가

기준은 06편의 포인터 판단 기준과 동일하다:

| 상황 | receiver |
|---|---|
| 필드를 수정해야 할 때 | pointer |
| struct가 클 때 (복사 비용) | pointer |
| 일관성 (타입의 다른 메서드가 pointer receiver면) | pointer |
| 읽기 전용, struct가 작을 때 | value |

실무에서는 한 타입의 메서드를 pointer receiver로 통일하는 경우가 많다. 확신이 없으면 pointer receiver를 쓴다.

## constructor 패턴

Go에는 constructor가 없다. JavaScript의 `new User()`처럼 인스턴스를 생성하는 특수한 문법이 없다. 대신 `New`로 시작하는 함수를 작성하는 것이 관례다:

```javascript
// JavaScript
class Server {
  constructor(port, host = "localhost") {
    this.port = port;
    this.host = host;
    this.connections = [];
  }
}

const server = new Server(8080);
```

```go
type Server struct {
    Port        int
    Host        string
    connections []string
}

func NewServer(port int) *Server {
    return &Server{
        Port:        port,
        Host:        "localhost",
        connections: make([]string, 0),
    }
}

func main() {
    server := NewServer(8080)
    fmt.Println(server.Host) // localhost
}
```

`NewServer`는 그냥 함수다. 특별한 키워드나 문법이 아니라 관례일 뿐이다. 하지만 Go 생태계 전체가 이 관례를 따른다.

`New` 함수가 포인터를 반환하는 이유:

- struct가 클 경우 복사를 피한다
- 반환값에 메서드(pointer receiver)를 바로 호출할 수 있다
- `nil`로 "생성 실패"를 표현할 수 있다

package에 주요 타입이 하나뿐이면 `New`만 쓰기도 한다. 예를 들어 `errors` package의 `errors.New()`가 그렇다.

### validation이 필요한 constructor

```go
func NewServer(port int) (*Server, error) {
    if port < 1 || port > 65535 {
        return nil, fmt.Errorf("invalid port: %d", port)
    }
    return &Server{
        Port:        port,
        Host:        "localhost",
        connections: make([]string, 0),
    }, nil
}
```

04편에서 다룬 다중 반환으로 에러를 처리한다. JavaScript에서 constructor에서 throw하는 것을 Go에서는 `(nil, error)`로 표현한다.

## embedding

JavaScript의 class 상속을 떠올려 보자:

```javascript
// JavaScript
class Animal {
  constructor(name) {
    this.name = name;
  }

  speak() {
    return `${this.name} makes a sound`;
  }
}

class Dog extends Animal {
  bark() {
    return `${this.name} barks`;
  }
}

const dog = new Dog("Rex");
console.log(dog.speak()); // Rex makes a sound
console.log(dog.bark());  // Rex barks
```

Go에는 `extends`가 없다. 대신 struct 안에 다른 struct를 필드 이름 없이 넣는다. 이것이 embedding이다:

```go
type Animal struct {
    Name string
}

func (a Animal) Speak() string {
    return fmt.Sprintf("%s makes a sound", a.Name)
}

type Dog struct {
    Animal // embedding: 필드 이름 없이 타입만
}

func (d Dog) Bark() string {
    return fmt.Sprintf("%s barks", d.Name)
}

func main() {
    dog := Dog{
        Animal: Animal{Name: "Rex"},
    }
    fmt.Println(dog.Speak()) // Rex makes a sound
    fmt.Println(dog.Bark())  // Rex barks
    fmt.Println(dog.Name)    // Rex
}
```

`Dog`에 `Speak` 메서드를 정의하지 않았지만 호출할 수 있다. `Animal`의 메서드와 필드가 `Dog`로 "승격(promoted)"된다. `dog.Speak()`는 실제로 `dog.Animal.Speak()`와 같다.

### embedding은 상속이 아니다

형태는 비슷해 보이지만 본질이 다르다.

상속에서는 자식이 부모의 일종이다. `Dog`는 `Animal`이다(is-a 관계). Go의 embedding은 `Dog`가 `Animal`을 가지고 있다(has-a 관계). 문법적 편의로 메서드를 바로 호출할 수 있을 뿐이다.

차이가 드러나는 상황:

```go
func feed(a Animal) {
    fmt.Println("Feeding", a.Name)
}

func main() {
    dog := Dog{Animal: Animal{Name: "Rex"}}

    feed(dog.Animal) // OK: Animal을 꺼내서 전달
    // feed(dog)     // 컴파일 에러: Dog는 Animal이 아니다
}
```

JavaScript의 `extends`였다면 `Dog`를 `Animal` 타입으로 전달할 수 있다. Go에서는 불가능하다. `Dog`와 `Animal`은 별개의 타입이다. 다형성이 필요하면 interface를 쓴다.

### 메서드 오버라이드

embedded 타입과 같은 이름의 메서드를 정의하면 외부 타입의 메서드가 우선한다:

```go
func (a Animal) Speak() string {
    return fmt.Sprintf("%s makes a sound", a.Name)
}

func (d Dog) Speak() string {
    return fmt.Sprintf("%s says woof!", d.Name)
}

func main() {
    dog := Dog{Animal: Animal{Name: "Rex"}}
    fmt.Println(dog.Speak())        // Rex says woof!
    fmt.Println(dog.Animal.Speak()) // Rex makes a sound
}
```

`Dog`의 `Speak`가 `Animal`의 `Speak`를 가린다. 원래 메서드는 `dog.Animal.Speak()`로 접근할 수 있다. JavaScript의 `super.speak()`와 비슷하지만, `super` 같은 키워드 없이 embedded 필드를 직접 참조한다.

### 다중 embedding

여러 struct를 동시에 embed할 수 있다. JavaScript의 단일 상속과 다르다:

```go
type Logger struct{}

func (l Logger) Log(msg string) {
    fmt.Println("[LOG]", msg)
}

type Metrics struct{}

func (m Metrics) Record(name string, value float64) {
    fmt.Printf("[METRIC] %s=%.2f\n", name, value)
}

type Service struct {
    Logger
    Metrics
}

func main() {
    svc := Service{}
    svc.Log("started")              // [LOG] started
    svc.Record("latency", 0.42)     // [METRIC] latency=0.42
}
```

`Service`가 `Logger`와 `Metrics`의 메서드를 모두 사용할 수 있다. JavaScript에서 mixin 패턴으로 달성하는 것을 Go는 embedding으로 해결한다.

두 embedded 타입에 같은 이름의 메서드가 있으면 컴파일 에러가 발생한다. 이 경우 외부 타입에서 해당 메서드를 직접 정의하여 해결한다.

## struct와 포인터

06편에서 struct의 값 전달과 포인터 전달을 다뤘다. 메서드와 결합하면 패턴이 정해진다:

```go
type Counter struct {
    count int
}

func (c *Counter) Increment() {
    c.count++
}

func (c *Counter) Value() int {
    return c.count
}

func main() {
    c := &Counter{}
    c.Increment()
    c.Increment()
    fmt.Println(c.Value()) // 2
}
```

상태를 변경하는 타입은 pointer receiver로 통일하고, 생성 시에도 포인터를 반환하는 것이 일반적 패턴이다. JavaScript에서 class 인스턴스가 항상 참조로 다뤄지는 것과 같은 효과다.

Go의 struct와 method는 class보다 단순하다. 데이터(struct)와 동작(method)이 분리되어 있고, 상속 대신 합성을 쓴다. embedding과 interface가 결합되면 class 없이도 다형성을 구현할 수 있다.
