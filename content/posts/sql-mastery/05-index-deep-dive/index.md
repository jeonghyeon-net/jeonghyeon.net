# 인덱스 심화

04편에서 인덱스의 구조를 살펴봤다. B+tree가 검색을 빠르게 만드는 원리, clustered index와 secondary index의 차이를 이해했다. 이번에는 실무에서 인덱스를 설계하고 활용하는 데 필요한 심화 개념을 다룬다.

## 복합 인덱스와 leftmost prefix rule

단일 컬럼 인덱스는 하나의 컬럼 값으로 B+tree를 구성한다. 복합 인덱스(composite index)는 여러 컬럼을 조합하여 하나의 B+tree를 만든다:

```sql
CREATE INDEX idx_customer_date ON orders (customer_id, order_date);
```

이 인덱스의 B+tree는 먼저 `customer_id`로 정렬하고, 같은 `customer_id` 안에서 `order_date`로 정렬한다. 전화번호부가 성(last name)으로 먼저 정렬하고, 같은 성 안에서 이름(first name)으로 정렬하는 것과 같다.

리프 노드의 데이터 배치:

```
[customer_id=1, order_date=2024-01-15, PK=7]
[customer_id=1, order_date=2024-03-20, PK=23]
[customer_id=1, order_date=2024-07-01, PK=45]
[customer_id=2, order_date=2024-02-10, PK=12]
[customer_id=2, order_date=2024-05-30, PK=67]
...
```

이 정렬 구조 때문에, 복합 인덱스는 특정 규칙 하에서만 사용될 수 있다. 이것이 leftmost prefix rule이다.

### leftmost prefix rule

`(customer_id, order_date)` 인덱스가 사용되는 경우:

```sql
-- customer_id만 사용 → 인덱스 사용 가능
SELECT * FROM orders WHERE customer_id = 42;

-- customer_id + order_date 모두 사용 → 인덱스 사용 가능
SELECT * FROM orders WHERE customer_id = 42 AND order_date = '2024-01-15';

-- customer_id 범위 + order_date → customer_id 부분만 인덱스 사용
SELECT * FROM orders WHERE customer_id > 10 AND order_date = '2024-01-15';
```

인덱스가 사용되지 않는 경우:

```sql
-- order_date만 사용 → 인덱스 사용 불가
SELECT * FROM orders WHERE order_date = '2024-01-15';
```

전화번호부에서 이름만 가지고 찾을 수 없는 것과 같다. 성을 모르면 전화번호부의 정렬이 아무 도움이 되지 않는다.

규칙을 정리하면: 복합 인덱스 `(A, B, C)`는 다음 조합에서 사용된다:

- `A`
- `A, B`
- `A, B, C`

`B` 단독, `C` 단독, `B, C` 조합으로는 이 인덱스를 활용할 수 없다. 항상 왼쪽부터 연속된 컬럼이어야 한다.

### 컬럼 순서가 중요하다

`(customer_id, order_date)` 인덱스와 `(order_date, customer_id)` 인덱스는 완전히 다른 B+tree다. 어떤 쿼리를 지원해야 하는지에 따라 컬럼 순서를 결정해야 한다.

일반적인 원칙:

- 등호(`=`) 조건에 사용되는 컬럼을 앞에 놓는다.
- 범위 조건(`>`, `<`, `BETWEEN`)에 사용되는 컬럼을 뒤에 놓는다.
- 범위 조건 이후의 컬럼은 인덱스 탐색에 활용되지 않는다.

```sql
-- (customer_id, order_date) 인덱스에서:

-- customer_id=42 → 등호. order_date BETWEEN → 범위.
-- customer_id로 정확히 필터링 후, order_date 범위를 효율적으로 탐색.
SELECT * FROM orders
WHERE customer_id = 42
AND order_date BETWEEN '2024-01-01' AND '2024-06-30';
```

만약 인덱스가 `(order_date, customer_id)`였다면, `order_date` 범위 이후의 `customer_id` 조건은 인덱스 탐색이 아닌 필터링으로 처리된다. 효율이 떨어진다.

## Covering index

04편에서 secondary index 검색의 두 번째 단계인 PK lookup을 설명했다. Secondary index의 리프 노드에는 인덱스 키와 PK만 있으므로, `SELECT *`를 실행하면 나머지 컬럼을 가져오기 위해 clustered index를 다시 탐색해야 한다.

그런데 쿼리가 필요로 하는 모든 컬럼이 인덱스에 포함되어 있다면? PK lookup이 필요 없다. 인덱스만으로 쿼리 결과를 완성할 수 있다. 이것을 covering index라고 한다.

```sql
-- (customer_id, order_date) 인덱스가 있을 때:

-- Covering index가 되는 쿼리
SELECT customer_id, order_date FROM orders WHERE customer_id = 42;

-- Covering index가 되지 않는 쿼리 (amount는 인덱스에 없다)
SELECT customer_id, order_date, amount FROM orders WHERE customer_id = 42;
```

첫 번째 쿼리는 `customer_id`와 `order_date`만 필요하다. 둘 다 인덱스에 있다. PK도 secondary index 리프 노드에 항상 포함되어 있으므로, PK를 SELECT해도 covering index가 된다. Clustered index를 읽을 필요가 없으므로 PK lookup이 생략된다.

`EXPLAIN`으로 확인하면 `Extra` 컬럼에 `Using index`가 표시된다:

```sql
EXPLAIN SELECT customer_id, order_date FROM orders WHERE customer_id = 42;
```

```
+----+------+----------------+---------+------+-------------+
| id | type | key            | key_len | rows | Extra       |
+----+------+----------------+---------+------+-------------+
|  1 | ref  | idx_cust_date  | 4       |   10 | Using index |
+----+------+----------------+---------+------+-------------+
```

`Using index`는 "인덱스만으로 쿼리를 해결했다"는 뜻이다. PK lookup이 없으므로 I/O가 크게 줄어든다. 특히 결과 행이 많을 때 차이가 극명하다. 100건의 결과가 있다면, PK lookup 없이 100회의 clustered index 탐색을 절약하는 셈이다.

실무에서는 자주 실행되는 쿼리를 분석하여, 해당 쿼리가 covering index의 혜택을 받도록 인덱스를 설계하기도 한다. 하지만 모든 쿼리를 covering index로 만들겠다고 인덱스에 컬럼을 과도하게 추가하면, 인덱스 크기가 비대해지고 쓰기 성능이 저하된다. 균형이 중요하다.

## 인덱스 선택도(cardinality)

cardinality는 인덱스 컬럼에 존재하는 고유 값(distinct value)의 수를 의미한다. MySQL 옵티마이저는 이 값을 기준으로 인덱스 사용 여부를 결정한다.

100만 건의 `orders` 테이블에서:

- `customer_id`의 cardinality가 50,000이면, 하나의 `customer_id` 값에 평균 20건이 매칭된다. 선택도가 높다(좋다).
- `status` 컬럼의 cardinality가 5이면('pending', 'confirmed', 'shipped', 'delivered', 'cancelled'), 하나의 값에 평균 20만 건이 매칭된다. 선택도가 낮다(나쁘다).

선택도가 낮은 컬럼의 인덱스는 효과가 미미하다. `status = 'delivered'`로 검색하면 20만 건이 매칭되고, 20만 번의 PK lookup이 발생한다. 이 정도면 풀 테이블 스캔이 더 빠르다. 옵티마이저도 이를 알고, 선택도가 낮은 인덱스는 무시하고 풀 스캔을 선택하는 경우가 많다.

```sql
-- cardinality 확인
SHOW INDEX FROM orders;
```

```
+---------+----------+-----------+-------------+----------+
| Table   | Key_name | Seq_in_idx| Column_name | Cardinality|
+---------+----------+-----------+-------------+----------+
| orders  | PRIMARY  |         1 | id          | 1000000  |
| orders  | idx_cust |         1 | customer_id |   50000  |
| orders  | idx_stat |         1 | status      |       5  |
+---------+----------+-----------+-------------+----------+
```

주의할 점: MySQL이 표시하는 cardinality는 정확한 값이 아니라 추정치다. InnoDB는 무작위로 일부 page를 샘플링하여 cardinality를 추정한다. `ANALYZE TABLE`을 실행하면 통계를 갱신한다:

```sql
ANALYZE TABLE orders;
```

통계가 실제 데이터 분포와 크게 다르면, 옵티마이저가 잘못된 실행 계획을 선택할 수 있다. 대량의 데이터 변경 후에는 `ANALYZE TABLE`을 실행하는 것이 좋다.

## 인덱스를 탈 수 없는 조건들

인덱스가 존재하더라도, 쿼리 조건에 따라 인덱스를 사용하지 못하는 경우가 있다. 이 상황들을 알아두어야 불필요한 풀 스캔을 피할 수 있다.

### 컬럼에 함수를 적용한 경우

```sql
-- 인덱스 사용 불가
SELECT * FROM orders WHERE YEAR(order_date) = 2024;

-- 인덱스 사용 가능 (범위 조건으로 변환)
SELECT * FROM orders WHERE order_date >= '2024-01-01' AND order_date < '2025-01-01';
```

`YEAR(order_date)`는 `order_date`의 원본 값이 아니라 함수 적용 결과로 비교한다. B+tree는 `order_date`의 원본 값으로 정렬되어 있으므로, 함수가 적용된 값으로는 트리를 탐색할 수 없다.

MySQL 8.0부터는 functional index로 이 문제를 해결할 수 있다:

```sql
CREATE INDEX idx_order_year ON orders ((YEAR(order_date)));
```

하지만 범위 조건으로 변환하는 것이 더 범용적이고, 기존 인덱스를 그대로 활용할 수 있다.

### 암묵적 타입 변환

```sql
-- phone_number가 VARCHAR 컬럼인 경우
-- 인덱스 사용 불가 (숫자와 비교하면 암묵적 변환 발생)
SELECT * FROM users WHERE phone_number = 01012345678;

-- 인덱스 사용 가능
SELECT * FROM users WHERE phone_number = '01012345678';
```

`phone_number`는 VARCHAR인데 비교 값이 숫자(정수 리터럴)이면, MySQL은 `phone_number` 컬럼의 모든 값을 숫자로 변환하여 비교한다. 컬럼에 함수를 적용한 것과 같은 효과다. 인덱스를 탈 수 없다.

반대로, 정수 컬럼에 문자열을 비교하면 문자열이 정수로 변환된다. 이 경우 컬럼 자체에 변환이 일어나지 않으므로 인덱스를 사용할 수 있다. 하지만 혼동을 피하려면 항상 타입을 맞추는 것이 안전하다.

### LIKE 패턴의 위치

```sql
-- 인덱스 사용 가능 (prefix 검색)
SELECT * FROM users WHERE name LIKE '김%';

-- 인덱스 사용 불가 (앞에 와일드카드)
SELECT * FROM users WHERE name LIKE '%영수';

-- 인덱스 사용 불가
SELECT * FROM users WHERE name LIKE '%영%';
```

B+tree는 값의 앞부분부터 정렬되어 있다. `'김%'`은 '김'으로 시작하는 범위를 탐색할 수 있다. `'%영수'`는 앞부분을 모르므로 트리 탐색이 불가능하다.

### OR 조건

```sql
-- 각 컬럼에 개별 인덱스가 있어도 비효율적일 수 있다
SELECT * FROM orders WHERE customer_id = 42 OR status = 'pending';
```

OR 조건은 두 조건 중 하나만 만족해도 결과에 포함된다. MySQL은 index merge 최적화를 시도할 수 있지만, 항상 효율적이지는 않다. 가능하다면 UNION으로 분리하는 것이 더 나은 실행 계획을 만들 수 있다:

```sql
SELECT * FROM orders WHERE customer_id = 42
UNION
SELECT * FROM orders WHERE status = 'pending';
```

### NOT, != 조건

```sql
-- 인덱스 활용이 제한적
SELECT * FROM orders WHERE status != 'cancelled';
```

`!=`(또는 `<>`)은 해당 값을 제외한 나머지 전체를 읽어야 한다. 테이블의 대부분을 읽게 되므로 옵티마이저가 풀 스캔을 선택하는 경우가 많다. 단, 제외되는 값의 비율이 매우 크면(거의 모든 행이 'cancelled'이고, 그렇지 않은 행이 소수라면) 인덱스가 사용될 수 있다.

## 인덱스 힌트

MySQL 옵티마이저가 최적의 인덱스를 선택하지 못하는 경우가 드물게 있다. 통계가 부정확하거나, 데이터 분포가 특이한 경우다. 이때 인덱스 힌트로 옵티마이저의 선택을 유도할 수 있다.

### USE INDEX

특정 인덱스를 사용하도록 제안한다. 옵티마이저가 이를 고려하되, 반드시 따르지는 않는다:

```sql
SELECT * FROM orders USE INDEX (idx_customer)
WHERE customer_id = 42;
```

### FORCE INDEX

특정 인덱스를 강제한다. 풀 테이블 스캔보다 비용이 높다고 판단해도 해당 인덱스를 사용한다:

```sql
SELECT * FROM orders FORCE INDEX (idx_customer)
WHERE customer_id = 42;
```

### IGNORE INDEX

특정 인덱스를 무시하도록 한다. 옵티마이저가 잘못된 인덱스를 선택하는 경우에 유용하다:

```sql
SELECT * FROM orders IGNORE INDEX (idx_status)
WHERE customer_id = 42 AND status = 'pending';
```

인덱스 힌트는 최후의 수단이다. 힌트를 사용하기 전에 먼저 확인해야 할 것들:

1. `ANALYZE TABLE`로 통계를 갱신했는가
2. 쿼리 자체를 더 효율적으로 바꿀 수 있는가
3. 인덱스 설계를 변경할 수 있는가

힌트는 코드에 하드코딩되므로, 데이터 분포가 바뀌면 오히려 성능을 악화시킬 수 있다. 사용했다면 주기적으로 재검토해야 한다.

## 인덱스 설계 전략

인덱스 설계에 정답은 없다. 워크로드(어떤 쿼리가 얼마나 자주 실행되는가)에 따라 달라진다. 하지만 실무에서 반복적으로 적용되는 원칙이 있다.

### 1. 쿼리에서 출발한다

테이블 구조를 보고 인덱스를 만드는 것이 아니라, 실제 실행되는 쿼리를 분석하고 인덱스를 만든다. 슬로우 쿼리 로그에서 자주 등장하는 쿼리, 실행 시간이 긴 쿼리를 우선 대상으로 삼는다.

### 2. WHERE, JOIN, ORDER BY를 본다

인덱스가 도움이 되는 위치는 세 군데다:

- **WHERE**: 검색 조건에 사용되는 컬럼
- **JOIN**: 조인 조건에 사용되는 컬럼 (특히 FK)
- **ORDER BY**: 정렬에 사용되는 컬럼 (인덱스가 이미 정렬되어 있으면 filesort를 피할 수 있다)

```sql
SELECT o.id, o.order_date, c.name
FROM orders o
JOIN customers c ON c.id = o.customer_id
WHERE o.status = 'pending'
ORDER BY o.order_date DESC;
```

이 쿼리에서 인덱스 후보:
- `orders.customer_id` (JOIN 조건)
- `orders.status` (WHERE 조건)
- `orders.order_date` (ORDER BY)
- 또는 복합 인덱스 `orders(status, order_date)`

### 3. 복합 인덱스로 여러 쿼리를 커버한다

개별 인덱스를 여러 개 만드는 것보다, 잘 설계된 복합 인덱스 하나가 여러 쿼리를 커버하는 것이 효율적이다. Leftmost prefix rule에 의해 `(A, B, C)` 인덱스는 `A`, `A+B`, `A+B+C` 조건을 모두 지원한다.

### 4. 선택도가 높은 컬럼을 우선한다

같은 복합 인덱스 안에서도, 선택도가 높은 컬럼을 앞에 놓으면 탐색 범위가 빨리 줄어든다. 단, 등호 조건과 범위 조건의 우선순위도 함께 고려해야 한다.

### 5. PK는 작게 유지한다

InnoDB에서 모든 secondary index의 리프 노드에 PK가 포함된다. PK가 크면 secondary index의 크기가 커지고, buffer pool에서 차지하는 메모리도 늘어난다. `BIGINT`(8바이트)과 `UUID`(36바이트 문자열, 또는 16바이트 BINARY)의 차이가 인덱스 전체에 누적되면 상당하다.

### 6. 사용하지 않는 인덱스를 제거한다

인덱스는 존재 자체가 비용이다. 쓰기마다 유지 비용이 발생하고, 디스크 공간과 buffer pool 메모리를 차지한다. 주기적으로 `sys.schema_unused_indexes`를 확인하고, 불필요한 인덱스를 제거한다. 중복 인덱스(다른 인덱스의 leftmost prefix에 해당하는 인덱스)도 제거 대상이다:

```sql
-- idx_a(customer_id)와 idx_ab(customer_id, order_date)가 모두 있으면
-- idx_a는 중복이다. idx_ab가 customer_id 단독 검색도 커버한다.
```

인덱스 설계는 한 번 하고 끝나는 작업이 아니다. 서비스가 성장하면서 쿼리 패턴이 변하고, 데이터 분포가 달라진다. 정기적인 점검과 조정이 필요하다.
