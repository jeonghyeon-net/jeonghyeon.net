# INSERT, UPDATE, DELETE

SELECT는 데이터를 읽는다. INSERT, UPDATE, DELETE는 데이터를 변경한다. DML(Data Manipulation Language)이라 부르는 이 세 가지 문은 단순히 "행을 쓰는" 것이 아니다. InnoDB 내부에서 redo log, undo log, buffer pool, 인덱스가 연쇄적으로 동작한다.

## INSERT의 내부 동작

```sql
INSERT INTO users (name, email) VALUES ('Alice', 'alice@example.com');
```

이 한 줄이 실행될 때 InnoDB 내부에서 일어나는 일:

1. **Undo log 기록**: 이 INSERT를 되돌리기 위한 정보를 undo log에 기록한다. INSERT의 undo는 DELETE다.
2. **Buffer pool에 페이지 적재**: 대상 테이블의 clustered index에서 삽입할 위치의 페이지를 buffer pool에 올린다. 이미 올라와 있으면 그대로 사용한다.
3. **페이지에 행 삽입**: buffer pool의 페이지에 새 행을 기록한다. 디스크에는 아직 쓰지 않는다.
4. **Redo log 기록**: 변경 내용을 redo log에 기록한다. 장애 시 복구에 사용된다.
5. **Secondary index 업데이트**: 테이블에 secondary index가 있으면, 해당 인덱스에도 새 항목을 추가한다. InnoDB는 change buffer를 사용하여 secondary index 업데이트를 지연시킬 수 있다.

디스크에 직접 쓰는 것이 아니라 buffer pool과 redo log에 기록한다는 것이 핵심이다. 실제 디스크 쓰기는 background thread가 비동기로 처리한다(checkpoint). 이 구조 덕분에 랜덤 I/O를 순차 I/O(redo log)로 변환할 수 있다.

## AUTO_INCREMENT

```sql
CREATE TABLE orders (
    id BIGINT AUTO_INCREMENT PRIMARY KEY,
    user_id INT NOT NULL,
    amount DECIMAL(10, 2)
);

INSERT INTO orders (user_id, amount) VALUES (1, 15000);
-- id = 1 자동 할당
INSERT INTO orders (user_id, amount) VALUES (2, 23000);
-- id = 2 자동 할당
```

AUTO_INCREMENT는 단순히 "1씩 증가하는 숫자"가 아니다. 내부 동작에 몇 가지 특성이 있다.

### 값의 연속성이 보장되지 않는다

```sql
START TRANSACTION;
INSERT INTO orders (user_id, amount) VALUES (1, 10000);
-- id = 3 할당
ROLLBACK;
-- id = 3은 사라지지만, 다음 INSERT는 id = 4부터 시작
```

AUTO_INCREMENT 값은 트랜잭션이 롤백되어도 되돌아가지 않는다. 값을 할당하는 시점과 트랜잭션을 커밋하는 시점이 다르기 때문이다. id가 1, 2, 4, 5처럼 중간에 빈 번호가 생길 수 있다. id의 연속성에 의존하는 로직은 틀린 로직이다.

### innodb_autoinc_lock_mode

AUTO_INCREMENT 값 할당 시 동시성을 제어하는 설정이다.

| 모드 | 값 | 동작 |
|---|---|---|
| Traditional | 0 | INSERT 문 전체에 테이블 레벨 락. 가장 느리지만 값이 예측 가능하다. |
| Consecutive | 1 | 단순 INSERT는 경량 뮤텍스, bulk INSERT는 테이블 레벨 락. MySQL 8.0 이전 기본값. |
| Interleaved | 2 | 모든 INSERT에 경량 뮤텍스. 가장 빠르지만 bulk INSERT에서 값이 연속적이지 않을 수 있다. MySQL 8.0 기본값. |

MySQL 8.0부터 기본값이 2(interleaved)다. statement-based replication을 사용하지 않는 한 이 설정이 가장 효율적이다.

## PK 순서와 INSERT 성능

InnoDB의 clustered index 구조를 떠올려야 한다. 테이블 데이터는 primary key 순서로 정렬되어 B-tree의 leaf node에 저장된다.

### 순차 PK (AUTO_INCREMENT)

AUTO_INCREMENT PK를 사용하면, 새 행은 항상 B-tree의 끝에 추가된다. 페이지가 가득 차면 새 페이지를 할당하고 이어서 쓴다. 순차 쓰기이므로 효율적이다.

### 랜덤 PK (UUID)

```sql
CREATE TABLE sessions (
    id CHAR(36) PRIMARY KEY,  -- UUID
    user_id INT,
    data TEXT
);
```

UUID를 PK로 사용하면, 새 행이 B-tree의 임의 위치에 삽입된다. 이미 다른 데이터로 가득 찬 페이지에 삽입해야 할 수 있다. 이때 page split이 발생한다. 하나의 페이지를 둘로 쪼개고 데이터를 재배치하는 작업이다.

page split의 비용:

- 새 페이지 할당
- 기존 페이지의 약 절반을 새 페이지로 복사
- 상위 B-tree node의 포인터 갱신
- 쪼개진 페이지들의 공간 활용률 저하 (약 50%만 사용)

대량 INSERT에서 랜덤 PK는 순차 PK 대비 수 배 느릴 수 있다. UUID를 PK로 사용해야 한다면 `UUID_TO_BIN(UUID(), 1)`처럼 시간 기반 정렬이 가능한 형태를 사용하거나, UUID v7처럼 시간순으로 정렬되는 형식을 선택한다.

## Bulk INSERT 최적화

### Multi-row INSERT

```sql
-- 느리다: 행마다 왕복
INSERT INTO logs (message) VALUES ('msg1');
INSERT INTO logs (message) VALUES ('msg2');
INSERT INTO logs (message) VALUES ('msg3');

-- 빠르다: 한 번에 전송
INSERT INTO logs (message) VALUES ('msg1'), ('msg2'), ('msg3');
```

단일 행 INSERT를 1,000번 실행하면 서버와 1,000번 왕복한다. multi-row INSERT는 한 번의 왕복으로 처리한다. 네트워크 비용뿐 아니라 파싱, 옵티마이저, 로그 기록 비용도 줄어든다.

단, 하나의 INSERT 문에 너무 많은 행을 넣으면 `max_allowed_packet`을 초과할 수 있다. 1,000~10,000행 단위로 나누는 것이 적절하다.

### LOAD DATA INFILE

```sql
LOAD DATA INFILE '/tmp/users.csv'
INTO TABLE users
FIELDS TERMINATED BY ','
LINES TERMINATED BY '\n'
(name, email, age);
```

파일에서 직접 데이터를 읽어 삽입한다. SQL 파싱을 생략하고, 내부적으로 최적화된 경로로 데이터를 적재한다. multi-row INSERT보다 빠르다. 대량 데이터 마이그레이션이나 초기 데이터 적재에 적합하다.

### Bulk INSERT 시 추가 최적화

```sql
-- 유니크 검사를 일시적으로 비활성화
SET unique_checks = 0;
-- 외래 키 검사를 일시적으로 비활성화
SET foreign_key_checks = 0;

-- bulk insert 실행
LOAD DATA INFILE ...;

-- 다시 활성화
SET unique_checks = 1;
SET foreign_key_checks = 1;
```

유니크 인덱스 검사와 외래 키 검사를 비활성화하면 삽입 속도가 향상된다. 데이터의 무결성이 보장되는 상황(정합성을 이미 검증한 데이터)에서만 사용해야 한다.

## UPDATE의 내부 동작

```sql
UPDATE employees SET salary = 60000 WHERE id = 1;
```

UPDATE는 내부적으로 "읽기 + 삭제 + 삽입"에 가깝다.

1. WHERE 조건으로 대상 행을 찾는다 (읽기).
2. 변경 전 값을 undo log에 기록한다 (롤백 및 MVCC용).
3. Buffer pool에서 해당 페이지의 행을 수정한다.
4. 변경 내용을 redo log에 기록한다.
5. 영향받는 secondary index를 갱신한다.

### 인덱스 업데이트 비용

UPDATE에서 가장 비용이 큰 부분은 인덱스 갱신이다. 변경된 컬럼에 인덱스가 걸려 있으면, 해당 인덱스에서 기존 값을 삭제하고 새 값을 삽입해야 한다.

```sql
-- email에 인덱스가 있는 경우
UPDATE users SET email = 'new@example.com' WHERE id = 1;
-- clustered index 갱신 + email 인덱스에서 기존 값 삭제 + 새 값 삽입
```

인덱스가 5개인 테이블에서 인덱스가 걸린 컬럼을 UPDATE하면, 최대 5개의 인덱스 트리를 수정해야 한다. 인덱스는 읽기를 빠르게 하지만 쓰기를 느리게 한다.

인덱스가 걸리지 않은 컬럼만 UPDATE하면 secondary index 갱신이 필요 없다. clustered index의 데이터만 수정하면 된다.

### 변경이 없는 UPDATE

```sql
UPDATE users SET name = 'Alice' WHERE id = 1;
-- name이 이미 'Alice'인 경우
```

InnoDB는 실제로 값이 변경되는지 확인한다. 현재 값과 새 값이 같으면 행을 수정하지 않는다. redo log, undo log 기록도 발생하지 않는다. `Rows matched: 1  Changed: 0`으로 확인할 수 있다.

## DELETE vs TRUNCATE

### DELETE

```sql
DELETE FROM logs WHERE created_at < '2023-01-01';
```

DELETE는 행 단위로 동작한다.

- 각 행마다 undo log를 기록한다 (롤백 가능).
- 각 행마다 redo log를 기록한다.
- 각 행의 secondary index 항목을 삭제한다.
- 실제로 디스크에서 데이터를 즉시 제거하지 않는다. 행에 삭제 표시(delete mark)를 한 뒤, purge thread가 나중에 정리한다.

DELETE는 트랜잭션 안에서 ROLLBACK이 가능하다.

### TRUNCATE

```sql
TRUNCATE TABLE logs;
```

TRUNCATE는 테이블의 모든 데이터를 제거한다. 내부적으로 테이블을 DROP하고 재생성하는 것에 가깝다.

- 행 단위 undo log를 기록하지 않는다.
- 행 단위 redo log를 기록하지 않는다.
- AUTO_INCREMENT 값이 초기화된다.
- WHERE 절을 사용할 수 없다. 전체 삭제만 가능하다.
- 트랜잭션 안에서 ROLLBACK이 되지 않는다 (DDL이므로 암묵적 COMMIT 발생).

| 항목 | DELETE | TRUNCATE |
|---|---|---|
| 대상 | 조건부 삭제 가능 | 전체만 |
| 속도 | 행 수에 비례하여 느림 | 거의 즉시 |
| ROLLBACK | 가능 | 불가 |
| AUTO_INCREMENT | 유지 | 초기화 |
| 트리거 | 실행됨 | 실행 안 됨 |
| 로그 | 행 단위 기록 | 최소한의 기록 |

100만 행 전체를 지울 때, DELETE는 100만 개의 undo/redo 기록을 남긴다. TRUNCATE는 테이블 구조를 재생성하므로 수 밀리초 내에 완료된다.

## INSERT ... ON DUPLICATE KEY UPDATE

PK 또는 유니크 인덱스에서 충돌이 발생하면 UPDATE를 실행하는 문법이다. "없으면 삽입, 있으면 갱신"(upsert) 패턴을 단일 문으로 처리한다.

```sql
CREATE TABLE page_views (
    page_url VARCHAR(255) PRIMARY KEY,
    view_count INT DEFAULT 0,
    last_viewed_at DATETIME
);

INSERT INTO page_views (page_url, view_count, last_viewed_at)
VALUES ('/about', 1, NOW())
ON DUPLICATE KEY UPDATE
    view_count = view_count + 1,
    last_viewed_at = NOW();
```

`/about`이 없으면 새 행이 삽입된다. 이미 있으면 `view_count`가 1 증가하고 `last_viewed_at`이 갱신된다.

`VALUES()` 함수로 INSERT에 지정한 값을 참조할 수 있다.

```sql
INSERT INTO inventory (product_id, quantity)
VALUES (101, 50)
ON DUPLICATE KEY UPDATE
    quantity = quantity + VALUES(quantity);
```

MySQL 8.0.19부터 `VALUES()` 대신 alias를 사용할 수 있다.

```sql
INSERT INTO inventory (product_id, quantity)
VALUES (101, 50) AS new
ON DUPLICATE KEY UPDATE
    quantity = quantity + new.quantity;
```

주의: ON DUPLICATE KEY UPDATE에서 AUTO_INCREMENT 값은 충돌 시에도 증가한다. 빈번한 충돌이 발생하면 AUTO_INCREMENT 값이 빠르게 소진될 수 있다.

## REPLACE

REPLACE는 충돌 시 기존 행을 DELETE한 뒤 새 행을 INSERT한다.

```sql
REPLACE INTO users (id, name, email)
VALUES (1, 'Alice', 'alice@new.com');
```

`id = 1`이 없으면 INSERT한다. 있으면 기존 행을 DELETE하고 새 행을 INSERT한다.

ON DUPLICATE KEY UPDATE와 달리, REPLACE는 기존 행을 완전히 삭제한다. 이로 인해 몇 가지 문제가 발생한다.

- DELETE + INSERT이므로 AUTO_INCREMENT 값이 바뀔 수 있다 (PK가 AUTO_INCREMENT가 아닌 다른 유니크 키에서 충돌하는 경우).
- 기존 행에 있던 다른 컬럼의 값이 사라진다. REPLACE에서 명시하지 않은 컬럼은 기본값으로 초기화된다.
- DELETE 트리거와 INSERT 트리거가 모두 실행된다.
- 외래 키로 참조되는 행이면 CASCADE DELETE가 발동될 수 있다.

대부분의 경우 REPLACE보다 INSERT ... ON DUPLICATE KEY UPDATE가 안전하다.

## 대량 데이터 변경 시 주의사항

### 한 번에 너무 많은 행을 변경하지 않는다

```sql
-- 위험: 100만 행을 한 트랜잭션에서 삭제
DELETE FROM logs WHERE created_at < '2023-01-01';
```

100만 행을 한 번에 DELETE하면:

- undo log가 100만 행분 누적된다. 이 트랜잭션이 실행되는 동안 시작된 다른 트랜잭션은 MVCC를 위해 이 undo log를 참조해야 하므로, purge가 지연된다.
- 장시간 락을 유지하여 다른 트랜잭션을 블로킹할 수 있다.
- 롤백 시 100만 행의 undo를 적용해야 하므로 롤백 자체도 오래 걸린다.

배치로 나눠서 처리한다.

```sql
-- 안전: 1만 행씩 나눠서 삭제
DELETE FROM logs WHERE created_at < '2023-01-01' LIMIT 10000;
-- 영향받은 행이 0이 될 때까지 반복
```

각 배치 사이에 짧은 간격을 두면 다른 트랜잭션이 실행될 여유를 준다. replication 환경에서는 배치 크기를 더 작게 잡아야 replica lag을 줄일 수 있다.

### UPDATE도 마찬가지다

```sql
-- 위험: 전체 행 갱신
UPDATE products SET price = price * 1.1;

-- 안전: 배치 처리
UPDATE products SET price = price * 1.1
WHERE id BETWEEN 1 AND 10000;
-- 다음 배치: WHERE id BETWEEN 10001 AND 20000
```

### 인덱스와 DML의 트레이드오프

인덱스는 SELECT를 빠르게 하지만, INSERT/UPDATE/DELETE를 느리게 한다. 테이블에 인덱스가 N개 있으면:

- INSERT: N개 인덱스에 모두 새 항목을 추가해야 한다.
- DELETE: N개 인덱스에서 모두 해당 항목을 삭제 표시해야 한다.
- UPDATE: 변경된 컬럼에 걸린 인덱스마다 기존 항목 삭제 + 새 항목 추가.

쓰기 비중이 높은 테이블에서는 불필요한 인덱스를 제거하는 것만으로도 성능이 크게 개선될 수 있다.

## 정리

- INSERT는 buffer pool과 redo log에 기록하며, 실제 디스크 쓰기는 비동기로 처리된다.
- AUTO_INCREMENT 값은 롤백되어도 되돌아가지 않으며, 연속성이 보장되지 않는다.
- 랜덤 PK(UUID)는 페이지 분할을 유발하여 순차 PK 대비 삽입 성능이 크게 저하된다.
- 대량 데이터 변경은 배치로 나누어 처리해야 undo log 누적과 락 경합을 방지할 수 있다.
- REPLACE보다 INSERT ... ON DUPLICATE KEY UPDATE가 안전하며, TRUNCATE는 DDL이므로 롤백이 불가능하다.
