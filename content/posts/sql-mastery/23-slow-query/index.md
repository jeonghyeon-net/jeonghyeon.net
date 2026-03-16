# 느린 쿼리 추적과 해결

프로덕션 환경에서 성능 문제가 발생했을 때 "어떤 쿼리가 느린지"를 빠르게 파악하는 것이 첫 번째 단계다. MySQL은 느린 쿼리를 식별하고 분석하기 위한 여러 도구를 제공한다. slow query log로 문제 쿼리를 수집하고, Performance Schema로 실행 통계를 확인하고, EXPLAIN으로 원인을 분석하고, 최적화 후 효과를 검증하는 흐름을 익히면 대부분의 쿼리 성능 문제에 체계적으로 대응할 수 있다.

## Slow Query Log

slow query log는 설정된 시간(`long_query_time`)보다 오래 걸린 쿼리를 파일에 기록하는 기능이다.

### 설정

```sql
-- slow query log 활성화 여부 확인
SELECT @@slow_query_log;

-- 기록 기준 시간 (기본 10초)
SELECT @@long_query_time;

-- 로그 파일 경로
SELECT @@slow_query_log_file;
```

런타임에 활성화할 수 있다.

```sql
-- slow query log 켜기
SET GLOBAL slow_query_log = ON;

-- 기준 시간을 1초로 낮추기
SET GLOBAL long_query_time = 1;

-- 인덱스를 사용하지 않는 쿼리도 기록
SET GLOBAL log_queries_not_using_indexes = ON;
```

영구 적용은 `my.cnf`에서 설정한다.

```ini
[mysqld]
slow_query_log = ON
long_query_time = 1
slow_query_log_file = /var/log/mysql/slow.log
log_queries_not_using_indexes = ON
```

`long_query_time`의 적절한 값은 서비스에 따라 다르다. 웹 애플리케이션에서는 1초가 일반적이다. 초기 분석 단계에서는 0.5초나 0.1초로 낮추어 더 많은 쿼리를 수집하기도 한다. 값이 너무 낮으면 로그 파일이 급격히 커지므로 주의한다.

### 로그 형식

slow query log의 각 항목은 다음과 같은 형태다.

```
# Time: 2026-03-15T10:23:45.123456Z
# User@Host: app[app] @ 10.0.0.5 []  Id: 12345
# Query_time: 3.456789  Lock_time: 0.000123  Rows_sent: 1  Rows_examined: 1250000
SET timestamp=1742033025;
SELECT u.name, u.email
FROM users u
JOIN orders o ON u.id = o.user_id
WHERE o.created_at > '2026-01-01'
ORDER BY o.created_at DESC
LIMIT 10;
```

각 필드의 의미:

- **Query_time**: 쿼리 실행에 걸린 시간(초)
- **Lock_time**: 락을 기다린 시간
- **Rows_sent**: 클라이언트에 반환된 행 수
- **Rows_examined**: 쿼리 실행 중 검사한 행 수

`Rows_examined`와 `Rows_sent`의 비율이 핵심이다. 위 예시에서 125만 행을 검사하여 10행을 반환했다면, 인덱스가 없거나 부적절한 인덱스를 사용하고 있을 가능성이 높다.

## Slow Query Log 분석 도구

로그 파일을 직접 읽는 것은 비효율적이다. 분석 도구를 사용하여 패턴별로 집계하고 우선순위를 파악한다.

### mysqldumpslow

MySQL에 기본 포함된 분석 도구다.

```bash
# 총 실행 시간 기준 상위 10개 쿼리 패턴
mysqldumpslow -s t -t 10 /var/log/mysql/slow.log

# 실행 횟수 기준 정렬
mysqldumpslow -s c -t 10 /var/log/mysql/slow.log

# 평균 실행 시간 기준 정렬
mysqldumpslow -s at -t 10 /var/log/mysql/slow.log
```

출력 예시:

```
Count: 1523  Time=2.45s (3731s)  Lock=0.00s (0s)  Rows=10.0 (15230), app[app]@10.0.0.5
  SELECT u.name, u.email FROM users u JOIN orders o ON u.id = o.user_id WHERE o.created_at > 'S' ORDER BY o.created_at DESC LIMIT N;
```

`mysqldumpslow`는 리터럴 값을 `S`(문자열)와 `N`(숫자)으로 추상화하여 같은 패턴의 쿼리를 하나로 묶는다. 1,523회 실행되었고 평균 2.45초, 총 3,731초를 소비한 쿼리라는 것을 한눈에 파악할 수 있다.

### pt-query-digest

Percona Toolkit의 `pt-query-digest`는 `mysqldumpslow`보다 훨씬 상세한 분석을 제공한다.

```bash
# 기본 분석
pt-query-digest /var/log/mysql/slow.log

# 특정 시간 범위만 분석
pt-query-digest --since '2026-03-15 00:00:00' --until '2026-03-15 23:59:59' /var/log/mysql/slow.log

# 상위 20개 쿼리만 출력
pt-query-digest --limit 20 /var/log/mysql/slow.log
```

`pt-query-digest`의 출력은 세 부분으로 구성된다.

**전체 요약**: 분석 기간, 총 쿼리 수, 총 실행 시간 등

**쿼리 프로파일**: 가장 많은 시간을 소비한 쿼리 패턴을 순위별로 나열

```
# Profile
# Rank Query ID                     Response time   Calls  R/Call
# ==== ============================ =============== ====== ======
#    1 0xABC123DEF456...            3731.2345 52.3%   1523 2.4500
#    2 0x789DEF012ABC...            1205.6789 16.9%    890 1.3543
```

**쿼리별 상세 분석**: 각 쿼리 패턴의 실행 시간 분포, 행 검사 수, 락 대기 시간 등의 통계

`pt-query-digest`의 가장 큰 장점은 **응답 시간 비율**을 보여준다는 점이다. 위 예시에서 1번 쿼리가 전체 느린 쿼리 시간의 52.3%를 차지한다. 이 쿼리를 최적화하면 절반 이상의 개선 효과를 얻을 수 있다.

## Performance Schema

Performance Schema는 MySQL 서버 내부의 실행 이벤트를 실시간으로 수집하는 계측(instrumentation) 프레임워크다. slow query log가 파일 기반의 사후 분석이라면, Performance Schema는 메모리 기반의 실시간 분석이다.

### 활성화 확인

```sql
-- Performance Schema 활성화 여부
SELECT @@performance_schema;

-- 활성화된 instrument 확인
SELECT * FROM performance_schema.setup_instruments
WHERE NAME LIKE 'statement/%' AND ENABLED = 'YES';

-- 활성화된 consumer 확인
SELECT * FROM performance_schema.setup_consumers
WHERE ENABLED = 'YES';
```

MySQL 5.7 이상에서는 기본적으로 활성화되어 있다. 성능 오버헤드는 일반적으로 5% 미만이다.

### 주요 테이블

Performance Schema는 수십 개의 테이블로 구성된다. 쿼리 성능 분석에서 자주 사용하는 테이블은 다음과 같다.

**events_statements_current**: 현재 실행 중인 쿼리

```sql
SELECT THREAD_ID, SQL_TEXT, TIMER_WAIT/1000000000 AS time_sec
FROM performance_schema.events_statements_current
WHERE SQL_TEXT IS NOT NULL;
```

**events_statements_history**: 최근 실행 완료된 쿼리 (thread당 마지막 10개)

**events_statements_summary_by_digest**: 쿼리 패턴(digest)별 집계 통계

이 중 가장 유용한 것은 `events_statements_summary_by_digest`다.

### events_statements_summary_by_digest로 top query 찾기

이 테이블은 쿼리를 **digest**로 그룹화하여 통계를 집계한다. digest는 리터럴 값을 `?`로 치환하여 같은 패턴의 쿼리를 하나로 묶은 것이다.

```sql
-- 총 실행 시간 기준 상위 쿼리
SELECT
    DIGEST_TEXT,
    COUNT_STAR AS exec_count,
    ROUND(SUM_TIMER_WAIT / 1000000000000, 2) AS total_time_sec,
    ROUND(AVG_TIMER_WAIT / 1000000000000, 4) AS avg_time_sec,
    SUM_ROWS_EXAMINED AS rows_examined,
    SUM_ROWS_SENT AS rows_sent
FROM performance_schema.events_statements_summary_by_digest
ORDER BY SUM_TIMER_WAIT DESC
LIMIT 10;
```

```
+------------------------------------------+------------+----------------+--------------+---------------+-----------+
| DIGEST_TEXT                              | exec_count | total_time_sec | avg_time_sec | rows_examined | rows_sent |
+------------------------------------------+------------+----------------+--------------+---------------+-----------+
| SELECT `u` . `name` , `u` . `email` ... |       1523 |        3731.23 |       2.4500 |    1903750000 |     15230 |
| SELECT * FROM `products` WHERE ...       |       8901 |        1205.67 |       0.1354 |      89010000 |   8901000 |
+------------------------------------------+------------+----------------+--------------+---------------+-----------+
```

slow query log와 달리 별도의 분석 도구 없이 SQL만으로 top query를 조회할 수 있다. 집계는 서버 시작 시점(또는 마지막 리셋 시점)부터 누적된다.

```sql
-- 통계 리셋
TRUNCATE TABLE performance_schema.events_statements_summary_by_digest;
```

### full query text 확인

`DIGEST_TEXT`는 1024바이트로 잘릴 수 있다. 전체 쿼리를 확인하려면 `events_statements_history_long`을 사용한다.

```sql
SELECT SQL_TEXT
FROM performance_schema.events_statements_history_long
WHERE DIGEST = '해당 digest 값'
LIMIT 1;
```

## sys 스키마

sys 스키마는 Performance Schema의 데이터를 사람이 읽기 쉬운 형태로 가공한 뷰 모음이다. MySQL 5.7.7 이상에서 기본 포함되어 있다.

### 유용한 뷰들

**statements_with_runtimes_in_95th_percentile**: 실행 시간이 95th percentile 이상인 쿼리

```sql
SELECT * FROM sys.statements_with_runtimes_in_95th_percentile
LIMIT 10;
```

**statements_with_full_table_scans**: full table scan을 수행하는 쿼리

```sql
SELECT * FROM sys.statements_with_full_table_scans
ORDER BY no_index_used_count DESC
LIMIT 10;
```

**statements_with_temp_tables**: 임시 테이블을 사용하는 쿼리

```sql
SELECT * FROM sys.statements_with_temp_tables
ORDER BY disk_tmp_tables DESC
LIMIT 10;
```

**schema_table_statistics**: 테이블별 I/O 통계

```sql
SELECT * FROM sys.schema_table_statistics
WHERE table_schema = 'mydb'
ORDER BY io_read + io_write DESC
LIMIT 10;
```

**schema_index_statistics**: 인덱스 사용 통계

```sql
SELECT * FROM sys.schema_index_statistics
WHERE table_schema = 'mydb';
```

**schema_unused_indexes**: 사용되지 않는 인덱스

```sql
SELECT * FROM sys.schema_unused_indexes
WHERE object_schema = 'mydb';
```

사용되지 않는 인덱스는 INSERT, UPDATE, DELETE 시 불필요한 오버헤드를 발생시킨다. 서버를 충분히 오래 운영한 후 이 뷰를 확인하여 제거 대상을 파악한다.

**innodb_buffer_stats_by_table**: buffer pool에 캐시된 데이터의 테이블별 분포

```sql
SELECT * FROM sys.innodb_buffer_stats_by_table
ORDER BY allocated DESC
LIMIT 10;
```

## 쿼리 프로파일링

개별 쿼리가 내부적으로 어디에서 시간을 소비하는지 확인하는 것이 프로파일링이다.

### SHOW PROFILE (deprecated)

`SHOW PROFILE`은 MySQL 5.6.7부터 deprecated이고 향후 제거될 예정이다. 간단한 프로파일링에는 여전히 사용할 수 있지만 Performance Schema로 대체하는 것이 권장된다.

```sql
-- 프로파일링 활성화
SET profiling = 1;

-- 쿼리 실행
SELECT * FROM users WHERE email = 'test@example.com';

-- 프로파일 확인
SHOW PROFILES;

-- 특정 쿼리의 상세 프로파일
SHOW PROFILE FOR QUERY 1;
```

```
+----------------------+----------+
| Status               | Duration |
+----------------------+----------+
| starting             | 0.000082 |
| checking permissions | 0.000012 |
| Opening tables       | 0.000025 |
| init                 | 0.000035 |
| System lock          | 0.000015 |
| optimizing           | 0.000018 |
| statistics           | 0.000120 |
| preparing            | 0.000022 |
| executing            | 0.000008 |
| Sending data         | 1.234567 |
| end                  | 0.000010 |
| query end            | 0.000012 |
| closing tables       | 0.000015 |
| freeing items        | 0.000025 |
| cleaning up          | 0.000010 |
+----------------------+----------+
```

`Sending data` 단계가 대부분의 시간을 차지하는 경우가 많다. 이 단계는 이름과 달리 InnoDB에서 데이터를 읽고 처리하는 전체 과정을 포함한다.

### Performance Schema로 프로파일링

Performance Schema의 stage instrument를 활성화하면 쿼리의 각 단계별 소요 시간을 확인할 수 있다.

```sql
-- stage instrument 활성화
UPDATE performance_schema.setup_instruments
SET ENABLED = 'YES', TIMED = 'YES'
WHERE NAME LIKE 'stage/%';

UPDATE performance_schema.setup_consumers
SET ENABLED = 'YES'
WHERE NAME LIKE 'events_stages%';

-- 쿼리 실행 후 해당 thread의 stage 이벤트 확인
SELECT
    EVENT_NAME,
    ROUND(TIMER_WAIT / 1000000000, 4) AS time_ms
FROM performance_schema.events_stages_history_long
WHERE NESTING_EVENT_ID = (
    SELECT EVENT_ID
    FROM performance_schema.events_statements_history_long
    WHERE SQL_TEXT LIKE '%특정 쿼리 패턴%'
    ORDER BY TIMER_START DESC
    LIMIT 1
)
ORDER BY TIMER_START;
```

## 실전 트러블슈팅 흐름

느린 쿼리 문제를 체계적으로 해결하는 흐름은 다음과 같다.

### 1단계: 문제 인지

서비스 응답 시간이 느려지거나, 모니터링 알림이 발생하거나, 사용자 불만이 접수된다.

```sql
-- 현재 실행 중인 쿼리 확인
SHOW PROCESSLIST;

-- 오래 실행 중인 쿼리 필터링
SELECT *
FROM information_schema.PROCESSLIST
WHERE COMMAND != 'Sleep'
  AND TIME > 5
ORDER BY TIME DESC;
```

### 2단계: 문제 쿼리 식별

```sql
-- Performance Schema에서 최근 가장 느린 쿼리
SELECT
    DIGEST_TEXT,
    COUNT_STAR,
    ROUND(AVG_TIMER_WAIT / 1000000000000, 2) AS avg_sec,
    ROUND(MAX_TIMER_WAIT / 1000000000000, 2) AS max_sec,
    SUM_ROWS_EXAMINED,
    SUM_ROWS_SENT
FROM performance_schema.events_statements_summary_by_digest
ORDER BY AVG_TIMER_WAIT DESC
LIMIT 5;
```

또는 slow query log를 `pt-query-digest`로 분석하여 가장 많은 시간을 소비하는 쿼리를 찾는다.

### 3단계: EXPLAIN 분석

문제 쿼리를 식별했으면 `EXPLAIN`으로 실행 계획을 확인한다.

```sql
EXPLAIN SELECT u.name, u.email
FROM users u
JOIN orders o ON u.id = o.user_id
WHERE o.created_at > '2026-01-01'
ORDER BY o.created_at DESC
LIMIT 10;
```

확인할 주요 항목:

- **type**: `ALL`(full scan)이면 인덱스를 사용하지 않는다
- **key**: 실제 사용된 인덱스. `NULL`이면 인덱스 미사용
- **rows**: 검사할 것으로 예상되는 행 수
- **Extra**: `Using filesort`(추가 정렬), `Using temporary`(임시 테이블), `Using where`(스토리지 엔진이 반환한 행을 서버에서 재필터링)

MySQL 8.0.18 이상에서는 `EXPLAIN ANALYZE`로 실제 실행 통계를 확인할 수 있다.

```sql
EXPLAIN ANALYZE SELECT u.name, u.email
FROM users u
JOIN orders o ON u.id = o.user_id
WHERE o.created_at > '2026-01-01'
ORDER BY o.created_at DESC
LIMIT 10;
```

`EXPLAIN ANALYZE`는 쿼리를 실제로 실행하므로 프로덕션에서 주의해서 사용한다.

### 4단계: 최적화

EXPLAIN 결과를 기반으로 최적화를 적용한다. 흔한 최적화 방향:

**인덱스 추가 또는 변경**

```sql
-- orders 테이블에 created_at + user_id 복합 인덱스 추가
ALTER TABLE orders ADD INDEX idx_created_user (created_at, user_id);
```

**쿼리 리팩토링**

```sql
-- 서브쿼리를 조인으로 변환, 불필요한 컬럼 제거 등
```

**테이블 구조 변경**

```sql
-- 정규화/비정규화, 파티셔닝 적용 등
```

### 5단계: 검증

최적화 적용 후 동일 쿼리의 `EXPLAIN`을 다시 확인하고, 실제 실행 시간을 측정한다.

```sql
-- 최적화 후 EXPLAIN 재확인
EXPLAIN SELECT u.name, u.email
FROM users u
JOIN orders o ON u.id = o.user_id
WHERE o.created_at > '2026-01-01'
ORDER BY o.created_at DESC
LIMIT 10;

-- type: ALL → ref로 변경되었는지
-- rows: 125만 → 수천으로 줄었는지
-- Extra: Using filesort가 사라졌는지
```

변경 후 시간이 지나면 Performance Schema 통계를 리셋하고, 새로 집계된 수치로 개선 효과를 확인한다.

## 모니터링 도구와의 연계

자체 도구만으로도 분석은 가능하지만, 프로덕션 환경에서는 전문 모니터링 도구를 함께 사용하는 것이 효율적이다.

### PMM (Percona Monitoring and Management)

Percona에서 제공하는 오픈소스 모니터링 플랫폼이다. Performance Schema와 slow query log 데이터를 수집하여 웹 대시보드로 제공한다.

- **Query Analytics (QAN)**: 쿼리별 실행 시간, 호출 횟수, 행 검사 수를 시각화한다. `pt-query-digest`의 웹 버전에 해당한다.
- 쿼리 실행 계획을 UI에서 직접 확인할 수 있다.
- 시계열 그래프로 특정 시점에 느려진 쿼리를 추적할 수 있다.

### Datadog, Grafana 등

상용 모니터링 서비스나 Grafana + Prometheus 조합에서도 MySQL 메트릭을 수집할 수 있다.

공통적으로 모니터링해야 할 핵심 메트릭:

- **Queries per second**: 초당 쿼리 실행 수
- **Slow queries**: slow query log에 기록된 쿼리 수
- **Threads connected / Threads running**: 연결된 커넥션 수와 실제 쿼리를 실행 중인 thread 수
- **InnoDB row operations**: 초당 읽기/삽입/수정/삭제 행 수
- **InnoDB buffer pool hit ratio**: buffer pool 히트율
- **Lock wait time**: 락 대기 시간

`Threads_running`이 급증하는 시점을 기준으로 해당 시간대의 slow query log를 분석하면, 문제 쿼리를 빠르게 특정할 수 있다.

```sql
-- 주요 모니터링 메트릭 조회
SHOW GLOBAL STATUS LIKE 'Questions';
SHOW GLOBAL STATUS LIKE 'Slow_queries';
SHOW GLOBAL STATUS LIKE 'Threads_running';
SHOW GLOBAL STATUS LIKE 'Innodb_rows_%';
```

## 정리

- 느린 쿼리 해결은 식별, 분석, 최적화, 검증의 반복이다.
- slow query log는 기준 시간을 초과한 쿼리를 파일로 기록하고, `pt-query-digest`로 패턴별 집계를 확인한다.
- Performance Schema는 실시간으로 쿼리 통계를 수집하며, `events_statements_summary_by_digest` 테이블로 top query를 조회할 수 있다.
- sys 스키마는 Performance Schema 데이터를 읽기 쉬운 뷰로 제공한다. full table scan, 임시 테이블 사용, 미사용 인덱스 등을 쉽게 파악할 수 있다.
- 문제 쿼리를 찾으면 EXPLAIN으로 실행 계획을 분석하고, 인덱스 추가나 쿼리 리팩토링으로 최적화한 뒤 효과를 검증한다.
- slow query log를 상시 활성화하고, Performance Schema 통계를 주기적으로 확인하며, 모니터링 도구로 추세를 관찰하는 것이 안정적인 운영의 기본이다.
