# 서브쿼리와 CTE

SELECT 안에 또 다른 SELECT를 넣을 수 있다. 서브쿼리(subquery)다. 서브쿼리의 종류에 따라 MySQL이 내부적으로 처리하는 방식이 달라지고, 이 차이가 성능을 결정한다. CTE(Common Table Expression)는 서브쿼리를 이름 붙여 재사용하는 문법이다.

## 서브쿼리의 세 가지 유형

서브쿼리는 반환하는 결과와 위치에 따라 세 가지로 나뉜다.

### 스칼라 서브쿼리

단일 값(1행 1열)을 반환하는 서브쿼리다. SELECT 절이나 WHERE 절에서 하나의 값이 필요한 자리에 쓴다.

```sql
SELECT
    name,
    salary,
    (SELECT AVG(salary) FROM employees) AS avg_salary
FROM employees;
```

```
+-------+--------+------------+
| name  | salary | avg_salary |
+-------+--------+------------+
| Alice |  50000 |   45000.00 |
| Bob   |  40000 |   45000.00 |
| Carol |  45000 |   45000.00 |
+-------+--------+------------+
```

스칼라 서브쿼리가 2행 이상을 반환하면 에러가 발생한다. MySQL은 스칼라 서브쿼리의 결과를 캐싱한다. 외부 쿼리의 값에 의존하지 않는 경우(비상관 서브쿼리), 한 번만 실행하고 결과를 재사용한다.

### 인라인 뷰

FROM 절에 쓰는 서브쿼리다. 결과가 임시 테이블처럼 동작한다.

```sql
SELECT dept_name, avg_sal
FROM (
    SELECT department_id, AVG(salary) AS avg_sal
    FROM employees
    GROUP BY department_id
) AS dept_avg
JOIN departments d ON d.id = dept_avg.department_id
WHERE avg_sal > 50000;
```

MySQL은 인라인 뷰를 두 가지 방식으로 처리한다.

- **Derived table merge**: 서브쿼리를 외부 쿼리에 병합한다. 임시 테이블을 만들지 않는다.
- **Materialization**: 서브쿼리 결과를 임시 테이블로 만들어 저장한다.

MySQL 8.0 optimizer는 가능하면 merge를 선택한다. GROUP BY, DISTINCT, LIMIT, UNION, 집계 함수가 포함된 서브쿼리는 merge가 불가능하므로 materialization으로 처리된다.

### 상관 서브쿼리

외부 쿼리의 행을 참조하는 서브쿼리다. 외부 쿼리의 각 행마다 서브쿼리가 실행된다.

```sql
SELECT e.name, e.salary, e.department_id
FROM employees e
WHERE e.salary > (
    SELECT AVG(e2.salary)
    FROM employees e2
    WHERE e2.department_id = e.department_id
);
```

외부 쿼리의 `e.department_id`를 서브쿼리가 참조한다. employees 테이블에 1,000행이 있으면, 서브쿼리가 최대 1,000번 실행될 수 있다. 상관 서브쿼리는 이 반복 실행 때문에 느려지기 쉽다.

## EXISTS vs IN

WHERE 절에서 서브쿼리를 사용하는 대표적인 두 가지 방식이다.

### IN

```sql
SELECT name
FROM employees
WHERE department_id IN (
    SELECT id FROM departments WHERE location = 'Seoul'
);
```

IN은 서브쿼리의 결과 집합을 먼저 구한 뒤, 외부 쿼리의 각 행이 그 집합에 포함되는지 확인한다. 서브쿼리 결과가 작으면 효율적이다.

### EXISTS

```sql
SELECT name
FROM employees e
WHERE EXISTS (
    SELECT 1 FROM departments d
    WHERE d.id = e.department_id AND d.location = 'Seoul'
);
```

EXISTS는 서브쿼리가 최소 1행이라도 반환하면 TRUE다. 서브쿼리 안에서 일치하는 행을 하나라도 찾으면 즉시 멈춘다. 전체 결과를 구할 필요가 없다.

### 성능 차이

| 상황 | 유리한 쪽 | 이유 |
|---|---|---|
| 서브쿼리 결과가 적다 | IN | 작은 집합을 한 번 구해서 재사용 |
| 서브쿼리 결과가 많다 | EXISTS | 일치하는 행을 찾으면 즉시 중단 |
| 외부 테이블이 작다 | EXISTS | 반복 횟수 자체가 적다 |
| 서브쿼리에 인덱스가 있다 | EXISTS | 인덱스 lookup으로 빠르게 확인 |

실제로는 MySQL optimizer가 IN 서브쿼리를 내부적으로 EXISTS로 변환하거나, 세미조인으로 최적화하기 때문에 단순 비교가 어렵다. EXPLAIN으로 확인하는 것이 정확하다.

## MySQL의 서브쿼리 최적화

MySQL 5.5 이전에는 IN 서브쿼리를 상관 서브쿼리(EXISTS)로 무조건 변환했다. 외부 테이블이 크면 성능이 심각하게 나빴다. MySQL 5.6부터 optimizer가 크게 개선되었다.

### 세미조인 최적화

IN 또는 EXISTS 서브쿼리가 세미조인(semi-join) 조건을 만족하면, MySQL은 이를 조인으로 변환한다.

```sql
-- 원본
SELECT * FROM employees
WHERE department_id IN (SELECT id FROM departments WHERE location = 'Seoul');

-- optimizer가 내부적으로 변환
SELECT employees.*
FROM employees SEMI JOIN departments
ON employees.department_id = departments.id
WHERE departments.location = 'Seoul';
```

세미조인은 일반 조인과 달리 외부 테이블의 행을 중복 없이 한 번만 반환한다. MySQL은 세미조인을 다섯 가지 전략으로 실행한다.

- **Table pullout**: 서브쿼리 테이블을 외부 쿼리로 끌어올려 일반 조인으로 처리한다.
- **FirstMatch**: EXISTS처럼 첫 번째 일치를 찾으면 멈춘다.
- **LooseScan**: 인덱스를 느슨하게 스캔하여 중복을 건너뛴다.
- **Materialization**: 서브쿼리 결과를 임시 테이블로 만들고 조인한다.
- **DuplicateWeedout**: 조인 후 중복을 제거한다.

EXPLAIN에서 `SEMIJOIN`, `FirstMatch`, `LooseScan` 등이 보이면 세미조인 최적화가 적용된 것이다.

### Materialization

서브쿼리 결과를 임시 테이블에 저장하고, 이후 조회에 재사용하는 방식이다.

```sql
SELECT * FROM employees
WHERE department_id IN (SELECT department_id FROM projects WHERE budget > 1000000);
```

서브쿼리 결과가 `[1, 3, 7]`이면, MySQL은 이 세 값을 임시 테이블에 저장한다. 임시 테이블에는 자동으로 유니크 인덱스가 생성되어 lookup이 빠르다. 서브쿼리가 한 번만 실행되므로, 결과 집합이 크지 않으면 효율적이다.

## 서브쿼리가 느려지는 패턴

### SELECT 절의 상관 서브쿼리

```sql
-- 느리다: 행마다 서브쿼리 실행
SELECT
    e.name,
    (SELECT d.name FROM departments d WHERE d.id = e.department_id) AS dept_name
FROM employees e;
```

이 패턴은 employees의 행 수만큼 서브쿼리가 반복된다. JOIN으로 바꾸면 한 번에 처리된다.

```sql
-- 빠르다: JOIN
SELECT e.name, d.name AS dept_name
FROM employees e
JOIN departments d ON d.id = e.department_id;
```

### WHERE 절의 비효율적 상관 서브쿼리

```sql
-- 느리다: 인덱스가 없으면 매번 전체 스캔
SELECT * FROM orders o
WHERE (SELECT SUM(amount) FROM order_items oi WHERE oi.order_id = o.id) > 10000;
```

`order_items.order_id`에 인덱스가 없으면 외부 행마다 order_items 전체를 스캔한다. orders가 10,000행이고 order_items가 100,000행이면, 최악의 경우 10억 행을 읽는다.

대안:

```sql
SELECT o.*
FROM orders o
JOIN (
    SELECT order_id, SUM(amount) AS total
    FROM order_items
    GROUP BY order_id
    HAVING total > 10000
) AS oi ON oi.order_id = o.id;
```

서브쿼리를 인라인 뷰로 옮기면 order_items를 한 번만 스캔한다.

### NOT IN과 NULL

```sql
SELECT * FROM employees
WHERE department_id NOT IN (SELECT id FROM departments);
```

서브쿼리 결과에 NULL이 하나라도 포함되면, NOT IN은 항상 빈 결과를 반환한다. SQL의 3값 논리(TRUE, FALSE, UNKNOWN) 때문이다. `1 NOT IN (2, NULL)`은 `1 <> 2 AND 1 <> NULL`이고, `1 <> NULL`은 UNKNOWN이므로 전체가 UNKNOWN이 된다.

NOT EXISTS는 이 문제가 없다.

```sql
SELECT * FROM employees e
WHERE NOT EXISTS (
    SELECT 1 FROM departments d WHERE d.id = e.department_id
);
```

NOT IN 대신 NOT EXISTS를 사용하는 것이 안전하다.

## CTE (WITH 절)

CTE는 쿼리 내에서 이름 붙인 임시 결과 집합이다. MySQL 8.0부터 지원한다.

```sql
WITH dept_avg AS (
    SELECT department_id, AVG(salary) AS avg_salary
    FROM employees
    GROUP BY department_id
)
SELECT e.name, e.salary, da.avg_salary
FROM employees e
JOIN dept_avg da ON da.department_id = e.department_id
WHERE e.salary > da.avg_salary;
```

인라인 뷰와 결과는 같다. 차이점은 가독성이다. WITH 절이 쿼리 상단에 위치하므로 "먼저 부서별 평균을 구하고, 그 결과를 사용한다"는 흐름이 명확하다.

### 여러 CTE 정의

CTE는 쉼표로 구분하여 여러 개를 정의할 수 있다. 앞에서 정의한 CTE를 뒤의 CTE에서 참조할 수도 있다.

```sql
WITH
active_employees AS (
    SELECT * FROM employees WHERE status = 'active'
),
dept_stats AS (
    SELECT department_id, COUNT(*) AS cnt, AVG(salary) AS avg_sal
    FROM active_employees
    GROUP BY department_id
)
SELECT d.name, ds.cnt, ds.avg_sal
FROM dept_stats ds
JOIN departments d ON d.id = ds.department_id;
```

`dept_stats`가 `active_employees`를 참조한다. 서브쿼리를 중첩하면 읽기 어려운 쿼리도, CTE로 분리하면 단계별로 읽을 수 있다.

### CTE 최적화: materialized vs merged

MySQL 8.0 optimizer는 CTE를 두 가지 방식으로 처리한다.

- **Merged**: CTE 정의를 외부 쿼리에 인라인으로 삽입한다. 임시 테이블을 만들지 않는다.
- **Materialized**: CTE 결과를 임시 테이블에 저장한다.

CTE가 한 번만 참조되면 merge를 시도한다. 여러 번 참조되면 materialization이 유리하므로 자동으로 materialized된다.

```sql
-- 한 번 참조: merge 가능
WITH cte AS (SELECT * FROM employees WHERE salary > 50000)
SELECT * FROM cte WHERE department_id = 1;

-- 두 번 참조: materialization
WITH cte AS (SELECT * FROM employees WHERE salary > 50000)
SELECT * FROM cte c1
JOIN cte c2 ON c1.department_id = c2.department_id AND c1.id <> c2.id;
```

EXPLAIN에서 `<subquery>` 대신 `<derived>` 또는 CTE 이름이 보이면 materialization이 적용된 것이다.

## 재귀 CTE

재귀 CTE는 자기 자신을 참조하는 CTE다. 계층 구조 데이터(조직도, 카테고리 트리, 댓글 스레드)를 처리할 때 사용한다.

```sql
CREATE TABLE categories (
    id INT PRIMARY KEY,
    name VARCHAR(100),
    parent_id INT
);

INSERT INTO categories VALUES
(1, '전자제품', NULL),
(2, '컴퓨터', 1),
(3, '노트북', 2),
(4, '데스크탑', 2),
(5, '스마트폰', 1),
(6, '의류', NULL);
```

특정 카테고리의 모든 하위 카테고리를 찾으려면:

```sql
WITH RECURSIVE category_tree AS (
    -- 앵커 멤버: 시작점
    SELECT id, name, parent_id, 0 AS depth
    FROM categories
    WHERE id = 1

    UNION ALL

    -- 재귀 멤버: 이전 결과를 참조
    SELECT c.id, c.name, c.parent_id, ct.depth + 1
    FROM categories c
    JOIN category_tree ct ON c.parent_id = ct.id
)
SELECT * FROM category_tree;
```

```
+----+-----------+-----------+-------+
| id | name      | parent_id | depth |
+----+-----------+-----------+-------+
|  1 | 전자제품  |      NULL |     0 |
|  2 | 컴퓨터    |         1 |     1 |
|  5 | 스마트폰  |         1 |     1 |
|  3 | 노트북    |         2 |     2 |
|  4 | 데스크탑  |         2 |     2 |
+----+-----------+-----------+-------+
```

재귀 CTE의 실행 과정:

1. 앵커 멤버를 실행한다. `id = 1`인 행(전자제품)이 결과에 추가된다.
2. 재귀 멤버를 실행한다. `parent_id = 1`인 행(컴퓨터, 스마트폰)이 결과에 추가된다.
3. 다시 재귀 멤버를 실행한다. `parent_id`가 2 또는 5인 행(노트북, 데스크탑)이 추가된다.
4. 더 이상 새로운 행이 없으면 종료한다.

### 경로 문자열 만들기

재귀 CTE에서 경로를 누적할 수 있다.

```sql
WITH RECURSIVE category_path AS (
    SELECT id, name, parent_id, CAST(name AS CHAR(500)) AS path
    FROM categories
    WHERE parent_id IS NULL

    UNION ALL

    SELECT c.id, c.name, c.parent_id, CONCAT(cp.path, ' > ', c.name)
    FROM categories c
    JOIN category_path cp ON c.parent_id = cp.id
)
SELECT name, path FROM category_path;
```

```
+-----------+---------------------------+
| name      | path                      |
+-----------+---------------------------+
| 전자제품  | 전자제품                  |
| 의류      | 의류                      |
| 컴퓨터    | 전자제품 > 컴퓨터         |
| 스마트폰  | 전자제품 > 스마트폰       |
| 노트북    | 전자제품 > 컴퓨터 > 노트북|
| 데스크탑  | 전자제품 > 컴퓨터 > 데스크탑|
+-----------+---------------------------+
```

### 무한 루프 방지

재귀 CTE는 무한 루프에 빠질 수 있다. 데이터에 순환 참조가 있으면(A의 parent가 B, B의 parent가 A) 재귀가 끝나지 않는다. MySQL은 `cte_max_recursion_depth` 변수로 최대 재귀 깊이를 제한한다. 기본값은 1000이다.

```sql
SET cte_max_recursion_depth = 100;
```

필요에 따라 조정할 수 있지만, 깊이가 지나치게 깊다면 데이터 구조나 쿼리 설계를 재검토해야 한다.

## 실전 예제: 연속 날짜 생성

재귀 CTE로 날짜 시퀀스를 만들 수 있다. 날짜별 통계에서 데이터가 없는 날짜를 0으로 채울 때 유용하다.

```sql
WITH RECURSIVE dates AS (
    SELECT DATE('2024-01-01') AS dt

    UNION ALL

    SELECT dt + INTERVAL 1 DAY
    FROM dates
    WHERE dt < '2024-01-31'
)
SELECT d.dt, COALESCE(COUNT(o.id), 0) AS order_count
FROM dates d
LEFT JOIN orders o ON DATE(o.created_at) = d.dt
GROUP BY d.dt
ORDER BY d.dt;
```

```
+------------+-------------+
| dt         | order_count |
+------------+-------------+
| 2024-01-01 |           5 |
| 2024-01-02 |           0 |
| 2024-01-03 |          12 |
| ...        |         ... |
+------------+-------------+
```

서브쿼리는 중첩이 깊어질수록 읽기 어렵다. CTE는 이름을 붙여 단계별로 분리한다. MySQL 8.0에서는 CTE가 성능상 불이익 없이 사용 가능하므로, 복잡한 서브쿼리는 CTE로 바꾸는 것이 유지보수에 유리하다.
