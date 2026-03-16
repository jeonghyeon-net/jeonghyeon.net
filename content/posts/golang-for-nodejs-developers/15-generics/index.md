# 제네릭

Go 1.18 이전까지 "같은 로직인데 타입만 다른 함수를 여러 벌 작성해야 하는" 문제는 Go 개발자의 오랜 고통이었다. 2022년 Go 1.18에서 제네릭이 추가되면서 이 문제가 해결되었다. Go 역사상 가장 많은 요청을 받은 기능이다.

## 제네릭 이전 — interface{}의 시대

08편의 `any`(= `interface{}`)는 모든 타입을 받을 수 있다. 제네릭이 없던 시절에는 이것이 타입을 추상화하는 유일한 방법이었다:

```go
// 제네릭 이전: 슬라이스에서 값을 찾는 함수
func Contains(slice []interface{}, target interface{}) bool {
    for _, v := range slice {
        if v == target {
            return true
        }
    }
    return false
}

func main() {
    nums := []interface{}{1, 2, 3}
    fmt.Println(Contains(nums, 2)) // true
}
```

문제점이 여럿 있다:

1. **타입 안전성 없음.** `[]interface{}`에 `int`와 `string`을 섞어 넣어도 컴파일러가 잡지 못한다.
2. **성능 저하.** 값을 `interface{}`로 감쌀 때 heap allocation이 발생할 수 있다.
3. **사용이 불편하다.** `[]int`를 `[]interface{}`로 바로 변환할 수 없다. 원소를 하나씩 복사해야 한다.

그래서 실무에서는 타입별로 함수를 복사하는 방식이 흔했다:

```go
func ContainsInt(slice []int, target int) bool {
    for _, v := range slice {
        if v == target {
            return true
        }
    }
    return false
}

func ContainsString(slice []string, target string) bool {
    for _, v := range slice {
        if v == target {
            return true
        }
    }
    return false
}
```

로직이 동일한데 타입만 다르다. 이 코드 중복이 제네릭 도입의 직접적인 동기다.

## 타입 파라미터

Go 1.18부터 함수와 타입에 타입 파라미터를 선언할 수 있다:

```go
func Contains[T comparable](slice []T, target T) bool {
    for _, v := range slice {
        if v == target {
            return true
        }
    }
    return false
}

func main() {
    fmt.Println(Contains([]int{1, 2, 3}, 2))          // true
    fmt.Println(Contains([]string{"a", "b"}, "c"))     // false
}
```

`[T comparable]`이 타입 파라미터 선언이다. `T`는 타입 변수이고, `comparable`은 `T`가 만족해야 하는 제약 조건(constraint)이다. `==` 연산이 가능한 타입만 허용한다는 뜻이다.

호출할 때 `Contains[int]([]int{1, 2, 3}, 2)`처럼 타입을 명시할 수도 있지만, 컴파일러가 인자에서 타입을 추론하므로 대부분 생략한다.

TypeScript는 `<T>`, Go는 `[T constraint]`. 꺾쇠 대신 대괄호를 쓰는 이유는 Go 파서에서 `<`가 비교 연산자와 충돌하기 때문이다.

## 타입 제약 조건

타입 파라미터에는 반드시 constraint를 지정해야 한다. constraint는 interface로 정의된다.

### 내장 constraint

```go
// any — 모든 타입 허용
func Print[T any](v T) {
    fmt.Println(v)
}

// comparable — == 연산이 가능한 타입
func Equal[T comparable](a, b T) bool {
    return a == b
}
```

`any`는 아무 제약이 없다. `comparable`은 `==`와 `!=`를 지원하는 타입만 허용한다. map의 키 타입도 `comparable`이어야 하므로 이 constraint가 자주 쓰인다.

### 커스텀 constraint

interface에 타입 요소를 나열해서 constraint를 직접 정의할 수 있다:

```go
type Number interface {
    int | int8 | int16 | int32 | int64 |
    float32 | float64
}

func Sum[T Number](nums []T) T {
    var total T
    for _, n := range nums {
        total += n
    }
    return total
}

func main() {
    fmt.Println(Sum([]int{1, 2, 3}))         // 6
    fmt.Println(Sum([]float64{1.1, 2.2}))    // 3.3000000000000003
}
```

`|`로 타입을 나열하면 union constraint가 된다. `Sum`은 `Number`에 나열된 타입만 받는다. `+` 연산이 가능한 타입을 명시적으로 제한한 것이다.

`~` 접두사를 붙이면 해당 타입을 underlying type으로 가진 타입도 포함한다:

```go
type Integer interface {
    ~int | ~int8 | ~int16 | ~int32 | ~int64
}

type UserID int // underlying type이 int

func Double[T Integer](v T) T {
    return v * 2
}

func main() {
    var id UserID = 5
    fmt.Println(Double(id)) // 10
}
```

`~int`는 `int` 자체뿐 아니라 `type UserID int`처럼 `int`를 기반으로 정의된 타입까지 허용한다. `~`가 없으면 `UserID`는 `int`와 다른 타입이므로 constraint를 만족하지 못한다.

### constraints 패키지

표준 라이브러리 `golang.org/x/exp/constraints`에 자주 쓰는 constraint가 정의되어 있다. Go 1.21부터는 `cmp` 패키지의 `Ordered`가 표준에 포함되었다:

```go
import "cmp"

func Max[T cmp.Ordered](a, b T) T {
    if a > b {
        return a
    }
    return b
}

func main() {
    fmt.Println(Max(3, 7))       // 7
    fmt.Println(Max("a", "z"))   // z
}
```

`cmp.Ordered`는 `<`, `>`, `<=`, `>=` 연산을 지원하는 모든 타입을 포함한다. 정수, 실수, 문자열이 해당된다.

## 제네릭 타입

함수뿐 아니라 타입에도 타입 파라미터를 쓸 수 있다:

```go
type Stack[T any] struct {
    items []T
}

func (s *Stack[T]) Push(v T) {
    s.items = append(s.items, v)
}

func (s *Stack[T]) Pop() (T, bool) {
    if len(s.items) == 0 {
        var zero T
        return zero, false
    }
    v := s.items[len(s.items)-1]
    s.items = s.items[:len(s.items)-1]
    return v, true
}

func main() {
    s := Stack[int]{}
    s.Push(1)
    s.Push(2)
    v, _ := s.Pop()
    fmt.Println(v) // 2
}
```

`Stack[int]`로 인스턴스화하면 `int` 전용 스택이 된다. `Stack[string]`은 `string` 전용이다. 제네릭 이전에는 `interface{}`를 담는 스택을 만들고 꺼낼 때마다 type assertion(08편)을 해야 했다.

## type erasure vs monomorphization

TypeScript의 제네릭은 컴파일 과정에서 완전히 지워진다(type erasure). 런타임에는 타입 파라미터 정보가 남지 않는다:

```typescript
// TypeScript 소스
function identity<T>(v: T): T { return v; }

// 컴파일 후 JavaScript
function identity(v) { return v; }
// T가 사라졌다
```

Go의 제네릭은 컴파일 타임에 구체적인 타입으로 특수화(monomorphization)된다. `Contains[int]`와 `Contains[string]`은 내부적으로 별도의 함수 코드가 생성된다. 실제로는 Go 컴파일러가 GC shape stenciling이라는 최적화를 적용하여, 포인터 크기가 같은 타입끼리 코드를 공유한다. 완전한 monomorphization과 완전한 type erasure 사이의 절충이다.

정리하면:

| | TypeScript | Go |
|---|---|---|
| 타입 정보 | 런타임에 없음 | 컴파일 타임에 구체화 |
| 런타임 오버헤드 | 없음 (제네릭 자체는) | 없음 (네이티브 코드) |
| 타입 검사 시점 | 컴파일 타임만 | 컴파일 타임 (런타임 타입도 유지) |
| constraint 표현 | `extends`, conditional type 등 | interface 기반 |

Go의 constraint 시스템은 의도적으로 단순하다. interface와 타입 union만으로 구성된다.

## 제네릭 함수 실전 예제

### Map, Filter, Reduce

제네릭으로 `map`, `filter` 같은 고차 함수를 직접 만들 수 있다:

```go
func Map[T any, U any](slice []T, f func(T) U) []U {
    result := make([]U, len(slice))
    for i, v := range slice {
        result[i] = f(v)
    }
    return result
}

func Filter[T any](slice []T, f func(T) bool) []T {
    var result []T
    for _, v := range slice {
        if f(v) {
            result = append(result, v)
        }
    }
    return result
}

func main() {
    nums := []int{1, 2, 3, 4, 5}

    doubled := Map(nums, func(n int) int { return n * 2 })
    fmt.Println(doubled) // [2 4 6 8 10]

    evens := Filter(nums, func(n int) bool { return n%2 == 0 })
    fmt.Println(evens) // [2 4]
}
```

Go 1.21부터 `slices` 패키지에 이런 유틸리티가 포함되기 시작했다. `slices.SortFunc`, `slices.Contains` 등이 제네릭으로 구현되어 있다:

```go
import "slices"

func main() {
    nums := []int{3, 1, 4, 1, 5}
    slices.Sort(nums)
    fmt.Println(nums) // [1 1 3 4 5]
    fmt.Println(slices.Contains(nums, 4)) // true
}
```

### 제네릭 map 유틸리티

```go
func Keys[K comparable, V any](m map[K]V) []K {
    keys := make([]K, 0, len(m))
    for k := range m {
        keys = append(keys, k)
    }
    return keys
}

func main() {
    m := map[string]int{"a": 1, "b": 2, "c": 3}
    fmt.Println(Keys(m)) // [a b c] (순서 무작위)
}
```

`maps` 패키지(Go 1.21+)에 `maps.Keys`, `maps.Values` 등이 이미 있다. 직접 구현할 일은 줄고 있지만, 타입 파라미터가 여러 개일 때의 문법을 보여주는 예시다.

## 언제 제네릭을 쓰고 언제 쓰지 않을까

Go 커뮤니티는 제네릭 사용에 보수적이다. Go 팀 자체가 다음 가이드라인을 제시했다:

**쓰기 좋은 경우:**

- 컬렉션 자료구조 (스택, 큐, 트리 등)
- `slices`, `maps` 같은 범용 유틸리티
- 타입에 독립적인 알고리즘
- 타입별로 동일한 코드를 반복 작성하고 있을 때

**쓰지 않는 것이 나은 경우:**

- 메서드 호출이 핵심인 경우 — interface가 더 적합하다(08편)
- 구현이 타입마다 다른 경우 — 제네릭은 동일한 로직에 타입만 다를 때 쓴다
- 코드가 더 복잡해지는 경우 — 구체적인 타입으로 2-3번 쓰는 것이 제네릭 한 번보다 나을 수 있다

```go
// interface가 더 적합한 경우
type Handler interface {
    Handle(req Request) Response
}

// 제네릭이 불필요하다
// func Handle[T Handler](h T, req Request) Response {
//     return h.Handle(req)
// }

// 이렇게 쓰면 된다
func Process(h Handler, req Request) Response {
    return h.Handle(req)
}
```

interface는 "이 타입이 무엇을 할 수 있는가"를 추상화한다. 제네릭은 "이 로직을 어떤 타입에든 적용할 수 있다"를 표현한다. 목적이 다르다.

Go 프로버브 중 하나인 "A little copying is better than a little dependency"의 정신이 여기서도 적용된다. 제네릭을 도입하면 코드의 추상화 수준이 올라간다. 그 추상화가 충분한 가치를 제공하는지 먼저 따져봐야 한다.

## 제약 사항

Go의 제네릭에는 몇 가지 제약이 있다:

**메서드에는 타입 파라미터를 쓸 수 없다:**

```go
type Converter struct{}

// 컴파일 에러: method must have no type parameters
// func (c Converter) Convert[T any](v T) string {
//     return fmt.Sprint(v)
// }

// 함수로 대체해야 한다
func Convert[T any](v T) string {
    return fmt.Sprint(v)
}
```

타입 자체에 타입 파라미터를 선언하는 것은 가능하지만, 개별 메서드에 추가 타입 파라미터를 선언하는 것은 허용되지 않는다. 이것은 Go 런타임의 메서드 디스패치 방식과 관련된 의도적인 제한이다.

**타입 파라미터로 타입 단언을 할 수 없다:**

```go
func convert[T any](v any) T {
    // return v.(T) // 컴파일 에러
    return v.(T) // Go 1.24에서는 허용
}
```

Go 1.18에서는 불가능했으나 이후 버전에서 제한이 완화되었다.

Go의 제네릭은 10년 넘게 논의 끝에 추가되었다. 그 기간만큼 보수적으로 설계되었다. 타입 수준의 프로그래밍이 아니라, 코드 중복을 제거하는 실용적 도구로 자리 잡았다. `slices`, `maps`, `cmp` 같은 표준 라이브러리가 제네릭의 가장 좋은 사용 예시다.
