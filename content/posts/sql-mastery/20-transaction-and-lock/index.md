# 트랜잭션과 락

데이터베이스에서 여러 작업을 하나의 논리적 단위로 묶어야 할 때 트랜잭션을 사용한다. 계좌 이체에서 출금과 입금이 둘 다 성공하거나 둘 다 실패해야 하는 것이 대표적인 예다. InnoDB는 트랜잭션을 지원하면서 동시에 여러 클라이언트가 같은 데이터에 접근할 수 있도록 락 메커니즘을 제공한다.

## ACID 원칙

트랜잭션이 보장해야 하는 네 가지 속성이다.

### Atomicity (원자성)

트랜잭션 내의 모든 작업은 전부 성공하거나 전부 실패한다. 중간 상태는 없다.

InnoDB는 **undo log**로 원자성을 구현한다. 트랜잭션이 데이터를 변경하기 전에 원래 값을 undo log에 기록한다. ROLLBACK이 발생하면 undo log를 사용하여 변경 전 상태로 되돌린다.

### Consistency (일관성)

트랜잭션 실행 전과 후에 데이터베이스가 일관된 상태를 유지한다. 외래 키 제약, NOT NULL 제약, CHECK 제약 등을 위반하는 변경은 거부된다.

일관성은 데이터베이스만의 책임이 아니다. "계좌 잔액은 음수가 되면 안 된다"는 규칙은 애플리케이션이 보장해야 할 수도 있다. 데이터베이스는 제약 조건으로 정의된 규칙만 강제한다.

### Isolation (격리성)

동시에 실행되는 트랜잭션들이 서로의 중간 상태를 볼 수 없다. 각 트랜잭션은 마치 혼자 실행되는 것처럼 동작한다.

InnoDB는 **MVCC**(Multi-Version Concurrency Control)와 **락**으로 격리성을 구현한다. MVCC는 데이터의 여러 버전을 유지하여 읽기 작업이 쓰기 작업을 차단하지 않게 한다. 격리 수준에 따라 다른 트랜잭션의 변경이 보이는 시점이 달라진다.

### Durability (지속성)

커밋된 트랜잭션의 변경은 시스템 장애가 발생해도 유실되지 않는다.

InnoDB는 **redo log**(WAL, Write-Ahead Logging)로 지속성을 구현한다. 데이터를 변경하기 전에 redo log에 변경 내용을 기록하고, 이 로그가 디스크에 기록된 후에 커밋을 완료한다. 서버가 비정상 종료되면 재시작 시 redo log를 재생하여 커밋된 변경을 복구한다.

## 트랜잭션의 생명주기

### 기본 사용법

```sql
-- 트랜잭션 시작
BEGIN;
-- 또는 START TRANSACTION;

-- 작업 수행
UPDATE accounts SET balance = balance - 10000 WHERE id = 1;
UPDATE accounts SET balance = balance + 10000 WHERE id = 2;

-- 성공: 변경 확정
COMMIT;

-- 또는 실패: 변경 취소
ROLLBACK;
```

`BEGIN`으로 트랜잭션을 시작하고, `COMMIT`으로 확정하거나 `ROLLBACK`으로 취소한다.

### autocommit

MySQL은 기본적으로 **autocommit** 모드가 켜져 있다. 각 SQL 문이 자동으로 하나의 트랜잭션으로 처리되어 즉시 커밋된다.

```sql
-- autocommit 상태 확인
SHOW VARIABLES LIKE 'autocommit';
-- +---------------+-------+
-- | Variable_name | Value |
-- +---------------+-------+
-- | autocommit    | ON    |
-- +---------------+-------+

-- autocommit이 ON이면 이 UPDATE는 즉시 커밋됨
UPDATE users SET name = 'Alice' WHERE id = 1;
```

`BEGIN`을 실행하면 autocommit이 일시적으로 비활성화되고, `COMMIT` 또는 `ROLLBACK`까지 하나의 트랜잭션으로 묶인다.

### 트랜잭션을 짧게 유지하기

트랜잭션이 길어지면 여러 문제가 발생한다.

- 락을 오래 잡고 있어 다른 트랜잭션이 대기한다.
- undo log가 커져 메모리를 많이 사용한다.
- MVCC에서 오래된 데이터 버전을 유지해야 하므로 성능이 저하된다.

트랜잭션 내에서 외부 API 호출이나 사용자 입력 대기 같은 긴 작업을 포함하면 안 된다. 필요한 데이터를 먼저 조회하고, 외부 처리를 마친 뒤, 최종 변경만 트랜잭션으로 처리한다.

## InnoDB의 락 종류

### 행 수준 락 (row-level lock)

InnoDB는 행 수준에서 락을 건다. 정확히는 행 자체가 아니라 **인덱스 레코드**에 락을 건다. 이 점이 InnoDB 락의 핵심 특성이다.

### 공유 락 (S lock)과 배타 락 (X lock)

- **공유 락(S lock, shared lock)**: 행을 읽기 위한 락이다. 여러 트랜잭션이 동시에 같은 행에 S lock을 걸 수 있다.
- **배타 락(X lock, exclusive lock)**: 행을 수정하기 위한 락이다. X lock이 걸린 행에는 다른 트랜잭션이 S lock이든 X lock이든 걸 수 없다.

| | S lock 보유 | X lock 보유 |
|---|---|---|
| S lock 요청 | 허용 | 대기 |
| X lock 요청 | 대기 | 대기 |

```sql
-- S lock: 읽기 락 (SELECT ... FOR SHARE)
SELECT * FROM accounts WHERE id = 1 FOR SHARE;

-- X lock: 쓰기 락 (SELECT ... FOR UPDATE)
SELECT * FROM accounts WHERE id = 1 FOR UPDATE;

-- UPDATE, DELETE는 자동으로 X lock
UPDATE accounts SET balance = balance - 10000 WHERE id = 1;
```

일반 `SELECT`는 MVCC를 사용하여 락을 걸지 않고 데이터를 읽는다. 락이 필요한 읽기가 필요할 때만 `FOR SHARE`나 `FOR UPDATE`를 명시한다.

### 인텐션 락 (intention lock)

테이블 수준의 락으로, 행 수준 락의 의도를 표시한다.

- **IS lock (intention shared)**: 테이블의 특정 행에 S lock을 걸겠다는 의도.
- **IX lock (intention exclusive)**: 테이블의 특정 행에 X lock을 걸겠다는 의도.

인텐션 락은 테이블 전체에 락을 걸려는 작업(`LOCK TABLES`, `ALTER TABLE`)과 행 수준 락 사이의 충돌을 효율적으로 감지하기 위해 존재한다. 행 수준 락끼리는 인텐션 락이 충돌하지 않으므로, 일반적인 트랜잭션 처리에서는 성능에 영향을 미치지 않는다.

## 갭 락과 넥스트키 락

### 갭 락 (gap lock)

인덱스 레코드 사이의 **간격**에 거는 락이다. 해당 간격에 새로운 행이 삽입되는 것을 방지한다.

```sql
-- 인덱스: (age)
-- 현재 데이터: age = 10, 20, 30이 있다고 가정

BEGIN;
SELECT * FROM users WHERE age BETWEEN 15 AND 25 FOR UPDATE;
```

이 쿼리는 `age = 20`인 행에 레코드 락을 걸고, `(10, 20)`과 `(20, 30)` 사이의 간격에 갭 락을 건다. 다른 트랜잭션이 `age = 12`나 `age = 22`인 행을 삽입하려 하면 대기한다.

### 넥스트키 락 (next-key lock)

**레코드 락 + 해당 레코드 앞의 갭 락**을 합친 것이다. InnoDB의 기본 락 단위다.

```sql
-- age = 10, 20, 30이 존재할 때
-- age = 20에 대한 넥스트키 락은 (10, 20] 범위를 잠근다
```

넥스트키 락은 `(이전 레코드, 현재 레코드]` 구간을 잠근다. 이전 레코드와 현재 레코드 사이의 갭과 현재 레코드를 함께 보호한다.

### 왜 갭 락이 필요한가

갭 락은 **phantom read**를 방지하기 위해 존재한다. phantom read란 같은 쿼리를 두 번 실행했을 때, 다른 트랜잭션이 삽입한 행이 새로 보이는 현상이다.

```
트랜잭션 A                          트랜잭션 B
────────────────────────────────────────────────────
BEGIN;
SELECT * FROM users
WHERE age BETWEEN 20 AND 30;
-- 결과: 2행
                                    BEGIN;
                                    INSERT INTO users (age) VALUES (25);
                                    COMMIT;

SELECT * FROM users
WHERE age BETWEEN 20 AND 30;
-- 갭 락 없으면: 3행 (phantom read)
-- 갭 락 있으면: B의 INSERT가 대기하므로 여전히 2행
```

REPEATABLE READ 격리 수준에서 InnoDB는 넥스트키 락을 사용하여 phantom read를 방지한다. READ COMMITTED 격리 수준에서는 갭 락이 비활성화된다.

## 데드락

### 발생 조건

두 개 이상의 트랜잭션이 서로 상대방이 보유한 락을 기다리는 상태다.

```
트랜잭션 A                          트랜잭션 B
────────────────────────────────────────────────────
BEGIN;                              BEGIN;
UPDATE accounts SET balance = 0
WHERE id = 1;
-- id=1에 X lock 획득
                                    UPDATE accounts SET balance = 0
                                    WHERE id = 2;
                                    -- id=2에 X lock 획득

UPDATE accounts SET balance = 0
WHERE id = 2;
-- id=2의 X lock 대기 (B가 보유)
                                    UPDATE accounts SET balance = 0
                                    WHERE id = 1;
                                    -- id=1의 X lock 대기 (A가 보유)
                                    -- 데드락 발생!
```

### 데드락 탐지

InnoDB는 **wait-for graph**(대기 그래프)를 사용하여 데드락을 실시간으로 탐지한다. 데드락이 감지되면 두 트랜잭션 중 하나를 선택하여 강제로 롤백한다. 일반적으로 undo log 양이 적은(변경이 적은) 트랜잭션이 롤백 대상이 된다.

롤백된 트랜잭션은 다음과 같은 에러를 받는다.

```
ERROR 1213 (40001): Deadlock found when trying to get lock; try restarting transaction
```

### SHOW ENGINE INNODB STATUS로 데드락 분석

가장 최근에 발생한 데드락의 상세 정보를 확인할 수 있다.

```sql
SHOW ENGINE INNODB STATUS\G
```

출력의 `LATEST DETECTED DEADLOCK` 섹션에서 데드락 정보를 확인한다.

```
------------------------
LATEST DETECTED DEADLOCK
------------------------
*** (1) TRANSACTION:
TRANSACTION 842934, ACTIVE 0 sec starting index read
mysql tables in use 1, locked 1
LOCK WAIT 3 lock struct(s), heap size 1136, 2 row lock(s)
MySQL thread id 10, OS thread handle 140234, query id 5678
UPDATE accounts SET balance = 0 WHERE id = 2

*** (1) HOLDS THE LOCK(S):
RECORD LOCKS space id 58 page no 3 n bits 72
index PRIMARY of table `mydb`.`accounts` trx id 842934 lock_mode X locks rec but not gap
Record lock, heap no 2 PHYSICAL RECORD: n_fields 5; ...

*** (1) WAITING FOR THIS LOCK TO BE GRANTED:
RECORD LOCKS space id 58 page no 3 n bits 72
index PRIMARY of table `mydb`.`accounts` trx id 842934 lock_mode X locks rec but not gap waiting
Record lock, heap no 3 PHYSICAL RECORD: n_fields 5; ...

*** (2) TRANSACTION:
TRANSACTION 842935, ACTIVE 0 sec starting index read
...

*** WE ROLL BACK TRANSACTION (2)
```

이 출력에서 확인할 수 있는 정보는 다음과 같다.

- 각 트랜잭션이 실행한 쿼리
- 각 트랜잭션이 보유한 락(HOLDS THE LOCK)
- 각 트랜잭션이 대기 중인 락(WAITING FOR THIS LOCK)
- 어떤 트랜잭션이 롤백되었는지(WE ROLL BACK TRANSACTION)

### 데드락 회피 전략

데드락을 완전히 제거하는 것은 불가능하지만, 발생 빈도를 줄일 수 있다.

**1. 일관된 접근 순서**: 여러 행을 수정할 때 항상 같은 순서로 접근한다.

```sql
-- 나쁜 예: 트랜잭션마다 접근 순서가 다름
-- 트랜잭션 A: id=1 → id=2
-- 트랜잭션 B: id=2 → id=1

-- 좋은 예: 항상 id 오름차순으로 접근
-- 트랜잭션 A: id=1 → id=2
-- 트랜잭션 B: id=1 → id=2
```

**2. 트랜잭션을 짧게 유지한다.** 락을 보유하는 시간이 짧을수록 다른 트랜잭션과 충돌할 확률이 줄어든다.

**3. 필요한 락만 건다.** `SELECT ... FOR UPDATE`를 꼭 필요한 경우에만 사용한다. 일반 SELECT로 충분한 상황에서 불필요하게 락을 거는 것을 피한다.

**4. 인덱스를 활용한다.** 인덱스가 없으면 InnoDB는 테이블의 모든 행에 락을 걸 수 있다. 적절한 인덱스가 있으면 필요한 행에만 락이 걸린다.

## 락 대기와 타임아웃

트랜잭션이 락을 기다리는 최대 시간은 `innodb_lock_wait_timeout`으로 설정한다. 기본값은 50초다.

```sql
SHOW VARIABLES LIKE 'innodb_lock_wait_timeout';
-- +---------------------------+-------+
-- | Variable_name             | Value |
-- +---------------------------+-------+
-- | innodb_lock_wait_timeout  | 50    |
-- +---------------------------+-------+
```

타임아웃이 발생하면 대기 중이던 **해당 SQL 문만** 롤백된다. 트랜잭션 전체가 롤백되는 것이 아니다. 트랜잭션의 나머지 부분은 그대로 유지되므로, 애플리케이션에서 에러를 감지하여 트랜잭션 전체를 롤백해야 한다.

```
ERROR 1205 (HY000): Lock wait timeout exceeded; try restarting transaction
```

현재 락을 대기 중인 트랜잭션을 확인하려면 `performance_schema`를 조회한다.

```sql
SELECT
    r.trx_id AS waiting_trx_id,
    r.trx_mysql_thread_id AS waiting_thread,
    r.trx_query AS waiting_query,
    b.trx_id AS blocking_trx_id,
    b.trx_mysql_thread_id AS blocking_thread,
    b.trx_query AS blocking_query
FROM performance_schema.data_lock_waits w
JOIN information_schema.innodb_trx r ON r.trx_id = w.REQUESTING_ENGINE_TRANSACTION_ID
JOIN information_schema.innodb_trx b ON b.trx_id = w.BLOCKING_ENGINE_TRANSACTION_ID;
```

이 쿼리로 어떤 트랜잭션이 어떤 트랜잭션을 차단하고 있는지, 각각 어떤 쿼리를 실행 중인지 확인할 수 있다.

## 실전에서 락 문제를 줄이는 패턴

### 낙관적 락 (optimistic locking)

데이터를 읽을 때 락을 걸지 않고, 수정할 때 데이터가 변경되지 않았는지 확인한다. 충돌이 드문 환경에서 효과적이다.

```sql
-- 1. 데이터 읽기 (락 없음)
SELECT id, name, version FROM products WHERE id = 1;
-- 결과: id=1, name='Widget', version=3

-- 2. 수정 시 version 확인
UPDATE products
SET name = 'Widget Pro', version = version + 1
WHERE id = 1 AND version = 3;

-- affected rows가 0이면 다른 트랜잭션이 먼저 수정한 것
-- 애플리케이션에서 재시도 로직 필요
```

`version` 컬럼으로 수정 시점에 충돌을 감지한다. `UPDATE`의 affected rows가 0이면 다른 트랜잭션이 먼저 수정한 것이므로, 데이터를 다시 읽고 재시도한다.

### SELECT FOR UPDATE 범위 최소화

```sql
-- 나쁜 예: 넓은 범위에 X lock
BEGIN;
SELECT * FROM orders WHERE status = 'pending' FOR UPDATE;
-- 수천 행에 락이 걸릴 수 있음

-- 좋은 예: 필요한 행만 락
BEGIN;
SELECT * FROM orders WHERE id = 12345 FOR UPDATE;
```

`FOR UPDATE`의 대상을 PK나 고유 인덱스로 한정하면 정확히 필요한 행에만 락이 걸린다.

### 배치 처리에서의 락 관리

대량의 행을 수정할 때 한 번에 처리하면 락이 오래 유지된다.

```sql
-- 나쁜 예: 10만 행에 한꺼번에 락
BEGIN;
UPDATE orders SET status = 'archived' WHERE created_at < '2024-01-01';
COMMIT;

-- 좋은 예: 1,000행씩 나누어 처리
-- 애플리케이션 코드에서 반복
UPDATE orders SET status = 'archived'
WHERE created_at < '2024-01-01' AND status != 'archived'
LIMIT 1000;
-- COMMIT 후 다음 배치
```

각 배치가 별도의 트랜잭션으로 처리되므로, 다른 트랜잭션이 그 사이에 작업할 수 있다. 전체 처리 시간은 길어질 수 있지만, 다른 작업의 대기 시간은 줄어든다.

## 정리

- InnoDB는 undo log로 원자성을, redo log로 지속성을, MVCC와 락으로 격리성을 구현한다.
- 행 수준 락은 행 자체가 아니라 인덱스 레코드에 걸린다. 인덱스가 없으면 테이블의 모든 행에 락이 걸릴 수 있다.
- 갭 락과 넥스트키 락은 REPEATABLE READ 격리 수준에서 phantom read를 방지한다.
- 데드락은 완전히 피할 수 없지만, 일관된 접근 순서, 짧은 트랜잭션, 필요한 최소 범위의 락으로 빈도를 줄일 수 있다.
- 낙관적 락은 충돌이 드문 환경에서 락 없이 동시성을 확보하는 방법이다. version 컬럼으로 수정 시점에 충돌을 감지한다.
