# 쿼리 최적화

EXPLAIN으로 실행 계획을 읽을 수 있게 되었다면, 그다음은 실제로 쿼리를 개선하는 단계다. 쿼리 최적화는 크게 두 가지로 나뉜다. 인덱스를 올바르게 설계하는 것과, 옵티마이저가 그 인덱스를 효과적으로 사용하도록 쿼리를 작성하는 것이다.

## 느린 쿼리의 일반적인 패턴

쿼리가 느려지는 원인은 대부분 몇 가지 패턴으로 수렴한다.

- **전체 테이블 스캔**: 인덱스가 없거나, 있어도 사용되지 않는 경우.
- **과도한 행 접근**: 조건에 맞는 행은 소수인데 전체를 읽고 필터링하는 경우.
- **불필요한 정렬**: ORDER BY를 인덱스로 해결하지 못해 filesort가 발생하는 경우.
- **임시 테이블 생성**: GROUP BY와 ORDER BY의 기준이 달라 내부 임시 테이블이 필요한 경우.
- **상관 서브쿼리**: 외부 쿼리의 각 행마다 서브쿼리가 반복 실행되는 경우.
- **락 대기**: 다른 트랜잭션이 잡고 있는 락을 기다리는 경우. 쿼리 자체의 문제가 아니라 동시성 문제다.

이 중 처음 다섯 가지는 쿼리와 인덱스로 해결할 수 있다.

## 인덱스 설계 전략

인덱스를 만들 때는 "어떤 쿼리가 이 테이블을 사용하는가"에서 출발한다. 테이블의 컬럼을 보고 인덱스를 만드는 것이 아니라, 쿼리의 WHERE, JOIN, ORDER BY, GROUP BY를 보고 인덱스를 설계한다.

### 복합 인덱스의 컬럼 순서

복합 인덱스에서 컬럼 순서가 성능을 결정한다. 기본 원칙은 다음과 같다.

1. **동등 조건(=)** 컬럼을 앞에 놓는다.
2. **범위 조건(>, <, BETWEEN)** 컬럼은 그 다음에 놓는다. 범위 조건 이후의 컬럼은 인덱스에서 필터링에 활용되지 않는다.
3. **ORDER BY** 컬럼을 마지막에 놓으면 filesort를 피할 수 있다. 단, 앞선 컬럼들이 모두 동등 조건이어야 한다.

```sql
-- 이 쿼리에 최적화된 인덱스를 설계한다고 가정한다.
SELECT * FROM orders
WHERE status = 'paid' AND created_at >= '2025-01-01'
ORDER BY created_at DESC
LIMIT 20;
```

`status`는 동등 조건, `created_at`은 범위 조건이면서 ORDER BY 대상이다.

```sql
-- 최적 인덱스
ALTER TABLE orders ADD INDEX idx_status_created (status, created_at);
```

이 인덱스로 `status = 'paid'`인 범위를 먼저 좁힌 뒤, 그 범위 내에서 `created_at`의 인덱스 순서를 활용하여 정렬 없이 결과를 반환한다.

만약 순서를 바꾸어 `(created_at, status)`로 만들면, `created_at >= '2025-01-01'` 범위 스캔은 가능하지만 `status = 'paid'` 필터링은 인덱스 수준에서 효율적으로 처리되지 않는다.

### 선택도(selectivity) 고려

같은 조건이라도 선택도에 따라 인덱스의 효과가 다르다.

```sql
-- 나쁜 선택도: gender 컬럼은 값이 2~3개뿐
SELECT * FROM users WHERE gender = 'M';

-- 좋은 선택도: email은 거의 고유
SELECT * FROM users WHERE email = 'user@example.com';
```

선택도가 낮은 컬럼(값의 종류가 적은 컬럼)만으로 구성된 인덱스는 효과가 미미하다. 옵티마이저가 전체 스캔이 낫다고 판단할 수 있다. 다만 복합 인덱스에서 다른 컬럼과 조합하면 효과적일 수 있다.

## 커버링 인덱스 활용

커버링 인덱스는 쿼리에 필요한 모든 컬럼이 인덱스에 포함된 경우를 말한다. 테이블 데이터에 접근하지 않고 인덱스만으로 결과를 반환하므로, 디스크 I/O가 크게 줄어든다.

```sql
-- 인덱스: (status, created_at)
-- 커버링 인덱스가 아닌 경우 (total은 인덱스에 없음)
SELECT status, created_at, total FROM orders WHERE status = 'paid';

-- 커버링 인덱스로 만들려면
ALTER TABLE orders ADD INDEX idx_covering (status, created_at, total);

-- 이제 인덱스만으로 결과 반환 가능
SELECT status, created_at, total FROM orders WHERE status = 'paid';
```

EXPLAIN에서 Extra에 `Using index`가 표시되면 커버링 인덱스가 적용된 것이다.

InnoDB에서는 모든 secondary index에 PK 값이 자동으로 포함된다. 따라서 PK 컬럼은 커버링 인덱스 계산에서 항상 포함된 것으로 취급한다.

```sql
-- PK: id, 인덱스: (user_id)
-- id는 인덱스에 자동 포함
SELECT id, user_id FROM orders WHERE user_id = 42;
-- Using index
```

커버링 인덱스를 위해 인덱스에 컬럼을 무작정 추가하면 인덱스 크기가 커지고, 쓰기 성능이 저하된다. 빈번하게 실행되는 핵심 쿼리에 대해서만 고려한다.

## 페이지네이션 최적화

### OFFSET의 문제

```sql
SELECT * FROM posts ORDER BY created_at DESC LIMIT 20 OFFSET 10000;
```

이 쿼리는 10,020행을 읽고 앞의 10,000행을 버린 뒤 20행만 반환한다. OFFSET이 클수록 읽고 버리는 행이 많아져서 느려진다.

```sql
-- OFFSET 0: 20행 읽기
-- OFFSET 10000: 10,020행 읽고 10,000행 버리기
-- OFFSET 100000: 100,020행 읽고 100,000행 버리기
```

### cursor 기반 페이지네이션

이전 페이지의 마지막 행의 값을 기준으로 다음 페이지를 가져온다.

```sql
-- 첫 페이지
SELECT * FROM posts ORDER BY created_at DESC, id DESC LIMIT 20;

-- 다음 페이지: 이전 페이지 마지막 행의 created_at과 id를 사용
SELECT * FROM posts
WHERE (created_at, id) < ('2025-03-15 10:00:00', 9500)
ORDER BY created_at DESC, id DESC
LIMIT 20;
```

`(created_at, id)` 인덱스가 있으면 항상 인덱스의 특정 지점부터 20행만 읽는다. OFFSET과 달리 몇 번째 페이지든 일정한 성능을 보인다.

단점은 "N번째 페이지로 바로 이동"이 불가능하다는 것이다. 무한 스크롤이나 "다음 페이지" 방식에서 사용한다.

### deferred join

OFFSET을 써야 하는 상황에서의 차선책이다. PK만 먼저 OFFSET으로 가져온 뒤, 해당 PK의 전체 데이터를 JOIN으로 조회한다.

```sql
SELECT p.*
FROM posts p
JOIN (
    SELECT id FROM posts ORDER BY created_at DESC LIMIT 20 OFFSET 10000
) AS t ON t.id = p.id
ORDER BY p.created_at DESC;
```

서브쿼리에서는 커버링 인덱스(`created_at`, PK인 `id`는 자동 포함)만 사용하여 10,020행을 읽는다. 테이블 데이터 접근은 최종 20행에 대해서만 발생한다. 전체 컬럼이 많은 테이블에서 효과가 크다.

## 불필요한 정렬 제거

ORDER BY가 인덱스 순서와 일치하면 정렬이 필요 없다. 인덱스 설계 시 ORDER BY를 고려하면 filesort를 피할 수 있다.

```sql
-- 인덱스: (user_id, created_at)

-- filesort 없음: 인덱스 순서와 ORDER BY가 일치
SELECT * FROM orders WHERE user_id = 42 ORDER BY created_at;

-- filesort 없음: DESC도 인덱스를 역순으로 읽어 해결
SELECT * FROM orders WHERE user_id = 42 ORDER BY created_at DESC;

-- filesort 발생: 인덱스에 없는 컬럼으로 정렬
SELECT * FROM orders WHERE user_id = 42 ORDER BY total;

-- filesort 발생: 정렬 방향이 혼합됨
SELECT * FROM orders WHERE user_id = 42 ORDER BY created_at ASC, total DESC;
```

GROUP BY도 정렬을 유발한다. MySQL은 GROUP BY 결과를 기본적으로 그룹 키 순서로 정렬한다. 정렬이 필요 없다면 `ORDER BY NULL`을 명시하여 불필요한 정렬을 방지할 수 있다. 다만 MySQL 8.0부터는 GROUP BY가 암묵적 정렬을 보장하지 않으므로, 이 최적화가 필요한 경우는 줄어들었다.

## 서브쿼리를 JOIN으로 변환

MySQL의 옵티마이저는 많은 경우 서브쿼리를 내부적으로 최적화하지만, 상관 서브쿼리(correlated subquery)는 여전히 성능 문제를 일으킬 수 있다.

```sql
-- 상관 서브쿼리: users의 각 행마다 orders를 조회
SELECT u.id, u.name,
    (SELECT COUNT(*) FROM orders o WHERE o.user_id = u.id) AS order_count
FROM users u;

-- JOIN으로 변환
SELECT u.id, u.name, COUNT(o.id) AS order_count
FROM users u
LEFT JOIN orders o ON o.user_id = u.id
GROUP BY u.id;
```

상관 서브쿼리는 외부 쿼리의 행 수만큼 내부 쿼리가 반복 실행된다. `users`가 10만 행이면 `orders` 테이블에 10만 번 접근한다. JOIN으로 변환하면 한 번의 조인 연산으로 처리된다.

다만 모든 서브쿼리를 JOIN으로 변환해야 하는 것은 아니다. EXISTS나 IN 서브쿼리는 MySQL 8.0에서 semi-join 최적화가 적용되어 효율적으로 처리되는 경우가 많다.

```sql
-- semi-join으로 최적화될 수 있음
SELECT * FROM users u
WHERE EXISTS (SELECT 1 FROM orders o WHERE o.user_id = u.id AND o.total > 10000);
```

EXPLAIN에서 `select_type`이 `DEPENDENT SUBQUERY`로 표시되면 상관 서브쿼리가 최적화되지 않고 그대로 실행되고 있다는 뜻이다. 이때는 JOIN이나 다른 방식으로 변환을 고려한다.

## COUNT(*) 최적화

`COUNT(*)`는 단순해 보이지만 대규모 테이블에서 성능 문제를 일으키는 대표적인 쿼리다.

### 전체 행 수

InnoDB는 MVCC 특성상 트랜잭션마다 보이는 행이 다를 수 있어, 정확한 전체 행 수를 미리 저장하지 않는다. `SELECT COUNT(*) FROM orders`는 매번 인덱스를 전체 스캔한다.

자주 필요한 전체 행 수가 정확하지 않아도 되는 경우, 근사값을 사용할 수 있다.

```sql
-- 근사값 (정확하지 않지만 빠름)
SELECT TABLE_ROWS
FROM information_schema.TABLES
WHERE TABLE_SCHEMA = 'mydb' AND TABLE_NAME = 'orders';
```

### 조건부 COUNT

WHERE 조건이 있는 COUNT는 인덱스를 활용할 수 있다.

```sql
-- 인덱스가 없으면 전체 스캔
SELECT COUNT(*) FROM orders WHERE status = 'pending';

-- 인덱스: (status)가 있으면 해당 범위만 스캔
ALTER TABLE orders ADD INDEX idx_status (status);
SELECT COUNT(*) FROM orders WHERE status = 'pending';
```

커버링 인덱스가 적용되면 테이블 데이터를 읽지 않고 인덱스만 스캔하므로 효율적이다.

### COUNT 대안

정확한 수가 필요 없고 "있는지 없는지"만 확인하면 되는 경우, `EXISTS`를 사용한다.

```sql
-- 비효율: 전체 COUNT 후 비교
SELECT CASE WHEN COUNT(*) > 0 THEN 1 ELSE 0 END
FROM orders WHERE user_id = 42;

-- 효율: 하나만 찾으면 즉시 반환
SELECT EXISTS(SELECT 1 FROM orders WHERE user_id = 42);
```

## 조건절 최적화: sargable vs non-sargable

**sargable**(Search ARGument ABLE)은 인덱스를 사용할 수 있는 조건 형태를 말한다. 컬럼에 함수를 적용하거나 연산을 수행하면 인덱스를 사용할 수 없게 된다.

### non-sargable 조건과 변환

```sql
-- non-sargable: 컬럼에 함수 적용
SELECT * FROM orders WHERE YEAR(created_at) = 2025;

-- sargable: 범위 조건으로 변환
SELECT * FROM orders
WHERE created_at >= '2025-01-01' AND created_at < '2026-01-01';
```

```sql
-- non-sargable: 컬럼에 연산
SELECT * FROM products WHERE price * 1.1 > 10000;

-- sargable: 상수 쪽으로 연산을 옮김
SELECT * FROM products WHERE price > 10000 / 1.1;
```

```sql
-- non-sargable: 타입 불일치로 암묵적 변환 발생
-- phone_number가 VARCHAR인데 숫자로 비교
SELECT * FROM users WHERE phone_number = 01012345678;

-- sargable: 문자열로 비교
SELECT * FROM users WHERE phone_number = '01012345678';
```

```sql
-- non-sargable: LIKE 패턴이 %로 시작
SELECT * FROM products WHERE name LIKE '%키보드%';

-- sargable: 접두사 매칭
SELECT * FROM products WHERE name LIKE '키보드%';
```

`LIKE '%키보드%'`처럼 앞에 %가 오는 패턴은 인덱스를 사용할 수 없다. 전문 검색이 필요하면 FULLTEXT 인덱스나 별도의 검색 엔진을 사용한다.

### OR 조건

OR로 연결된 조건은 인덱스 사용이 제한적이다.

```sql
-- 각각 인덱스가 있어도 OR이면 비효율적일 수 있음
SELECT * FROM orders WHERE user_id = 42 OR status = 'pending';
```

MySQL은 index_merge 최적화로 두 인덱스를 각각 검색한 결과를 합칠 수 있지만, 항상 적용되지는 않는다. 가능하면 UNION ALL로 분리하는 것이 명시적이다.

```sql
SELECT * FROM orders WHERE user_id = 42
UNION ALL
SELECT * FROM orders WHERE status = 'pending' AND user_id != 42;
```

## UNION vs UNION ALL

`UNION`은 중복 행을 제거한다. 중복 제거를 위해 정렬이나 해시 연산이 필요하므로 `UNION ALL`보다 느리다.

```sql
-- UNION: 중복 제거 (내부적으로 DISTINCT 처리)
SELECT user_id FROM orders WHERE status = 'paid'
UNION
SELECT user_id FROM orders WHERE status = 'shipped';

-- UNION ALL: 중복 허용 (그대로 합침)
SELECT user_id FROM orders WHERE status = 'paid'
UNION ALL
SELECT user_id FROM orders WHERE status = 'shipped';
```

중복이 발생하지 않는 것이 보장되거나, 중복이 있어도 상관없는 경우에는 항상 `UNION ALL`을 사용한다.

## 실전 쿼리 튜닝 사례

### 사례 1: 주문 목록 조회

관리자 페이지에서 최근 주문을 조회하는 쿼리가 느리다.

```sql
SELECT o.id, o.total, o.status, o.created_at,
       u.name, u.email
FROM orders o
JOIN users u ON u.id = o.user_id
WHERE o.created_at >= '2025-03-01'
ORDER BY o.created_at DESC
LIMIT 50;
```

EXPLAIN 결과, `orders` 테이블에서 type이 `ALL`이다.

**분석**: `created_at`에 인덱스가 없어 전체 스캔이 발생한다.

**해결**:

```sql
ALTER TABLE orders ADD INDEX idx_created_at (created_at);
```

인덱스 추가 후 type이 `range`로 바뀌고, `Backward index scan`으로 정렬도 해결된다. `users` 테이블은 PK JOIN이므로 `eq_ref`로 이미 최적이다.

### 사례 2: 사용자별 주문 통계

대시보드에서 활성 사용자의 주문 통계를 조회한다.

```sql
SELECT u.id, u.name,
    (SELECT COUNT(*) FROM orders WHERE user_id = u.id) AS total_orders,
    (SELECT SUM(total) FROM orders WHERE user_id = u.id) AS total_amount
FROM users u
WHERE u.status = 'active';
```

**분석**: 상관 서브쿼리가 2개 있다. 활성 사용자가 1만 명이면 `orders` 테이블에 2만 번 접근한다.

**해결**: JOIN으로 변환한다.

```sql
SELECT u.id, u.name,
       COUNT(o.id) AS total_orders,
       COALESCE(SUM(o.total), 0) AS total_amount
FROM users u
LEFT JOIN orders o ON o.user_id = u.id
WHERE u.status = 'active'
GROUP BY u.id;
```

한 번의 JOIN과 집계로 처리된다. `orders.user_id`에 인덱스가 있으면 JOIN 성능도 보장된다.

### 사례 3: 검색 결과 페이지네이션

상품 검색 결과를 10페이지까지는 빠르지만, 100페이지 이후부터 눈에 띄게 느려진다.

```sql
SELECT * FROM products
WHERE category_id = 5 AND is_active = 1
ORDER BY created_at DESC
LIMIT 20 OFFSET 2000;
```

**분석**: OFFSET 2000이므로 2,020행을 읽고 2,000행을 버린다. 페이지가 깊어질수록 악화된다.

**해결**: cursor 기반 페이지네이션으로 전환한다.

```sql
-- 인덱스: (category_id, is_active, created_at)
SELECT * FROM products
WHERE category_id = 5 AND is_active = 1
  AND (created_at, id) < ('2025-03-10 15:30:00', 48210)
ORDER BY created_at DESC, id DESC
LIMIT 20;
```

인덱스의 특정 지점부터 20행만 읽으므로, 어느 페이지든 동일한 성능이다.

## 정리

쿼리 최적화의 핵심은 불필요한 행 접근과 정렬을 줄이는 것이다. 인덱스는 쿼리의 WHERE, ORDER BY, GROUP BY를 기준으로 설계한다. 컬럼에 함수를 적용하면 인덱스가 무력화된다. OFFSET 기반 페이지네이션은 깊은 페이지에서 성능이 저하되므로 cursor 기반으로 전환한다. 서브쿼리가 상관 서브쿼리로 실행되고 있다면 JOIN 변환을 검토한다. 모든 최적화의 시작은 EXPLAIN이다.
