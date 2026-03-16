# PK 설계와 제약 조건

InnoDB에서 PK는 단순한 식별자가 아니다. clustered index의 키이며, 모든 secondary index가 PK 값을 포함한다. PK 선택이 테이블의 물리적 저장 구조와 전체 인덱스 성능을 결정한다.

## PK와 clustered index

InnoDB는 테이블 데이터를 clustered index(B+tree) 순서대로 저장한다. PK가 clustered index의 키다. 행 데이터가 PK 순서에 따라 물리적으로 정렬된다.

이 구조가 의미하는 것:

- PK 순서로 데이터를 읽으면 디스크 순차 접근이 가능하다.
- PK가 아닌 값으로 범위 조회하면 랜덤 I/O가 발생할 수 있다.
- 모든 secondary index의 leaf node에 PK 값이 저장된다. PK가 크면 모든 secondary index가 커진다.

PK를 명시하지 않으면 InnoDB가 내부적으로 6바이트 hidden row ID를 생성한다. 이 값은 사용자가 접근할 수 없으므로, 항상 PK를 명시적으로 정의하는 것이 좋다.

## AUTO_INCREMENT PK

가장 일반적인 PK 전략이다.

```sql
CREATE TABLE users (
    user_id  INT UNSIGNED PRIMARY KEY AUTO_INCREMENT,
    email    VARCHAR(255) NOT NULL,
    name     VARCHAR(100) NOT NULL
);
```

### 장점

**순차 삽입**: 새로운 행은 항상 B+tree의 끝에 추가된다. 이미 가득 찬 페이지를 분할할 필요가 없다. 쓰기 성능이 안정적이다.

**페이지 분할 최소화**: AUTO_INCREMENT 값은 항상 증가하므로, 기존 페이지의 중간에 데이터를 끼워넣는 일이 없다. 페이지 분할(page split)이 발생하면 하나의 INSERT에 여러 페이지를 수정해야 하고, 페이지 사용률도 낮아진다. AUTO_INCREMENT는 이 문제를 원천적으로 피한다.

**저장 크기**: INT는 4바이트, BIGINT는 8바이트다. PK 값이 모든 secondary index에 포함되므로, PK 크기가 작을수록 인덱스 전체 크기가 줄어든다.

**정렬 가능**: 생성 순서와 PK 순서가 일치하므로, `ORDER BY user_id`가 곧 생성순 정렬이다. 별도의 `created_at` 인덱스 없이 정렬이 가능하다.

### 단점

**분산 환경**: 여러 서버에서 AUTO_INCREMENT를 사용하면 충돌이 발생한다. `auto_increment_increment`와 `auto_increment_offset`으로 우회할 수 있지만, 서버 추가/제거 시 관리가 복잡하다.

**예측 가능성**: 외부에 노출되면 전체 행 수를 추측할 수 있다. 사용자 ID가 10,000이면 "약 1만 명의 사용자가 있구나"라고 추론할 수 있다. 보안이 중요한 경우 외부 노출용 식별자를 별도로 사용한다.

## UUID PK

분산 환경에서 ID 충돌 없이 고유 값을 생성하기 위해 UUID를 PK로 사용하는 경우가 있다.

```sql
CREATE TABLE orders (
    order_id  BINARY(16) PRIMARY KEY,  -- UUID를 바이너리로 저장
    user_id   INT UNSIGNED NOT NULL,
    total     DECIMAL(10,2) NOT NULL
);
```

### 문제점

**랜덤 삽입**: UUID v4는 완전히 랜덤한 값이다. 새로운 행이 B+tree의 임의 위치에 삽입된다. 이미 디스크에서 내려간 페이지를 다시 읽어와야 하고, 페이지 분할이 빈번하게 발생한다.

**페이지 분할 비용**: 랜덤 삽입으로 인한 페이지 분할은 두 가지 비용을 발생시킨다. 분할 자체의 I/O 비용과 분할 후 페이지 사용률 저하다. AUTO_INCREMENT에서 페이지가 거의 100% 채워지는 것과 달리, 랜덤 삽입에서는 페이지가 약 50~70%만 채워진다. 같은 데이터를 저장하는 데 더 많은 디스크 공간이 필요하다.

**저장 크기**: UUID는 16바이트(BINARY(16))다. INT(4바이트)의 4배, BIGINT(8바이트)의 2배다. 모든 secondary index에 이 16바이트가 포함되므로, secondary index 크기가 크게 증가한다.

**buffer pool 효율 저하**: 랜덤 삽입은 buffer pool의 다양한 페이지를 건드린다. 지역성(locality)이 없으므로 cache hit rate가 낮아진다.

### 대안: UUID v7 (시간 순서 UUID)

UUID v7은 타임스탬프 기반으로 생성되어 시간 순서가 보장된다. 랜덤 삽입 문제가 해소된다:

```sql
-- MySQL 8.0.13+에서 UUID를 바이너리로 변환
CREATE TABLE orders (
    order_id  BINARY(16) PRIMARY KEY,
    user_id   INT UNSIGNED NOT NULL
);

-- 삽입 시 UUID v7 사용 (애플리케이션에서 생성)
INSERT INTO orders (order_id, user_id)
VALUES (UUID_TO_BIN('019426a0-b1c2-7def-8abc-1234567890ab'), 1);
```

UUID v7은 AUTO_INCREMENT의 순차 삽입 장점을 유지하면서 분산 환경의 ID 충돌 문제를 해결한다. 다만 16바이트 크기 문제는 여전하다.

MySQL 8.0의 `UUID_TO_BIN()` 함수는 두 번째 인자로 `1`을 전달하면 시간 부분을 앞으로 재배치한다. UUID v1을 사용할 때 순차 삽입 효과를 얻을 수 있다:

```sql
INSERT INTO orders (order_id) VALUES (UUID_TO_BIN(UUID(), 1));
```

## 복합 PK

두 개 이상의 컬럼을 조합한 PK다. 관계 테이블(junction table)에서 주로 사용한다.

```sql
CREATE TABLE user_roles (
    user_id  INT UNSIGNED,
    role_id  INT UNSIGNED,
    PRIMARY KEY (user_id, role_id)
);
```

`user_id`와 `role_id`의 조합이 유일하다. 한 사용자에게 같은 역할이 중복 부여되지 않는다.

복합 PK에서 컬럼 순서가 중요하다. InnoDB가 `(user_id, role_id)` 순서로 clustered index를 구성하므로, `WHERE user_id = 42`는 인덱스를 효율적으로 사용하지만, `WHERE role_id = 5`만으로는 clustered index를 사용할 수 없다. 왼쪽 prefix 규칙이 적용된다.

```sql
-- 이 쿼리는 clustered index를 사용
SELECT * FROM user_roles WHERE user_id = 42;

-- 이 쿼리는 clustered index를 사용하지 못함. 별도 인덱스 필요
SELECT * FROM user_roles WHERE role_id = 5;
```

복합 PK를 사용할 때는 자주 조회하는 패턴에 맞춰 컬럼 순서를 결정한다.

## UNIQUE 제약 조건

UNIQUE 제약 조건을 설정하면 InnoDB가 내부적으로 unique index를 생성한다.

```sql
CREATE TABLE users (
    user_id  INT UNSIGNED PRIMARY KEY AUTO_INCREMENT,
    email    VARCHAR(255) NOT NULL,
    UNIQUE KEY uk_email (email)
);
```

`uk_email`은 secondary index이면서 동시에 유일성 제약이다. 중복된 이메일로 INSERT하면 에러가 발생한다:

```sql
INSERT INTO users (email) VALUES ('alice@example.com');
INSERT INTO users (email) VALUES ('alice@example.com');
-- ERROR 1062: Duplicate entry 'alice@example.com' for key 'uk_email'
```

UNIQUE 제약 조건은 NULL을 허용한다. InnoDB에서 NULL은 NULL과 같지 않으므로(`NULL != NULL`), UNIQUE 컬럼에 여러 개의 NULL이 존재할 수 있다:

```sql
CREATE TABLE profiles (
    profile_id  INT PRIMARY KEY AUTO_INCREMENT,
    phone       VARCHAR(20),
    UNIQUE KEY uk_phone (phone)
);

INSERT INTO profiles (phone) VALUES (NULL);
INSERT INTO profiles (phone) VALUES (NULL);  -- 성공. NULL은 중복으로 취급되지 않음
```

이 동작을 원하지 않으면 NOT NULL과 함께 사용한다.

## NOT NULL

NULL은 "값이 없음"을 표현하지만, 비용이 있다.

### NULL의 비용

**저장 비용**: InnoDB는 각 행에 NULL bitmap을 유지한다. nullable 컬럼이 하나라도 있으면 행마다 추가 바이트가 사용된다. 컬럼 수에 따라 1바이트(8개 컬럼까지), 2바이트(16개까지) 등으로 증가한다.

**인덱스 비용**: nullable 컬럼의 인덱스는 NULL 값도 포함한다. `IS NULL` 조건으로 검색할 수 있어야 하기 때문이다. 하지만 대부분의 쿼리에서 NULL 행은 의미가 없으므로, 인덱스 공간이 낭비된다.

**쿼리 복잡성**: NULL은 비교 연산에서 특이하게 동작한다. `NULL = NULL`은 FALSE이고, `NULL != 1`도 FALSE다. 집계 함수에서 NULL은 무시된다. 이런 동작을 올바르게 처리하려면 `IS NULL`, `IS NOT NULL`, `COALESCE`, `IFNULL` 등을 사용해야 한다.

```sql
-- 의도하지 않은 결과를 만드는 예
SELECT COUNT(phone) FROM users;           -- NULL은 세지 않음
SELECT COUNT(*) FROM users;               -- NULL 행도 셈
SELECT AVG(score) FROM students;          -- NULL은 평균 계산에서 제외
```

### 설계 원칙

가능하면 NOT NULL을 기본으로 사용한다. 값이 반드시 존재해야 하는 컬럼은 NOT NULL로 선언하고, 기본값을 설정한다:

```sql
CREATE TABLE articles (
    article_id   INT UNSIGNED PRIMARY KEY AUTO_INCREMENT,
    title        VARCHAR(200) NOT NULL,
    content      TEXT NOT NULL,
    view_count   INT NOT NULL DEFAULT 0,
    created_at   DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);
```

NULL이 의미 있는 경우에만 nullable로 둔다. 예를 들어, `deleted_at`은 삭제되지 않은 행에서 NULL이 "삭제되지 않았음"을 의미한다. 이 경우 NULL은 정보를 담고 있다.

## FK (Foreign Key)

FK 제약 조건은 참조 무결성(referential integrity)을 데이터베이스 레벨에서 보장한다.

```sql
CREATE TABLE orders (
    order_id    INT UNSIGNED PRIMARY KEY AUTO_INCREMENT,
    user_id     INT UNSIGNED NOT NULL,
    total       DECIMAL(10,2) NOT NULL,
    CONSTRAINT fk_orders_user
        FOREIGN KEY (user_id) REFERENCES users(user_id)
);
```

`orders.user_id`에 존재하지 않는 `users.user_id` 값을 넣으면 에러가 발생한다. `users`에서 참조되는 행을 삭제하려 해도 에러가 발생한다.

### FK의 동작 옵션

```sql
CONSTRAINT fk_orders_user
    FOREIGN KEY (user_id) REFERENCES users(user_id)
    ON DELETE CASCADE
    ON UPDATE CASCADE;
```

- `CASCADE`: 부모 행이 삭제/수정되면 자식 행도 함께 삭제/수정된다.
- `SET NULL`: 부모 행이 삭제/수정되면 자식의 FK 컬럼이 NULL로 설정된다.
- `RESTRICT` (기본값): 참조하는 자식 행이 있으면 부모 행의 삭제/수정을 거부한다.
- `NO ACTION`: MySQL에서는 RESTRICT와 동일하게 동작한다.

`CASCADE`는 편리하지만 위험할 수 있다. 부모 테이블에서 행 하나를 삭제했는데, 연쇄적으로 수천 건의 자식 행이 삭제될 수 있다. 의도치 않은 대량 삭제가 발생할 수 있으므로, 사용 전에 연쇄 범위를 충분히 파악해야 한다.

### FK의 성능 비용

FK 제약 조건은 공짜가 아니다:

**INSERT 비용**: 자식 테이블에 행을 추가할 때, 부모 테이블에 해당 값이 존재하는지 확인해야 한다. 부모 테이블의 인덱스를 조회하는 추가 비용이 발생한다.

**DELETE 비용**: 부모 테이블에서 행을 삭제할 때, 자식 테이블에 참조하는 행이 있는지 확인해야 한다. 자식 테이블의 FK 컬럼에 인덱스가 없으면 full table scan이 발생한다. InnoDB는 FK가 참조하는 컬럼에 자동으로 인덱스를 생성하지만, 명시적으로 확인하는 것이 좋다.

**락 비용**: FK 검사 시 부모 행에 shared lock이 걸린다. 동시에 많은 자식 행을 삽입하면 부모 행의 lock contention이 발생할 수 있다.

**대량 데이터 처리**: 데이터 마이그레이션이나 벌크 로드 시 FK 검사가 병목이 될 수 있다. 이 경우 임시로 FK 검사를 비활성화하기도 한다:

```sql
SET FOREIGN_KEY_CHECKS = 0;
-- 대량 데이터 로드
SET FOREIGN_KEY_CHECKS = 1;
```

### FK를 사용하지 않는 경우

많은 팀이 FK 제약 조건을 사용하지 않는다. 성능 부담, DDL 변경의 복잡성, 마이크로서비스 환경에서 DB 간 참조 불가 등이 이유다.

FK를 사용하지 않기로 했다면, 애플리케이션 레벨에서 참조 무결성을 보장해야 한다:

- 삽입 전에 부모 행의 존재를 확인하는 코드를 작성한다.
- 삭제 시 자식 행을 먼저 처리하는 로직을 구현한다.
- 고아 행(orphan row)이 발생하지 않는지 주기적으로 검증한다.
- 참조 관계를 문서화하여 팀이 인지할 수 있게 한다.

```sql
-- 고아 행 검출
SELECT o.order_id, o.user_id
FROM orders o
LEFT JOIN users u ON o.user_id = u.user_id
WHERE u.user_id IS NULL;
```

FK를 안 쓰는 것은 자유지만, 참조 무결성을 포기하는 것은 아니다. 보장 방식이 데이터베이스에서 애플리케이션으로 이동하는 것이다. 이 책임을 인지하지 못하면 데이터 정합성 문제가 시간이 지날수록 축적된다.

## CHECK 제약 조건

MySQL 8.0.16부터 CHECK 제약 조건이 실제로 동작한다. 이전 버전에서는 문법은 허용되었지만 무시되었다.

```sql
CREATE TABLE products (
    product_id  INT UNSIGNED PRIMARY KEY AUTO_INCREMENT,
    name        VARCHAR(200) NOT NULL,
    price       DECIMAL(10,2) NOT NULL,
    stock       INT NOT NULL DEFAULT 0,
    CONSTRAINT chk_price_positive CHECK (price > 0),
    CONSTRAINT chk_stock_nonneg CHECK (stock >= 0)
);
```

가격이 0 이하이거나 재고가 음수인 데이터의 삽입을 차단한다:

```sql
INSERT INTO products (name, price, stock) VALUES ('키보드', -1000, 10);
-- ERROR 3819: Check constraint 'chk_price_positive' is violated
```

CHECK 제약 조건은 단순한 유효성 검사에 적합하다. 복잡한 비즈니스 규칙은 애플리케이션 레벨에서 검증하는 것이 관리하기 쉽다. 다만 CHECK는 데이터베이스 레벨의 최후 방어선 역할을 한다. 어떤 경로로 데이터가 들어오든(API, 관리자 도구, 직접 SQL) 잘못된 값이 저장되는 것을 방지한다.

```sql
-- 날짜 범위 검증
ALTER TABLE events
ADD CONSTRAINT chk_date_range CHECK (end_date >= start_date);

-- ENUM 대체
ALTER TABLE orders
ADD CONSTRAINT chk_status CHECK (status IN ('pending', 'confirmed', 'shipped', 'delivered'));
```

CHECK에서 서브쿼리나 stored function 호출은 허용되지 않는다. 다른 테이블을 참조하는 제약은 FK나 트리거로 처리해야 한다.

## 정리

- InnoDB에서 PK는 clustered index의 키이며, 모든 secondary index가 PK 값을 포함하므로 PK 크기가 전체 인덱스 성능에 영향을 미친다.
- AUTO_INCREMENT 정수형 PK는 순차 삽입, 페이지 분할 최소화, 작은 저장 크기라는 이점이 있다.
- UUID를 PK로 사용해야 하면 UUID v7처럼 시간 순서가 보장되는 방식을 선택한다.
- NOT NULL을 기본으로 하고, FK는 참조 무결성을 보장하지만 성능 비용이 있으므로 팀 정책에 따라 결정한다.
- CHECK 제약 조건은 어떤 경로로 데이터가 들어오든 잘못된 값을 방지하는 마지막 방어선이다.
