# 함수

Go의 함수 선언, 다중 반환값, 클로저를 살펴본다. JavaScript의 함수와 형태는 비슷하지만 async/await이 없고, callback 대신 다중 반환으로 에러를 처리하는 등 근본적인 차이가 있다.

## 함수 선언

JavaScript에서 함수를 선언하는 방법은 여러 가지다:

```javascript
// function declaration
function add(a, b) {
  return a + b;
}

// arrow function
const add = (a, b) => a + b;

// method
const obj = {
  add(a, b) { return a + b; }
};
```

Go에는 `func` 키워드 하나뿐이다:

```go
func add(a int, b int) int {
    return a + b
}
```

파라미터 타입이 같으면 마지막에 한 번만 쓸 수 있다:

```go
func add(a, b int) int {
    return a + b
}
```

Go 함수의 특징:

- 반환 타입을 파라미터 뒤에 명시한다
- 중괄호 `{`는 반드시 `func`와 같은 줄에 있어야 한다 (개행하면 컴파일 에러)
- arrow function이 없다
- function overloading이 없다
- default parameter가 없다

default parameter가 없다는 건 실무에서 체감이 크다. JavaScript에서 흔히 쓰는 패턴이 Go에서는 불가능하다:

```javascript
// JavaScript
function connect(host, port = 3000) {
  // ...
}
connect("localhost"); // port는 3000
```

Go에서 같은 효과를 내려면 options struct 패턴을 쓴다:

```go
type ConnectOptions struct {
    Host string
    Port int
}

func connect(opts ConnectOptions) {
    if opts.Port == 0 {
        opts.Port = 3000 // zero value를 default로 활용
    }
    // ...
}

func main() {
    connect(ConnectOptions{Host: "localhost"})
}
```

zero value를 활용해서 "설정하지 않은 필드"에 기본값을 부여하는 패턴이다. 03편에서 다룬 zero value 개념이 여기서도 쓰인다.

## 다중 반환값

Go 함수는 값을 여러 개 반환할 수 있다. 이것이 Go 함수의 가장 큰 특징이다.

```go
func divide(a, b float64) (float64, error) {
    if b == 0 {
        return 0, fmt.Errorf("division by zero")
    }
    return a / b, nil
}

func main() {
    result, err := divide(10, 3)
    if err != nil {
        log.Fatal(err)
    }
    fmt.Println(result) // 3.3333333333333335
}
```

JavaScript에서는 값을 여러 개 반환할 수 없다. 배열이나 객체로 감싸서 destructuring하는 것이 관례다:

```javascript
// JavaScript
function divide(a, b) {
  if (b === 0) return [null, new Error("division by zero")];
  return [a / b, null];
}

const [result, err] = divide(10, 3);
```

비슷해 보이지만 본질적으로 다르다. JavaScript의 destructuring은 배열을 만들고 분해하는 것이다. 런타임에 배열 객체가 생성된다. Go의 다중 반환은 언어 레벨에서 지원하는 기능이며, 별도의 객체 할당 없이 레지스터나 스택을 통해 값이 전달된다.

반환값 중 쓰지 않는 것이 있으면 `_`(blank identifier)로 무시한다:

```go
result, _ := divide(10, 3) // 에러 무시
```

Go에서는 사용하지 않는 변수가 컴파일 에러이므로, 의도적으로 무시할 때 `_`가 필수다.

## named return

반환값에 이름을 붙일 수 있다:

```go
func divide(a, b float64) (result float64, err error) {
    if b == 0 {
        err = fmt.Errorf("division by zero")
        return // naked return: result=0, err=에러
    }
    result = a / b
    return // naked return: result=결과값, err=nil
}
```

named return을 쓰면 `return` 뒤에 값을 생략할 수 있다(naked return). 반환 변수는 함수 시작 시 zero value로 초기화된다.

장점:

- godoc에서 반환값의 의미를 문서화하는 역할을 한다
- `defer`와 함께 쓸 때 유용하다 (에러 처리 편에서 다룬다)

단점:

- 함수가 길어지면 어떤 값이 반환되는지 추적하기 어렵다
- naked return은 가독성을 해친다

실무에서의 관례: 짧은 함수에서 반환값의 의미를 명확히 할 때만 사용하고, naked return은 피한다. 공식 코드 리뷰 가이드라인도 같은 입장이다.

## variadic function

JavaScript의 rest parameter와 비슷한 기능이다:

```javascript
// JavaScript
function sum(...numbers) {
  return numbers.reduce((a, b) => a + b, 0);
}

sum(1, 2, 3); // 6
```

```go
func sum(numbers ...int) int {
    total := 0
    for _, n := range numbers {
        total += n
    }
    return total
}

func main() {
    fmt.Println(sum(1, 2, 3))    // 6
    fmt.Println(sum())            // 0

    nums := []int{1, 2, 3}
    fmt.Println(sum(nums...))     // slice 전개: 6
}
```

`...`의 위치가 다르다. JavaScript에서는 파라미터 이름 앞(`...numbers`)에, Go에서는 타입 앞(`numbers ...int`)에 붙는다. slice를 전개할 때도 JavaScript는 `...nums`를 호출 시에, Go는 `nums...`를 호출 시에 쓴다.

variadic parameter는 함수 시그니처의 마지막에만 올 수 있다. 이 점은 JavaScript와 동일하다.

## first-class function과 클로저

Go의 함수는 first-class citizen이다. 변수에 대입하고, 함수의 인자로 전달하고, 반환값으로 쓸 수 있다.

```go
// 함수를 변수에 대입
add := func(a, b int) int {
    return a + b
}
fmt.Println(add(1, 2)) // 3
```

Go에는 arrow function이 없지만 익명 함수(anonymous function)가 있다. `func` 키워드 뒤에 이름 없이 바로 파라미터를 쓴다:

```go
// 즉시 실행
func() {
    fmt.Println("immediately invoked")
}()

// 함수를 인자로 전달
func apply(a, b int, op func(int, int) int) int {
    return op(a, b)
}

result := apply(3, 4, func(a, b int) int {
    return a * b
})
fmt.Println(result) // 12
```

클로저도 지원한다. 외부 변수를 캡처하는 방식은 JavaScript와 동일하다:

```go
func counter() func() int {
    count := 0
    return func() int {
        count++
        return count
    }
}

func main() {
    next := counter()
    fmt.Println(next()) // 1
    fmt.Println(next()) // 2
    fmt.Println(next()) // 3
}
```

JavaScript의 클로저와 동작이 같다. `count` 변수는 `counter` 함수가 반환된 후에도 살아 있고, 반환된 함수가 참조를 유지한다.

## callback에서 다중 반환으로

Node.js 초기에는 에러 처리를 callback의 첫 번째 인자로 했다:

```javascript
// Node.js: error-first callback
fs.readFile("config.json", "utf-8", (err, data) => {
  if (err) {
    console.error(err);
    return;
  }
  console.log(data);
});
```

이후 Promise와 async/await으로 발전했다:

```javascript
// Node.js: async/await
try {
  const data = await fs.promises.readFile("config.json", "utf-8");
  console.log(data);
} catch (err) {
  console.error(err);
}
```

Go에서는 callback도 async/await도 쓰지 않는다. 다중 반환으로 에러를 처리한다:

```go
data, err := os.ReadFile("config.json")
if err != nil {
    log.Fatal(err)
}
fmt.Println(string(data))
```

`async function`이 없는 이유는 간단하다. Go에는 비동기 함수라는 개념이 없기 때문이다. `os.ReadFile`은 호출한 goroutine을 블로킹하지만, 다른 goroutine은 계속 실행된다. 비동기 처리가 필요한 상황에서 goroutine을 쓴다. 이 부분은 goroutine 편에서 자세히 다룬다.

결과적으로 Go 코드는 위에서 아래로 순차적으로 읽힌다. callback 중첩도 없고, `.then()` 체인도 없고, `await`을 빼먹을 걱정도 없다.

## init() 함수

Node.js에는 없는 개념이다. `init`은 package가 로드될 때 자동으로 실행되는 특수 함수다.

```go
package main

import "fmt"

var config string

func init() {
    config = "loaded"
    fmt.Println("init: config loaded")
}

func main() {
    fmt.Println("main:", config)
}
```

```
init: config loaded
main: loaded
```

`init` 함수의 규칙:

- 파라미터도 반환값도 없다
- 직접 호출할 수 없다 (`init()`을 코드에서 부르면 컴파일 에러)
- 한 파일에 여러 개 정의할 수 있다 (위에서 아래로 순서대로 실행)
- 한 package에 여러 파일이 있으면 파일 이름 알파벳 순으로 실행

실행 순서는 이렇다:

1. import된 package의 `init`이 먼저 실행 (의존성 순서대로)
2. package level 변수 초기화
3. `init` 함수 실행
4. `main` 함수 실행

Node.js에서 비슷한 패턴을 찾자면 모듈의 top-level 코드다:

```javascript
// Node.js: 모듈 로드 시 실행되는 top-level 코드
const config = loadConfig();
console.log("module loaded");

export function getConfig() {
  return config;
}
```

차이점: Node.js의 top-level 코드는 모듈이 처음 `import`될 때 한 번 실행된다. Go의 `init`도 package가 처음 import될 때 한 번 실행된다. 동작은 비슷하지만, Go는 이를 별도의 함수 형태로 명확하게 분리했다.

`init`의 남용은 피해야 한다. 전역 상태를 암묵적으로 변경하면 테스트하기 어렵고 의존성을 추적하기 힘들다. 데이터베이스 드라이버 등록처럼 side effect가 필요한 경우에 주로 쓰인다:

```go
import _ "github.com/lib/pq" // init()만 실행하기 위한 blank import
```

`_`로 import하면 package를 직접 사용하지 않아도 컴파일 에러가 나지 않는다. `init` 함수의 side effect만 필요한 경우에 쓰는 관용적 패턴이다.

## 정리

| 개념 | JavaScript | Go |
|---|---|---|
| 함수 선언 | `function`, arrow function, method | `func` 하나 |
| 다중 반환 | 불가 (배열/객체로 흉내) | 언어 레벨 지원 |
| default parameter | `function f(x = 1)` | 없음 (options struct 패턴) |
| rest/variadic | `...args` (이름 앞) | `args ...T` (타입 앞) |
| 에러 처리 | try/catch, Promise | 다중 반환 `(result, error)` |
| 비동기 | async/await | 없음 (goroutine으로 대체) |
| 클로저 | 지원 | 동일하게 지원 |
| 모듈 초기화 | top-level 코드 | `init()` 함수 |
| overloading | 없음 (TypeScript는 시그니처만) | 없음 |

Go의 함수는 단순하다. 선언 방법이 하나이고, 특수 문법이 적다. 대신 다중 반환이라는 강력한 기능이 있고, 이것이 에러 처리의 기반이 된다. 다음 편에서는 Go의 제어 흐름을 살펴본다.