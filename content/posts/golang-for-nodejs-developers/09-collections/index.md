# 배열, 슬라이스, 맵

Go의 컬렉션 타입을 살펴본다. JavaScript의 Array는 만능 도구지만 Go는 용도에 따라 array, slice, map으로 나눈다. 특히 slice의 내부 구조는 06편의 포인터 개념과 직결된다.

## JavaScript의 Array는 배열이 아니다

JavaScript의 Array는 이름과 달리 진짜 배열이 아니다. C나 Go의 배열은 연속된 메모리 블록에 같은 타입의 값을 나란히 저장한다. JavaScript의 Array는 내부적으로 해시맵에 가깝다.

```javascript
// JavaScript
const arr = [1, "hello", true, { name: "Alice" }];
arr[100] = "sparse";
console.log(arr.length); // 101 (중간은 비어 있다)
```

타입이 섞이고, 인덱스를 건너뛸 수 있고, 크기가 자동으로 늘어난다. V8 같은 엔진은 연속된 정수 인덱스에 같은 타입이 들어 있으면 내부적으로 진짜 배열처럼 최적화하지만, 이는 구현 세부사항이다.

Go는 이런 유연함을 포기하는 대신 메모리 레이아웃이 명확하다. 컬렉션 타입이 세 가지로 나뉘고, 각각의 특성이 다르다.

## array - 고정 크기

array는 크기가 타입의 일부다. `[3]int`와 `[5]int`는 서로 다른 타입이다.

```go
var a [3]int             // [0, 0, 0] - zero value로 초기화
b := [3]int{1, 2, 3}    // [1, 2, 3]
c := [...]int{1, 2, 3}  // [1, 2, 3] - 컴파일러가 크기를 세어준다

fmt.Println(len(a))     // 3
```

JavaScript의 Array와 근본적으로 다른 점:

- 크기가 고정이다. 선언 후 늘리거나 줄일 수 없다.
- 같은 타입만 들어간다.
- 대입하면 전체가 복사된다.

```go
a := [3]int{1, 2, 3}
b := a           // 배열 전체가 복사된다
b[0] = 999
fmt.Println(a[0]) // 1 - 원본은 그대로
```

JavaScript에서는 배열을 대입하면 참조가 복사된다. 한쪽을 수정하면 다른 쪽도 바뀐다. Go의 array는 값 타입이므로 독립적인 복사본이 만들어진다. 06편에서 다룬 "모든 대입이 값 복사"라는 원칙이 array에도 적용된다.

실무에서 array를 직접 쓸 일은 거의 없다. 크기가 고정이라는 제약이 너무 크기 때문이다. 대부분의 경우 slice를 쓴다.

## slice - 동적 크기

slice는 Go에서 가장 많이 쓰는 컬렉션 타입이다. JavaScript의 Array와 가장 비슷한 역할을 한다.

```go
// slice 생성
s := []int{1, 2, 3}     // 크기를 지정하지 않으면 slice
var s2 []int             // nil slice (선언만)
s3 := make([]int, 3)     // 길이 3, 용량 3
s4 := make([]int, 3, 10) // 길이 3, 용량 10
```

array와 slice의 선언은 대괄호 안에 크기가 있느냐 없느냐로 구분된다. `[3]int`는 array, `[]int`는 slice다.

### slice header의 구조

slice가 array와 다른 핵심은 내부 구조에 있다. slice는 세 가지 필드로 이루어진 작은 struct다:

```
slice header (24바이트)
+---------+---------+---------+
| pointer | length  | capacity|
+---------+---------+---------+
     |
     v
+---+---+---+---+---+---+---+---+
| 1 | 2 | 3 |   |   |   |   |   |  <- underlying array (힙)
+---+---+---+---+---+---+---+---+
```

- **pointer** - underlying array의 시작 위치를 가리키는 포인터 (06편에서 다룬 그 포인터다)
- **length** - 현재 원소 수
- **capacity** - underlying array의 전체 크기

06편에서 "map, slice, channel은 내부적으로 이미 포인터를 포함하고 있다"고 했다. slice header의 pointer 필드가 바로 그것이다. slice를 함수에 넘기면 header(24바이트)만 복사되고, 실제 데이터는 공유된다.

```go
func printFirst(s []int) {
    fmt.Println(s[0])
}

func main() {
    nums := []int{10, 20, 30}
    printFirst(nums) // header만 복사. 데이터는 복사하지 않는다
}
```

`len()`과 `cap()`으로 길이와 용량을 확인할 수 있다:

```go
s := make([]int, 3, 10)
fmt.Println(len(s)) // 3
fmt.Println(cap(s)) // 10
```

### append

slice에 원소를 추가할 때 `append`를 쓴다. JavaScript의 `push`에 해당한다.

```javascript
// JavaScript
const arr = [1, 2, 3];
arr.push(4); // 원본을 수정
```

```go
s := []int{1, 2, 3}
s = append(s, 4) // 새 slice를 반환. 반드시 재대입해야 한다
```

JavaScript의 `push`는 원본 배열을 변경(mutate)한다. Go의 `append`는 새 slice를 반환한다. `append`의 결과를 재대입하지 않으면 추가된 원소가 사라진다. 컴파일러가 `append`의 반환값을 사용하지 않으면 에러를 발생시키므로, 실수로 누락할 가능성은 낮다.

`append`는 용량이 부족하면 더 큰 underlying array를 새로 할당하고 기존 데이터를 복사한다:

```go
s := make([]int, 0, 2)
fmt.Println(len(s), cap(s)) // 0 2

s = append(s, 1)
fmt.Println(len(s), cap(s)) // 1 2

s = append(s, 2)
fmt.Println(len(s), cap(s)) // 2 2

s = append(s, 3) // 용량 초과 -> 새 배열 할당
fmt.Println(len(s), cap(s)) // 3 4
```

용량이 부족할 때 Go 런타임이 새 배열을 할당하는 방식은 버전에 따라 다르다. Go 1.24 기준으로 작은 slice는 대략 2배씩 늘어나고, 큰 slice는 약 1.25배씩 늘어난다. 정확한 증가율은 런타임 구현의 세부사항이므로 의존하면 안 된다.

### slicing 연산

기존 slice에서 부분을 잘라낼 수 있다. JavaScript의 `slice()`와 문법이 비슷하다.

```javascript
// JavaScript
const arr = [0, 1, 2, 3, 4];
const sub = arr.slice(1, 3); // [1, 2] - 새 배열
sub[0] = 999;
console.log(arr[1]); // 1 - 원본 불변
```

```go
arr := []int{0, 1, 2, 3, 4}
sub := arr[1:3] // [1, 2] - 같은 underlying array를 공유
sub[0] = 999
fmt.Println(arr[1]) // 999 - 원본도 바뀐다!
```

JavaScript의 `slice()`는 새 배열을 만들지만, Go의 slicing은 같은 underlying array를 공유하는 새 slice header를 만든다. 이것이 가장 주의해야 할 차이점이다.

### slice 함정 - underlying array 공유

같은 underlying array를 공유하는 slice들은 서로 영향을 준다:

```go
original := []int{0, 1, 2, 3, 4}
slice1 := original[1:3] // [1, 2]
slice2 := original[2:4] // [2, 3]

slice1[1] = 999          // original[2]를 수정
fmt.Println(slice2[0])   // 999 - slice2도 영향을 받는다
```

`slice1[1]`과 `slice2[0]`과 `original[2]`는 모두 같은 메모리 주소를 가리킨다. 하나를 수정하면 전부 바뀐다. 06편에서 "포인터가 같은 메모리 주소를 가리키면 원본을 수정할 수 있다"고 했는데, slice의 공유도 같은 원리다.

더 교묘한 함정은 `append`에서 발생한다:

```go
base := make([]int, 2, 5) // len=2, cap=5
base[0] = 1
base[1] = 2

a := base[:2]
b := append(a, 3) // 용량이 남아 있으므로 기존 array에 쓴다

fmt.Println(base[:3]) // [1 2 3] - base의 underlying array도 바뀌었다
```

`a`에 `append`했는데 `base`도 바뀌었다. 용량이 남아 있으면 `append`는 새 배열을 할당하지 않고 기존 배열의 빈 공간에 쓴다. 용량이 부족할 때만 새 배열이 만들어진다.

이 문제를 피하려면 독립적인 복사본을 만들어야 한다:

```go
original := []int{1, 2, 3, 4, 5}

// 방법 1: slices.Clone (Go 1.21+, 권장)
import "slices"
copied := slices.Clone(original[1:3])

// 방법 2: append로 복사
copied2 := append([]int{}, original[1:3]...)

// 방법 3: full slice expression으로 용량 제한
sub := original[1:3:3] // [low:high:max] - cap이 2로 제한된다
// append하면 반드시 새 배열이 할당된다
```

full slice expression `[low:high:max]`는 세 번째 인덱스로 용량을 제한한다. `original[1:3:3]`은 길이 2, 용량 2인 slice를 만든다. 이후 `append`하면 용량이 부족하므로 새 배열이 할당되어 원본과의 공유가 끊어진다.

## map

map은 key-value 쌍을 저장한다. JavaScript의 `Object`나 `Map`에 해당한다.

```javascript
// JavaScript
const ages = { alice: 30, bob: 25 };
ages.charlie = 35;
console.log(ages.alice); // 30
delete ages.bob;
```

```go
ages := map[string]int{
    "alice": 30,
    "bob":   25,
}
ages["charlie"] = 35
fmt.Println(ages["alice"]) // 30
delete(ages, "bob")
```

### map 생성

```go
// 리터럴로 생성
m1 := map[string]int{"a": 1, "b": 2}

// make로 생성
m2 := make(map[string]int)
m2["a"] = 1

// 선언만 (nil map)
var m3 map[string]int
// m3["a"] = 1  // panic: nil map에 쓰기 불가
```

nil map에서 읽기는 가능하다(zero value를 반환한다). 하지만 쓰기를 시도하면 panic이 발생한다. `make`나 리터럴로 초기화한 후 사용해야 한다.

### key 존재 여부 확인

JavaScript에서 key 존재 여부를 확인하는 방법이 여러 가지다:

```javascript
// JavaScript
if ("alice" in ages) { /* ... */ }
if (ages.alice !== undefined) { /* ... */ }
if (ages.hasOwnProperty("alice")) { /* ... */ }
```

Go는 comma ok 패턴을 쓴다. 04편에서 다룬 패턴과 동일하다:

```go
age, ok := ages["alice"]
if ok {
    fmt.Println("Alice is", age)
} else {
    fmt.Println("Alice not found")
}
```

존재하지 않는 key에 접근하면 panic이 아니라 value 타입의 zero value를 반환한다. 이 때문에 comma ok 패턴이 필요하다. "값이 0인 것"과 "key가 없는 것"을 구분해야 하기 때문이다:

```go
scores := map[string]int{"alice": 0}

score := scores["alice"]
fmt.Println(score) // 0

score2 := scores["bob"]
fmt.Println(score2) // 0 - alice와 구분이 안 된다

// comma ok로 구분
_, exists := scores["alice"] // exists: true
_, exists2 := scores["bob"]  // exists2: false
```

### 순서 미보장이 진짜로 미보장

JavaScript의 `Object`는 ES2015부터 정수 key는 오름차순, 나머지 key는 삽입 순서를 보장한다. `Map`은 항상 삽입 순서를 보장한다.

Go의 map은 순서를 보장하지 않는다. 그리고 이 "미보장"은 의도적으로 랜덤화되어 있다:

```go
m := map[string]int{"a": 1, "b": 2, "c": 3, "d": 4}

for k, v := range m {
    fmt.Println(k, v)
}
// 실행할 때마다 순서가 달라진다
```

Go 초기 버전에서는 map 순회 순서가 우연히 일정했다. 개발자들이 이 순서에 의존하는 코드를 작성하는 일이 발생했고, Go 팀은 의도적으로 순회 순서를 랜덤화했다. "보장하지 않는다"는 스펙을 코드로 강제한 것이다.

정렬된 순서가 필요하면 key를 별도로 정렬해야 한다:

```go
import "slices"
import "maps"

m := map[string]int{"banana": 2, "apple": 1, "cherry": 3}

keys := slices.Sorted(maps.Keys(m))
for _, k := range keys {
    fmt.Println(k, m[k])
}
// apple 1
// banana 2
// cherry 3
```

### map의 key 타입 제약

JavaScript의 `Object`는 key가 문자열(또는 Symbol)이다. `Map`은 어떤 값이든 key로 쓸 수 있다.

Go의 map은 `==`로 비교할 수 있는 타입만 key로 쓸 수 있다. 숫자, 문자열, bool, 포인터, struct(모든 필드가 comparable하면) 등이 가능하다. slice, map, function은 key로 쓸 수 없다.

```go
// OK
m1 := map[string]int{}
m2 := map[int]string{}
m3 := map[[2]int]string{} // array도 가능

// 컴파일 에러
// m4 := map[[]int]string{} // slice는 key 불가
```

## range로 순회

05편에서 `range`를 소개했다. 컬렉션 타입별로 `range`가 반환하는 값이 다르다:

```go
// slice: index, value
nums := []int{10, 20, 30}
for i, v := range nums {
    fmt.Println(i, v) // 0 10, 1 20, 2 30
}

// map: key, value
ages := map[string]int{"alice": 30, "bob": 25}
for k, v := range ages {
    fmt.Println(k, v)
}

// string: byte index, rune
for i, r := range "Go 언어" {
    fmt.Printf("%d: %c\n", i, r)
}
```

필요 없는 값은 `_`로 무시한다:

```go
// value만 필요
for _, v := range nums {
    fmt.Println(v)
}

// key(index)만 필요
for i := range nums {
    fmt.Println(i)
}
```

## nil slice vs empty slice

이 구분은 JavaScript에 없는 개념이다. JavaScript에서 빈 배열은 `[]` 하나뿐이다. Go에서는 nil slice와 empty slice가 다르다.

```go
var nilSlice []int          // nil slice
emptySlice := []int{}       // empty slice
emptySlice2 := make([]int, 0) // empty slice

fmt.Println(nilSlice == nil)    // true
fmt.Println(emptySlice == nil)  // false
fmt.Println(emptySlice2 == nil) // false

fmt.Println(len(nilSlice))     // 0
fmt.Println(len(emptySlice))   // 0
```

nil slice는 포인터가 nil이다. empty slice는 포인터가 빈 배열을 가리킨다. 하지만 `len`, `cap`, `append`, `range` 모두 nil slice에서 정상 동작한다:

```go
var s []int // nil

fmt.Println(len(s)) // 0
s = append(s, 1)    // OK
fmt.Println(s)       // [1]

for _, v := range s {
    fmt.Println(v)   // 1
}
```

nil slice에 `append`할 수 있다는 점이 중요하다. 함수에서 조건부로 slice를 만들 때 `make`로 미리 초기화할 필요 없이 nil인 채로 시작해도 된다:

```go
func filter(nums []int, predicate func(int) bool) []int {
    var result []int // nil slice로 시작
    for _, n := range nums {
        if predicate(n) {
            result = append(result, n)
        }
    }
    return result
}
```

차이가 드러나는 곳은 JSON 직렬화다:

```go
import "encoding/json"

var nilSlice []int
emptySlice := []int{}

b1, _ := json.Marshal(nilSlice)
b2, _ := json.Marshal(emptySlice)

fmt.Println(string(b1)) // null
fmt.Println(string(b2)) // []
```

API 응답에서 `null`과 `[]`의 차이가 중요할 때 주의해야 한다. 빈 JSON 배열을 반환하고 싶으면 nil slice가 아니라 `[]int{}`나 `make([]int, 0)`을 써야 한다.

nil map도 비슷하다. 읽기는 가능하지만 쓰기는 panic이다:

```go
var m map[string]int // nil map

v := m["key"]       // 0 (zero value, panic 아님)
fmt.Println(len(m)) // 0

// m["key"] = 1     // panic: assignment to entry in nil map
```

Go의 컬렉션은 JavaScript보다 저수준이다. slice header의 구조를 이해해야 공유 문제를 피할 수 있고, nil과 empty의 차이를 알아야 API에서 예상치 못한 `null`을 방지할 수 있다.
