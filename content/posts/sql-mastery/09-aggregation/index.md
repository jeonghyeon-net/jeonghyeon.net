# 집계와 그룹핑

GROUP BY는 행을 그룹으로 묶고, 집계 함수는 그룹 단위로 값을 계산한다. 단순해 보이지만 내부적으로 임시 테이블과 filesort가 관여하며, 인덱스 활용 여부에 따라 성능 차이가 크다.

## GROUP BY의 동작 원리

```sql
SELECT dept_id, COUNT(*) AS cnt
FROM employees
GROUP BY dept_id;
```

```
+---------+-----+
| dept_id | cnt |
+---------+-----+
|       1 |   5 |
|       2 |   3 |
|       3 |   8 |
+---------+-----+
```

MySQL은 GROUP BY를 처리할 때 같은 값을 가진 행을 모아야 한다. 모으는 방법은 두 가지다:

1. **인덱스를 이용한 그룹핑**: `dept_id`에 인덱스가 있으면 이미 같은 값끼리 연속해서 저장되어 있다. 순서대로 읽으면서 값이 바뀌는 시점에 그룹 경계를 나누면 된다. 정렬이나 임시 테이블 없이 처리 가능하다.

2. **임시 테이블을 이용한 그룹핑**: 인덱스가 없으면 테이블을 스캔하면서 내부 임시 테이블에 그룹별 결과를 누적한다. 각 행을 읽을 때마다 해당 그룹이 임시 테이블에 있는지 확인하고, 있으면 집계 값을 갱신하고, 없으면 새 그룹을 추가한다.

EXPLAIN에서 `Extra`에 `Using temporary`가 나타나면 임시 테이블이 사용된 것이다.

## 집계 함수

### COUNT

```sql
-- 전체 행 수
SELECT COUNT(*) FROM orders;

-- NULL이 아닌 값의 수
SELECT COUNT(shipped_at) FROM orders;

-- 고유한 값의 수
SELECT COUNT(DISTINCT status) FROM orders;
```

`COUNT(*)`와 `COUNT(컬럼)`은 다르다. `COUNT(*)`는 NULL을 포함한 모든 행을 센다. `COUNT(컬럼)`은 해당 컬럼이 NULL이 아닌 행만 센다. 의도와 맞는 것을 써야 한다.

`COUNT(*)`의 성능에 대해 흔한 오해가 있다. "COUNT(*)는 모든 컬럼을 읽으니까 COUNT(1)이 빠르다"는 것인데, 사실이 아니다. MySQL은 `COUNT(*)`를 `COUNT(1)`과 동일하게 최적화한다. 둘의 실행 계획은 같다.

InnoDB에서 `COUNT(*)`는 조건 없이 실행해도 전체 테이블(또는 가장 작은 secondary index)을 스캔해야 한다. MVCC 구조 때문에 트랜잭션마다 보이는 행이 다를 수 있어서, 정확한 행 수를 미리 저장해두지 못한다.

### SUM, AVG

```sql
SELECT dept_id,
       SUM(salary) AS total_salary,
       AVG(salary) AS avg_salary
FROM employees
GROUP BY dept_id;
```

NULL 값은 무시된다. 모든 값이 NULL이면 SUM은 NULL을, AVG도 NULL을 반환한다(0이 아니다).

### MIN, MAX

```sql
SELECT MIN(price), MAX(price) FROM products;
```

GROUP BY 없이 쓰면 전체 테이블에서 최솟값과 최댓값을 구한다. `price`에 인덱스가 있으면 B-tree의 가장 왼쪽(MIN)과 가장 오른쪽(MAX) 리프 노드만 읽으면 된다. EXPLAIN에서 `Select tables optimized away`라고 표시되며, 테이블을 스캔하지 않고 인덱스 메타데이터만으로 결과를 반환한다.

## HAVING vs WHERE

```sql
-- WHERE: 그룹핑 전에 행을 필터링한다
SELECT dept_id, COUNT(*) AS cnt
FROM employees
WHERE status = 'active'
GROUP BY dept_id;

-- HAVING: 그룹핑 후에 그룹을 필터링한다
SELECT dept_id, COUNT(*) AS cnt
FROM employees
GROUP BY dept_id
HAVING cnt >= 5;
```

실행 순서를 다시 떠올리면:

```
FROM → WHERE → GROUP BY → HAVING → SELECT → ORDER BY → LIMIT
```

WHERE는 GROUP BY 이전에 실행된다. 개별 행 단위로 필터링하여 그룹핑할 대상을 줄인다. HAVING은 GROUP BY 이후에 실행된다. 집계 결과를 기준으로 그룹을 필터링한다.

성능 차이가 크다:

```sql
-- 비효율: 전체를 그룹핑한 뒤 필터링
SELECT dept_id, COUNT(*) AS cnt
FROM employees
GROUP BY dept_id
HAVING dept_id IN (1, 2, 3);

-- 효율: 먼저 필터링하여 그룹핑 대상을 줄임
SELECT dept_id, COUNT(*) AS cnt
FROM employees
WHERE dept_id IN (1, 2, 3)
GROUP BY dept_id;
```

HAVING에 쓸 수 있는 조건이 집계 함수와 무관하다면, WHERE로 옮기는 것이 낫다. 그룹핑할 행 수가 줄어들어 임시 테이블도 작아지고 처리 속도도 빨라진다.

HAVING은 집계 결과에 대한 조건에만 사용한다:

```sql
-- HAVING의 올바른 사용: 집계 결과를 기준으로 필터링
SELECT dept_id, AVG(salary) AS avg_salary
FROM employees
GROUP BY dept_id
HAVING avg_salary > 5000000;
```

## 임시 테이블이 생기는 조건

GROUP BY가 임시 테이블을 사용하는 주요 조건:

- GROUP BY 컬럼에 인덱스가 없을 때
- GROUP BY와 ORDER BY의 컬럼이 다를 때
- GROUP BY에 표현식이 포함될 때 (예: `GROUP BY YEAR(created_at)`)
- DISTINCT와 GROUP BY가 함께 쓰일 때

임시 테이블의 크기가 `tmp_table_size` 또는 `max_heap_table_size` 중 작은 값을 초과하면 메모리 임시 테이블이 디스크 기반 임시 테이블로 변환된다. 디스크 I/O가 발생하므로 성능이 급격히 떨어진다.

```sql
-- 임시 테이블 + filesort 발생
SELECT dept_id, COUNT(*) AS cnt
FROM employees
GROUP BY dept_id
ORDER BY cnt DESC;
```

GROUP BY로 그룹핑(임시 테이블)한 뒤, `cnt`로 정렬(filesort)해야 한다. EXPLAIN의 `Extra`에 `Using temporary; Using filesort`가 나타난다.

## 인덱스로 GROUP BY 처리하기

GROUP BY 컬럼에 적절한 인덱스가 있으면 임시 테이블 없이 처리할 수 있다. MySQL은 두 가지 방식으로 인덱스를 활용한다.

### Tight index scan

인덱스를 순서대로 전부 읽으면서 그룹 경계를 나누는 방식이다:

```sql
-- 인덱스: (dept_id)
SELECT dept_id, COUNT(*)
FROM employees
GROUP BY dept_id;
```

인덱스에서 `dept_id`가 정렬되어 있으므로 순서대로 읽으면서 값이 바뀔 때마다 새 그룹으로 넘어간다. 인덱스의 모든 항목을 읽지만, 정렬이나 임시 테이블이 필요 없다. EXPLAIN에서 `Extra`에 `Using index`가 나타난다.

### Loose index scan

인덱스에서 각 그룹의 첫 번째(또는 마지막) 항목만 읽는 방식이다. 그룹 수가 적고 각 그룹의 행 수가 많을 때 효과적이다:

```sql
-- 인덱스: (dept_id, salary)
SELECT dept_id, MIN(salary)
FROM employees
GROUP BY dept_id;
```

`(dept_id, salary)` 인덱스에서 각 `dept_id` 그룹의 첫 번째 항목이 해당 그룹의 최솟값이다. 그룹마다 한 건만 읽으면 된다. EXPLAIN에서 `Extra`에 `Using index for group-by`가 나타난다.

loose index scan이 적용되려면 조건이 까다롭다:

- GROUP BY 컬럼이 인덱스의 왼쪽 prefix여야 한다
- 집계 함수가 MIN, MAX만 가능하다 (MySQL 8.0 기준)
- WHERE 조건이 있다면 인덱스 컬럼에 대한 상수 비교만 가능하다

### 복합 인덱스 활용

WHERE 조건과 GROUP BY를 함께 처리하려면 복합 인덱스 설계가 중요하다:

```sql
-- 쿼리
SELECT status, COUNT(*)
FROM orders
WHERE created_at >= '2025-01-01'
GROUP BY status;
```

- 인덱스 `(created_at)`: WHERE 필터링은 되지만 GROUP BY에 임시 테이블 필요
- 인덱스 `(status)`: GROUP BY는 되지만 WHERE 필터링에서 풀 스캔
- 인덱스 `(status, created_at)`: GROUP BY와 WHERE 범위 조건 모두 처리 가능

05편에서 다룬 복합 인덱스의 컬럼 순서 원칙이 그대로 적용된다. 동등 조건 컬럼을 앞에, 범위 조건 컬럼을 뒤에 놓는다. GROUP BY 컬럼은 동등 조건과 유사하게 취급된다.

## DISTINCT와 GROUP BY

```sql
-- 이 두 쿼리는 같은 결과를 반환한다
SELECT DISTINCT dept_id FROM employees;
SELECT dept_id FROM employees GROUP BY dept_id;
```

내부적으로도 거의 동일하게 처리된다. MySQL은 DISTINCT를 GROUP BY의 특수한 형태로 취급한다. 인덱스 활용 조건도 같다.

차이가 나는 경우:

```sql
-- DISTINCT: 모든 컬럼 조합의 고유한 행
SELECT DISTINCT dept_id, status FROM employees;

-- GROUP BY: 집계 함수와 함께 사용 가능
SELECT dept_id, status, COUNT(*) FROM employees GROUP BY dept_id, status;
```

DISTINCT는 집계 없이 중복을 제거할 때, GROUP BY는 집계가 필요할 때 쓴다. 집계 없이 `GROUP BY`를 중복 제거 목적으로 쓰는 것은 의도가 불명확하므로 `DISTINCT`를 쓰는 것이 낫다.

### COUNT(DISTINCT)의 성능

```sql
SELECT COUNT(DISTINCT customer_id) FROM orders;
```

전체 고유 값의 수를 세려면 모든 값을 확인해야 한다. 내부적으로 임시 테이블에 고유 값을 저장하면서 중복을 제거한다. 고유 값이 많으면 임시 테이블이 커지고 디스크로 넘어갈 수 있다.

```sql
-- 여러 컬럼의 COUNT(DISTINCT)는 더 비싸다
SELECT COUNT(DISTINCT customer_id), COUNT(DISTINCT product_id) FROM orders;
```

각 DISTINCT마다 별도의 임시 테이블이 필요하다.

## 실전 예제

월별 매출 집계:

```sql
SELECT
    DATE_FORMAT(o.created_at, '%Y-%m') AS month,
    COUNT(DISTINCT o.id) AS order_count,
    SUM(oi.quantity * p.price) AS revenue
FROM orders o
INNER JOIN order_items oi ON o.id = oi.order_id
INNER JOIN products p ON oi.product_id = p.id
WHERE o.created_at >= '2025-01-01' AND o.created_at < '2026-01-01'
GROUP BY DATE_FORMAT(o.created_at, '%Y-%m')
ORDER BY month;
```

이 쿼리의 성능 포인트:

- `GROUP BY DATE_FORMAT(...)`은 함수 적용이므로 인덱스를 활용할 수 없다. 임시 테이블이 생긴다.
- `COUNT(DISTINCT o.id)`는 JOIN으로 행이 늘어난 상태에서 주문 수를 정확히 세기 위한 것이다. `order_items`가 여러 건이면 같은 `o.id`가 반복되기 때문이다.
- `ORDER BY month`는 GROUP BY 결과를 정렬한다. GROUP BY가 이미 임시 테이블을 만들었으므로 추가 filesort가 발생한다.

월 단위 집계처럼 그룹 수가 적으면(12건) 임시 테이블과 filesort의 부담은 크지 않다. 문제가 되는 것은 그룹 수가 수만~수십만인 경우다.

카테고리별 상위 매출 상품:

```sql
SELECT
    p.category_id,
    p.name,
    SUM(oi.quantity) AS total_sold
FROM order_items oi
INNER JOIN products p ON oi.product_id = p.id
GROUP BY p.category_id, p.name
HAVING total_sold >= 100
ORDER BY p.category_id, total_sold DESC;
```

- `GROUP BY p.category_id, p.name`으로 카테고리-상품 조합별 판매량을 구한다
- `HAVING total_sold >= 100`으로 100개 이상 팔린 상품만 필터링한다
- 이 HAVING 조건은 집계 결과에 대한 것이므로 WHERE로 옮길 수 없다

GROUP BY는 강력하지만, 대량 데이터에서 임시 테이블과 filesort를 유발하기 쉽다. EXPLAIN으로 `Using temporary`와 `Using filesort`를 확인하고, 가능하면 인덱스를 활용하도록 쿼리와 인덱스를 설계한다.