# 커넥션과 메모리 관리

MySQL 서버의 성능 문제는 쿼리 자체가 아니라 커넥션 관리와 메모리 설정에서 비롯되는 경우가 많다. 커넥션이 과도하게 많으면 메모리 부족(OOM)이 발생하고, 메모리 설정이 부적절하면 디스크 I/O가 급증한다. 쿼리를 최적화하기 전에 서버 자원이 올바르게 구성되어 있는지 확인하는 것이 먼저다.

## 커넥션의 생명주기

MySQL 클라이언트가 서버에 접속하면 하나의 **커넥션**이 생성된다. 커넥션은 네 단계를 거친다.

### 1. 연결 수립

클라이언트가 TCP/IP(기본 포트 3306) 또는 Unix socket을 통해 서버에 접속을 요청한다. 서버는 이 요청을 수락하고 내부적으로 **thread**를 할당한다. MySQL은 커넥션당 하나의 thread를 사용하는 **thread-per-connection** 모델이다.

```sql
-- 현재 연결된 커넥션 확인
SHOW PROCESSLIST;

-- 더 상세한 정보
SELECT * FROM information_schema.PROCESSLIST;
```

### 2. 인증

서버는 클라이언트가 보낸 사용자명, 비밀번호, 접속 호스트를 `mysql.user` 테이블과 대조한다. 인증에 실패하면 `ERROR 1045 (28000): Access denied` 에러가 반환되고 커넥션이 종료된다.

인증이 성공하면 해당 사용자의 권한(privilege)이 확인되어 세션에 적용된다.

### 3. 쿼리 실행

인증을 통과한 커넥션은 SQL 문을 보내고 결과를 받는다. 하나의 커넥션에서 여러 쿼리를 순차적으로 실행할 수 있다. 쿼리를 실행하지 않는 동안 커넥션은 `Sleep` 상태가 된다.

```
+----+------+-----------+------+---------+------+-------+------+
| Id | User | Host      | db   | Command | Time | State | Info |
+----+------+-----------+------+---------+------+-------+------+
| 15 | app  | 10.0.0.5  | mydb | Sleep   |  120 |       | NULL |
+----+------+-----------+------+---------+------+-------+------+
```

`Time`은 현재 상태가 유지된 시간(초)이다. `Sleep` 상태에서 오래 머무는 커넥션은 connection pool에서 유지 중이거나, 애플리케이션이 커넥션을 닫지 않고 방치한 경우다.

### 4. 종료

클라이언트가 명시적으로 연결을 끊거나(`QUIT` 명령), `wait_timeout`에 설정된 시간 동안 아무 요청이 없으면 서버가 커넥션을 강제 종료한다.

```sql
-- 비활성 커넥션 타임아웃 (기본 28800초 = 8시간)
SELECT @@wait_timeout;

-- interactive 클라이언트(mysql CLI 등)의 타임아웃
SELECT @@interactive_timeout;
```

커넥션이 종료되면 할당된 thread와 세션 메모리가 반환된다.

## 커넥션 풀

매 요청마다 커넥션을 생성하고 종료하는 것은 비효율적이다. TCP 연결 수립, 인증, thread 할당에는 무시할 수 없는 비용이 든다. **커넥션 풀**(connection pool)은 미리 일정 수의 커넥션을 만들어 두고 재사용하여 이 비용을 제거한다.

### 동작 방식

1. 애플리케이션 시작 시 설정된 수만큼 커넥션을 미리 생성한다.
2. 쿼리가 필요할 때 풀에서 유휴(idle) 커넥션을 꺼낸다.
3. 쿼리가 끝나면 커넥션을 닫지 않고 풀에 반환한다.
4. 풀에 유휴 커넥션이 없으면 새 커넥션을 생성하거나, 최대 크기에 도달했으면 대기한다.

대부분의 웹 프레임워크와 ORM은 내장 커넥션 풀을 제공한다. HikariCP(Java), SQLAlchemy pool(Python), database/sql(Go) 등이 대표적이다.

### 커넥션 풀 크기 설정

커넥션 풀 크기는 "클수록 좋다"가 아니다. 동시에 활성 상태인 커넥션이 많아지면 CPU 컨텍스트 스위칭, 락 경합, 메모리 사용량이 증가한다.

일반적인 가이드라인은 다음과 같다.

```text
pool_size = CPU 코어 수 * 2 + 디스크 수
```

이 공식은 PostgreSQL 문서에서 유래한 경험적 값이지만 MySQL에도 적용할 수 있다. 4코어 서버에 SSD 1개라면 풀 크기 9~10이 출발점이다. 대부분의 웹 애플리케이션에서 커넥션 풀 크기는 10~30 사이면 충분하다.

## max_connections와 Too many connections

```sql
-- 최대 커넥션 수 확인 (기본 151)
SELECT @@max_connections;

-- 현재 사용 중인 커넥션 수
SHOW STATUS LIKE 'Threads_connected';

-- 서버 시작 이후 최대 동시 커넥션 수
SHOW STATUS LIKE 'Max_used_connections';
```

`Threads_connected`가 `max_connections`에 도달하면 새 커넥션 요청은 `ERROR 1040 (HY000): Too many connections` 에러를 받는다.

이 에러가 발생하면 대부분의 반응은 `max_connections`를 올리는 것이다. 하지만 이는 근본적인 해결이 아닌 경우가 많다.

### Too many connections의 실제 원인

**커넥션 누수**: 애플리케이션이 커넥션을 사용 후 반환하지 않는다. `SHOW PROCESSLIST`에서 `Sleep` 상태가 수백 초 이상인 커넥션이 대량으로 보이면 의심한다.

**느린 쿼리**: 쿼리 실행 시간이 길어지면 커넥션이 오래 점유된다. 동시 요청이 밀리면서 커넥션이 부족해진다.

**커넥션 풀 과다 설정**: 애플리케이션 서버가 여러 대이고 각각의 풀 크기가 크면, 전체 커넥션 수가 `max_connections`를 초과한다. 애플리케이션 서버 10대가 각각 풀 크기 50이면 최대 500개의 커넥션이 필요하다.

```sql
-- 커넥션이 부족할 때 긴급 접속용 (super 권한 필요)
-- max_connections + 1개의 예비 커넥션이 존재
mysql -u root -p
```

`max_connections`를 올리기 전에 `SHOW PROCESSLIST`로 현재 커넥션 상태를 확인하고, 불필요한 커넥션을 정리하는 것이 우선이다.

## 세션 메모리

MySQL은 각 커넥션(세션)에 독립적인 메모리 버퍼를 할당한다. 이 메모리는 해당 세션에서만 사용되며, 커넥션이 종료되면 반환된다.

### sort_buffer_size

`ORDER BY`나 `GROUP BY`로 정렬이 필요할 때 사용하는 버퍼다. 정렬할 데이터가 이 버퍼보다 크면 디스크 임시 파일을 사용하는 **filesort**가 발생한다.

```sql
-- 기본값: 256KB
SELECT @@sort_buffer_size;

-- 세션 단위 변경 (특정 쿼리에만 적용)
SET SESSION sort_buffer_size = 2 * 1024 * 1024;  -- 2MB
```

기본값 256KB로 대부분의 쿼리에 충분하다. 무작정 크게 설정하면 커넥션 수만큼 메모리가 곱해지므로 위험하다.

### join_buffer_size

인덱스를 사용하지 못하는 조인(Block Nested Loop 또는 Hash Join)에서 사용하는 버퍼다. `EXPLAIN`에서 `Using join buffer`가 나타나면 이 버퍼가 사용된 것이다.

```sql
-- 기본값: 256KB
SELECT @@join_buffer_size;
```

인덱스를 적절히 설계하면 이 버퍼가 사용되는 상황 자체를 줄일 수 있다. 버퍼 크기를 올리기 전에 조인 조건에 인덱스를 추가하는 것이 올바른 해결 방향이다.

### read_buffer_size, read_rnd_buffer_size

- **read_buffer_size**: full table scan 시 사용하는 버퍼 (기본 128KB)
- **read_rnd_buffer_size**: 정렬 후 정렬된 순서로 행을 읽을 때 사용하는 버퍼 (기본 256KB)

```sql
SELECT @@read_buffer_size;
SELECT @@read_rnd_buffer_size;
```

### tmp_table_size와 max_heap_table_size

`GROUP BY`, `DISTINCT`, `UNION` 등의 연산에서 중간 결과를 저장하기 위해 **내부 임시 테이블**이 생성될 수 있다. MySQL은 먼저 메모리 기반 임시 테이블(MEMORY 또는 TempTable 엔진)을 사용하다가, 크기 제한을 초과하면 디스크 기반 임시 테이블로 전환한다.

```sql
-- 메모리 임시 테이블의 최대 크기
SELECT @@tmp_table_size;        -- 기본 16MB
SELECT @@max_heap_table_size;   -- 기본 16MB
```

실제 적용되는 제한은 두 값 중 **작은 값**이다. `tmp_table_size`를 32MB로 올려도 `max_heap_table_size`가 16MB이면 16MB에서 디스크로 전환된다.

디스크 전환 빈도는 다음으로 확인한다.

```sql
SHOW STATUS LIKE 'Created_tmp_disk_tables';
SHOW STATUS LIKE 'Created_tmp_tables';
```

`Created_tmp_disk_tables / Created_tmp_tables` 비율이 높으면(10% 이상) 두 값을 함께 올리는 것을 고려한다. 단, 임시 테이블이 디스크로 넘어가는 다른 이유도 있다.

- 임시 테이블에 `TEXT`, `BLOB` 컬럼이 포함된 경우 (MEMORY 엔진은 이 타입을 지원하지 않음)
- MySQL 8.0의 TempTable 엔진에서는 `temptable_max_ram`(기본 1GB)이 별도 제한으로 존재

## 전역 메모리: innodb_buffer_pool_size

InnoDB에서 가장 중요한 메모리 설정이다. **buffer pool**은 테이블 데이터와 인덱스를 캐싱하는 메모리 영역이다. 데이터를 디스크에서 읽을 필요 없이 메모리에서 바로 반환할 수 있으므로, buffer pool이 클수록 디스크 I/O가 줄어든다.

```sql
-- 기본값: 128MB (대부분의 프로덕션 환경에서 너무 작음)
SELECT @@innodb_buffer_pool_size;
```

### 설정 전략

전용 데이터베이스 서버에서 buffer pool은 **전체 메모리의 70~80%**를 할당하는 것이 일반적이다.

- 서버 메모리 16GB → buffer pool 10~12GB
- 서버 메모리 64GB → buffer pool 45~50GB

나머지 20~30%는 운영체제, MySQL 서버 프로세스, 세션 메모리, 기타 전역 버퍼에 필요하다.

```sql
-- my.cnf에서 설정
-- [mysqld]
-- innodb_buffer_pool_size = 10G

-- 런타임 변경 (MySQL 5.7.5+)
SET GLOBAL innodb_buffer_pool_size = 10 * 1024 * 1024 * 1024;
```

### buffer pool 히트율 확인

buffer pool이 충분한지는 **히트율**(hit ratio)로 판단한다. 디스크에서 읽지 않고 buffer pool에서 바로 반환된 비율이다.

```sql
SHOW STATUS LIKE 'Innodb_buffer_pool_read_requests';   -- buffer pool에서 읽기 요청
SHOW STATUS LIKE 'Innodb_buffer_pool_reads';            -- 디스크에서 읽은 횟수
```

```text
히트율(%) = (1 - Innodb_buffer_pool_reads / Innodb_buffer_pool_read_requests) * 100
```

히트율이 **99% 이상**이면 양호하다. 95% 이하라면 buffer pool 증설을 검토한다. 단, 데이터 전체 크기가 buffer pool보다 훨씬 크고 접근 패턴이 랜덤한 경우에는 buffer pool을 아무리 올려도 히트율이 낮을 수 있다.

### buffer pool 인스턴스

buffer pool이 크면 내부 mutex 경합이 발생할 수 있다. `innodb_buffer_pool_instances`로 buffer pool을 여러 인스턴스로 분할하면 경합이 줄어든다.

```sql
-- buffer pool 1GB 이상일 때 인스턴스 분할 권장
-- [mysqld]
-- innodb_buffer_pool_size = 8G
-- innodb_buffer_pool_instances = 8
```

일반적으로 인스턴스당 1GB 이상이 되도록 설정한다. buffer pool이 8GB이면 8개 인스턴스가 적절하다.

## 메모리 사용량 계산

MySQL 서버의 전체 메모리 사용량은 크게 **전역 메모리**와 **세션 메모리 x 커넥션 수**로 나뉜다.

```text
전체 메모리 ≈ 전역 메모리 + (세션 메모리 × max_connections)
```

### 전역 메모리 (서버당 1개)

| 설정 | 기본값 | 설명 |
|---|---|---|
| innodb_buffer_pool_size | 128MB | 데이터와 인덱스 캐시 |
| innodb_log_buffer_size | 16MB | redo log 버퍼 |
| key_buffer_size | 8MB | MyISAM 인덱스 캐시 (InnoDB만 쓰면 최소화) |
| query_cache_size | 0 (8.0에서 제거) | 쿼리 캐시 |

### 세션 메모리 (커넥션당 할당)

| 설정 | 기본값 | 할당 시점 |
|---|---|---|
| sort_buffer_size | 256KB | ORDER BY 실행 시 |
| join_buffer_size | 256KB | 인덱스 없는 조인 시 |
| read_buffer_size | 128KB | full scan 시 |
| read_rnd_buffer_size | 256KB | 정렬 후 읽기 시 |
| tmp_table_size | 16MB | 임시 테이블 생성 시 |
| net_buffer_length | 16KB | 결과 전송 시 |
| thread_stack | 256KB~1MB | 항상 |

세션 메모리는 모든 커넥션이 항상 최대치를 사용하는 것은 아니다. `sort_buffer_size`는 정렬이 필요한 쿼리를 실행할 때만 할당된다. 하지만 **최악의 경우**를 기준으로 메모리가 충분한지 확인해야 한다.

### 계산 예시

```text
서버 메모리: 16GB
innodb_buffer_pool_size: 10GB
max_connections: 200
세션 메모리 최대치 (보수적 추정): 약 20MB

전역: 10GB + 약 200MB (기타 전역 버퍼)
세션: 200 × 20MB = 4GB (최악의 경우)
합계: 약 14.2GB

남은 메모리: 1.8GB (운영체제용)
```

이 계산에서 max_connections를 500으로 올리면 세션 메모리가 10GB로 증가하여 전체 합계가 20GB를 초과한다. 실제로 200개의 커넥션이 동시에 정렬과 조인을 수행할 가능성은 낮지만, 피크 시간에 발생하면 OOM killer가 MySQL 프로세스를 종료시킨다.

## OOM이 발생하는 시나리오

### 시나리오 1: max_connections 과다 설정

`max_connections`를 1000으로 올리고 `sort_buffer_size`를 4MB로 설정한 경우, 동시에 정렬 쿼리가 몰리면 세션 메모리만 4GB를 소비한다. buffer pool까지 합치면 물리 메모리를 초과할 수 있다.

### 시나리오 2: 긴 트랜잭션과 undo log

REPEATABLE READ에서 오래 실행되는 트랜잭션이 있으면 undo log를 정리할 수 없다. undo log가 계속 쌓이면서 메모리를 소비한다. `SHOW ENGINE INNODB STATUS`의 `History list length`가 수만 이상으로 증가하면 위험 신호다.

```sql
SHOW ENGINE INNODB STATUS\G
-- History list length 항목 확인
```

### 시나리오 3: 대량 결과셋

수백만 건의 결과를 한 번에 클라이언트로 보내는 쿼리는 net buffer에 결과를 적재하면서 메모리를 소비한다. 클라이언트가 결과를 소비하는 속도가 느리면 서버 측 메모리가 계속 쌓인다.

### OOM 방지 전략

```sql
-- 1. max_connections를 필요한 만큼만 설정
SET GLOBAL max_connections = 200;

-- 2. 세션 버퍼를 보수적으로 유지
-- sort_buffer_size, join_buffer_size는 기본값 유지
-- 특정 쿼리에만 세션 단위로 올리기

-- 3. wait_timeout을 줄여서 유휴 커넥션 정리
SET GLOBAL wait_timeout = 300;  -- 5분

-- 4. 커넥션 풀 크기를 적절히 제한
-- 애플리케이션 서버 수 × 풀 크기 ≤ max_connections
```

## 실전 메모리 설정 가이드

메모리 16GB 전용 데이터베이스 서버를 기준으로 한 설정 예시다.

```ini
[mysqld]
# 전역 메모리
innodb_buffer_pool_size = 10G
innodb_buffer_pool_instances = 8
innodb_log_buffer_size = 64M
key_buffer_size = 32M

# 세션 메모리 (기본값 유지 또는 소폭 조정)
sort_buffer_size = 256K
join_buffer_size = 256K
read_buffer_size = 128K
read_rnd_buffer_size = 256K
tmp_table_size = 64M
max_heap_table_size = 64M

# 커넥션
max_connections = 200
wait_timeout = 300
interactive_timeout = 3600
```

설정 변경 후에는 다음을 모니터링한다.

```sql
-- buffer pool 히트율
SHOW STATUS LIKE 'Innodb_buffer_pool%';

-- 임시 테이블 디스크 전환 비율
SHOW STATUS LIKE 'Created_tmp%tables';

-- 커넥션 사용량
SHOW STATUS LIKE 'Threads_connected';
SHOW STATUS LIKE 'Max_used_connections';

-- 정렬 관련
SHOW STATUS LIKE 'Sort_merge_passes';  -- 0이 아니면 sort_buffer가 부족할 수 있음
```

## 정리

- MySQL의 메모리는 모든 커넥션이 공유하는 전역 메모리와 커넥션별로 독립 할당되는 세션 메모리로 나뉜다.
- `innodb_buffer_pool_size`는 전체 메모리의 70~80%를 할당하되, 세션 메모리와 `max_connections`의 곱이 남은 메모리를 초과하지 않도록 한다.
- `max_connections`를 무작정 올리면 OOM의 원인이 된다. Too many connections 에러가 발생하면 먼저 커넥션 누수와 느린 쿼리를 점검한다.
- 커넥션 풀 크기는 `CPU 코어 수 * 2 + 디스크 수`가 출발점이며, 대부분의 웹 애플리케이션에서 10~30이면 충분하다.
- 세션 버퍼는 기본값을 유지하되 필요한 세션에서만 일시적으로 올리는 것이 안전하다.
