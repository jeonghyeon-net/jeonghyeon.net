# JOIN의 동작 원리

두 개 이상의 테이블에서 관련된 행을 연결하는 것이 JOIN이다. SQL의 핵심 기능이지만, 내부적으로 어떤 알고리즘이 동작하는지 모르면 느린 쿼리의 원인을 찾기 어렵다.

## JOIN의 종류

예제에 사용할 테이블:

```sql
CREATE TABLE departments (
    id INT PRIMARY KEY,
    name VARCHAR(50)
);

CREATE TABLE employees (
    id INT PRIMARY KEY,
    name VARCHAR(50),
    dept_id INT
);

INSERT INTO departments VALUES (1, 'Engineering'), (2, 'Marketing'), (3, 'HR');
INSERT INTO employees VALUES
    (1, 'Alice', 1), (2, 'Bob', 1), (3, 'Carol', 2), (4, 'Dave', NULL);
```

`departments` 3건, `employees` 4건. Dave는 부서가 없다(`dept_id`가 NULL). HR 부서에는 소속 직원이 없다.

### INNER JOIN

양쪽 테이블 모두에서 매칭되는 행만 반환한다:

```sql
SELECT e.name, d.name AS dept
FROM employees e
INNER JOIN departments d ON e.dept_id = d.id;
```

```
+-------+-------------+
| name  | dept        |
+-------+-------------+
| Alice | Engineering |
| Bob   | Engineering |
| Carol | Marketing   |
+-------+-------------+
```

Dave는 `dept_id`가 NULL이므로 매칭되지 않아 결과에 없다. HR도 매칭되는 직원이 없으므로 나타나지 않는다.

### LEFT JOIN

왼쪽 테이블의 모든 행을 유지한다. 오른쪽 테이블에 매칭이 없으면 NULL로 채운다:

```sql
SELECT e.name, d.name AS dept
FROM employees e
LEFT JOIN departments d ON e.dept_id = d.id;
```

```
+-------+-------------+
| name  | dept        |
+-------+-------------+
| Alice | Engineering |
| Bob   | Engineering |
| Carol | Marketing   |
| Dave  | NULL        |
+-------+-------------+
```

Dave가 포함된다. `dept`는 NULL이다.

LEFT JOIN의 흔한 활용은 "매칭되지 않는 행 찾기"다:

```sql
-- 주문이 없는 고객 찾기
SELECT c.id, c.name
FROM customers c
LEFT JOIN orders o ON c.id = o.customer_id
WHERE o.id IS NULL;
```

### RIGHT JOIN

LEFT JOIN의 반대다. 오른쪽 테이블의 모든 행을 유지한다:

```sql
SELECT e.name, d.name AS dept
FROM employees e
RIGHT JOIN departments d ON e.dept_id = d.id;
```

```
+-------+-------------+
| name  | dept        |
+-------+-------------+
| Alice | Engineering |
| Bob   | Engineering |
| Carol | Marketing   |
| NULL  | HR          |
+-------+-------------+
```

HR 부서가 포함된다. 실무에서 RIGHT JOIN은 거의 쓰지 않는다. 테이블 순서를 바꾸고 LEFT JOIN을 쓰는 것이 읽기 쉽다.

### CROSS JOIN

양쪽 테이블의 모든 행 조합을 만든다. 조건 없이 곱집합(Cartesian product)을 생성한다:

```sql
SELECT e.name, d.name AS dept
FROM employees e
CROSS JOIN departments d;
```

employees 4건 x departments 3건 = 12건이 반환된다. 의도적으로 모든 조합이 필요한 경우가 아니면 쓸 일이 드물다. ON 조건 없이 JOIN을 쓰면 CROSS JOIN과 동일하게 동작하므로 조건 누락에 주의해야 한다.

## Nested Loop Join

MySQL은 JOIN을 처리할 때 기본적으로 Nested Loop Join(NLJ) 알고리즘을 사용한다. 이름 그대로 중첩 반복문이다.

```
for each row in 드라이빙_테이블:
    for each row in 드리븐_테이블 where 조건 매칭:
        결과에 추가
```

앞에서 본 INNER JOIN을 예로 들면:

```sql
SELECT e.name, d.name AS dept
FROM employees e
INNER JOIN departments d ON e.dept_id = d.id;
```

1. `employees`의 첫 번째 행(Alice, dept_id=1)을 읽는다
2. `departments`에서 `id = 1`인 행을 찾는다
3. 매칭되면 결과에 추가한다
4. `employees`의 다음 행으로 이동하여 반복한다

바깥 루프를 도는 테이블이 **드라이빙 테이블**(driving table), 안쪽 루프에서 검색되는 테이블이 **드리븐 테이블**(driven table)이다.

드리븐 테이블의 조인 컬럼에 인덱스가 있으면 각 반복에서 index lookup으로 빠르게 찾는다. 인덱스가 없으면 매 반복마다 드리븐 테이블을 풀 스캔해야 한다. 드라이빙 테이블이 1,000건이고 드리븐 테이블이 10,000건이면, 인덱스 없이는 1,000 x 10,000 = 10,000,000번의 비교가 발생한다.

## Block Nested Loop Join

인덱스가 없는 경우의 성능을 개선하기 위해 MySQL은 Block Nested Loop(BNL) 알고리즘을 사용한다.

NLJ는 드라이빙 테이블에서 행을 하나씩 읽고 드리븐 테이블을 반복 스캔한다. BNL은 드라이빙 테이블의 행을 **join buffer**에 모아둔 뒤, 드리븐 테이블을 한 번 스캔하면서 buffer에 있는 모든 행과 비교한다.

```
join buffer에 드라이빙_테이블의 행을 채운다
for each row in 드리븐_테이블:
    buffer의 모든 행과 비교하여 매칭되면 결과에 추가
buffer가 다 차면 비우고 드라이빙_테이블의 다음 행들로 반복
```

join buffer 크기는 `join_buffer_size` 시스템 변수로 설정한다(기본값 256KB). 드라이빙 테이블의 행이 buffer에 한 번에 들어가면 드리븐 테이블을 딱 한 번만 스캔하면 된다.

EXPLAIN에서 `Extra`에 `Using join buffer (Block Nested Loop)`가 나타나면 BNL이 사용된 것이다. 이 표시가 보이면 드리븐 테이블의 조인 컬럼에 인덱스를 추가하는 것을 검토해야 한다.

## Hash Join

MySQL 8.0.18부터 도입된 알고리즘이다. BNL을 대체한다.

동작 원리:

1. **Build 단계**: 작은 쪽 테이블의 조인 컬럼 값을 hash table로 만든다
2. **Probe 단계**: 큰 쪽 테이블을 한 행씩 읽으면서 hash table에서 매칭을 찾는다

```
-- Build
hash_table = {}
for each row in 작은_테이블:
    hash_table[hash(조인_컬럼)] = row

-- Probe
for each row in 큰_테이블:
    hash_table에서 hash(조인_컬럼)으로 매칭 검색
```

hash lookup은 O(1)이므로 NLJ의 O(N)보다 빠르다. 인덱스가 없는 등가 조건(`=`) JOIN에서 BNL보다 성능이 좋다.

MySQL 8.0.20부터는 BNL이 완전히 제거되고 인덱스 없는 등가 조건 JOIN은 모두 hash join으로 처리된다. EXPLAIN에서 `Using join buffer (hash join)`으로 표시된다.

hash join의 제약:

- 등가 조건(`=`)에서만 사용 가능하다. 범위 조건(`>`, `<`, `BETWEEN`)에서는 사용할 수 없다
- hash table이 메모리에 들어가지 않으면 디스크에 임시 파일을 만든다

## 드라이빙 테이블 선택

JOIN 성능에서 가장 중요한 요소 중 하나가 어떤 테이블이 드라이빙 테이블이 되느냐다.

Nested Loop Join에서 바깥 루프는 드라이빙 테이블의 행 수만큼 반복된다. 안쪽 루프에서 드리븐 테이블을 탐색할 때 인덱스를 사용한다면, 한 번의 탐색 비용은 행 수와 무관하게 일정하다(B-tree 높이만큼의 I/O). 따라서 총 비용은 대략 이렇다:

```
총 비용 = 드라이빙_테이블_행수 x 드리븐_테이블_탐색_비용
```

드리븐 테이블 탐색 비용이 인덱스 덕분에 상수라면, 드라이빙 테이블의 행 수가 적을수록 전체 비용이 줄어든다. 이것이 **"작은 테이블이 드라이빙 테이블이 되어야 한다"**는 원칙의 근거다.

MySQL 옵티마이저는 테이블 통계를 기반으로 자동으로 드라이빙 테이블을 결정한다. FROM 절에 쓴 순서와 무관하다:

```sql
-- 아래 두 쿼리는 옵티마이저가 같은 실행 계획을 만들 수 있다
SELECT * FROM big_table b JOIN small_table s ON b.id = s.big_id;
SELECT * FROM small_table s JOIN big_table b ON b.id = s.big_id;
```

단, LEFT JOIN은 왼쪽 테이블이 반드시 드라이빙 테이블이 된다. 왼쪽 테이블의 모든 행을 유지해야 하므로 옵티마이저가 순서를 바꿀 수 없다. LEFT JOIN을 쓸 때는 왼쪽에 작은 테이블을 놓는 것이 유리하다.

EXPLAIN 결과에서 테이블이 나열된 순서가 실제 조인 순서다. 첫 번째 행이 드라이빙 테이블이다.

## JOIN과 인덱스

JOIN 성능의 핵심은 드리븐 테이블의 조인 컬럼에 인덱스가 있느냐다.

```sql
SELECT e.name, d.name
FROM employees e
INNER JOIN departments d ON e.dept_id = d.id;
```

옵티마이저가 `employees`를 드라이빙 테이블로 선택했다고 가정하면:

- `departments.id`는 PRIMARY KEY이므로 인덱스가 있다. 각 `dept_id` 값에 대해 primary key lookup으로 O(1)에 가깝게 찾는다.
- 만약 반대로 `departments`가 드라이빙이면 `employees.dept_id`에 인덱스가 필요하다. 없으면 매번 `employees` 테이블을 풀 스캔한다.

인덱스 설계 원칙:

```sql
SELECT *
FROM orders o
INNER JOIN order_items oi ON o.id = oi.order_id
INNER JOIN products p ON oi.product_id = p.id
WHERE o.status = 'shipped';
```

이 쿼리에서 필요한 인덱스:

- `order_items.order_id` — `orders`에서 `order_items`로의 조인에 사용
- `order_items.product_id` 또는 `products.id`(PK) — `order_items`에서 `products`로의 조인에 사용
- `orders.status` — WHERE 조건 필터링에 사용

일반적으로 **드리븐 테이블의 조인 컬럼**에 인덱스를 만든다. foreign key 컬럼에 인덱스를 거는 것이 관례인 이유다.

## 세 개 이상의 테이블 JOIN

MySQL은 다중 테이블 JOIN도 Nested Loop로 처리한다. 테이블이 3개면 3중 중첩 루프다:

```
for each row in 테이블A:
    for each row in 테이블B where 조건:
        for each row in 테이블C where 조건:
            결과에 추가
```

테이블 수가 늘어날수록 순서 조합이 급격히 증가한다. 3개 테이블은 3! = 6가지, 5개 테이블은 5! = 120가지 순서가 가능하다. 옵티마이저는 모든 조합의 비용을 추정하여 최적의 순서를 선택한다. 테이블이 너무 많으면(`optimizer_search_depth` 초과) 일부만 탐색한다.

## 실전 예제

주문과 상품 정보를 함께 조회하는 전형적인 쿼리:

```sql
CREATE TABLE customers (
    id INT PRIMARY KEY,
    name VARCHAR(50)
);

CREATE TABLE orders (
    id INT PRIMARY KEY,
    customer_id INT,
    created_at DATETIME,
    INDEX idx_customer_id (customer_id)
);

CREATE TABLE order_items (
    id INT PRIMARY KEY,
    order_id INT,
    product_id INT,
    quantity INT,
    INDEX idx_order_id (order_id),
    INDEX idx_product_id (product_id)
);

CREATE TABLE products (
    id INT PRIMARY KEY,
    name VARCHAR(100),
    price DECIMAL(10, 2)
);
```

```sql
SELECT c.name AS customer, p.name AS product, oi.quantity, p.price
FROM customers c
INNER JOIN orders o ON c.id = o.customer_id
INNER JOIN order_items oi ON o.id = oi.order_id
INNER JOIN products p ON oi.product_id = p.id
WHERE o.created_at >= '2025-01-01';
```

이 쿼리의 실행 과정(옵티마이저가 `orders`를 드라이빙 테이블로 선택한 경우):

1. `orders`에서 `created_at >= '2025-01-01'` 조건으로 행을 필터링한다
2. 각 주문에 대해 `customers.id`(PK)로 고객 정보를 찾는다
3. 각 주문에 대해 `order_items.order_id` 인덱스로 주문 항목을 찾는다
4. 각 주문 항목에 대해 `products.id`(PK)로 상품 정보를 찾는다

`orders.created_at`에 인덱스가 있으면 1단계에서 range scan으로 대상을 좁힌다. 없으면 `orders` 전체를 읽고 조건을 적용한다.

EXPLAIN으로 확인하면 각 단계에서 어떤 인덱스를 사용하는지, 예상 행 수가 얼마인지 볼 수 있다. JOIN이 포함된 쿼리에서 성능 문제가 생기면 EXPLAIN 결과에서 `type`이 `ALL`인 테이블을 먼저 찾는다. 해당 테이블의 조인 컬럼에 인덱스가 없을 가능성이 높다.