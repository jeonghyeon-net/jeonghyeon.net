# 격리 수준과 MVCC

트랜잭션은 ACID를 보장한다. 그중 Isolation(격리성)은 동시에 실행되는 트랜잭션이 서로의 중간 상태를 얼마나 볼 수 있는지를 결정한다. 격리 수준을 높이면 데이터 일관성은 좋아지지만 동시 처리 성능이 떨어진다. 반대로 낮추면 성능은 좋아지지만 의도하지 않은 데이터를 읽을 수 있다.

## 4가지 격리 수준

SQL 표준은 네 가지 격리 수준을 정의한다. 아래로 갈수록 격리가 강하다.

| 격리 수준 | dirty read | non-repeatable read | phantom read |
|---|---|---|---|
| READ UNCOMMITTED | 발생 | 발생 | 발생 |
| READ COMMITTED | 차단 | 발생 | 발생 |
| REPEATABLE READ | 차단 | 차단 | 발생 (InnoDB는 차단) |
| SERIALIZABLE | 차단 | 차단 | 차단 |

각 현상의 의미는 다음과 같다.

- **dirty read**: 다른 트랜잭션이 아직 커밋하지 않은 데이터를 읽는 현상이다. 해당 트랜잭션이 롤백하면 존재한 적 없는 데이터를 읽은 셈이 된다.
- **non-repeatable read**: 같은 트랜잭션 안에서 동일한 행을 두 번 읽었는데, 그 사이에 다른 트랜잭션이 해당 행을 수정하여 값이 달라지는 현상이다.
- **phantom read**: 같은 트랜잭션 안에서 동일한 조건으로 두 번 조회했는데, 그 사이에 다른 트랜잭션이 새로운 행을 삽입하거나 삭제하여 결과 집합이 달라지는 현상이다.

현재 세션의 격리 수준은 다음과 같이 확인하고 변경한다.

```sql
-- 현재 격리 수준 확인
SELECT @@transaction_isolation;

-- 세션 단위 변경
SET SESSION TRANSACTION ISOLATION LEVEL READ COMMITTED;

-- 전역 변경 (새 연결부터 적용)
SET GLOBAL TRANSACTION ISOLATION LEVEL READ COMMITTED;
```

## InnoDB의 기본 격리 수준: REPEATABLE READ

InnoDB의 기본 격리 수준은 REPEATABLE READ이다. 많은 다른 DBMS(PostgreSQL, Oracle, SQL Server)가 READ COMMITTED를 기본으로 사용하는 것과 다르다.

이유는 MySQL의 역사에 있다. MySQL의 replication은 초기에 **statement-based replication**(SBR)만 지원했다. SBR에서 READ COMMITTED를 사용하면, 마스터와 슬레이브 사이에 데이터 불일치가 발생할 수 있다. 같은 SQL 문이 실행 시점에 따라 다른 결과를 만들기 때문이다.

REPEATABLE READ에서는 트랜잭션 시작 시점의 스냅샷을 기준으로 읽기 때문에 이 문제가 완화된다. MySQL 5.1에서 **row-based replication**(RBR)이 도입된 이후에는 이 제약이 사라졌지만, 기본값은 그대로 유지되고 있다.

## MVCC 동작 원리

InnoDB는 **MVCC**(Multi-Version Concurrency Control)를 사용하여 읽기와 쓰기가 서로를 차단하지 않도록 한다. 핵심 아이디어는 단순하다. 데이터를 수정할 때 이전 버전을 유지하여, 읽기 트랜잭션이 수정 전 데이터를 볼 수 있게 하는 것이다.

### undo log와 버전 체인

InnoDB의 모든 행에는 두 개의 숨겨진 컬럼이 존재한다.

- **DB_TRX_ID**: 해당 행을 마지막으로 수정한 트랜잭션의 ID
- **DB_ROLL_PTR**: undo log에 저장된 이전 버전을 가리키는 포인터

행이 수정되면 다음과 같은 일이 일어난다.

1. 현재 행의 내용을 **undo log**에 복사한다.
2. 행을 새 값으로 갱신하고, `DB_TRX_ID`를 현재 트랜잭션 ID로 설정한다.
3. `DB_ROLL_PTR`이 undo log의 이전 버전을 가리키도록 설정한다.

행이 여러 번 수정되면 undo log에 버전이 체인 형태로 연결된다.

```
현재 행 (TRX_ID=200)
    │ DB_ROLL_PTR
    ▼
undo log 버전 (TRX_ID=150)
    │ DB_ROLL_PTR
    ▼
undo log 버전 (TRX_ID=100)
```

### ReadView와 consistent read

트랜잭션이 `SELECT`를 실행하면 InnoDB는 **ReadView**를 생성한다. ReadView에는 다음 정보가 담긴다.

- **m_ids**: ReadView 생성 시점에 아직 커밋되지 않은 활성 트랜잭션 ID 목록
- **m_low_limit_id**: ReadView 생성 시점에서 다음에 할당될 트랜잭션 ID (이 값 이상의 트랜잭션은 보이지 않는다)
- **m_up_limit_id**: 활성 트랜잭션 중 가장 작은 ID (이 값 미만의 트랜잭션은 모두 커밋 완료)
- **m_creator_trx_id**: ReadView를 생성한 트랜잭션 자신의 ID

행을 읽을 때 InnoDB는 해당 행의 `DB_TRX_ID`를 ReadView와 비교하여 가시성을 판단한다.

1. `DB_TRX_ID`가 `m_creator_trx_id`와 같으면 자신이 수정한 행이므로 보인다.
2. `DB_TRX_ID`가 `m_up_limit_id`보다 작으면 ReadView 생성 전에 커밋된 트랜잭션이므로 보인다.
3. `DB_TRX_ID`가 `m_low_limit_id` 이상이면 ReadView 생성 후 시작된 트랜잭션이므로 보이지 않는다.
4. `DB_TRX_ID`가 `m_ids` 목록에 있으면 아직 커밋되지 않은 트랜잭션이므로 보이지 않는다.
5. 보이지 않는 경우, `DB_ROLL_PTR`을 따라 undo log의 이전 버전으로 이동하여 같은 판단을 반복한다.

이 과정을 **consistent read**(일관된 읽기)라고 한다. 락을 잡지 않고 undo log를 참조하여 특정 시점의 데이터를 읽는 방식이다.

## READ COMMITTED vs REPEATABLE READ

두 격리 수준의 차이는 **ReadView를 언제 생성하느냐**에 있다.

- **READ COMMITTED**: 매 `SELECT` 문마다 새로운 ReadView를 생성한다.
- **REPEATABLE READ**: 트랜잭션의 첫 번째 `SELECT`에서 ReadView를 생성하고, 트랜잭션이 끝날 때까지 재사용한다.

다음 시나리오로 차이를 확인한다. `accounts` 테이블에 `balance = 1000`인 행이 하나 있다고 가정한다.

```
시점    트랜잭션 A (읽기)              트랜잭션 B (쓰기)
────    ─────────────────           ─────────────────
T1      BEGIN;
T2      SELECT balance
        FROM accounts
        WHERE id = 1;
        → 1000
T3                                  BEGIN;
T4                                  UPDATE accounts
                                    SET balance = 2000
                                    WHERE id = 1;
T5                                  COMMIT;
T6      SELECT balance
        FROM accounts
        WHERE id = 1;
        → ???
T7      COMMIT;
```

T6 시점의 결과가 격리 수준에 따라 달라진다.

- **READ COMMITTED**: T6에서 새 ReadView를 생성한다. 트랜잭션 B는 T5에서 이미 커밋되었으므로 변경이 보인다. 결과는 **2000**이다.
- **REPEATABLE READ**: T2에서 생성한 ReadView를 재사용한다. 트랜잭션 B는 이 ReadView 생성 시점에 존재하지 않았으므로(또는 활성 상태이므로) 변경이 보이지 않는다. 결과는 **1000**이다.

REPEATABLE READ에서는 트랜잭션이 진행되는 동안 읽는 데이터가 변하지 않으므로, 같은 쿼리를 여러 번 실행해도 같은 결과를 얻는다. 이것이 "repeatable read"라는 이름의 의미다.

### 주의: consistent read와 locking read의 차이

MVCC는 일반 `SELECT`에만 적용된다. `SELECT ... FOR UPDATE`나 `SELECT ... FOR SHARE` 같은 **locking read**는 undo log가 아니라 현재 최신 데이터를 읽고 락을 건다.

```sql
-- consistent read: undo log에서 스냅샷을 읽음
SELECT balance FROM accounts WHERE id = 1;

-- locking read: 최신 데이터를 읽고 배타 락을 걸음
SELECT balance FROM accounts WHERE id = 1 FOR UPDATE;
```

REPEATABLE READ에서 consistent read로는 이전 스냅샷이 보이지만, locking read로는 최신 데이터가 보인다. 이 차이를 인지하지 못하면 의도하지 않은 동작이 발생할 수 있다.

## 팬텀 리드와 넥스트키 락

SQL 표준에 따르면 REPEATABLE READ는 phantom read를 허용한다. 그러나 InnoDB는 **넥스트키 락**(next-key lock)을 사용하여 REPEATABLE READ에서도 phantom read를 방지한다.

### phantom read 시나리오

```
시점    트랜잭션 A                     트랜잭션 B
────    ─────────────────           ─────────────────
T1      BEGIN;
T2      SELECT * FROM users
        WHERE age BETWEEN 20 AND 30;
        → 3건
T3                                  BEGIN;
T4                                  INSERT INTO users
                                    (name, age)
                                    VALUES ('Kim', 25);
T5                                  COMMIT;
T6      SELECT * FROM users
        WHERE age BETWEEN 20 AND 30;
        → phantom read면 4건
T7      COMMIT;
```

READ COMMITTED에서는 T6에서 4건이 반환된다. 새로 삽입된 행이 보이기 때문이다.

### InnoDB의 넥스트키 락

InnoDB의 REPEATABLE READ에서 locking read(`SELECT ... FOR UPDATE`)나 `UPDATE`, `DELETE`를 실행하면, 조건에 해당하는 인덱스 레코드뿐 아니라 그 **사이의 간격**(gap)에도 락을 건다.

넥스트키 락은 **레코드 락**(record lock)과 **갭 락**(gap lock)의 결합이다.

- **레코드 락**: 특정 인덱스 레코드에 대한 락
- **갭 락**: 인덱스 레코드 사이의 빈 공간에 대한 락. 해당 범위에 새로운 행이 삽입되는 것을 방지한다.

`age` 컬럼에 인덱스가 있고 현재 값이 `[18, 22, 25, 28, 35]`인 경우, `WHERE age BETWEEN 20 AND 30`으로 locking read를 하면 다음 범위에 넥스트키 락이 걸린다.

```
(18, 22] — 22에 레코드 락 + (18, 22) 갭 락
(22, 25] — 25에 레코드 락 + (22, 25) 갭 락
(25, 28] — 28에 레코드 락 + (25, 28) 갭 락
(28, 35] — 35에 레코드 락 + (28, 35) 갭 락
```

이 범위에 `INSERT`를 시도하는 다른 트랜잭션은 갭 락에 의해 대기 상태에 들어간다. 결과적으로 REPEATABLE READ에서도 phantom read가 발생하지 않는다.

단, 일반 `SELECT`(consistent read)는 MVCC의 ReadView에 의해 팬텀 리드가 방지되므로 넥스트키 락과 무관하다. 넥스트키 락이 필요한 것은 locking read와 DML 문이다.

### 갭 락의 부작용

갭 락은 phantom read를 방지하지만 동시성을 저하시킨다. 존재하지 않는 행에 대한 조건으로 `SELECT ... FOR UPDATE`를 실행해도 갭 락이 걸릴 수 있다.

```sql
-- users 테이블에 id=5인 행이 없다고 가정
-- id가 3, 7인 행이 존재한다면

-- 트랜잭션 A
SELECT * FROM users WHERE id = 5 FOR UPDATE;
-- 결과: 0건이지만 (3, 7) 범위에 갭 락이 걸림

-- 트랜잭션 B
INSERT INTO users (id, name) VALUES (4, 'Lee');
-- 갭 락에 의해 대기
```

이런 특성 때문에 REPEATABLE READ에서 갭 락으로 인한 데드락이 발생하는 경우가 있다.

## 격리 수준 선택 기준

### READ COMMITTED를 선택하는 이유

실무에서 많은 서비스가 READ COMMITTED를 사용한다. 주요 이유는 다음과 같다.

**갭 락 회피**: READ COMMITTED에서는 갭 락이 사용되지 않는다. 갭 락으로 인한 데드락이 빈번한 환경에서 격리 수준을 READ COMMITTED로 낮추면 데드락이 크게 줄어든다.

**긴 트랜잭션의 undo log 부담 감소**: REPEATABLE READ에서는 트랜잭션이 살아 있는 동안 ReadView에 의해 참조되는 undo log를 정리(purge)할 수 없다. 긴 트랜잭션이 존재하면 undo log가 계속 쌓여 디스크 사용량이 증가하고 성능이 저하된다. READ COMMITTED에서는 매 `SELECT`마다 새 ReadView를 생성하므로 이전 undo log를 더 빨리 정리할 수 있다.

**다른 DBMS와의 일관성**: PostgreSQL, Oracle, SQL Server 등 대부분의 DBMS가 READ COMMITTED를 기본으로 사용한다. 애플리케이션이 여러 DBMS를 지원해야 하는 경우 동작을 맞추기 쉽다.

```sql
-- my.cnf에서 전역 설정
-- [mysqld]
-- transaction-isolation = READ-COMMITTED

-- 또는 런타임에 변경
SET GLOBAL TRANSACTION ISOLATION LEVEL READ COMMITTED;
```

READ COMMITTED로 변경할 때는 반드시 **binlog_format**을 `ROW`로 설정해야 한다. `STATEMENT` 형식에서 READ COMMITTED를 사용하면 복제 불일치가 발생할 수 있다.

```sql
-- binlog 형식 확인
SELECT @@binlog_format;

-- ROW 형식으로 변경 (my.cnf에서 설정 권장)
-- [mysqld]
-- binlog_format = ROW
```

### REPEATABLE READ를 유지하는 이유

REPEATABLE READ가 더 적합한 경우도 있다.

- 트랜잭션 내에서 **여러 번의 읽기가 동일한 결과**를 반환해야 하는 비즈니스 로직이 있는 경우
- 백업 도구(`mysqldump --single-transaction`)가 REPEATABLE READ의 consistent read를 활용하여 락 없이 일관된 백업을 생성하는 경우
- 갭 락으로 인한 데드락이 문제되지 않는 환경

### READ UNCOMMITTED과 SERIALIZABLE

READ UNCOMMITTED는 커밋되지 않은 데이터를 읽을 수 있으므로 데이터 무결성이 중요한 환경에서는 사용하지 않는다. 성능상의 이점도 READ COMMITTED 대비 거의 없다.

SERIALIZABLE은 모든 `SELECT`를 `SELECT ... FOR SHARE`로 변환한다. 읽기도 공유 락을 잡으므로 동시성이 크게 떨어진다. 매우 엄격한 데이터 정합성이 필요한 특수한 경우에만 고려한다.

## 실전에서 격리 수준 확인하기

격리 수준에 의한 동작 차이를 직접 확인하는 것이 가장 확실하다.

```sql
-- 테스트 테이블 생성
CREATE TABLE test_isolation (
    id INT PRIMARY KEY,
    val INT
);
INSERT INTO test_isolation VALUES (1, 100);
```

두 개의 터미널(세션)을 열어서 다음 순서로 실행한다.

```sql
-- 세션 1
SET SESSION TRANSACTION ISOLATION LEVEL REPEATABLE READ;
BEGIN;
SELECT val FROM test_isolation WHERE id = 1;  -- 100

-- 세션 2
UPDATE test_isolation SET val = 200 WHERE id = 1;

-- 세션 1
SELECT val FROM test_isolation WHERE id = 1;  -- 여전히 100 (REPEATABLE READ)
COMMIT;
SELECT val FROM test_isolation WHERE id = 1;  -- 200 (새 트랜잭션)
```

같은 시나리오를 READ COMMITTED로 바꾸면 두 번째 `SELECT`에서 200이 반환된다.

## 정리

격리 수준은 동시성과 일관성 사이의 트레이드오프를 결정한다. InnoDB는 MVCC를 통해 읽기와 쓰기가 서로를 차단하지 않으면서도 일관된 읽기를 보장한다. REPEATABLE READ와 READ COMMITTED의 차이는 ReadView 생성 시점이며, 넥스트키 락의 사용 여부가 동시성에 큰 영향을 미친다.

격리 수준을 변경하기 전에 애플리케이션의 동작을 충분히 테스트해야 한다. 특히 REPEATABLE READ에서 READ COMMITTED로 변경하면 같은 트랜잭션 안에서 읽는 데이터가 달라질 수 있으므로, 이를 전제로 작성된 로직이 없는지 확인한다.
