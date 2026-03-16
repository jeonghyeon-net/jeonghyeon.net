# 반정규화와 트레이드오프

정규화는 데이터 무결성을 보장한다. 하지만 무결성을 위해 분리한 테이블들은 조인을 요구하고, 조인은 비용이다. 읽기 성능이 쓰기 정합성보다 중요한 상황에서는 의도적으로 정규화를 깨는 선택을 한다. 이것이 반정규화(denormalization)다.

## 반정규화를 고려하는 시점

정규화된 구조에서 출발했는데 다음과 같은 문제가 반복된다면 반정규화를 검토할 수 있다:

- 특정 조회 쿼리가 3개 이상의 테이블을 조인하고, 응답 시간이 허용 범위를 넘긴다.
- 집계 쿼리(COUNT, SUM 등)가 대량의 행을 매번 스캔한다.
- 읽기 빈도가 쓰기 빈도보다 압도적으로 높다.

핵심 판단 기준: 정규화된 구조에서 이미 인덱스 최적화, 쿼리 튜닝을 시도했는데도 성능이 부족한 경우에만 반정규화를 고려한다. 정규화를 깨기 전에 인덱스를 먼저 점검한다.

## 중복 컬럼

가장 흔한 반정규화 패턴이다. 조인을 피하기 위해 다른 테이블의 데이터를 복사해서 저장한다.

정규화된 구조:

```sql
CREATE TABLE orders (
    order_id     INT PRIMARY KEY AUTO_INCREMENT,
    customer_id  INT,
    product_id   INT,
    quantity     INT,
    order_date   DATE
);

CREATE TABLE products (
    product_id   INT PRIMARY KEY AUTO_INCREMENT,
    name         VARCHAR(200),
    price        DECIMAL(10,2)
);
```

주문 목록을 조회할 때마다 상품명을 가져오려면 조인이 필요하다:

```sql
SELECT o.order_id, p.name, o.quantity, p.price
FROM orders o
JOIN products p ON o.product_id = p.product_id
WHERE o.customer_id = 42;
```

주문 목록 조회가 초당 수천 건 발생하고, `products` 테이블 조인이 병목이라면:

```sql
ALTER TABLE orders ADD COLUMN product_name VARCHAR(200);
ALTER TABLE orders ADD COLUMN unit_price DECIMAL(10,2);
```

주문 시점의 상품명과 가격을 `orders` 테이블에 함께 저장한다. 이제 조인 없이 조회할 수 있다:

```sql
SELECT order_id, product_name, quantity, unit_price
FROM orders
WHERE customer_id = 42;
```

이 경우는 반정규화이면서 동시에 비즈니스 요구사항이기도 하다. 주문 시점의 가격을 보존해야 하기 때문이다. 상품 가격이 변경되어도 과거 주문의 가격은 바뀌지 않아야 한다. 정규화 원칙만 따르면 이 요구사항을 놓칠 수 있다.

## 요약 테이블

집계 결과를 미리 계산해서 별도 테이블에 저장하는 패턴이다.

게시글의 댓글 수를 매번 COUNT로 계산하는 상황을 가정한다:

```sql
SELECT p.post_id, p.title, COUNT(c.comment_id) AS comment_count
FROM posts p
LEFT JOIN comments c ON p.post_id = c.post_id
GROUP BY p.post_id, p.title;
```

게시글이 1만 건이고 댓글이 100만 건이면, 게시글 목록을 조회할 때마다 100만 행을 스캔해서 GROUP BY를 수행해야 한다.

반정규화: `posts` 테이블에 `comment_count` 컬럼을 추가한다.

```sql
ALTER TABLE posts ADD COLUMN comment_count INT NOT NULL DEFAULT 0;
```

댓글이 추가되거나 삭제될 때 이 값을 갱신한다:

```sql
-- 댓글 추가 시
UPDATE posts SET comment_count = comment_count + 1 WHERE post_id = 100;

-- 댓글 삭제 시
UPDATE posts SET comment_count = comment_count - 1 WHERE post_id = 100;
```

게시글 목록 조회는 조인 없이 가능해진다:

```sql
SELECT post_id, title, comment_count FROM posts;
```

일별 매출 요약처럼 더 복잡한 집계는 별도의 요약 테이블을 만든다:

```sql
CREATE TABLE daily_sales (
    sale_date    DATE PRIMARY KEY,
    total_orders INT NOT NULL DEFAULT 0,
    total_amount DECIMAL(15,2) NOT NULL DEFAULT 0
);
```

매일 배치로 계산하거나, 주문이 발생할 때마다 증분 갱신한다.

## 이력 테이블

데이터의 변경 내역을 추적해야 할 때, 현재 상태와 과거 상태를 같은 테이블에 넣으면 쿼리가 복잡해진다. 현재 값을 빠르게 조회할 수 있도록 현재 상태 테이블과 이력 테이블을 분리하는 것도 반정규화의 일종이다.

```sql
-- 현재 상품 가격
CREATE TABLE products (
    product_id   INT PRIMARY KEY AUTO_INCREMENT,
    name         VARCHAR(200),
    price        DECIMAL(10,2)
);

-- 가격 변경 이력
CREATE TABLE product_price_history (
    history_id   INT PRIMARY KEY AUTO_INCREMENT,
    product_id   INT,
    price        DECIMAL(10,2),
    changed_at   DATETIME,
    INDEX idx_product_changed (product_id, changed_at)
);
```

`products.price`와 `product_price_history`의 최신 행은 같은 값이다. 데이터가 중복되지만, 현재 가격을 조회할 때 이력 테이블을 뒤질 필요가 없다.

## 계산 컬럼

계산 비용이 높은 값을 미리 저장해두는 패턴이다.

```sql
ALTER TABLE orders ADD COLUMN total_amount DECIMAL(15,2);
```

`total_amount`는 `quantity * unit_price`에서 파생된 값이다. 정규화 관점에서는 중복이다. 하지만 주문 총액을 매번 주문 항목을 합산해서 계산하는 대신, 주문 생성 시 한 번 계산해서 저장하면 조회 성능이 향상된다.

MySQL 8.0에서는 generated column으로 이를 선언적으로 처리할 수 있다:

```sql
ALTER TABLE order_items
ADD COLUMN line_total DECIMAL(15,2)
GENERATED ALWAYS AS (quantity * unit_price) STORED;
```

`STORED` generated column은 데이터 변경 시 자동으로 재계산되어 디스크에 저장된다. 인덱스도 생성할 수 있다. `VIRTUAL`은 조회 시점에 계산되므로 저장 공간을 쓰지 않지만, InnoDB secondary index에서만 인덱스가 가능하다.

## 정합성 유지 전략

반정규화의 대가는 정합성 관리 부담이다. 같은 사실이 여러 곳에 저장되므로, 원본이 변경될 때 복사본도 함께 변경해야 한다. 이 동기화가 깨지면 데이터가 모순된다.

### 트리거

데이터 변경 시 자동으로 중복 데이터를 갱신한다:

```sql
DELIMITER //
CREATE TRIGGER after_comment_insert
AFTER INSERT ON comments
FOR EACH ROW
BEGIN
    UPDATE posts
    SET comment_count = comment_count + 1
    WHERE post_id = NEW.post_id;
END//

CREATE TRIGGER after_comment_delete
AFTER DELETE ON comments
FOR EACH ROW
BEGIN
    UPDATE posts
    SET comment_count = comment_count - 1
    WHERE post_id = OLD.post_id;
END//
DELIMITER ;
```

트리거의 장점은 애플리케이션 코드가 정합성을 신경 쓰지 않아도 된다는 것이다. 어떤 경로로 댓글이 추가되든(API, 관리자 도구, 직접 SQL 실행) 트리거가 동작한다.

단점도 있다:

- 트리거는 디버깅이 어렵다. 데이터가 어떻게 변경되었는지 추적할 때 트리거의 존재를 모르면 혼란스럽다.
- 트리거 안에서 발생한 에러는 원본 쿼리를 롤백시킨다.
- 대량 데이터 처리 시 트리거가 행마다 실행되어 성능이 저하될 수 있다.
- InnoDB에서 트리거는 같은 트랜잭션 안에서 실행되므로 락 경합이 발생할 수 있다.

### 애플리케이션 레벨 동기화

중복 데이터의 갱신을 애플리케이션 코드에서 처리한다:

```sql
START TRANSACTION;
INSERT INTO comments (post_id, content) VALUES (100, '좋은 글입니다');
UPDATE posts SET comment_count = comment_count + 1 WHERE post_id = 100;
COMMIT;
```

장점은 갱신 로직이 코드에 명시적으로 드러난다는 것이다. 단점은 모든 데이터 변경 경로에서 빠짐없이 갱신 코드를 작성해야 한다는 것이다. 한 곳이라도 빠지면 정합성이 깨진다.

### 비동기 갱신

정합성 요구 수준이 "즉시"가 아니라 "수 초 이내"로 허용되는 경우, 비동기로 처리할 수 있다:

- 댓글 추가 이벤트를 메시지 큐에 넣고, consumer가 `comment_count`를 갱신한다.
- 일정 주기로 배치를 돌려서 요약 테이블을 재계산한다.

```sql
-- 주기적으로 실제 데이터와 동기화
UPDATE posts p
SET comment_count = (
    SELECT COUNT(*) FROM comments c WHERE c.post_id = p.post_id
)
WHERE p.post_id IN (
    SELECT DISTINCT post_id FROM comments
    WHERE created_at > DATE_SUB(NOW(), INTERVAL 1 HOUR)
);
```

비동기 갱신의 장점은 쓰기 경로의 성능 부담이 없다는 것이다. 원본 테이블에 데이터를 쓸 때 추가적인 UPDATE가 발생하지 않는다. 단점은 일시적인 불일치(eventual consistency)가 발생한다는 것이다.

### 정합성 검증

어떤 방식을 쓰든, 정합성이 깨지지 않았는지 주기적으로 검증하는 것이 좋다:

```sql
-- comment_count와 실제 댓글 수가 일치하는지 확인
SELECT p.post_id, p.comment_count, COUNT(c.comment_id) AS actual_count
FROM posts p
LEFT JOIN comments c ON p.post_id = c.post_id
GROUP BY p.post_id, p.comment_count
HAVING p.comment_count != COUNT(c.comment_id);
```

불일치가 발견되면 보정한다:

```sql
UPDATE posts p
JOIN (
    SELECT post_id, COUNT(*) AS cnt
    FROM comments
    GROUP BY post_id
) c ON p.post_id = c.post_id
SET p.comment_count = c.cnt
WHERE p.comment_count != c.cnt;
```

## 트레이드오프 판단 기준

반정규화는 읽기 성능을 얻고 쓰기 복잡성과 정합성 위험을 대가로 지불하는 거래다.

반정규화가 적합한 경우:

- 읽기 빈도가 쓰기 빈도보다 10배 이상 높다.
- 일시적인 불일치가 비즈니스에 치명적이지 않다. (댓글 수가 1초간 정확하지 않아도 서비스에 문제가 없다.)
- 인덱스 최적화만으로는 조회 성능 목표를 달성할 수 없다.

반정규화를 피해야 하는 경우:

- 쓰기 빈도가 높다. 중복 데이터를 갱신하는 비용이 읽기에서 절약하는 비용을 초과한다.
- 데이터 정합성이 핵심이다. 금융 거래에서 잔액이 일시적으로라도 틀리면 안 된다.
- 아직 인덱스 최적화를 시도하지 않았다. 반정규화 전에 인덱스를 먼저 점검한다.

## 실전 사례

### 주문 테이블에 상품명 저장

이커머스에서 주문 내역을 조회할 때 상품명이 필요하다. 정규화된 구조에서는 `products` 테이블을 조인해야 한다. 하지만 주문 시점의 상품명을 보존해야 하는 비즈니스 요구도 있다. 상품명이 "프리미엄 키보드"에서 "프로 키보드"로 바뀌어도, 과거 주문에는 구매 당시의 이름이 표시되어야 한다.

```sql
CREATE TABLE order_items (
    item_id       INT PRIMARY KEY AUTO_INCREMENT,
    order_id      INT NOT NULL,
    product_id    INT NOT NULL,
    product_name  VARCHAR(200) NOT NULL,  -- 주문 시점의 상품명
    unit_price    DECIMAL(10,2) NOT NULL, -- 주문 시점의 가격
    quantity      INT NOT NULL
);
```

이 경우 반정규화가 정합성을 해치는 것이 아니라 오히려 비즈니스 정합성을 보장한다. `products.name`이 현재 상품명이고, `order_items.product_name`은 주문 시점의 상품명이다. 두 값이 다른 것이 정상이다.

### 게시글 댓글 수

게시글 목록 페이지에서 각 게시글의 댓글 수를 표시한다. 페이지당 20건의 게시글을 보여주고, 각 게시글의 댓글 수를 COUNT로 계산하면:

```sql
SELECT p.post_id, p.title, COUNT(c.comment_id) AS comment_count
FROM posts p
LEFT JOIN comments c ON p.post_id = c.post_id
WHERE p.board_id = 1
GROUP BY p.post_id, p.title
ORDER BY p.created_at DESC
LIMIT 20;
```

댓글이 수백만 건이면 이 쿼리가 느려진다. `posts` 테이블에 `comment_count` 컬럼을 추가하고, 댓글 추가/삭제 시 증감하면 게시글 목록 조회는 단순한 SELECT가 된다.

댓글 수가 일시적으로 1 정도 틀려도 사용자 경험에 큰 영향이 없다. 반면 게시글 목록은 초당 수천 번 조회된다. 이런 상황이 반정규화의 전형적인 적용 사례다.

정규화가 원칙이고, 반정규화는 원칙을 의도적으로 깨는 것이다. "의도적"이 핵심이다. 왜 깨는지, 어떤 대가를 치르는지, 정합성을 어떻게 유지할지를 명확히 결정한 후에 반정규화를 적용한다.

## 정리

- 반정규화는 읽기 성능을 얻기 위해 쓰기 복잡성과 정합성 위험을 대가로 지불하는 거래다.
- 중복 컬럼, 요약 테이블, 계산 컬럼이 대표적인 반정규화 패턴이다.
- 정합성 유지는 트리거, 애플리케이션 레벨 동기화, 비동기 갱신 중 상황에 맞는 방식을 선택한다.
- 인덱스 최적화와 쿼리 튜닝을 먼저 시도한 후에 반정규화를 고려해야 한다.
- 어떤 방식을 쓰든 정합성 검증 쿼리를 주기적으로 실행하여 불일치를 감지하고 보정한다.
