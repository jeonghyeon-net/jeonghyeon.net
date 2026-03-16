# EXPLAIN 완전 분석

쿼리가 느릴 때 가장 먼저 해야 할 일은 `EXPLAIN`을 실행하는 것이다. EXPLAIN은 옵티마이저가 선택한 실행 계획을 보여준다. 이 출력을 정확히 읽을 수 있으면, 쿼리의 병목이 어디에 있는지 코드를 수정하기 전에 알 수 있다.

## EXPLAIN 기본 사용법

쿼리 앞에 `EXPLAIN`을 붙이면 된다. 쿼리를 실제로 실행하지 않고 실행 계획만 반환한다.

```sql
EXPLAIN SELECT * FROM orders WHERE user_id = 42 ORDER BY created_at DESC;
```

```
+----+-------------+--------+------+---------------+--------------+---------+-------+------+-------------+
| id | select_type | table  | type | possible_keys | key          | key_len | ref   | rows | Extra       |
+----+-------------+--------+------+---------------+--------------+---------+-------+------+-------------+
|  1 | SIMPLE      | orders | ref  | idx_user_id   | idx_user_id  | 4       | const |   15 | Using where |
+----+-------------+--------+------+---------------+--------------+---------+-------+------+-------------+
```

출력의 각 행은 쿼리에서 접근하는 하나의 테이블에 대응한다. JOIN이 있으면 행이 여러 개 나온다. 각 컬럼이 무엇을 의미하는지 하나씩 살펴본다.

## 모든 컬럼 해석

### id

쿼리 내의 SELECT 번호다. 단순 쿼리는 항상 1이다. 서브쿼리나 UNION이 있으면 번호가 증가한다.

```sql
EXPLAIN
SELECT * FROM users
WHERE id IN (SELECT user_id FROM orders WHERE total > 10000);
```

이 경우 외부 SELECT는 `id = 1`, 서브쿼리는 `id = 2`가 된다. 같은 `id`를 가진 행들은 JOIN으로 처리되는 테이블들이다.

### select_type

해당 SELECT의 유형을 나타낸다. 주요 값은 다음과 같다.

- **SIMPLE**: 서브쿼리나 UNION이 없는 단순 SELECT.
- **PRIMARY**: 가장 바깥쪽 SELECT.
- **SUBQUERY**: SELECT 절이나 WHERE 절의 서브쿼리.
- **DERIVED**: FROM 절의 서브쿼리. 임시 테이블로 구체화(materialize)된다.
- **UNION**: UNION의 두 번째 이후 SELECT.
- **DEPENDENT SUBQUERY**: 외부 쿼리의 값에 의존하는 상관 서브쿼리. 외부 행마다 반복 실행되므로 성능에 주의가 필요하다.

### table

접근하는 테이블 이름이다. `<derived2>`처럼 표시되면 `id = 2`의 파생 테이블(FROM 절 서브쿼리)을 의미한다. `<subquery2>`는 구체화된 서브쿼리다.

### partitions

파티셔닝된 테이블에서 쿼리가 접근하는 파티션 목록이다. 파티셔닝을 사용하지 않으면 NULL이다. 파티셔닝된 테이블에서 이 값을 확인하면 파티션 프루닝이 제대로 작동하는지 알 수 있다.

### type

**가장 중요한 컬럼이다.** 테이블에 어떤 방식으로 접근하는지를 나타낸다. 뒤에서 별도로 상세히 다룬다.

### possible_keys

옵티마이저가 사용을 고려한 인덱스 목록이다. 여기에 인덱스가 나열되어 있어도 실제로 사용되지 않을 수 있다. NULL이면 사용 가능한 인덱스가 아예 없다는 뜻이다.

### key

실제로 사용된 인덱스다. NULL이면 인덱스를 사용하지 않았다.

`possible_keys`에는 있지만 `key`에는 없는 경우가 흔하다. 옵티마이저가 비용을 계산한 결과, 인덱스를 쓰는 것보다 전체 스캔이 낫다고 판단한 것이다. 이때 인덱스 통계가 부정확하면 잘못된 판단일 수 있다.

### key_len

사용된 인덱스의 바이트 길이다. 복합 인덱스에서 어디까지 사용되었는지 판단하는 핵심 정보다. 뒤에서 별도로 다룬다.

### ref

인덱스와 비교되는 값의 출처다. 주요 값은 다음과 같다.

- **const**: 상수 값과 비교. `WHERE id = 1` 같은 경우.
- **테이블.컬럼**: JOIN에서 다른 테이블의 컬럼과 비교. `orders.user_id`처럼 표시된다.
- **func**: 함수의 결과와 비교.
- **NULL**: 인덱스를 사용하지 않거나 range 스캔인 경우.

### rows

옵티마이저가 추정한, 읽어야 할 행의 수다. **실제 결과 행 수가 아니라 추정치다.** 테이블 통계를 기반으로 계산된다.

JOIN에서 이 값이 중요하다. 두 테이블의 `rows`가 각각 1,000과 500이면, 최악의 경우 1,000 x 500 = 50만 번의 비교가 발생할 수 있다는 뜻이다.

### filtered

`rows`에서 읽은 행 중 WHERE 조건을 통과할 것으로 예상되는 비율(%)이다. `rows`가 1,000이고 `filtered`가 10.00이면 약 100행이 최종 결과에 포함될 것으로 추정한다. JOIN에서 다음 테이블로 전달되는 행 수를 추정할 때 `rows * filtered / 100`으로 계산한다.

### Extra

실행 계획에 대한 추가 정보다. 가장 다양한 값이 나타나는 컬럼이며, 쿼리 최적화의 핵심 단서가 된다. 뒤에서 별도로 다룬다.

## type 컬럼 상세

type은 성능 순으로 나열하면 다음과 같다. 위에서 아래로 갈수록 느려진다.

### system

테이블에 행이 정확히 하나만 있는 경우다. `const`의 특수한 형태다. 시스템 테이블에서 간혹 보이지만 일반적인 쿼리에서는 거의 나타나지 않는다.

### const

PRIMARY KEY 또는 UNIQUE 인덱스로 상수 값을 비교하여, 최대 하나의 행만 반환되는 경우다.

```sql
EXPLAIN SELECT * FROM users WHERE id = 1;
```

```
+----+-------------+-------+-------+---------------+---------+---------+-------+------+-------+
| id | select_type | table | type  | possible_keys | key     | key_len | ref   | rows | Extra |
+----+-------------+-------+-------+---------------+---------+---------+-------+------+-------+
|  1 | SIMPLE      | users | const | PRIMARY       | PRIMARY | 4       | const |    1 | NULL  |
+----+-------------+-------+-------+---------------+---------+---------+-------+------+-------+
```

옵티마이저가 쿼리 최적화 시점에 값을 읽어서 상수로 치환한다. 가장 빠른 접근 방식이다.

### eq_ref

JOIN에서 두 번째 테이블을 PRIMARY KEY 또는 UNIQUE NOT NULL 인덱스로 정확히 하나의 행만 읽는 경우다.

```sql
EXPLAIN
SELECT o.*, u.name
FROM orders o
JOIN users u ON u.id = o.user_id;
```

`users` 테이블의 접근 type이 `eq_ref`로 표시된다. `orders`의 각 행에 대해 `users`에서 PK로 딱 하나의 행만 찾기 때문이다.

### ref

비고유(non-unique) 인덱스로 동등 비교하여 여러 행이 반환될 수 있는 경우다.

```sql
EXPLAIN SELECT * FROM orders WHERE user_id = 42;
```

`user_id`에 일반 인덱스가 있으면 `ref`가 된다. 한 사용자가 여러 주문을 가질 수 있으므로 결과가 여러 행이다.

### range

인덱스를 사용한 범위 검색이다. `>`, `<`, `>=`, `<=`, `BETWEEN`, `IN`, `LIKE 'prefix%'` 등이 해당한다.

```sql
EXPLAIN SELECT * FROM orders WHERE created_at BETWEEN '2025-01-01' AND '2025-12-31';
```

인덱스의 특정 구간만 스캔하므로 전체 스캔보다 효율적이다. 하지만 범위가 넓으면 많은 행을 읽게 되어 느려질 수 있다.

### index

인덱스 전체를 스캔한다. 테이블 전체 스캔(ALL)과 비슷하지만, 인덱스만 읽으므로 데이터 크기가 작다.

```sql
EXPLAIN SELECT user_id FROM orders;
```

`user_id`에 인덱스가 있고 다른 컬럼이 필요 없으면, 인덱스만 순회하여 결과를 반환한다. 테이블 데이터 파일을 읽지 않아도 된다.

### ALL

**전체 테이블 스캔이다.** 인덱스를 사용하지 않고 테이블의 모든 행을 읽는다. 대부분의 경우 피해야 하는 접근 방식이다.

```sql
EXPLAIN SELECT * FROM orders WHERE YEAR(created_at) = 2025;
```

`created_at`에 인덱스가 있어도 `YEAR()` 함수로 감싸면 인덱스를 사용할 수 없다. 전체 테이블을 읽으며 각 행에서 `YEAR(created_at)`을 계산하여 비교한다.

## Extra 컬럼의 주요 값

Extra에는 다양한 정보가 표시된다. 자주 보게 되는 값들을 다룬다.

### Using index

커버링 인덱스로 처리되었다는 뜻이다. 인덱스만 읽으면 쿼리에 필요한 모든 컬럼을 얻을 수 있으므로, 테이블 데이터에 접근하지 않는다. 가장 효율적인 상태다.

```sql
-- 인덱스: (user_id, created_at)
EXPLAIN SELECT user_id, created_at FROM orders WHERE user_id = 42;
```

SELECT에 필요한 `user_id`, `created_at` 모두 인덱스에 포함되어 있으므로 `Using index`가 표시된다.

### Using where

스토리지 엔진이 반환한 행에 대해 서버 레이어에서 추가로 WHERE 조건을 필터링했다는 뜻이다. 인덱스로 걸러내지 못한 조건이 있다는 의미이므로, 인덱스 설계를 재검토할 여지가 있다.

### Using temporary

쿼리 처리를 위해 내부 임시 테이블이 생성되었다. GROUP BY와 ORDER BY의 기준이 다른 경우, DISTINCT와 ORDER BY를 함께 사용하는 경우 등에서 발생한다. 데이터 양이 많으면 임시 테이블이 디스크에 생성되어 성능이 크게 저하된다.

```sql
EXPLAIN SELECT DISTINCT user_id FROM orders ORDER BY created_at;
```

### Using filesort

결과를 정렬하기 위해 추가적인 정렬 작업이 필요하다는 뜻이다. 이름에 "file"이 들어 있지만 반드시 디스크를 사용하는 것은 아니다. 메모리에서 정렬되기도 한다. 인덱스 순서를 활용할 수 없을 때 발생한다.

```sql
-- 인덱스: (user_id)
EXPLAIN SELECT * FROM orders WHERE user_id = 42 ORDER BY total DESC;
```

`user_id`로 인덱스를 사용하여 행을 찾지만, `total` 기준 정렬은 인덱스로 해결되지 않으므로 filesort가 발생한다.

### Using index condition

**ICP**(Index Condition Pushdown)가 적용되었다. 일반적으로 인덱스로 행을 찾은 뒤 서버 레이어에서 WHERE 조건을 검사하지만, ICP가 적용되면 스토리지 엔진이 인덱스 수준에서 조건을 먼저 검사한다. 테이블 데이터 접근 횟수가 줄어든다.

```sql
-- 인덱스: (user_id, status)
EXPLAIN SELECT * FROM orders WHERE user_id > 100 AND status = 'shipped';
```

`user_id > 100`으로 range 스캔을 하면서, `status = 'shipped'` 조건을 인덱스 수준에서 필터링한다. 테이블 데이터를 읽기 전에 불필요한 행을 걸러내므로 I/O가 감소한다.

## key_len으로 인덱스 사용 범위 판단하기

복합 인덱스에서 `key_len`은 인덱스의 어떤 컬럼까지 사용되었는지 알려주는 핵심 지표다.

각 데이터 타입의 인덱스 크기는 다음과 같다.

| 타입 | 바이트 | NULL 허용 시 |
|------|--------|-------------|
| INT | 4 | +1 |
| BIGINT | 8 | +1 |
| DATE | 3 | +1 |
| DATETIME | 5 (MySQL 5.6.4+) | +1 |
| VARCHAR(N) | N * 문자셋 바이트 + 2 | +1 |

VARCHAR의 경우 utf8mb4이면 한 문자당 최대 4바이트다. `VARCHAR(50)`은 `50 * 4 + 2 = 202` 바이트다. `+2`는 가변 길이를 저장하는 length prefix다.

예를 들어 다음과 같은 복합 인덱스가 있다고 가정한다.

```sql
CREATE TABLE orders (
    user_id INT NOT NULL,
    status VARCHAR(20) NOT NULL,
    created_at DATETIME NOT NULL,
    INDEX idx_composite (user_id, status, created_at)
);
```

각 컬럼의 key_len은 다음과 같다.

- `user_id`: 4 바이트
- `status`: 20 * 4 + 2 = 82 바이트
- `created_at`: 5 바이트

```sql
-- user_id만 사용
EXPLAIN SELECT * FROM orders WHERE user_id = 1;
-- key_len = 4

-- user_id + status 사용
EXPLAIN SELECT * FROM orders WHERE user_id = 1 AND status = 'paid';
-- key_len = 86 (4 + 82)

-- 세 컬럼 모두 사용
EXPLAIN SELECT * FROM orders
WHERE user_id = 1 AND status = 'paid' AND created_at = '2025-03-01';
-- key_len = 91 (4 + 82 + 5)
```

key_len이 4라면 복합 인덱스의 첫 번째 컬럼(`user_id`)만 사용된 것이다. 86이면 두 번째 컬럼(`status`)까지, 91이면 세 번째 컬럼(`created_at`)까지 활용된 것이다.

범위 조건이 포함되면 그 다음 컬럼은 인덱스에서 활용되지 않는다.

```sql
EXPLAIN SELECT * FROM orders
WHERE user_id = 1 AND status > 'a' AND created_at = '2025-03-01';
-- key_len = 86
```

`status`에 범위 조건(`>`)이 걸렸으므로 `created_at`은 인덱스를 통해 필터링되지 않는다. key_len이 86에서 멈춘 것으로 이를 확인할 수 있다.

## EXPLAIN FORMAT=JSON

기본 테이블 형식보다 상세한 정보를 JSON으로 제공한다.

```sql
EXPLAIN FORMAT=JSON
SELECT * FROM orders WHERE user_id = 42 ORDER BY created_at DESC\G
```

```json
{
  "query_block": {
    "select_id": 1,
    "cost_info": {
      "query_cost": "5.46"
    },
    "ordering_operation": {
      "using_filesort": true,
      "table": {
        "table_name": "orders",
        "access_type": "ref",
        "possible_keys": ["idx_user_id"],
        "key": "idx_user_id",
        "used_key_parts": ["user_id"],
        "key_length": "4",
        "ref": ["const"],
        "rows_examined_per_scan": 15,
        "rows_produced_per_join": 15,
        "filtered": "100.00",
        "cost_info": {
          "read_cost": "3.96",
          "eval_cost": "1.50",
          "prefix_cost": "5.46",
          "data_read_per_join": "3K"
        },
        "used_columns": ["id", "user_id", "total", "status", "created_at"]
      }
    }
  }
}
```

JSON 형식에서 추가로 확인할 수 있는 정보는 다음과 같다.

- **cost_info**: 옵티마이저가 계산한 비용. `read_cost`는 데이터를 읽는 비용, `eval_cost`는 행을 평가하는 비용이다.
- **used_key_parts**: 복합 인덱스에서 실제로 사용된 컬럼 목록. key_len보다 직관적이다.
- **used_columns**: 쿼리에서 실제로 사용하는 컬럼 목록.

## EXPLAIN FORMAT=TREE

MySQL 8.0.16부터 지원하는 형식이다. 실행 계획을 트리 구조로 보여주어, 데이터가 어떤 순서로 흘러가는지 직관적으로 파악할 수 있다.

```sql
EXPLAIN FORMAT=TREE
SELECT u.name, COUNT(*) AS order_count
FROM users u
JOIN orders o ON o.user_id = u.id
WHERE u.status = 'active'
GROUP BY u.id\G
```

```
-> Group aggregate: count(0)
    -> Nested loop inner join  (cost=4.75 rows=10)
        -> Index lookup on u using idx_status (status='active')  (cost=2.50 rows=10)
        -> Index lookup on o using idx_user_id (user_id=u.id)  (cost=0.25 rows=1)
```

아래에서 위로 읽는다. 먼저 `users` 테이블에서 `status = 'active'`인 행을 인덱스로 찾고, 각 행에 대해 `orders` 테이블에서 매칭되는 행을 인덱스로 찾은 뒤, 그 결과를 그룹별로 집계한다.

## EXPLAIN ANALYZE

MySQL 8.0.18부터 지원된다. EXPLAIN과 달리 **쿼리를 실제로 실행**하고, 옵티마이저의 추정치와 실제 실행 결과를 나란히 보여준다.

```sql
EXPLAIN ANALYZE
SELECT u.name, COUNT(*) AS order_count
FROM users u
JOIN orders o ON o.user_id = u.id
WHERE u.status = 'active'
GROUP BY u.id\G
```

```
-> Group aggregate: count(0)  (actual time=0.512..0.892 rows=8 loops=1)
    -> Nested loop inner join  (cost=4.75 rows=10)
                               (actual time=0.089..0.834 rows=45 loops=1)
        -> Index lookup on u using idx_status (status='active')
           (cost=2.50 rows=10)
           (actual time=0.052..0.068 rows=8 loops=1)
        -> Index lookup on o using idx_user_id (user_id=u.id)
           (cost=0.25 rows=1)
           (actual time=0.008..0.091 rows=5.62 loops=8)
```

각 노드에 `actual time`, `rows`, `loops` 정보가 추가된다.

- **actual time**: `시작시간..종료시간` (밀리초). 첫 번째 행을 반환하기까지의 시간과 모든 행을 반환하기까지의 시간이다.
- **rows**: 실제로 반환된 행 수. 옵티마이저의 추정(`rows=10`)과 비교할 수 있다.
- **loops**: 해당 노드가 실행된 횟수. Nested loop join에서 내부 테이블은 외부 테이블의 행 수만큼 반복된다.

위 출력에서 옵티마이저는 `users`에서 10행을 예상했지만 실제로는 8행이 반환되었다. `orders` 조회는 `loops=8`로 8번 실행되었고, 평균 5.62행을 반환했다. 옵티마이저 추정(1행)과 실제(5.62행)의 차이가 크다면, 인덱스 통계가 부정확하거나 데이터 분포가 편향된 것일 수 있다.

**주의**: EXPLAIN ANALYZE는 쿼리를 실제로 실행하므로, 데이터를 변경하는 DML 문에 사용하면 안 된다. SELECT에만 사용한다.

## 실전: 느린 쿼리의 EXPLAIN 분석

다음과 같은 쿼리가 느리다는 보고가 들어왔다고 가정한다.

```sql
SELECT o.id, o.total, u.name, u.email
FROM orders o
JOIN users u ON u.id = o.user_id
WHERE o.status = 'pending'
  AND o.created_at >= '2025-01-01'
ORDER BY o.created_at DESC
LIMIT 20;
```

EXPLAIN을 실행한다.

```sql
EXPLAIN SELECT o.id, o.total, u.name, u.email
FROM orders o
JOIN users u ON u.id = o.user_id
WHERE o.status = 'pending'
  AND o.created_at >= '2025-01-01'
ORDER BY o.created_at DESC
LIMIT 20\G
```

```
*************************** 1. row ***************************
           id: 1
  select_type: SIMPLE
        table: o
         type: ALL
possible_keys: idx_user_id
          key: NULL
      key_len: NULL
          ref: NULL
         rows: 524288
     filtered: 1.11
        Extra: Using where; Using filesort
*************************** 2. row ***************************
           id: 1
  select_type: SIMPLE
        table: u
         type: eq_ref
possible_keys: PRIMARY
          key: PRIMARY
      key_len: 4
          ref: mydb.o.user_id
         rows: 1
     filtered: 100.00
        Extra: NULL
```

이 실행 계획의 문제점을 분석한다.

**1단계: type 확인**

`orders` 테이블의 type이 `ALL`이다. 52만 행 전체를 스캔하고 있다. `status`와 `created_at`에 대한 인덱스가 없기 때문이다.

**2단계: rows와 filtered 확인**

52만 행을 읽고 `filtered`가 1.11%다. 약 5,800행만 조건에 맞는다는 뜻이다. 나머지 51만 행은 읽기만 하고 버린다.

**3단계: Extra 확인**

`Using filesort`가 보인다. 조건을 통과한 5,800행을 `created_at DESC`로 정렬한 뒤 상위 20건만 반환한다. 인덱스를 활용하면 정렬을 생략할 수 있다.

**해결: 복합 인덱스 생성**

```sql
ALTER TABLE orders ADD INDEX idx_status_created (status, created_at);
```

이 인덱스를 만들고 다시 EXPLAIN을 실행한다.

```
*************************** 1. row ***************************
           id: 1
  select_type: SIMPLE
        table: o
         type: range
possible_keys: idx_user_id,idx_status_created
          key: idx_status_created
      key_len: 87
          ref: NULL
         rows: 5765
     filtered: 100.00
        Extra: Using index condition; Backward index scan
```

변화를 확인한다.

- type이 `ALL` -> `range`로 바뀌었다. 인덱스의 특정 범위만 스캔한다.
- rows가 52만 -> 5,765로 줄었다. 필요한 행만 읽는다.
- filtered가 100%다. 인덱스 수준에서 조건을 모두 처리했다.
- `Using filesort`가 사라졌다. 인덱스가 `(status, created_at)` 순서이므로, `status = 'pending'` 조건을 만족하는 범위 내에서 `created_at` 순서가 이미 보장된다. `Backward index scan`은 `DESC` 정렬을 위해 인덱스를 역순으로 읽고 있다는 뜻이다.

## 정리

- `type` 컬럼으로 테이블 접근 방식을 판단한다. `ALL`(전체 스캔)에서 `const`(PK 단건 조회)까지, 위로 갈수록 효율적이다.
- `key`와 `key_len`으로 인덱스 활용 범위를 확인한다. 복합 인덱스에서 key_len을 계산하면 어떤 컬럼까지 사용되었는지 알 수 있다.
- `Extra` 컬럼에서 `Using filesort`, `Using temporary` 같은 추가 작업의 존재를 확인한다. `Using index`는 커버링 인덱스가 적용된 가장 효율적인 상태다.
- EXPLAIN FORMAT=JSON은 옵티마이저의 비용 정보와 실제 사용된 인덱스 컬럼 목록을 제공한다.
- EXPLAIN ANALYZE는 쿼리를 실제로 실행하여 추정치와 실제 결과를 비교한다. SELECT에만 사용한다.
