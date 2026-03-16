# SELECT와 조건 필터링

SELECT는 가장 자주 쓰는 SQL 문이지만, 작성 순서와 실행 순서가 다르다. 이 차이를 이해하면 WHERE 조건이 인덱스를 타는지, ORDER BY가 filesort를 유발하는지 예측할 수 있다.

## SELECT 문의 실행 순서

SQL을 작성할 때는 `SELECT`부터 쓴다. 하지만 MySQL이 실제로 처리하는 순서는 다르다:

```
작성 순서: SELECT → FROM → WHERE → ORDER BY → LIMIT
실행 순서: FROM → WHERE → SELECT → ORDER BY → LIMIT
```

정확한 실행 순서는 이렇다:

1. **FROM** — 대상 테이블을 결정하고 데이터를 읽는다
2. **WHERE** — 조건에 맞지 않는 행을 제거한다
3. **SELECT** — 필요한 컬럼만 추출한다
4. **ORDER BY** — 결과를 정렬한다
5. **LIMIT** — 지정한 개수만큼 잘라낸다

이 순서가 중요한 이유는 각 단계에서 쓸 수 있는 것이 달라지기 때문이다:

```sql
SELECT name, salary * 12 AS annual
FROM employees
WHERE annual > 50000000;  -- 에러: WHERE는 SELECT보다 먼저 실행된다
```

`WHERE`는 `SELECT`보다 먼저 실행되므로 alias `annual`을 알지 못한다. 이 쿼리는 에러가 난다. 반면 `ORDER BY`는 `SELECT` 이후에 실행되므로 alias를 쓸 수 있다:

```sql
SELECT name, salary * 12 AS annual
FROM employees
ORDER BY annual DESC;  -- OK: ORDER BY는 SELECT 이후에 실행된다
```

## WHERE 조건

### 비교 연산자

```sql
SELECT * FROM orders WHERE amount >= 10000;
SELECT * FROM users WHERE status != 'inactive';
SELECT * FROM products WHERE price < 5000;
```

`!=`와 `<>`는 동일하다. MySQL에서는 둘 다 "같지 않음"을 의미한다.

### IN

```sql
SELECT * FROM orders WHERE status IN ('pending', 'processing', 'shipped');
```

`IN`은 내부적으로 동등 비교(`=`)의 OR 조합으로 처리된다. 값 목록이 상수이면 정렬 후 binary search로 매칭하므로 효율적이다. 인덱스가 있으면 각 값에 대해 index lookup을 수행한다.

`IN`에 서브쿼리를 넣는 것도 가능하지만, 성능 특성이 달라진다. 이 부분은 10편에서 다룬다.

### BETWEEN

```sql
SELECT * FROM orders
WHERE created_at BETWEEN '2025-01-01' AND '2025-12-31';
```

`BETWEEN`은 양 끝을 포함하는 범위 조건이다. `>=`와 `<=`의 조합과 동일하다. B-tree 인덱스에서 범위 스캔(range scan)으로 처리되므로 인덱스를 효과적으로 활용한다.

주의할 점이 있다. `BETWEEN '2025-01-01' AND '2025-12-31'`은 `created_at`이 `DATETIME` 타입일 때 `2025-12-31 00:00:00`까지만 포함한다. 12월 31일 오후의 데이터는 빠진다. 정확하게 하려면:

```sql
WHERE created_at >= '2025-01-01' AND created_at < '2026-01-01'
```

### IS NULL

```sql
SELECT * FROM users WHERE deleted_at IS NULL;
```

`= NULL`은 동작하지 않는다. SQL에서 NULL은 "알 수 없는 값"이므로 어떤 비교 연산도 NULL을 반환한다. `NULL = NULL`조차 NULL이다(TRUE가 아니다). 반드시 `IS NULL` 또는 `IS NOT NULL`을 써야 한다.

InnoDB의 B-tree 인덱스는 NULL 값도 저장한다. `IS NULL` 조건도 인덱스를 탈 수 있다.

## 조건별 인덱스 활용

04편에서 다룬 B-tree 구조를 떠올려 보자. B-tree는 값이 정렬된 상태로 저장되어 있어서, 특정 값을 찾거나 범위를 스캔하는 데 효율적이다. 하지만 모든 WHERE 조건이 인덱스를 활용하는 것은 아니다.

### 인덱스를 타는 조건

```sql
-- 동등 비교: B-tree에서 정확한 위치로 바로 이동
WHERE id = 100

-- 범위 조건: B-tree의 정렬 특성을 활용한 range scan
WHERE price BETWEEN 1000 AND 5000
WHERE created_at >= '2025-01-01'

-- IN: 각 값에 대해 개별 index lookup
WHERE status IN ('active', 'pending')

-- 선행 문자열 LIKE: B-tree 정렬 순서와 일치
WHERE name LIKE 'Kim%'
```

### 인덱스를 타지 못하는 조건

```sql
-- 컬럼에 함수 적용: 인덱스에 저장된 원본 값과 비교할 수 없다
WHERE YEAR(created_at) = 2025

-- 컬럼에 연산 적용: 같은 이유
WHERE price * 1.1 > 10000

-- 후방 LIKE: B-tree 정렬 순서와 맞지 않는다
WHERE name LIKE '%Kim'

-- 부정 조건: 특정 값을 제외한 나머지를 찾아야 하므로 범위를 특정할 수 없다
WHERE status != 'deleted'
```

함수를 적용한 경우의 해결책:

```sql
-- 나쁜 예: 인덱스를 못 탄다
WHERE YEAR(created_at) = 2025

-- 좋은 예: 범위 조건으로 변환하면 인덱스를 탄다
WHERE created_at >= '2025-01-01' AND created_at < '2026-01-01'
```

```sql
-- 나쁜 예: 인덱스를 못 탄다
WHERE price * 1.1 > 10000

-- 좋은 예: 연산을 상수 쪽으로 이동
WHERE price > 10000 / 1.1
```

핵심 원칙은 하나다. **인덱스 컬럼을 가공하지 말 것**. 컬럼이 인덱스에 저장된 원본 형태 그대로 비교되어야 B-tree 탐색이 가능하다.

## ORDER BY의 내부 동작

정렬은 두 가지 방식으로 처리된다.

### 인덱스 정렬

B-tree 인덱스는 이미 정렬되어 있다. ORDER BY의 정렬 순서가 인덱스 순서와 일치하면 별도의 정렬 작업 없이 인덱스를 순서대로 읽기만 하면 된다:

```sql
-- idx_created_at 인덱스가 있다면
SELECT * FROM orders ORDER BY created_at;
```

EXPLAIN 결과에서 `Extra` 컬럼에 `Using filesort`가 나타나지 않으면 인덱스로 정렬을 처리한 것이다. `Using index`는 커버링 인덱스(인덱스만으로 쿼리를 처리하여 테이블 데이터에 접근하지 않음)를 의미하는 것이므로, 정렬 방식의 판단 기준과 혼동하지 않아야 한다.

### filesort

인덱스를 활용할 수 없으면 MySQL은 filesort를 수행한다. filesort라는 이름과 달리 반드시 디스크에 쓰는 것은 아니다. 데이터가 sort buffer(`sort_buffer_size`)에 들어가면 메모리에서 정렬하고, 초과하면 임시 파일을 사용한다.

```sql
-- name 컬럼에 인덱스가 없다면 filesort 발생
SELECT * FROM users ORDER BY name;
```

EXPLAIN 결과에서 `Extra`에 `Using filesort`가 나타난다.

filesort의 알고리즘은 두 가지다:

- **Two-pass (original)**: 정렬 키와 row pointer만 정렬한 뒤, 정렬된 순서대로 테이블을 다시 읽어서 나머지 컬럼을 가져온다. 메모리를 적게 쓰지만 테이블을 두 번 읽는다.
- **Single-pass (modified)**: 정렬 키와 필요한 컬럼을 모두 sort buffer에 넣고 정렬한다. 테이블을 한 번만 읽지만 메모리를 더 쓴다.

MySQL은 행의 크기와 `max_length_for_sort_data` 설정을 보고 자동으로 선택한다. 단, `max_length_for_sort_data`는 MySQL 8.0.20에서 deprecated되었으며 현재 버전에서는 아무 효과가 없다. 8.0.20 이후에는 옵티마이저가 자체적으로 최적의 알고리즘을 결정한다.

### 복합 인덱스와 ORDER BY

WHERE 조건과 ORDER BY를 함께 처리하려면 복합 인덱스의 컬럼 순서가 맞아야 한다:

```sql
-- 복합 인덱스: (status, created_at)

-- 인덱스로 WHERE + ORDER BY 모두 처리 가능
SELECT * FROM orders
WHERE status = 'pending'
ORDER BY created_at;

-- status가 동등 조건이므로, 그 범위 내에서 created_at은 이미 정렬되어 있다
```

05편에서 다룬 복합 인덱스의 정렬 원리가 여기에 적용된다. `(status, created_at)` 인덱스에서 `status = 'pending'`인 영역은 `created_at` 순서로 정렬되어 있으므로 추가 정렬이 필요 없다.

## LIMIT의 동작과 함정

### 기본 동작

```sql
SELECT * FROM products ORDER BY price LIMIT 10;
```

LIMIT은 결과 행의 수를 제한한다. `LIMIT 10`은 상위 10건만 반환한다. `LIMIT 10, 20`은 11번째부터 20건을 반환한다(offset 10, count 20).

### LIMIT과 ORDER BY

LIMIT은 실행 순서의 마지막이다. 이 점이 성능 함정을 만든다:

```sql
SELECT * FROM orders ORDER BY created_at DESC LIMIT 10;
```

`created_at`에 인덱스가 있으면 인덱스를 역순으로 10건만 읽고 멈춘다. 매우 빠르다.

`created_at`에 인덱스가 없으면 전체 테이블을 읽고, 전부 정렬한 뒤, 10건만 반환한다. 100만 행을 정렬하고 10건만 쓰는 셈이다.

### 깊은 페이징의 성능 문제

```sql
-- 1페이지: 빠르다
SELECT * FROM orders ORDER BY id LIMIT 0, 20;

-- 5000페이지: 느리다
SELECT * FROM orders ORDER BY id LIMIT 99980, 20;
```

`LIMIT 99980, 20`은 100,000건을 읽은 뒤 앞의 99,980건을 버리고 20건만 반환한다. offset이 커질수록 읽고 버리는 행이 늘어난다.

해결 방법은 커서 기반 페이징이다:

```sql
-- 이전 페이지의 마지막 id가 99980이었다면
SELECT * FROM orders WHERE id > 99980 ORDER BY id LIMIT 20;
```

`WHERE id > 99980`은 인덱스에서 해당 위치로 바로 이동하므로, 앞선 행을 읽을 필요가 없다. 몇 번째 페이지든 일정한 성능을 보장한다.

## 풀 스캔 vs 인덱스 스캔

옵티마이저는 항상 인덱스를 쓰지 않는다. 인덱스 스캔이 오히려 느릴 수 있기 때문이다.

06편에서 다룬 것처럼 secondary index를 통한 조회는 인덱스에서 primary key를 찾고, 다시 클러스터드 인덱스에서 실제 행을 읽는 두 단계를 거친다. 행 하나마다 random I/O가 발생한다.

테이블의 대부분을 읽어야 한다면, random I/O를 반복하는 것보다 테이블을 sequential하게 전체 스캔하는 편이 빠르다. MySQL 옵티마이저는 대략적으로 전체 행의 20~30%를 초과하면 풀 스캔을 선택한다.

```sql
-- status가 'active'인 행이 전체의 90%라면
SELECT * FROM users WHERE status = 'active';
-- 인덱스가 있어도 풀 스캔을 선택한다
```

```sql
-- status가 'suspended'인 행이 전체의 0.1%라면
SELECT * FROM users WHERE status = 'suspended';
-- 인덱스를 사용한다
```

이 판단은 테이블 통계 정보를 기반으로 한다. InnoDB는 인덱스의 cardinality(고유 값의 수)와 행 수를 샘플링하여 통계를 유지한다. 통계가 부정확하면 옵티마이저가 잘못된 선택을 할 수 있다. `ANALYZE TABLE`로 통계를 갱신하면 해결되는 경우가 많다.

## EXPLAIN으로 확인하기

쿼리가 인덱스를 타는지, filesort가 발생하는지는 추측하지 말고 EXPLAIN으로 확인한다:

```sql
EXPLAIN SELECT * FROM orders WHERE status = 'pending' ORDER BY created_at;
```

주요 확인 항목:

- **type**: `ALL`이면 풀 스캔, `ref`나 `range`면 인덱스 스캔
- **key**: 실제 사용된 인덱스 이름
- **rows**: 옵티마이저가 예측한 읽을 행 수
- **Extra**: `Using filesort`(별도 정렬), `Using temporary`(임시 테이블), `Using index`(커버링 인덱스)

```sql
EXPLAIN SELECT * FROM orders
WHERE status = 'pending' ORDER BY amount;
```

```
+----+------+---------------+------+---------+------+------+-----------------------------+
| id | type | possible_keys | key  | key_len | ref  | rows | Extra                       |
+----+------+---------------+------+---------+------+------+-----------------------------+
|  1 | ref  | idx_status    | idx_status | 42 | const| 1500 | Using index condition;      |
|    |      |               |      |         |      |      | Using filesort              |
+----+------+---------------+------+---------+------+------+-----------------------------+
```

이 결과는 `idx_status` 인덱스로 `status = 'pending'`을 필터링한 뒤, `amount` 순으로 filesort가 발생한다는 의미다. `(status, amount)` 복합 인덱스를 만들면 filesort를 제거할 수 있다.

EXPLAIN의 상세한 읽는 방법은 18편에서 다룬다. 지금은 `type`, `key`, `Extra` 세 컬럼만 확인하는 습관을 들이면 충분하다.

## 정리

- SELECT의 작성 순서와 실행 순서는 다르다. FROM, WHERE, SELECT, ORDER BY, LIMIT 순으로 실행된다.
- 인덱스 컬럼을 가공하지 않아야 B-tree 탐색이 가능하다. 함수 적용, 암묵적 타입 변환, 후방 LIKE는 인덱스를 무력화한다.
- ORDER BY가 인덱스 순서와 일치하면 filesort 없이 결과를 반환할 수 있다.
- 깊은 페이지네이션에서 OFFSET은 성능 저하를 유발하므로, 커서 기반 페이지네이션이 대안이다.
- 테이블의 대부분을 읽어야 하는 쿼리에서는 인덱스보다 풀 스캔이 더 효율적일 수 있다.