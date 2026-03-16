# 데이터베이스

Go는 `database/sql`이라는 표준 인터페이스로 데이터베이스를 다룬다. 드라이버만 바꾸면 PostgreSQL이든 MySQL이든 동일한 API를 사용할 수 있고, 커넥션 풀도 내장이다. pg, knex, prisma 등 목적별로 라이브러리를 조합해야 하는 Node.js와 달리 출발점이 하나다.

## database/sql — 표준 인터페이스

`database/sql`은 데이터베이스 작업의 추상 계층이다. 실제 데이터베이스와 통신하는 것은 드라이버 패키지의 몫이다. PostgreSQL을 사용한다면:

```go
package main

import (
    "database/sql"
    "fmt"
    "log"

    _ "github.com/lib/pq" // PostgreSQL 드라이버 등록
)

func main() {
    db, err := sql.Open("postgres", "host=localhost port=5432 user=app dbname=mydb sslmode=disable")
    if err != nil {
        log.Fatal(err)
    }
    defer db.Close()

    if err := db.Ping(); err != nil {
        log.Fatal(err)
    }
    fmt.Println("connected")
}
```

`_ "github.com/lib/pq"`는 blank import다. 패키지의 `init()` 함수만 실행하여 드라이버를 `database/sql`에 등록한다. `sql.Open`은 실제로 연결을 만들지 않는다. 연결은 첫 번째 쿼리 실행 시 또는 `Ping()` 호출 시 생성된다.

`sql.DB`는 단일 연결이 아니라 커넥션 풀이다. pg의 `Pool`/`Client` 구분이 없다.

## 커넥션 풀 설정

`sql.DB`는 여러 goroutine에서 동시에 사용해도 안전하다. 풀 설정은 메서드로 조정한다:

```go
db.SetMaxOpenConns(25)              // 최대 열린 연결 수
db.SetMaxIdleConns(10)              // 최대 유휴 연결 수
db.SetConnMaxLifetime(5 * time.Minute) // 연결 최대 수명
db.SetConnMaxIdleTime(1 * time.Minute) // 유휴 연결 최대 시간
```

`sql.Open`이 반환하는 `*sql.DB`를 애플리케이션 전체에서 공유하면 된다. 요청마다 새로 여는 것이 아니다.

## CRUD 기본: Query, QueryRow, Exec

### 단일 행 조회 — QueryRow

```go
var name string
var age int

err := db.QueryRow("SELECT name, age FROM users WHERE id = $1", 42).Scan(&name, &age)
if err == sql.ErrNoRows {
    fmt.Println("user not found")
} else if err != nil {
    log.Fatal(err)
}
fmt.Println(name, age)
```

`QueryRow`는 결과가 없으면 `Scan` 시 `sql.ErrNoRows`를 반환한다. pg에서 빈 배열이 반환되는 것과 달리, 결과 없음이 명시적 에러다. `$1`은 PostgreSQL의 placeholder 문법이다. MySQL은 `?`를 사용한다.

### 여러 행 조회 — Query

```go
rows, err := db.Query("SELECT id, name, age FROM users WHERE age > $1", 20)
if err != nil {
    log.Fatal(err)
}
defer rows.Close()

for rows.Next() {
    var id int
    var name string
    var age int
    if err := rows.Scan(&id, &name, &age); err != nil {
        log.Fatal(err)
    }
    fmt.Printf("id=%d name=%s age=%d\n", id, name, age)
}

if err := rows.Err(); err != nil {
    log.Fatal(err)
}
```

`rows.Close()`를 반드시 호출해야 한다. 호출하지 않으면 커넥션이 풀로 반환되지 않아 커넥션 고갈이 발생한다. `defer rows.Close()`가 관례다. `rows.Err()`는 반복 중 발생한 에러를 확인한다. 네트워크 단절 등으로 중간에 실패할 수 있다.

### 삽입, 수정, 삭제 — Exec

```go
result, err := db.Exec("INSERT INTO users (name, age) VALUES ($1, $2)", "Alice", 30)
if err != nil {
    log.Fatal(err)
}

rowsAffected, _ := result.RowsAffected()
fmt.Println("inserted:", rowsAffected)
```

`Exec`은 결과 행을 반환하지 않는 쿼리에 사용한다. INSERT, UPDATE, DELETE가 해당한다. `RowsAffected()`로 영향받은 행 수를, `LastInsertId()`로 마지막 삽입 ID를 얻는다. 단, `LastInsertId`는 드라이버에 따라 지원 여부가 다르다. PostgreSQL은 지원하지 않으며, 대신 `RETURNING` 절과 `QueryRow`를 사용한다:

```go
var id int
err := db.QueryRow("INSERT INTO users (name, age) VALUES ($1, $2) RETURNING id", "Alice", 30).Scan(&id)
```

## 트랜잭션: sql.Tx

```go
tx, err := db.Begin()
if err != nil {
    log.Fatal(err)
}

_, err = tx.Exec("UPDATE accounts SET balance = balance - $1 WHERE id = $2", 100, 1)
if err != nil {
    tx.Rollback()
    log.Fatal(err)
}

_, err = tx.Exec("UPDATE accounts SET balance = balance + $1 WHERE id = $2", 100, 2)
if err != nil {
    tx.Rollback()
    log.Fatal(err)
}

if err := tx.Commit(); err != nil {
    log.Fatal(err)
}
```

`tx.Exec`은 `db.Exec`과 동일한 API다. 차이점은 같은 트랜잭션 안에서 실행된다는 것이다. `Commit()` 또는 `Rollback()`을 호출해야 트랜잭션이 종료된다.

에러 처리에서 `Rollback()`을 반복 작성하는 것이 번거롭다면, defer 패턴을 사용한다:

```go
tx, err := db.Begin()
if err != nil {
    return err
}
defer tx.Rollback() // Commit 후 Rollback은 no-op

_, err = tx.Exec("UPDATE accounts SET balance = balance - $1 WHERE id = $2", 100, 1)
if err != nil {
    return err
}

_, err = tx.Exec("UPDATE accounts SET balance = balance + $1 WHERE id = $2", 100, 2)
if err != nil {
    return err
}

return tx.Commit()
```

`defer tx.Rollback()`을 먼저 걸어두면, `Commit()` 전에 에러로 반환되었을 때 자동으로 Rollback된다. `Commit()` 이후의 `Rollback()`은 아무 일도 하지 않는다.

pg에서 BEGIN/COMMIT/ROLLBACK을 직접 SQL로 보내는 것과 달리, Go는 `Begin()`, `Commit()`, `Rollback()` 메서드로 추상화한다.

## Prepared Statement

동일한 쿼리를 반복 실행할 때 prepared statement를 사용하면 파싱 비용을 줄일 수 있다:

```go
stmt, err := db.Prepare("SELECT name, age FROM users WHERE id = $1")
if err != nil {
    log.Fatal(err)
}
defer stmt.Close()

var name string
var age int

// 여러 번 실행
err = stmt.QueryRow(1).Scan(&name, &age)
err = stmt.QueryRow(2).Scan(&name, &age)
err = stmt.QueryRow(3).Scan(&name, &age)
```

`db.Prepare`는 SQL을 데이터베이스 서버에 미리 파싱해 둔다. 이후 `stmt.QueryRow`를 호출할 때는 파라미터만 전송한다. 반복 루프에서 같은 쿼리를 수천 번 실행하는 경우에 효과적이다.

실제로는 `database/sql`이 내부적으로 prepared statement를 캐싱하므로, 명시적으로 `Prepare`를 호출하지 않아도 성능 차이가 크지 않은 경우가 많다.

## sql.Null 타입 — nullable 컬럼 처리

데이터베이스의 NULL은 Go의 zero value와 다르다. `age` 컬럼이 NULL인데 `int`로 Scan하면 에러가 발생한다. `sql.Null*` 타입을 사용한다:

```go
var name string
var bio sql.NullString

err := db.QueryRow("SELECT name, bio FROM users WHERE id = $1", 1).Scan(&name, &bio)
if err != nil {
    log.Fatal(err)
}

if bio.Valid {
    fmt.Println("bio:", bio.String)
} else {
    fmt.Println("bio is NULL")
}
```

`sql.NullString`은 `String`과 `Valid` 두 필드를 가진다. `Valid`가 `false`면 데이터베이스 값이 NULL이었다는 뜻이다. 같은 패턴으로 `sql.NullInt64`, `sql.NullFloat64`, `sql.NullBool`, `sql.NullTime` 등이 있다.

Go 1.22부터는 제네릭 타입 `sql.Null[T]`도 사용할 수 있다:

```go
var bio sql.Null[string]
// bio.V (값), bio.Valid (null 여부)
```

포인터를 사용하는 방법도 있다:

```go
var bio *string

err := db.QueryRow("SELECT bio FROM users WHERE id = $1", 1).Scan(&bio)
// bio가 nil이면 NULL, 아니면 *bio가 값
```

포인터 방식이 더 간결하지만, JSON 직렬화 시 `null`로 표현되므로 23편에서 다룬 포인터 필드의 null/부재 구분 문제가 함께 따라온다.

JavaScript에서는 NULL이 자연스럽게 `null`로 매핑되지만, Go는 zero value와 NULL을 구분하기 위해 이런 장치가 필요하다.

## Context와 함께: QueryContext, ExecContext

22편에서 다룬 context를 데이터베이스 작업에 적용하면 타임아웃과 취소를 제어할 수 있다. `Query`, `QueryRow`, `Exec` 각각에 대응하는 `QueryContext`, `QueryRowContext`, `ExecContext`가 있다:

```go
ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
defer cancel()

var name string
err := db.QueryRowContext(ctx, "SELECT name FROM users WHERE id = $1", 1).Scan(&name)
if err != nil {
    // context.DeadlineExceeded면 타임아웃
    log.Fatal(err)
}
```

HTTP 핸들러에서는 요청의 context를 전달한다. 클라이언트가 연결을 끊으면 쿼리도 취소된다:

```go
func getUser(w http.ResponseWriter, r *http.Request) {
    var name string
    err := db.QueryRowContext(r.Context(), "SELECT name FROM users WHERE id = $1", 1).Scan(&name)
    if err != nil {
        http.Error(w, err.Error(), http.StatusInternalServerError)
        return
    }
    fmt.Fprint(w, name)
}
```

트랜잭션도 context를 받는다:

```go
tx, err := db.BeginTx(ctx, nil)
```

프로덕션 코드에서는 Context가 없는 `Query`, `Exec` 대신 항상 `QueryContext`, `ExecContext`를 사용하는 것이 권장된다. 타임아웃 없는 쿼리는 슬로우 쿼리가 커넥션 풀을 고갈시킬 수 있다.

## ORM vs query builder vs raw SQL

Go 생태계에서는 데이터베이스 접근 방식에 대한 선택지가 몇 가지 있다.

### database/sql (raw SQL)

표준 라이브러리다. SQL을 직접 작성한다. 위에서 다룬 모든 내용이 여기에 해당한다. 장점은 의존성이 없고, SQL을 완전히 제어할 수 있다는 것이다. 단점은 `Scan`에서 필드를 하나씩 매핑하는 것이 번거롭다는 것이다.

### sqlx — database/sql의 확장

`database/sql`을 감싼 라이브러리다. 핵심 기능은 struct 자동 매핑이다:

```go
type User struct {
    ID   int    `db:"id"`
    Name string `db:"name"`
    Age  int    `db:"age"`
}

// database/sql - 필드를 하나씩 Scan
var u User
err := db.QueryRow("SELECT id, name, age FROM users WHERE id = $1", 1).Scan(&u.ID, &u.Name, &u.Age)

// sqlx - struct로 바로 매핑
var u User
err := db.Get(&u, "SELECT id, name, age FROM users WHERE id = $1", 1)
```

여러 행도 슬라이스로 받을 수 있다:

```go
var users []User
err := db.Select(&users, "SELECT id, name, age FROM users WHERE age > $1", 20)
```

SQL을 그대로 쓰면서 Scan의 번거로움만 해결한다. `database/sql`과 완전히 호환되므로 기존 코드에 점진적으로 도입할 수 있다.

### sqlc — SQL에서 Go 코드 생성

SQL 파일을 작성하면 타입 안전한 Go 코드를 생성해 주는 도구다:

```sql
-- query.sql
-- name: GetUser :one
SELECT id, name, age FROM users WHERE id = $1;

-- name: ListUsers :many
SELECT id, name, age FROM users WHERE age > $1;
```

`sqlc generate`를 실행하면 다음과 같은 Go 코드가 생성된다:

```go
// 자동 생성된 코드
func (q *Queries) GetUser(ctx context.Context, id int) (User, error) { ... }
func (q *Queries) ListUsers(ctx context.Context, age int) ([]User, error) { ... }
```

SQL을 직접 작성하면서도 컴파일 타임 타입 안전성을 얻는다. SQL 문법 오류도 코드 생성 시점에 잡힌다. 17편에서 다룬 코드 생성 패턴의 실전 사례다.

### GORM — ORM

Go에서 가장 널리 사용되는 ORM이다:

```go
type User struct {
    ID   uint   `gorm:"primaryKey"`
    Name string
    Age  int
}

// 조회
var user User
db.First(&user, 1)

// 생성
db.Create(&User{Name: "Alice", Age: 30})

// 조건 조회
var users []User
db.Where("age > ?", 20).Find(&users)
```

Prisma나 TypeORM과 비슷한 위치다. SQL을 직접 작성하지 않아도 되지만, 복잡한 쿼리에서는 ORM이 생성하는 SQL을 이해해야 한다.

### 어떤 것을 고를까

| 도구 | SQL 작성 | 타입 안전성 | Node.js 대응 |
|---|---|---|---|
| database/sql | 직접 | 런타임 Scan | pg, mysql2 |
| sqlx | 직접 | 런타임 (struct tag) | - |
| sqlc | 직접 | 컴파일 타임 | - |
| GORM | 자동 생성 | 런타임 (리플렉션) | Prisma, TypeORM |

Go 커뮤니티에서는 ORM보다 SQL을 직접 작성하는 쪽을 선호하는 경향이 있다. `database/sql` + sqlx로 시작하고, 타입 안전성이 필요하면 sqlc를 도입하는 것이 일반적인 경로다.

## 마이그레이션

스키마 마이그레이션 도구도 필요하다. 대표적인 두 가지:

- **golang-migrate** — SQL 파일 기반. 데이터베이스에 독립적. CLI와 라이브러리 모두 제공한다.
- **goose** — SQL 파일 또는 Go 코드로 마이그레이션을 작성할 수 있다.

둘 다 `database/sql`과 함께 동작한다. 마이그레이션 자체는 언어에 독립적인 SQL 작업이므로, 도구 선택은 취향 차이다.

`database/sql`은 드라이버만 교체하면 동일한 API로 모든 데이터베이스를 다룰 수 있게 한다. `Scan`의 번거로움은 sqlx나 sqlc로 해결할 수 있다.
