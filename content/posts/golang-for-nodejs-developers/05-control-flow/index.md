# 제어 흐름

Go의 if, for, switch는 JavaScript와 형태가 비슷하지만 미묘한 차이가 있다. defer는 Go 고유의 리소스 관리 도구다.

## if문

구조는 JavaScript와 같다. 조건에 괄호가 필요 없다는 점이 다르다.

```javascript
// JavaScript
if (x > 0) {
  console.log("positive");
} else if (x === 0) {
  console.log("zero");
} else {
  console.log("negative");
}
```

```go
if x > 0 {
    fmt.Println("positive")
} else if x == 0 {
    fmt.Println("zero")
} else {
    fmt.Println("negative")
}
```

Go에서 조건에 괄호를 쓰면 동작은 하지만 `gofmt`가 제거한다. 중괄호는 필수다. JavaScript처럼 한 줄짜리 if에서 중괄호를 생략할 수 없다.

### short statement

Go의 if는 조건 앞에 짧은 문장(short statement)을 넣을 수 있다.

```go
if err := doSomething(); err != nil {
    fmt.Println("error:", err)
}
// err는 여기서 접근 불가
```

세미콜론 앞의 `err := doSomething()`이 먼저 실행되고, 세미콜론 뒤의 `err != nil`이 조건으로 평가된다. 핵심은 `err`의 scope가 if-else 블록 안으로 제한된다는 점이다.

JavaScript에서 같은 패턴을 쓰면 변수가 바깥 scope에 남는다:

```javascript
// JavaScript
const err = doSomething();
if (err) {
  console.log("error:", err);
}
// err가 여기서도 접근 가능
```

Go의 short statement는 에러 처리에서 특히 많이 쓰인다. 함수 호출과 결과 검사를 한 줄에 묶으면서도 변수의 scope를 최소화할 수 있다.

```go
if f, err := os.Open("config.json"); err != nil {
    log.Fatal(err)
} else {
    defer f.Close()
    // f 사용
}
```

## for

Go에는 반복문이 `for` 하나뿐이다. `while`, `do...while`, `for...of`, `for...in`이 모두 `for`로 대체된다.

### 전통적인 for

괄호가 없을 뿐 형태는 동일하다.

```javascript
// JavaScript
for (let i = 0; i < 5; i++) {
  console.log(i);
}
```

```go
for i := 0; i < 5; i++ {
    fmt.Println(i)
}
```

### while 스타일

`while`에 해당하는 것은 조건만 있는 `for`다.

```javascript
// JavaScript
while (count > 0) {
  count--;
}
```

```go
for count > 0 {
    count--
}
```

### 무한 루프

```javascript
// JavaScript
while (true) {
  // ...
}
```

```go
for {
    // ...
}
```

조건을 생략하면 무한 루프다. `break`로 탈출한다.

### do...while 스타일

Go에 `do...while`은 없다. 같은 효과를 내려면:

```go
for {
    // 본문
    if !condition {
        break
    }
}
```

## range

`for...of`와 `for...in`에 대응하는 것이 `range`다.

### slice 순회

```javascript
// JavaScript
const fruits = ["apple", "banana", "cherry"];
for (const [i, fruit] of fruits.entries()) {
  console.log(i, fruit);
}
```

```go
fruits := []string{"apple", "banana", "cherry"}
for i, fruit := range fruits {
    fmt.Println(i, fruit)
}
```

`range`는 index와 value를 동시에 반환한다. `forEach`나 `entries()`를 별도로 호출할 필요 없이 기본 동작이다.

index가 필요 없으면 `_`로 무시한다:

```go
for _, fruit := range fruits {
    fmt.Println(fruit)
}
```

value가 필요 없으면 index만 받는다:

```go
for i := range fruits {
    fmt.Println(i)
}
```

### map 순회

```javascript
// JavaScript
const ages = { alice: 30, bob: 25 };
for (const [key, value] of Object.entries(ages)) {
  console.log(key, value);
}
```

```go
ages := map[string]int{"alice": 30, "bob": 25}
for key, value := range ages {
    fmt.Println(key, value)
}
```

map 순회 순서는 의도적으로 무작위다. `Object.entries()`가 속성 추가 순서를 보장하는 것과 다르다. 순서에 의존하는 코드를 작성하면 실행할 때마다 결과가 달라질 수 있다.

### string 순회

```go
for i, r := range "Hello, 세계" {
    fmt.Printf("%d: %c\n", i, r)
}
```

`range`로 string을 순회하면 byte 단위가 아니라 rune(Unicode code point) 단위로 순회한다. 03편에서 다룬 내용이다.

### 정수 range (Go 1.22+)

Go 1.22부터 정수에 대한 `range`가 추가되었다:

```go
for i := range 5 {
    fmt.Println(i) // 0, 1, 2, 3, 4
}
```

`for i := 0; i < 5; i++`와 동일하다. 단순 반복을 간결하게 쓸 수 있다.

## switch

JavaScript의 switch는 fall-through가 기본이고, `break`를 빼먹으면 다음 case까지 실행된다:

```javascript
// JavaScript
switch (status) {
  case 200:
    console.log("OK");
    break; // 빼먹으면 201 case도 실행
  case 201:
    console.log("Created");
    break;
  default:
    console.log("Other");
}
```

Go의 switch는 반대다. 각 case가 자동으로 break된다:

```go
switch status {
case 200:
    fmt.Println("OK")
    // 자동으로 break. 다음 case로 넘어가지 않는다
case 201:
    fmt.Println("Created")
default:
    fmt.Println("Other")
}
```

fall-through가 필요하면 명시적으로 `fallthrough` 키워드를 써야 한다:

```go
switch status {
case 200:
    fmt.Println("OK")
    fallthrough
case 201:
    fmt.Println("Success")
}
// status가 200이면 "OK"와 "Success" 모두 출력
```

실무에서 `fallthrough`를 쓸 일은 거의 없다. Go가 fall-through를 기본값에서 제외한 이유는 실수 방지다. C/C++과 JavaScript에서 `break` 누락으로 인한 버그가 워낙 흔하기 때문이다.

### 여러 값을 한 case에

JavaScript에서 여러 case를 묶으려면 fall-through를 이용한다:

```javascript
// JavaScript
switch (day) {
  case "Saturday":
  case "Sunday":
    console.log("weekend");
    break;
  default:
    console.log("weekday");
}
```

Go에서는 콤마로 나열한다:

```go
switch day {
case "Saturday", "Sunday":
    fmt.Println("weekend")
default:
    fmt.Println("weekday")
}
```

### 조건 없는 switch

switch 뒤에 값을 생략하면 `switch true`와 같다. if-else 체인을 대체할 수 있다:

```go
switch {
case score >= 90:
    fmt.Println("A")
case score >= 80:
    fmt.Println("B")
case score >= 70:
    fmt.Println("C")
default:
    fmt.Println("F")
}
```

if-else가 길어질 때 가독성이 좋다. 각 조건이 정렬되어 한눈에 들어온다.

### type switch

Go 고유의 기능이다. interface 값의 실제 타입에 따라 분기한다:

```go
func describe(v interface{}) string {
    switch t := v.(type) {
    case int:
        return fmt.Sprintf("integer: %d", t)
    case string:
        return fmt.Sprintf("string: %q", t)
    case bool:
        return fmt.Sprintf("boolean: %t", t)
    default:
        return fmt.Sprintf("unknown: %v", t)
    }
}

func main() {
    fmt.Println(describe(42))      // integer: 42
    fmt.Println(describe("hello")) // string: "hello"
    fmt.Println(describe(true))    // boolean: true
}
```

`typeof`와 비슷한 역할이지만, type switch는 컴파일러가 타입을 검증한다. 각 case 안에서 `t`는 해당 타입으로 자동 변환되므로 별도의 타입 단언이 필요 없다.

## defer

`defer`는 함수가 끝날 때 실행할 코드를 예약한다.

```go
func readFile() error {
    f, err := os.Open("data.txt")
    if err != nil {
        return err
    }
    defer f.Close()

    // 파일 읽기 작업
    // ...
    return nil
}
```

`defer f.Close()`는 `readFile` 함수가 return할 때 실행된다. 정상 반환이든 에러로 인한 조기 반환이든 상관없이 실행된다.

### defer가 해결하는 문제

defer가 없다면 리소스 해제를 모든 반환 경로에서 직접 해야 한다:

```go
// defer 없이 작성하면
func process() error {
    f, err := os.Open("data.txt")
    if err != nil {
        return err
    }

    data, err := io.ReadAll(f)
    if err != nil {
        f.Close() // 잊으면 leak
        return err
    }

    if err := validate(data); err != nil {
        f.Close() // 여기서도 잊으면 leak
        return err
    }

    f.Close()
    return nil
}
```

반환 경로가 늘어날수록 `f.Close()`를 빼먹을 가능성이 높아진다. defer를 쓰면 한 번만 선언하면 된다:

```go
func process() error {
    f, err := os.Open("data.txt")
    if err != nil {
        return err
    }
    defer f.Close()

    data, err := io.ReadAll(f)
    if err != nil {
        return err
    }

    return validate(data)
}
```

리소스 획득(`os.Open`) 직후에 해제(`defer f.Close()`)를 선언한다. 함수 본문이 아무리 길어져도, 반환 경로가 아무리 많아져도, 해제가 보장된다.

### 획득과 해제를 가까이 두는 설계

defer의 설계 철학은 리소스의 획득과 해제를 코드상에서 가까이 두는 것이다. Java의 `try-with-resources`, Python의 `with` 문, C++의 RAII가 같은 문제를 다루지만 접근 방식이 다르다.

Java의 `try-with-resources`는 `AutoCloseable` interface를 구현해야 하고, 블록 scope에 묶인다:

```java
// Java
try (var f = new FileInputStream("data.txt")) {
    // f 사용
} // 블록이 끝나면 자동으로 close
```

C++의 RAII는 객체의 소멸자에 해제 로직을 넣는다. scope를 벗어나면 소멸자가 호출되므로 블록 단위로 관리된다. Go의 defer는 블록이 아닌 함수 단위로 동작한다는 점이 다르다. 그리고 해제 대상이 특정 interface를 구현할 필요가 없다. 어떤 함수든 defer할 수 있다.

RAII는 소멸자와 클래스 시스템에 의존하는데, Go에는 클래스가 없다. `try-with-resources`는 별도의 문법 구조가 필요하다. defer는 기존 함수 호출 문법을 그대로 쓴다. `defer` 키워드 하나면 어떤 함수 호출이든 지연 실행할 수 있다.

### LIFO 순서

defer는 스택으로 동작한다. 여러 defer를 선언하면 마지막에 선언한 것이 먼저 실행된다(LIFO, Last In First Out).

```go
func main() {
    fmt.Println("start")
    defer fmt.Println("first")
    defer fmt.Println("second")
    defer fmt.Println("third")
    fmt.Println("end")
}
```

```
start
end
third
second
first
```

이 순서에는 이유가 있다. 리소스는 보통 의존 관계가 있다. 데이터베이스 연결을 열고, 그 연결로 트랜잭션을 시작했다면, 트랜잭션을 먼저 닫고 연결을 닫아야 한다. LIFO 순서가 이 의존 관계를 자연스럽게 처리한다:

```go
func transferMoney() error {
    db, err := sql.Open("postgres", connStr)
    if err != nil {
        return err
    }
    defer db.Close() // 두 번째로 실행

    tx, err := db.Begin()
    if err != nil {
        return err
    }
    defer tx.Rollback() // 첫 번째로 실행 (commit 성공 시 rollback은 no-op)

    // 이체 로직
    // ...

    return tx.Commit()
}
```

`tx.Rollback()`이 `db.Close()`보다 먼저 실행된다. 연결이 닫힌 후에 rollback을 시도하는 실수가 구조적으로 불가능하다.

### defer의 인자 평가 시점

defer를 선언한 시점에 인자가 평가된다. 실행 시점이 아니다.

```go
func main() {
    x := 0
    defer fmt.Println("deferred:", x) // x는 이 시점에 평가: 0
    x = 42
    fmt.Println("current:", x)
}
```

```
current: 42
deferred: 0
```

`x`가 나중에 42로 바뀌어도 defer에 전달된 값은 선언 시점의 0이다. 클로저를 사용하면 실행 시점의 값을 참조할 수 있다:

```go
func main() {
    x := 0
    defer func() {
        fmt.Println("deferred:", x) // 클로저: 실행 시점의 x 참조
    }()
    x = 42
    fmt.Println("current:", x)
}
```

```
current: 42
deferred: 42
```

이 차이를 이해하지 못하면 디버깅이 어려워진다. 값을 캡처하고 싶으면 인자로 전달하고, 변수를 참조하고 싶으면 클로저를 쓴다.

### try...finally와의 비교

비슷한 필요를 JavaScript에서는 `try...finally`로 처리한다:

```javascript
// Node.js
const f = await fs.promises.open("data.txt", "r");
try {
  const data = await f.readFile({ encoding: "utf-8" });
  // data 처리
} finally {
  await f.close();
}
```

`finally` 블록은 예외 발생 여부와 관계없이 실행된다. 기능적으로는 defer와 비슷하지만, 리소스 획득과 해제가 멀어질 수밖에 없다. `open`은 try 바깥에, `close`는 finally 안에 있다. 리소스가 여러 개면 try-finally가 중첩되거나 순서 관리가 복잡해진다.

최근 TC39에서 논의 중인 Explicit Resource Management 제안(`using` 선언)은 이 문제를 해결하려는 시도다. Go의 defer와 C++의 RAII에서 영감을 받은 것으로 알려져 있다.

Go의 제어 흐름은 키워드 수가 적다. 반복은 `for` 하나, fall-through는 명시적, scope는 if의 short statement로 제한한다. 그리고 defer는 리소스 관리를 함수 호출 하나로 해결한다. "열었으면 바로 닫기 코드를 선언한다"는 습관을 들이면 leak에서 벗어날 수 있다.