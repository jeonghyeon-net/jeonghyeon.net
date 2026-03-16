# 스토리지 엔진과 InnoDB

MySQL에서 SQL을 처리하는 서버 레이어와 데이터를 실제로 저장하고 읽는 스토리지 엔진 레이어는 분리되어 있다. 스토리지 엔진은 데이터가 디스크에 어떻게 배치되고, 메모리에 어떻게 캐싱되며, 장애 시 어떻게 복구되는지를 결정한다. 같은 SQL이라도 스토리지 엔진에 따라 성능 특성이 완전히 달라진다.

## Pluggable Storage Engine

MySQL의 스토리지 엔진은 교체 가능하다. 서버 레이어는 handler API라는 추상 인터페이스를 통해 스토리지 엔진과 통신하기 때문에, 그 아래에 어떤 엔진이 붙든 서버 레이어의 동작은 동일하다.

테이블 단위로 스토리지 엔진을 지정할 수 있다:

```sql
-- InnoDB 테이블
CREATE TABLE orders (
    id BIGINT AUTO_INCREMENT PRIMARY KEY,
    user_id BIGINT NOT NULL,
    total DECIMAL(10, 2)
) ENGINE=InnoDB;

-- MEMORY 테이블 (서버 재시작 시 데이터 유실)
CREATE TABLE sessions (
    session_id CHAR(36) PRIMARY KEY,
    data VARCHAR(4096)
) ENGINE=MEMORY;
```

같은 데이터베이스 안에서 `orders`는 InnoDB로, `sessions`는 MEMORY 엔진으로 동작한다. 실무에서 이렇게 섞어 쓰는 경우는 드물지만, 아키텍처적으로는 가능하다.

주요 스토리지 엔진의 차이를 한눈에 보면:

| 특성 | InnoDB | MyISAM | MEMORY |
|------|--------|--------|--------|
| 트랜잭션 | 지원 | 미지원 | 미지원 |
| 행 수준 락 | 지원 | 테이블 락만 | 테이블 락만 |
| 외래 키 | 지원 | 미지원 | 미지원 |
| crash recovery | 지원 | 미지원 | 해당 없음 (메모리) |
| MVCC | 지원 | 미지원 | 미지원 |

현재 테이블이 어떤 스토리지 엔진을 사용하는지 확인하려면:

```sql
SHOW TABLE STATUS FROM mydb WHERE Name = 'orders'\G
```

```text
*************************** 1. row ***************************
           Name: orders
         Engine: InnoDB
        Version: 10
     Row_format: Dynamic
           Rows: 15234
 Avg_row_length: 78
    Data_length: 1196032
   Index_length: 0
      Data_free: 4194304
 Auto_increment: 15235
    Create_time: 2024-01-15 09:30:00
    Update_time: NULL
     Check_time: NULL
      Collation: utf8mb4_0900_ai_ci
       Checksum: NULL
 Create_options:
        Comment:
```

`Engine: InnoDB`가 현재 스토리지 엔진이다. `Rows`는 정확한 행 수가 아니라 추정값이다. InnoDB는 정확한 행 수를 유지하지 않는다. 정확한 수가 필요하면 `SELECT COUNT(*) FROM orders`를 실행해야 하며, 이 쿼리는 전체 테이블을 스캔한다.

## InnoDB가 기본인 이유

MySQL 5.5부터 InnoDB가 기본 스토리지 엔진이 되었다. 그 전에는 MyISAM이 기본이었다. 교체된 이유는 세 가지다.

### 트랜잭션

은행 계좌 이체를 생각한다. A 계좌에서 10만 원을 빼고 B 계좌에 10만 원을 넣는 작업은 반드시 둘 다 성공하거나 둘 다 실패해야 한다. A에서 빠졌는데 B에 넣기 전에 서버가 죽으면 10만 원이 사라진다.

```sql
START TRANSACTION;

UPDATE accounts SET balance = balance - 100000 WHERE id = 1;
UPDATE accounts SET balance = balance + 100000 WHERE id = 2;

COMMIT;
```

InnoDB는 이 두 `UPDATE`를 하나의 **트랜잭션**으로 묶어 원자적으로 처리한다. `COMMIT` 전에 장애가 발생하면 두 변경 모두 취소(**rollback**)된다. MyISAM에서는 트랜잭션이 불가능하다. 첫 번째 `UPDATE`가 반영된 후 서버가 죽으면 데이터 불일치가 발생한다.

### Crash Recovery

서버가 비정상 종료된 후 재시작했을 때, InnoDB는 자동으로 데이터를 일관된 상태로 복구한다. commit된 트랜잭션은 반드시 디스크에 반영되고, commit되지 않은 트랜잭션은 취소된다. 이 과정은 **redo log**와 **undo log**를 기반으로 동작한다. 별도의 관리자 개입 없이 자동으로 이루어진다.

MyISAM은 crash recovery를 지원하지 않는다. 비정상 종료 후 테이블이 손상되면 `REPAIR TABLE`을 수동으로 실행해야 한다. 대량의 데이터가 있는 테이블에서는 복구에 수 시간이 걸릴 수 있고, 데이터 유실 가능성도 있다.

### 행 수준 락 (Row-level Locking)

MyISAM은 데이터를 수정할 때 테이블 전체에 락을 건다. 한 사용자가 `UPDATE`를 실행하면 다른 모든 사용자는 그 테이블에 대한 `SELECT`까지 대기해야 한다. 동시 접속자가 많은 웹 애플리케이션에서는 치명적이다.

InnoDB는 변경 대상인 행에만 락을 건다. A 사용자가 `id = 1`인 행을 수정하는 동안 B 사용자는 `id = 2`인 행을 자유롭게 수정할 수 있다. 동시 처리량(throughput)이 극적으로 향상된다.

이 세 가지 — 트랜잭션, crash recovery, 행 수준 락 — 때문에 현재 MySQL에서 InnoDB 외의 스토리지 엔진을 선택할 이유는 거의 없다.

## InnoDB 아키텍처

InnoDB의 내부는 크게 메모리 영역과 디스크 영역으로 나뉜다.

```
┌──────────────────────────────────────────────┐
│                  메모리                       │
│                                              │
│  ┌────────────────────────────────────────┐  │
│  │           Buffer Pool                  │  │
│  │                                        │  │
│  │  ┌──────┐ ┌──────┐ ┌──────┐ ┌──────┐  │  │
│  │  │ 데이터│ │ 인덱스│ │ undo │ │ 적응형│  │  │
│  │  │ 페이지│ │ 페이지│ │ 페이지│ │ 해시 │  │  │
│  │  └──────┘ └──────┘ └──────┘ └──────┘  │  │
│  └────────────────────────────────────────┘  │
│                                              │
│  ┌──────────────┐  ┌──────────────────────┐  │
│  │ Log Buffer   │  │  Change Buffer       │  │
│  └──────────────┘  └──────────────────────┘  │
│                                              │
├──────────────────────────────────────────────┤
│                  디스크                       │
│                                              │
│  ┌────────────┐  ┌───────────┐  ┌─────────┐ │
│  │ 테이블스페이스│  │ Redo Log  │  │Undo Log │ │
│  │ (.ibd 파일) │  │           │  │         │ │
│  └────────────┘  └───────────┘  └─────────┘ │
└──────────────────────────────────────────────┘
```

### Buffer Pool

InnoDB에서 가장 중요한 메모리 구조다. 디스크에서 읽어온 데이터 페이지를 캐싱하는 공간이다.

데이터베이스의 성능 병목은 거의 항상 디스크 I/O다. SSD라 해도 메모리 접근 대비 수천 배 느리다. buffer pool은 자주 사용되는 데이터를 메모리에 유지하여 디스크 접근을 최소화한다.

buffer pool의 크기는 `innodb_buffer_pool_size` 파라미터로 설정한다:

```sql
SHOW VARIABLES LIKE 'innodb_buffer_pool_size';
```

```text
+-------------------------+------------+
| Variable_name           | Value      |
+-------------------------+------------+
| innodb_buffer_pool_size | 134217728  |
+-------------------------+------------+
```

값은 byte 단위다. 134,217,728 bytes = 128MB. 기본값이다. 프로덕션 환경에서 128MB는 거의 항상 부족하다. 일반적으로 가용 메모리의 70~80%를 buffer pool에 할당한다. 16GB 메모리 서버라면 10~12GB 정도를 설정하는 것이 일반적이다.

buffer pool의 현재 상태를 확인하려면:

```sql
SHOW ENGINE INNODB STATUS\G
```

출력의 `BUFFER POOL AND MEMORY` 섹션에서 핵심 지표를 볼 수 있다:

```text
----------------------
BUFFER POOL AND MEMORY
----------------------
Buffer pool size        8192
Free buffers            1024
Database pages          7100
Modified db pages       45
Buffer pool hit rate    998 / 1000
```

**Buffer pool hit rate**가 가장 중요하다. 1000 / 1000이면 모든 읽기 요청이 메모리에서 처리된 것이다. 998 / 1000이면 99.8%가 메모리 히트. 이 비율이 95% 아래로 떨어지면 buffer pool 크기를 늘려야 한다는 신호다.

buffer pool은 **LRU**(Least Recently Used) 알고리즘의 변형으로 페이지를 관리한다. 자주 접근하는 페이지는 메모리에 남고, 오래 사용하지 않은 페이지는 밀려난다. InnoDB는 전통적인 LRU를 그대로 쓰지 않고 young 영역과 old 영역으로 나눈 midpoint insertion 전략을 사용한다. 전체 테이블 스캔처럼 일시적으로 대량의 페이지를 읽는 작업이 자주 사용하는 페이지를 밀어내지 않도록 하기 위한 설계다.

### Redo Log

**Redo log**는 crash recovery의 핵심이다. InnoDB가 데이터를 변경할 때 일어나는 일을 순서대로 보면:

1. 변경할 데이터 페이지를 buffer pool로 읽어온다 (이미 있으면 생략).
2. buffer pool의 페이지를 수정한다 (이 시점에서 메모리의 데이터와 디스크의 데이터가 달라진다).
3. 변경 내용을 redo log에 기록한다.
4. 트랜잭션이 commit되면 redo log를 디스크에 flush한다.

여기서 중요한 점: **변경된 데이터 페이지는 즉시 디스크에 쓰이지 않는다.** buffer pool에만 반영되고, 디스크 쓰기는 나중에 비동기로 이루어진다. 이 전략을 **WAL**(Write-Ahead Logging)이라 한다. 로그를 먼저 쓰고, 데이터는 나중에 쓴다.

WAL이 효율적인 이유는 쓰기 패턴의 차이에 있다. 데이터 페이지는 디스크의 여러 위치에 흩어져 있어 임의 쓰기(random write)가 발생한다. redo log는 항상 끝에 추가하는 순차 쓰기(sequential write)다. 디스크는 순차 쓰기가 임의 쓰기보다 수십 배 빠르다. SSD에서도 이 차이는 유의미하다.

서버가 비정상 종료되면 어떻게 되는가? buffer pool에만 반영되고 아직 디스크에 쓰이지 않은 변경("dirty page")이 유실된다. 하지만 redo log는 commit 시점에 이미 디스크에 기록되어 있다. MySQL이 재시작되면 redo log를 읽어서 아직 데이터 파일에 반영되지 않은 변경을 다시 적용한다. 이 과정이 **crash recovery**다.

```sql
-- redo log 설정 확인
SHOW VARIABLES LIKE 'innodb_redo_log_capacity';
```

```text
+--------------------------+------------+
| Variable_name            | Value      |
+--------------------------+------------+
| innodb_redo_log_capacity | 104857600  |
+--------------------------+------------+
```

100MB. MySQL 8.0.30부터 `innodb_redo_log_capacity`로 redo log 전체 크기를 설정한다. 쓰기가 많은 워크로드에서는 이 값을 키우면 checkpoint 빈도가 줄어 성능이 향상된다.

### Undo Log

**Undo log**는 redo log의 반대 방향이다. redo log가 "무엇을 했는가"를 기록한다면, undo log는 "원래 무엇이었는가"를 기록한다.

undo log의 두 가지 역할:

**1. 트랜잭션 롤백**

```sql
START TRANSACTION;
UPDATE users SET name = 'Bob' WHERE id = 1;  -- 기존값 'Alice'가 undo log에 저장됨
ROLLBACK;  -- undo log를 읽어 'Alice'로 복원
```

`ROLLBACK`이 실행되면 InnoDB는 undo log에 저장된 이전 값을 읽어 데이터를 원래 상태로 되돌린다.

**2. MVCC (Multi-Version Concurrency Control)**

InnoDB는 하나의 행에 대해 여러 버전을 동시에 유지한다. 트랜잭션 A가 행을 수정하는 동안 트랜잭션 B가 같은 행을 읽으면, B는 A가 수정하기 전의 값을 undo log에서 읽는다. 락 없이 읽기가 가능한 이유다.

```sql
-- 트랜잭션 A
START TRANSACTION;
UPDATE users SET name = 'Bob' WHERE id = 1;
-- 아직 COMMIT하지 않음

-- 트랜잭션 B (동시에 실행)
SELECT name FROM users WHERE id = 1;
-- 결과: 'Alice' (A가 수정하기 전의 값)
```

트랜잭션 B는 대기하지 않는다. undo log에 저장된 이전 버전을 읽고 바로 반환한다. 이것이 **MVCC**의 핵심이다. 읽기와 쓰기가 서로를 차단하지 않는다.

## 디스크 I/O와 Buffer Pool의 관계

데이터베이스 성능의 대부분은 디스크 I/O에 의해 결정된다. 이를 수치로 확인한다.

메모리(DRAM) 접근: 약 100 나노초
SSD 랜덤 읽기: 약 100 마이크로초 (메모리 대비 1,000배 느림)
HDD 랜덤 읽기: 약 10 밀리초 (메모리 대비 100,000배 느림)

buffer pool이 충분히 크면 대부분의 읽기가 메모리에서 처리된다. buffer pool이 부족하면 매 쿼리마다 디스크 접근이 발생하고, 응답 시간이 극적으로 느려진다.

실무에서의 시나리오를 보면:

- **데이터 크기 < buffer pool 크기**: 전체 데이터가 메모리에 올라간다. 디스크 I/O가 거의 발생하지 않는다. 이 상태가 이상적이다.
- **데이터 크기 > buffer pool 크기**: 자주 접근하는 데이터(working set)만 메모리에 유지된다. working set이 buffer pool에 들어가면 성능 저하는 미미하다.
- **working set > buffer pool 크기**: 자주 접근하는 데이터조차 메모리에 다 담지 못한다. 페이지가 계속 교체(eviction)되면서 디스크 I/O가 급증한다. 이 상태가 되면 buffer pool 크기 증가 또는 데이터 접근 패턴 변경이 필요하다.

buffer pool 사용량을 모니터링하는 쿼리:

```sql
SELECT
    FORMAT(pages_data.data * 16 / 1024, 0) AS buffer_pool_data_MB,
    FORMAT(pages_free.free * 16 / 1024, 0) AS buffer_pool_free_MB,
    FORMAT(pages_dirty.dirty * 16 / 1024, 0) AS dirty_pages_MB
FROM (
    SELECT
        variable_value AS data
    FROM performance_schema.global_status
    WHERE variable_name = 'Innodb_buffer_pool_pages_data'
) pages_data,
(
    SELECT
        variable_value AS free
    FROM performance_schema.global_status
    WHERE variable_name = 'Innodb_buffer_pool_pages_free'
) pages_free,
(
    SELECT
        variable_value AS dirty
    FROM performance_schema.global_status
    WHERE variable_name = 'Innodb_buffer_pool_pages_dirty'
) pages_dirty;
```

`free`가 0에 가깝고 `dirty`가 높다면 buffer pool이 부족하다는 의미다.

## 페이지 단위 읽기/쓰기

InnoDB가 디스크에서 데이터를 읽거나 쓸 때의 최소 단위는 **페이지**(page)다. 기본 크기는 16KB다.

행 하나가 100 bytes라 해도 InnoDB는 그 행이 속한 16KB 페이지 전체를 읽는다. 1 byte만 수정해도 16KB 페이지 전체가 dirty 상태가 되어 나중에 디스크에 다시 쓰인다.

```sql
-- 페이지 크기 확인
SHOW VARIABLES LIKE 'innodb_page_size';
```

```text
+------------------+-------+
| Variable_name    | Value |
+------------------+-------+
| innodb_page_size | 16384 |
+------------------+-------+
```

16,384 bytes = 16KB. 이 값은 테이블스페이스 생성 시 결정되며 이후 변경할 수 없다.

페이지 단위 I/O가 의미하는 것:

- 인접한 행을 함께 읽을 때가 가장 효율적이다. `WHERE id BETWEEN 1 AND 100`처럼 범위 조건은 같은 페이지나 인접한 페이지에서 데이터를 읽을 가능성이 높다.
- 랜덤한 행을 개별적으로 읽으면 매번 다른 페이지를 읽어야 한다. 100개의 행이 100개의 서로 다른 페이지에 흩어져 있으면 100번의 페이지 읽기가 필요하다.

이 개념은 인덱스 설계, 쿼리 최적화, 테이블 설계 등 데이터베이스 성능의 거의 모든 측면에 영향을 미친다.

## 정리

MySQL의 스토리지 엔진은 pluggable 아키텍처로 교체가 가능하며, InnoDB가 기본이다. InnoDB가 기본인 이유는 트랜잭션, crash recovery, 행 수준 락을 지원하기 때문이다.

InnoDB의 핵심 구조는 세 가지다:

- **Buffer pool**: 디스크의 데이터 페이지를 메모리에 캐싱한다. 데이터베이스 성능의 핵심.
- **Redo log**: 변경 내용을 순차적으로 기록한다. crash recovery의 기반.
- **Undo log**: 변경 전 데이터를 보존한다. 롤백과 MVCC에 사용.

디스크 I/O가 데이터베이스 성능의 가장 큰 병목이며, buffer pool이 이를 완화한다. InnoDB는 16KB 페이지 단위로 데이터를 읽고 쓴다. 이 페이지 개념은 데이터베이스의 물리적 저장 구조를 이해하는 출발점이다.
