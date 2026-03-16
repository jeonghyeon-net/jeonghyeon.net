# 실전 테이블 설계 패턴

정규화 이론과 제약 조건을 알았다면, 실제 서비스에서 반복적으로 등장하는 설계 문제와 그에 대한 패턴을 알아야 한다. 여기서 다루는 패턴들은 대부분의 서비스에서 한 번 이상 마주치는 문제들이다.

## 1:N 관계

가장 기본적인 관계다. 사용자와 주문, 게시글과 댓글, 부서와 직원 등.

```sql
CREATE TABLE users (
    user_id  INT UNSIGNED PRIMARY KEY AUTO_INCREMENT,
    name     VARCHAR(100) NOT NULL
);

CREATE TABLE orders (
    order_id  INT UNSIGNED PRIMARY KEY AUTO_INCREMENT,
    user_id   INT UNSIGNED NOT NULL,
    total     DECIMAL(10,2) NOT NULL,
    INDEX idx_user_id (user_id)
);
```

N 쪽 테이블(`orders`)에 FK 컬럼(`user_id`)을 둔다. FK 제약 조건을 사용할지 여부는 팀의 정책에 따르지만, FK 컬럼에 인덱스는 반드시 생성한다. FK 제약 조건을 선언하면 InnoDB가 자동으로 인덱스를 생성하지만, FK를 사용하지 않는 경우에는 직접 인덱스를 추가해야 한다.

FK 컬럼의 인덱스가 없으면 "특정 사용자의 주문 조회" 같은 기본적인 쿼리에서 full table scan이 발생한다:

```sql
-- idx_user_id가 없으면 전체 테이블을 스캔
SELECT * FROM orders WHERE user_id = 42;
```

## M:N 관계

사용자와 역할, 학생과 과목, 상품과 태그 등. 두 엔티티 사이에 M:N 관계가 있으면 중간 테이블(junction table)을 만든다.

```sql
CREATE TABLE tags (
    tag_id  INT UNSIGNED PRIMARY KEY AUTO_INCREMENT,
    name    VARCHAR(50) NOT NULL UNIQUE
);

CREATE TABLE product_tags (
    product_id  INT UNSIGNED,
    tag_id      INT UNSIGNED,
    PRIMARY KEY (product_id, tag_id),
    INDEX idx_tag_id (tag_id)
);
```

중간 테이블의 PK를 `(product_id, tag_id)` 복합키로 설정하면, 같은 상품에 같은 태그가 중복 부여되는 것을 방지한다. 동시에 `WHERE product_id = ?` 조회가 clustered index를 사용한다.

`tag_id`로 조회하는 패턴("이 태그가 달린 모든 상품")도 있다면, `tag_id`에 별도 인덱스를 추가한다. 복합 PK의 왼쪽 prefix가 `product_id`이므로, `tag_id` 단독 조회에는 사용할 수 없다.

중간 테이블에 관계 자체의 속성을 추가할 수도 있다:

```sql
CREATE TABLE enrollments (
    student_id   INT UNSIGNED,
    course_id    INT UNSIGNED,
    enrolled_at  DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    grade        CHAR(2),
    PRIMARY KEY (student_id, course_id),
    INDEX idx_course_id (course_id)
);
```

`enrolled_at`과 `grade`는 학생이나 과목의 속성이 아니라 "수강"이라는 관계의 속성이다. 이런 속성이 있으면 중간 테이블은 독립적인 엔티티로 취급하고, 별도의 AUTO_INCREMENT PK를 부여하는 것도 고려한다.

## 상속 구조 표현

사용자가 일반 회원, 판매자, 관리자로 나뉘는 경우. 결제 수단이 카드, 계좌이체, 간편결제로 나뉘는 경우. 객체지향의 상속을 관계형 테이블로 표현하는 세 가지 방법이 있다.

### 단일 테이블 (Single Table Inheritance, STI)

모든 타입의 데이터를 하나의 테이블에 넣고, `type` 컬럼으로 구분한다:

```sql
CREATE TABLE users (
    user_id       INT UNSIGNED PRIMARY KEY AUTO_INCREMENT,
    type          VARCHAR(20) NOT NULL,  -- 'member', 'seller', 'admin'
    name          VARCHAR(100) NOT NULL,
    email         VARCHAR(255) NOT NULL,
    -- member 전용
    membership    VARCHAR(20),
    -- seller 전용
    shop_name     VARCHAR(200),
    commission    DECIMAL(5,2),
    -- admin 전용
    department    VARCHAR(100),
    access_level  INT
);
```

장점:

- 조인 없이 모든 사용자를 조회할 수 있다.
- 구현이 단순하다. ORM에서 가장 쉽게 매핑된다.
- 타입 간 전환이 `type` 컬럼 수정으로 가능하다.

단점:

- 타입별 전용 컬럼이 다른 타입에서는 항상 NULL이다. 컬럼에 NOT NULL을 걸 수 없다.
- 타입이 많아지거나 타입별 컬럼이 많으면 테이블이 비대해진다.
- 비즈니스 규칙("판매자는 반드시 shop_name이 있어야 한다")을 데이터베이스 레벨에서 강제하기 어렵다.

타입 수가 적고, 타입별 전용 컬럼이 소수일 때 적합하다.

### 타입별 분리 (Concrete Table Inheritance, CTI)

각 타입을 별도 테이블로 완전히 분리한다:

```sql
CREATE TABLE members (
    member_id   INT UNSIGNED PRIMARY KEY AUTO_INCREMENT,
    name        VARCHAR(100) NOT NULL,
    email       VARCHAR(255) NOT NULL,
    membership  VARCHAR(20) NOT NULL
);

CREATE TABLE sellers (
    seller_id   INT UNSIGNED PRIMARY KEY AUTO_INCREMENT,
    name        VARCHAR(100) NOT NULL,
    email       VARCHAR(255) NOT NULL,
    shop_name   VARCHAR(200) NOT NULL,
    commission  DECIMAL(5,2) NOT NULL
);

CREATE TABLE admins (
    admin_id     INT UNSIGNED PRIMARY KEY AUTO_INCREMENT,
    name         VARCHAR(100) NOT NULL,
    email        VARCHAR(255) NOT NULL,
    department   VARCHAR(100) NOT NULL,
    access_level INT NOT NULL
);
```

장점:

- 각 테이블이 해당 타입에 필요한 컬럼만 가진다. NULL 컬럼이 없다.
- NOT NULL 제약 조건을 자유롭게 적용할 수 있다.

단점:

- 공통 컬럼(`name`, `email`)이 모든 테이블에 중복된다.
- "모든 사용자"를 조회하려면 UNION이 필요하다.
- "사용자 ID로 주문을 조회"할 때, 주문 테이블이 어떤 사용자 테이블을 참조하는지 모호하다.
- 타입 간 공통 로직을 변경할 때 모든 테이블을 수정해야 한다.

타입 간 공통점이 적고, 타입별 쿼리가 대부분일 때 적합하다.

### 공통 테이블 + 확장 테이블 (Class Table Inheritance)

공통 속성을 부모 테이블에, 타입별 속성을 확장 테이블에 분리한다:

```sql
CREATE TABLE users (
    user_id  INT UNSIGNED PRIMARY KEY AUTO_INCREMENT,
    type     VARCHAR(20) NOT NULL,
    name     VARCHAR(100) NOT NULL,
    email    VARCHAR(255) NOT NULL
);

CREATE TABLE member_profiles (
    user_id     INT UNSIGNED PRIMARY KEY,
    membership  VARCHAR(20) NOT NULL,
    FOREIGN KEY (user_id) REFERENCES users(user_id)
);

CREATE TABLE seller_profiles (
    user_id     INT UNSIGNED PRIMARY KEY,
    shop_name   VARCHAR(200) NOT NULL,
    commission  DECIMAL(5,2) NOT NULL,
    FOREIGN KEY (user_id) REFERENCES users(user_id)
);

CREATE TABLE admin_profiles (
    user_id       INT UNSIGNED PRIMARY KEY,
    department    VARCHAR(100) NOT NULL,
    access_level  INT NOT NULL,
    FOREIGN KEY (user_id) REFERENCES users(user_id)
);
```

장점:

- 공통 속성은 한 곳에서 관리된다. 중복이 없다.
- 각 확장 테이블에 NOT NULL을 적용할 수 있다.
- 전체 사용자 조회는 `users` 테이블만으로 가능하다.
- 타입별 상세 조회 시에만 확장 테이블을 조인한다.

단점:

- 타입별 상세 정보를 가져오려면 조인이 필요하다.
- 테이블 수가 늘어난다.

대부분의 경우에 균형 잡힌 선택이다. 공통 쿼리와 타입별 쿼리를 모두 효율적으로 처리할 수 있다.

## 소프트 삭제

데이터를 물리적으로 삭제하지 않고, 삭제 표시만 하는 패턴이다. 감사(audit) 요구사항, 데이터 복구 필요성, 참조 무결성 유지 등의 이유로 사용한다.

```sql
ALTER TABLE users ADD COLUMN deleted_at DATETIME DEFAULT NULL;
```

삭제 시:

```sql
UPDATE users SET deleted_at = NOW() WHERE user_id = 42;
```

조회 시:

```sql
SELECT * FROM users WHERE deleted_at IS NULL;
```

### 인덱스 영향

소프트 삭제의 가장 큰 문제는 모든 쿼리에 `WHERE deleted_at IS NULL` 조건이 추가된다는 것이다. 기존 인덱스가 삭제된 행과 삭제되지 않은 행을 모두 포함하므로, 삭제된 행이 쌓일수록 인덱스 효율이 떨어진다.

전체 사용자 중 10%가 삭제 상태라면, 인덱스 스캔 시 10%는 불필요한 데이터를 읽는 것이다. 삭제 비율이 높아지면 이 비효율이 커진다.

대응 방법:

```sql
-- 복합 인덱스에 deleted_at를 포함
CREATE INDEX idx_email_active ON users (email, deleted_at);

-- 또는 is_deleted 플래그를 사용
ALTER TABLE users ADD COLUMN is_deleted TINYINT NOT NULL DEFAULT 0;
CREATE INDEX idx_active_users ON users (is_deleted, created_at);
```

### UNIQUE 제약과의 충돌

이메일이 UNIQUE인 테이블에서 소프트 삭제를 사용하면, 삭제된 사용자의 이메일로 새로운 계정을 만들 수 없다:

```sql
-- user_id=42의 이메일이 alice@example.com이고 소프트 삭제된 상태
UPDATE users SET deleted_at = NOW() WHERE user_id = 42;

-- 같은 이메일로 새 계정 생성 시도
INSERT INTO users (email, name) VALUES ('alice@example.com', '새 앨리스');
-- ERROR: Duplicate entry 'alice@example.com' for key 'uk_email'
```

해결 방법 중 하나는 UNIQUE 인덱스에 조건을 포함하는 것인데, MySQL은 partial index를 지원하지 않는다. 대안으로 `deleted_at`을 UNIQUE 인덱스에 포함하는 방법이 있다:

```sql
-- deleted_at이 NULL이면 활성, 값이 있으면 삭제
-- (email, deleted_at) 조합이 UNIQUE
ALTER TABLE users DROP INDEX uk_email;
ALTER TABLE users ADD UNIQUE KEY uk_email_deleted (email, deleted_at);
```

이렇게 하면 활성 상태(`deleted_at IS NULL`)에서는 이메일이 유일하고, 삭제 시점이 다르면 같은 이메일이 여러 행에 존재할 수 있다.

## 이력 관리 패턴

데이터의 변경 내역을 추적해야 하는 경우가 많다. 가격 변경 이력, 상태 변경 이력, 사용자 정보 수정 이력 등.

### 이력 테이블

현재 데이터와 이력 데이터를 분리하는 패턴이다:

```sql
CREATE TABLE products (
    product_id  INT UNSIGNED PRIMARY KEY AUTO_INCREMENT,
    name        VARCHAR(200) NOT NULL,
    price       DECIMAL(10,2) NOT NULL,
    updated_at  DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE product_history (
    history_id   INT UNSIGNED PRIMARY KEY AUTO_INCREMENT,
    product_id   INT UNSIGNED NOT NULL,
    name         VARCHAR(200) NOT NULL,
    price        DECIMAL(10,2) NOT NULL,
    changed_at   DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    change_type  ENUM('INSERT', 'UPDATE', 'DELETE') NOT NULL,
    INDEX idx_product_changed (product_id, changed_at)
);
```

상품이 변경될 때마다 이전 상태를 이력 테이블에 기록한다:

```sql
-- 상품 가격 변경
INSERT INTO product_history (product_id, name, price, change_type)
SELECT product_id, name, price, 'UPDATE' FROM products WHERE product_id = 1;

UPDATE products SET price = 95000, updated_at = NOW() WHERE product_id = 1;
```

현재 상태 조회는 `products` 테이블만 보면 된다. 이력 조회는 `product_history`를 시간 순으로 조회한다.

### SCD (Slowly Changing Dimension)

데이터 웨어하우스에서 유래한 개념이지만, 서비스 테이블에서도 유용하다. Type 2 SCD가 가장 많이 사용된다.

```sql
CREATE TABLE customer_addresses (
    address_id    INT UNSIGNED PRIMARY KEY AUTO_INCREMENT,
    customer_id   INT UNSIGNED NOT NULL,
    address       VARCHAR(500) NOT NULL,
    valid_from    DATETIME NOT NULL,
    valid_to      DATETIME,          -- NULL이면 현재 유효
    is_current    TINYINT NOT NULL DEFAULT 1,
    INDEX idx_customer_current (customer_id, is_current)
);
```

주소가 변경될 때 기존 행의 `valid_to`를 설정하고, 새 행을 추가한다:

```sql
-- 기존 주소 만료
UPDATE customer_addresses
SET valid_to = NOW(), is_current = 0
WHERE customer_id = 42 AND is_current = 1;

-- 새 주소 추가
INSERT INTO customer_addresses (customer_id, address, valid_from)
VALUES (42, '서울시 강남구 ...', NOW());
```

특정 시점의 주소를 조회할 수 있다:

```sql
SELECT address
FROM customer_addresses
WHERE customer_id = 42
  AND valid_from <= '2025-06-15 12:00:00'
  AND (valid_to IS NULL OR valid_to > '2025-06-15 12:00:00');
```

이력 테이블 방식은 현재 상태와 이력이 분리되어 현재 조회가 빠르다. SCD Type 2는 시점 기반 조회가 자연스럽다. 요구사항에 따라 선택한다.

## 파티셔닝

테이블의 데이터가 수천만 건 이상으로 커지면, 인덱스 최적화만으로는 한계가 있다. 파티셔닝은 하나의 논리적 테이블을 여러 물리적 파티션으로 나누는 기능이다.

### RANGE 파티션

날짜나 숫자 범위로 데이터를 분할한다. 시계열 데이터에 적합하다:

```sql
CREATE TABLE access_logs (
    log_id      BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
    user_id     INT UNSIGNED,
    action      VARCHAR(50),
    created_at  DATE NOT NULL,
    PRIMARY KEY (log_id, created_at),
    INDEX idx_created_at (created_at)
)
PARTITION BY RANGE (YEAR(created_at)) (
    PARTITION p2024 VALUES LESS THAN (2025),
    PARTITION p2025 VALUES LESS THAN (2026),
    PARTITION p2026 VALUES LESS THAN (2027),
    PARTITION p_future VALUES LESS THAN MAXVALUE
);
```

`WHERE created_at BETWEEN '2025-01-01' AND '2025-12-31'` 쿼리는 `p2025` 파티션만 스캔한다. 나머지 파티션은 아예 접근하지 않는다(partition pruning).

오래된 데이터 삭제가 매우 빠르다. `DELETE FROM access_logs WHERE created_at < '2024-01-01'`은 수백만 행을 개별 삭제해야 하지만, `ALTER TABLE access_logs DROP PARTITION p2023`은 파일 하나를 삭제하는 것과 같다. 순식간에 완료된다.

### LIST 파티션

특정 값 목록으로 분할한다:

```sql
CREATE TABLE orders (
    order_id    BIGINT UNSIGNED PRIMARY KEY AUTO_INCREMENT,
    region      VARCHAR(10) NOT NULL,
    total       DECIMAL(10,2),
    created_at  DATE NOT NULL
)
PARTITION BY LIST COLUMNS (region) (
    PARTITION p_kr VALUES IN ('KR'),
    PARTITION p_jp VALUES IN ('JP'),
    PARTITION p_us VALUES IN ('US'),
    PARTITION p_other VALUES IN ('TW', 'TH', 'VN')
);
```

`WHERE region = 'KR'`으로 조회하면 `p_kr` 파티션만 스캔한다.

### HASH 파티션

데이터를 균등하게 분산시키고 싶을 때 사용한다:

```sql
CREATE TABLE sessions (
    session_id   BINARY(16) PRIMARY KEY,
    user_id      INT UNSIGNED,
    data         JSON,
    expires_at   DATETIME
)
PARTITION BY HASH (user_id)
PARTITIONS 8;
```

`user_id`의 해시값에 따라 8개 파티션에 분산된다. 특정 파티션에 데이터가 몰리는 skew를 방지한다. 다만 범위 조회에는 도움이 되지 않는다.

### 파티셔닝 판단 기준

파티셔닝이 효과적인 경우:

- 테이블 크기가 수천만 행 이상이고, 쿼리 패턴이 특정 파티션 키 범위에 집중된다.
- 오래된 데이터를 주기적으로 대량 삭제해야 한다. (RANGE 파티션의 `DROP PARTITION`)
- 시계열 데이터처럼 시간 기반 범위 조회가 주요 패턴이다.

파티셔닝이 적합하지 않은 경우:

- 쿼리가 파티션 키를 포함하지 않으면 모든 파티션을 스캔한다. 오히려 느려질 수 있다.
- 파티션 수가 너무 많으면 파일 핸들과 메모리 사용량이 증가한다.
- UNIQUE 제약 조건이 파티션 키를 포함해야 하는 제한이 있다. 파티션 키가 아닌 컬럼에 UNIQUE를 걸 수 없다.

파티셔닝은 마지막 수단에 가깝다. 인덱스 최적화, 쿼리 튜닝, 반정규화를 먼저 시도하고, 그래도 부족할 때 파티셔닝을 검토한다. 테이블이 크다는 이유만으로 파티셔닝을 적용하면, 파티션 키를 포함하지 않는 쿼리의 성능이 오히려 저하될 수 있다.

InnoDB에서 파티셔닝을 사용할 때 PK가 파티션 키를 포함해야 한다는 제약이 가장 큰 걸림돌이다:

```sql
-- 이 구조는 에러 발생
CREATE TABLE logs (
    log_id      BIGINT PRIMARY KEY AUTO_INCREMENT,
    created_at  DATE NOT NULL
)
PARTITION BY RANGE (YEAR(created_at)) (...);
-- ERROR: A PRIMARY KEY must include all columns in the table's partitioning function

-- PK에 파티션 키를 포함해야 함
CREATE TABLE logs (
    log_id      BIGINT AUTO_INCREMENT,
    created_at  DATE NOT NULL,
    PRIMARY KEY (log_id, created_at)
)
PARTITION BY RANGE (YEAR(created_at)) (...);
```

이 제약으로 인해 `log_id`만으로는 행을 유일하게 식별할 수 없게 된다. 외부 시스템에서 `log_id`로 참조하던 로직이 있다면 문제가 될 수 있다. 파티셔닝 도입 전에 이런 영향을 충분히 검토해야 한다.
