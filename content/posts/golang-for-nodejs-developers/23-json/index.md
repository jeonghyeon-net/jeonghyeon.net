# JSON과 직렬화

Node.js에서 `JSON.parse`와 `JSON.stringify`는 별다른 설정 없이 동작한다. JavaScript 객체 자체가 JSON과 거의 동일한 구조이기 때문이다. Go는 다르다. JSON 데이터를 다루려면 struct를 먼저 정의하고, struct tag로 필드 매핑을 지정해야 한다. 이 과정이 번거로워 보이지만, 컴파일 타임에 타입 안전성을 보장한다.

## 기본: Marshal과 Unmarshal

Go에서 JSON 직렬화는 `encoding/json` 패키지가 담당한다.

```go
package main

import (
    "encoding/json"
    "fmt"
)

type User struct {
    Name  string `json:"name"`
    Email string `json:"email"`
    Age   int    `json:"age"`
}

func main() {
    // struct -> JSON (Marshal)
    u := User{Name: "Alice", Email: "alice@example.com", Age: 30}
    data, err := json.Marshal(u)
    if err != nil {
        panic(err)
    }
    fmt.Println(string(data))
    // {"name":"Alice","email":"alice@example.com","age":30}

    // JSON -> struct (Unmarshal)
    raw := []byte(`{"name":"Bob","email":"bob@example.com","age":25}`)
    var u2 User
    if err := json.Unmarshal(raw, &u2); err != nil {
        panic(err)
    }
    fmt.Println(u2.Name) // Bob
}
```

Node.js 대응:

```javascript
const user = { name: "Alice", email: "alice@example.com", age: 30 };

// 직렬화
const json = JSON.stringify(user);

// 역직렬화
const parsed = JSON.parse(json);
console.log(parsed.name); // Alice
```

핵심적인 차이가 있다. Node.js에서 `JSON.parse`는 아무 JSON이나 바로 객체로 바꾸고, 존재하지 않는 필드에 접근하면 `undefined`를 반환한다. Go에서 `json.Unmarshal`은 struct에 정의된 필드만 채우고, 정의되지 않은 필드는 무시한다. JSON에 없는 필드는 해당 타입의 zero value가 된다.

## Struct tag

struct tag는 Go의 리플렉션 메타데이터다. 필드에 대한 추가 정보를 백틱(`` ` ``) 안에 기술한다:

```go
type Product struct {
    ID        int    `json:"id"`
    Name      string `json:"name"`
    Price     int    `json:"price"`
    InStock   bool   `json:"in_stock"`
    Internal  string `json:"-"`
    Comment   string `json:"comment,omitempty"`
}
```

- `json:"name"` — JSON 키 이름을 지정한다. 없으면 필드명 그대로 사용된다.
- `json:"-"` — JSON 직렬화에서 완전히 제외한다.
- `json:"comment,omitempty"` — 값이 zero value이면 JSON 출력에서 생략한다.

`omitempty`가 적용되는 zero value는 타입에 따라 다르다:

| 타입 | zero value |
|---|---|
| `bool` | `false` |
| `int`, `float64` 등 | `0` |
| `string` | `""` |
| pointer, slice, map | `nil` |

```go
p := Product{ID: 1, Name: "Widget", Price: 0}
data, _ := json.Marshal(p)
fmt.Println(string(data))
// {"id":1,"name":"Widget","price":0,"in_stock":false}
// Comment는 빈 문자열이라 omitempty에 의해 생략
// Internal은 "-"이라 항상 제외
```

Node.js에는 struct tag에 해당하는 개념이 없다. 필드 이름을 바꾸려면 새 객체를 만들어야 한다:

```javascript
// Go의 struct tag에 해당하는 작업을 수동으로
const toJSON = (product) => ({
  id: product.id,
  name: product.name,
  // internal 제외
});
```

## JSON과 Go 타입 매핑

`encoding/json`이 JSON 타입을 Go 타입으로 변환하는 규칙:

| JSON 타입 | Go 타입 |
|---|---|
| `string` | `string` |
| `number` | `float64`, `int`, `json.Number` |
| `boolean` | `bool` |
| `null` | pointer의 `nil`, slice/map의 `nil` |
| `array` | `[]T` (slice) |
| `object` | `struct`, `map[string]T` |

주의할 점이 있다. JSON의 number는 기본적으로 `float64`로 디코딩된다. struct 필드 타입이 `int`면 자동 변환되지만, `map[string]any`로 받으면 모든 숫자가 `float64`가 된다:

```go
raw := []byte(`{"count": 42}`)

var m map[string]any
json.Unmarshal(raw, &m)
fmt.Printf("%T\n", m["count"]) // float64 — int가 아니다
```

이것은 JSON 명세 자체에 정수 타입이 없기 때문이다. Node.js도 동일한 문제가 있다. `JSON.parse('{"id": 9007199254740993}')`에서 큰 정수가 부동소수점 정밀도 문제로 변형된다.

### 포인터 필드로 null과 부재 구분

JSON에서 값이 `null`인 것과 필드 자체가 없는 것을 구분해야 할 때가 있다. 포인터 타입을 사용한다:

```go
type Update struct {
    Name  *string `json:"name,omitempty"`
    Email *string `json:"email,omitempty"`
}

// {"name": "Alice"} -> Name = &"Alice", Email = nil (필드 부재)
// {"name": null}    -> Name = nil (명시적 null)
// 포인터가 아니면 둘 다 zero value ""가 되어 구분 불가
```

PATCH API를 구현할 때 흔히 사용하는 패턴이다.

## 동적 JSON: map[string]any

JSON 구조를 미리 알 수 없거나, 스키마가 유동적인 경우 `map[string]any`를 사용한다:

```go
raw := []byte(`{
    "event": "purchase",
    "data": {
        "item": "book",
        "quantity": 3
    },
    "tags": ["important", "processed"]
}`)

var m map[string]any
if err := json.Unmarshal(raw, &m); err != nil {
    panic(err)
}

event := m["event"].(string)
data := m["data"].(map[string]any)
item := data["item"].(string)
fmt.Println(event, item) // purchase book
```

type assertion(`.(string)`)이 필요하고, 잘못된 타입이면 panic이 발생한다. 안전하게 하려면 comma-ok 패턴을 사용한다:

```go
if event, ok := m["event"].(string); ok {
    fmt.Println(event)
}
```

Node.js에서는 `JSON.parse` 결과를 바로 동적으로 사용하므로 이런 불편함이 없다. 대신 존재하지 않는 필드에 접근해도 런타임 에러 없이 `undefined`가 되어 버그가 숨는다.

### json.RawMessage — 지연 파싱

JSON의 일부만 먼저 파싱하고, 나머지는 나중에 처리하고 싶을 때 `json.RawMessage`를 사용한다:

```go
type Event struct {
    Type string          `json:"type"`
    Data json.RawMessage `json:"data"` // 아직 파싱하지 않음
}

raw := []byte(`{"type":"user_created","data":{"name":"Alice","email":"a@b.com"}}`)

var event Event
json.Unmarshal(raw, &event)

// Type에 따라 다른 struct로 파싱
switch event.Type {
case "user_created":
    var user User
    json.Unmarshal(event.Data, &user)
    fmt.Println(user.Name) // Alice
}
```

`json.RawMessage`는 `[]byte`의 별칭이다. 바이트 그대로 유지하다가 필요한 시점에 적절한 타입으로 다시 Unmarshal한다. 이벤트 시스템이나 플러그인 구조에서 유용하다.

## Streaming: json.Decoder와 json.Encoder

`json.Marshal`/`json.Unmarshal`은 전체 데이터를 `[]byte`로 변환한다. 대용량 JSON이나 네트워크 스트림에서는 `json.Decoder`와 `json.Encoder`를 사용하여 `io.Reader`/`io.Writer`와 직접 연결한다:

```go
// HTTP 요청 본문에서 직접 디코딩
func createUser(w http.ResponseWriter, r *http.Request) {
    var user User
    if err := json.NewDecoder(r.Body).Decode(&user); err != nil {
        http.Error(w, err.Error(), http.StatusBadRequest)
        return
    }

    // 처리 후 응답도 직접 인코딩
    w.Header().Set("Content-Type", "application/json")
    json.NewEncoder(w).Encode(user)
}
```

20편의 JSON API 서버에서 이미 이 패턴을 사용했다. `json.NewDecoder`는 `io.Reader`를 받고, `json.NewEncoder`는 `io.Writer`를 받는다. 중간에 `[]byte`를 거치지 않아 메모리 효율적이다.

### 여러 JSON 값을 연속으로 읽기

`json.Decoder`는 하나의 스트림에서 여러 JSON 값을 순차적으로 읽을 수 있다:

```go
const input = `
{"name": "Alice"}
{"name": "Bob"}
{"name": "Charlie"}
`

decoder := json.NewDecoder(strings.NewReader(input))
for decoder.More() {
    var user User
    if err := decoder.Decode(&user); err != nil {
        break
    }
    fmt.Println(user.Name)
}
// Alice
// Bob
// Charlie
```

NDJSON(Newline Delimited JSON) 형식을 처리할 때 이 패턴이 필요하다. 로그 파일이나 스트리밍 API에서 흔하다.

### DisallowUnknownFields

기본적으로 JSON에 있지만 struct에 없는 필드는 조용히 무시된다. 엄격한 파싱이 필요하면:

```go
decoder := json.NewDecoder(strings.NewReader(raw))
decoder.DisallowUnknownFields()

var user User
if err := decoder.Decode(&user); err != nil {
    // "json: unknown field \"extra\"" 에러 발생
    fmt.Println(err)
}
```

API 서버에서 클라이언트가 오타가 있는 필드를 보냈을 때 조용히 무시하는 대신 에러를 반환하려면 이 옵션을 사용한다.

## 커스텀 MarshalJSON / UnmarshalJSON

`json.Marshaler`와 `json.Unmarshaler` interface를 구현하면 직렬화/역직렬화 동작을 완전히 제어할 수 있다:

```go
type Status int

const (
    StatusActive  Status = iota // 0
    StatusPaused                // 1
    StatusStopped               // 2
)

func (s Status) MarshalJSON() ([]byte, error) {
    names := [...]string{"active", "paused", "stopped"}
    if int(s) >= len(names) {
        return nil, fmt.Errorf("unknown status: %d", s)
    }
    return json.Marshal(names[s])
}

func (s *Status) UnmarshalJSON(data []byte) error {
    var name string
    if err := json.Unmarshal(data, &name); err != nil {
        return err
    }
    switch name {
    case "active":
        *s = StatusActive
    case "paused":
        *s = StatusPaused
    case "stopped":
        *s = StatusStopped
    default:
        return fmt.Errorf("unknown status: %s", name)
    }
    return nil
}
```

이제 `Status` 필드가 JSON에서 `"active"`, `"paused"`, `"stopped"` 문자열로 표현된다:

```go
type Job struct {
    ID     int    `json:"id"`
    Status Status `json:"status"`
}

j := Job{ID: 1, Status: StatusActive}
data, _ := json.Marshal(j)
fmt.Println(string(data))
// {"id":1,"status":"active"}
```

Node.js에서 비슷한 역할을 하는 것이 `toJSON` 메서드다:

```javascript
class Job {
  constructor(id, status) {
    this.id = id;
    this.status = status;
  }
  toJSON() {
    return { id: this.id, status: ["active", "paused", "stopped"][this.status] };
  }
}
```

Go는 직렬화와 역직렬화를 모두 커스터마이즈할 수 있지만, Node.js의 `toJSON`은 직렬화만 커스터마이즈한다. `JSON.parse`의 두 번째 인자 reviver를 사용하면 역직렬화도 가능하지만, 타입별이 아니라 전체 파싱에 대해 적용된다.

### 시간 포맷 커스터마이즈

`time.Time`은 기본적으로 RFC 3339 형식으로 직렬화된다. 다른 포맷이 필요하면 커스텀 타입을 만든다:

```go
type DateOnly struct {
    time.Time
}

func (d DateOnly) MarshalJSON() ([]byte, error) {
    return json.Marshal(d.Format("2006-01-02"))
}

func (d *DateOnly) UnmarshalJSON(data []byte) error {
    var s string
    if err := json.Unmarshal(data, &s); err != nil {
        return err
    }
    t, err := time.Parse("2006-01-02", s)
    if err != nil {
        return err
    }
    d.Time = t
    return nil
}

type Event struct {
    Name string   `json:"name"`
    Date DateOnly `json:"date"`
}
```

`time.Time`을 임베딩하면서 `MarshalJSON`/`UnmarshalJSON`만 오버라이드한다. `"2006-01-02"` 포맷 문자열은 Go의 특수한 레이아웃 규칙이다. Go는 `2006-01-02T15:04:05Z07:00`이라는 고정된 참조 시각(Mon Jan 2 15:04:05 MST 2006)의 각 구성 요소를 포맷 지시자로 사용한다.

## 성능

`encoding/json`은 리플렉션 기반이다. 매 호출마다 struct의 필드 정보를 리플렉션으로 조회하기 때문에 성능에 민감한 환경에서는 병목이 될 수 있다. 서드파티 라이브러리는 코드 생성이나 unsafe 포인터를 활용하여 이 오버헤드를 줄인다.

주요 대안:

| 라이브러리 | 특징 |
|---|---|
| [sonic](https://github.com/bytedance/sonic) | SIMD 활용, 가장 빠른 부류. amd64/arm64만 지원 |
| [go-json](https://github.com/goccy/go-json) | 코드 생성 없이 빠름. API 호환 |
| [jsoniter](https://github.com/json-iterator/go) | `encoding/json` 대체 가능한 API |
| [easyjson](https://github.com/mailru/easyjson) | 코드 생성 방식. 직접 코드 생성 필요 |

대부분의 프로젝트에서는 `encoding/json`으로 충분하다. 프로파일링을 통해 JSON 처리가 실제로 병목임을 확인한 후에 교체를 고려해야 한다. 19편에서 다룬 pprof로 확인할 수 있다.

서드파티 라이브러리 중 API 호환 라이브러리는 import 경로만 바꾸면 전환된다:

```go
// 기존
import "encoding/json"

// go-json으로 교체
import json "github.com/goccy/go-json"

// 나머지 코드 변경 없음
json.Marshal(v)
json.Unmarshal(data, &v)
```

struct와 tag를 먼저 정의하는 것이 `JSON.parse` 한 줄에 비해 번거로워 보이지만, struct가 정의되면 그 이후의 코드는 타입 안전하고, IDE 자동완성이 동작하며, 잘못된 필드 접근은 컴파일 타임에 잡힌다.
