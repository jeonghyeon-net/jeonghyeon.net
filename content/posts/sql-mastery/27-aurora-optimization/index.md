# Aurora 최적화 기법

Aurora는 MySQL 호환이므로 인덱스 설계, 쿼리 최적화, 트랜잭션 관리 등 기본적인 튜닝 원칙이 동일하다. 하지만 컴퓨트-스토리지 분리 아키텍처로 인해 기존 MySQL과 다르게 접근해야 하는 튜닝 포인트가 있다. 무엇이 같고, 무엇이 달라지는지 구분하는 것이 핵심이다.

## 바뀌지 않는 것

다음은 Aurora에서도 기존 MySQL과 동일하게 적용되는 영역이다:

- **인덱스 설계**: B+tree 구조, clustered index, secondary index의 동작은 동일하다. Covering index, 복합 인덱스의 컬럼 순서, 카디널리티 등 인덱스 전략은 그대로다.
- **쿼리 옵티마이저**: 실행 계획 생성 로직은 기존 MySQL과 같다. `EXPLAIN`의 해석 방법도 동일하다.
- **트랜잭션과 잠금**: InnoDB의 행 수준 잠금, gap lock, next-key lock은 그대로 동작한다. Deadlock 감지도 동일하다.
- **스키마 설계**: 정규화/비정규화, 데이터 타입 선택, 파티셔닝 전략 등은 바뀌지 않는다.

## 바뀌는 것

다음은 Aurora의 아키텍처 차이로 인해 튜닝 접근이 달라지는 영역이다:

- **I/O 튜닝**: 로컬 디스크가 아닌 네트워크 스토리지이므로, I/O 관련 파라미터의 의미가 달라진다
- **Buffer pool 관리**: Survivable buffer pool이라는 Aurora 고유 기능이 있다
- **복제 활용**: 공유 스토리지 기반 읽기 분산이 가능하다
- **Connection 관리**: 인스턴스 재시작이나 failover 시 connection 처리가 중요해진다
- **스케일링 전략**: Serverless v2, reader auto-scaling 등 Aurora 고유 옵션이 있다

## Buffer pool 워밍업: survivable buffer pool

기존 MySQL에서 인스턴스를 재시작하면 buffer pool이 비어 있다. 캐시가 차가운(cold) 상태에서 시작하므로, 모든 읽기가 디스크 I/O를 발생시킨다. Buffer pool이 충분히 채워질(warm) 때까지 성능이 저하된다.

MySQL 5.6 이상에서는 `innodb_buffer_pool_dump_at_shutdown`과 `innodb_buffer_pool_load_at_startup`을 설정하여 buffer pool의 page 목록을 저장하고 복원할 수 있다. 하지만 이것은 page 목록만 저장하는 것이지, page 내용 자체를 저장하는 것은 아니다. 재시작 후 해당 page를 다시 디스크에서 읽어와야 하므로 완전한 해결책은 아니다.

Aurora의 survivable buffer pool(또는 buffer pool warm cache)은 이 문제를 다르게 해결한다. Aurora는 buffer pool의 page 캐시를 컴퓨트 인스턴스의 프로세스 외부(별도 캐시 레이어)에 유지한다. 데이터베이스 프로세스가 재시작되어도 이 캐시가 사라지지 않는다.

```
[기존 MySQL 재시작]
재시작 전: buffer pool 100% warm → 재시작 → buffer pool 0% → 점진적 warm up

[Aurora 재시작]
재시작 전: buffer pool 100% warm → 재시작 → buffer pool ~100% warm (캐시 유지)
```

주의할 점: survivable buffer pool은 같은 인스턴스에서 프로세스가 재시작될 때 유효하다. 인스턴스 타입 변경(예: r6g.xlarge → r6g.2xlarge)이나 다른 인스턴스로의 failover에서는 buffer pool이 유지되지 않는다.

## Parallel query

Aurora parallel query는 데이터 스캔 작업을 스토리지 레이어에 내려보내는(push down) 기능이다. 기존 MySQL에서 풀 테이블 스캔이 발생하면, 스토리지에서 data page를 컴퓨트 노드의 buffer pool로 가져온 뒤 필터링한다. Parallel query는 필터링과 집계를 스토리지 노드에서 수행하고, 결과만 컴퓨트 노드에 반환한다.

```
[일반 쿼리 실행]
스토리지 ──(전체 data page)──→ 컴퓨트 ──(필터링)──→ 결과

[Parallel query]
스토리지 ──(필터링 + 집계)──→ 컴퓨트 ──→ 결과
           ↑ 조건 push down
```

### 적합한 쿼리

- 대용량 테이블의 풀 스캔이 필요한 분석 쿼리
- Buffer pool에 캐싱되지 않은 cold data에 대한 쿼리
- WHERE 조건으로 상당수의 행을 필터링하지만, 인덱스를 사용할 수 없는 경우

### 부적합한 쿼리

- 인덱스를 타는 OLTP 쿼리 (이미 빠르다)
- Buffer pool에 대부분 캐싱된 hot data (네트워크 왕복보다 메모리 접근이 빠르다)
- 아주 작은 테이블

### 확인 방법

```sql
-- parallel query 활성화
SET aurora_parallel_query = ON;

-- 실행 계획에서 확인
EXPLAIN SELECT COUNT(*), AVG(amount)
FROM orders
WHERE order_date BETWEEN '2024-01-01' AND '2024-12-31';
```

`EXPLAIN` 결과의 `Extra` 컬럼에 `Using parallel query`가 표시되면 parallel query가 사용된다. 옵티마이저가 판단하여 자동으로 적용하므로, 모든 쿼리에 적용되는 것은 아니다.

### 비용 고려

Parallel query는 스토리지 레이어에서 처리하므로, buffer pool을 거치지 않는다. 이는 해당 쿼리의 I/O가 전부 스토리지 I/O로 과금된다는 뜻이다. Buffer pool에 이미 캐싱된 데이터를 parallel query로 읽으면, 불필요한 I/O 비용이 발생할 수 있다.

## 인스턴스 사이징 전략

Aurora 컴퓨트 인스턴스의 크기를 결정할 때 가장 중요한 요소는 buffer pool 크기다. Buffer pool이 working set(자주 접근하는 데이터)을 충분히 수용하면 스토리지 I/O를 최소화할 수 있다.

### Buffer pool 크기 추정

InnoDB buffer pool은 인스턴스 메모리의 약 75%를 차지한다. Aurora에서도 이 비율은 비슷하다.

| 인스턴스 타입 | 메모리 | 대략적인 buffer pool |
|---|---|---|
| r6g.large | 16 GB | ~12 GB |
| r6g.xlarge | 32 GB | ~24 GB |
| r6g.2xlarge | 64 GB | ~48 GB |
| r6g.4xlarge | 128 GB | ~96 GB |
| r6g.8xlarge | 256 GB | ~192 GB |

### Buffer pool hit ratio

CloudWatch 메트릭 `BufferCacheHitRatio`가 99% 이상이면, 대부분의 읽기가 buffer pool에서 처리되고 있다는 뜻이다. 이 값이 95% 미만으로 떨어지면, 스토리지 I/O가 빈번하게 발생하고 있으므로 인스턴스 크기를 늘리는 것을 검토해야 한다.

```sql
-- buffer pool 상태 확인
SHOW ENGINE INNODB STATUS;

-- buffer pool 사용량
SELECT
  FORMAT(pages_data * 16 / 1024, 0) AS buffer_pool_data_mb,
  FORMAT(pages_free * 16 / 1024, 0) AS buffer_pool_free_mb,
  ROUND(100 * pages_data / pool_size, 1) AS pct_used
FROM (
  SELECT
    variable_value + 0 AS pool_size
  FROM performance_schema.global_status
  WHERE variable_name = 'Innodb_buffer_pool_pages_total'
) AS total,
(
  SELECT
    variable_value + 0 AS pages_data
  FROM performance_schema.global_status
  WHERE variable_name = 'Innodb_buffer_pool_pages_data'
) AS data_pages,
(
  SELECT
    variable_value + 0 AS pages_free
  FROM performance_schema.global_status
  WHERE variable_name = 'Innodb_buffer_pool_pages_free'
) AS free_pages;
```

### Writer와 reader의 사이징

Writer와 reader의 인스턴스 타입은 달라도 된다. 하지만 failover를 고려하면 전략이 필요하다.

- **Failover 대상 reader**: Writer와 동일한 인스턴스 타입을 권장한다. 작은 reader가 writer로 승격되면, 쓰기 부하를 감당하지 못할 수 있다.
- **읽기 전용 reader**: 읽기 워크로드에 맞게 독립적으로 사이징할 수 있다. Failover priority를 낮게(tier 값을 높게) 설정하여 writer 승격 대상에서 제외한다.

## Connection 관리: RDS Proxy

Aurora에서 connection 관리는 기존 MySQL보다 더 중요하다. Failover, 인스턴스 스케일링, reader 추가/제거 등으로 인스턴스가 변경되면 기존 connection이 끊어지기 때문이다.

RDS Proxy는 데이터베이스와 애플리케이션 사이에 위치하여 connection pooling과 failover 처리를 담당한다.

```
애플리케이션           RDS Proxy              Aurora 클러스터
┌─────────┐       ┌──────────┐          ┌──────────┐
│ conn 1  │──────→│          │─── 1 ───→│  Writer  │
│ conn 2  │──────→│ connection│─── 2 ───→│          │
│ conn 3  │──────→│   pool   │          └──────────┘
│  ...    │──────→│          │─── 1 ───→┌──────────┐
│ conn 100│──────→│          │─── 2 ───→│  Reader  │
└─────────┘       └──────────┘          └──────────┘
      100개                  ~6개
```

### RDS Proxy가 유용한 경우

- **Lambda 등 서버리스 함수**: 함수 호출마다 connection을 생성/해제하면 connection storm이 발생한다. Proxy가 connection을 재사용한다.
- **빈번한 failover**: Proxy가 failover를 감지하고 새 writer에 자동으로 연결한다. 애플리케이션은 connection 끊김을 인지하지 못할 수도 있다.
- **connection 수 제한**: Aurora 인스턴스의 `max_connections`에는 한계가 있다. Proxy가 multiplexing으로 적은 수의 실제 connection으로 많은 애플리케이션 connection을 처리한다.

### RDS Proxy가 불필요한 경우

- 애플리케이션이 자체 connection pool(HikariCP, pgbouncer 등)을 충분히 잘 관리하고 있는 경우
- Connection 수가 인스턴스 한계에 근접하지 않는 경우
- 추가 비용(Proxy는 vCPU 기준 과금)이 부담되는 경우

## Aurora Serverless v2

Aurora Serverless v2는 워크로드에 따라 컴퓨트 용량을 자동으로 조절하는 옵션이다. 용량은 ACU(Aurora Capacity Unit) 단위로 측정되며, 1 ACU는 약 2GB 메모리에 해당하는 컴퓨팅 리소스다.

```
[ACU 자동 조절]

       ACU
   16 ┤          ┌────┐
      │          │    │
   12 ┤       ┌──┘    └──┐
      │       │          │
    8 ┤    ┌──┘          └──┐
      │    │                │
    4 ┤────┘                └────
      │
    0 ┤─────────────────────────── 시간
      00:00   06:00   12:00  18:00
          야간(저부하)  주간(고부하)
```

### 설정

```
최소 ACU: 0.5 (1GB 메모리)
최대 ACU: 256 (512GB 메모리)
```

최소 ACU를 0.5로 설정하면 유휴 시 비용을 극적으로 줄일 수 있다. 하지만 최소 ACU가 작으면 부하가 급증할 때 scale-up에 시간이 걸리고, buffer pool이 작아 cold start 영향이 크다.

### 적합한 워크로드

- **예측 불가능한 트래픽**: 이벤트, 세일 등으로 트래픽이 급변하는 서비스
- **개발/스테이징 환경**: 업무 시간에만 사용하고 야간에는 최소 용량으로 유지
- **Reader auto-scaling**: Reader를 serverless v2로 구성하여 읽기 부하에 따라 자동 스케일링

### 주의사항

- Scale-up은 즉각적이지 않다. 수 초의 지연이 있으며, 이 과정에서 쿼리 지연이 발생할 수 있다
- Buffer pool 크기가 ACU에 따라 변하므로, scale-down 시 buffer pool이 줄어들어 cache hit ratio가 떨어질 수 있다
- Provisioned 인스턴스와 serverless v2를 혼합하여 구성할 수 있다. Writer는 provisioned, reader는 serverless v2로 운영하는 것이 일반적인 패턴이다

## 읽기/쓰기 분리 패턴

Aurora reader를 효과적으로 활용하려면, 어떤 쿼리를 reader로 보내야 하고 어떤 쿼리를 writer에서 실행해야 하는지 명확히 구분해야 한다.

### Reader로 보내야 할 쿼리

- 통계, 집계, 리포트 등 분석 쿼리
- 검색 결과 목록 조회
- 최신성이 중요하지 않은 읽기 (예: 상품 목록, 카탈로그)
- 캐시가 만료된 후의 재조회

### Writer에서 실행해야 할 쿼리

- **쓰기 직후 읽기**: INSERT/UPDATE 후 바로 해당 데이터를 조회하는 경우. Replica lag 때문에 reader에서 최신 데이터를 보지 못할 수 있다
- **정합성이 중요한 읽기**: 재고 확인 후 차감, 잔액 확인 후 이체 등 트랜잭션 내에서 읽기와 쓰기가 함께 이루어지는 경우
- **SELECT ... FOR UPDATE**: 잠금을 거는 읽기는 writer에서만 가능하다

### 일반적인 구현 패턴

```python
# Spring의 @Transactional(readOnly = true) 활용 예시의 개념적 구조

def get_product_list():
    # reader로 라우팅
    return db.reader.query("SELECT * FROM products WHERE category = ?", category)

def create_order(user_id, product_id):
    # writer로 라우팅
    db.writer.begin()
    stock = db.writer.query("SELECT stock FROM products WHERE id = ? FOR UPDATE", product_id)
    if stock > 0:
        db.writer.execute("UPDATE products SET stock = stock - 1 WHERE id = ?", product_id)
        db.writer.execute("INSERT INTO orders (...) VALUES (...)")
    db.writer.commit()

    # 주문 결과 조회 — writer에서 읽기 (replica lag 회피)
    return db.writer.query("SELECT * FROM orders WHERE id = ?", order_id)
```

## Aurora에서 의미 없는 파라미터들

기존 MySQL에서 I/O 성능 튜닝에 사용하던 파라미터 중 상당수가 Aurora에서는 무의미하다. Aurora의 스토리지 레이어가 I/O를 관리하기 때문이다.

### 무시되거나 효과 없는 파라미터

| 파라미터 | 기존 MySQL에서의 역할 | Aurora에서의 상태 |
|---|---|---|
| `innodb_read_io_threads` | 비동기 읽기 I/O thread 수 | 스토리지 레이어가 I/O를 관리하므로 효과 없음 |
| `innodb_write_io_threads` | 비동기 쓰기 I/O thread 수 | 동일 |
| `innodb_io_capacity` | 초당 I/O 작업 수 제한 | Aurora는 이 값을 무시함 |
| `innodb_io_capacity_max` | 최대 I/O 작업 수 | 동일 |
| `innodb_flush_method` | 데이터 flush 방식 (O_DIRECT 등) | 로컬 디스크 없으므로 의미 없음 |
| `innodb_flush_log_at_trx_commit` | redo log flush 정책 | Aurora는 항상 quorum 기반 쓰기. 값 변경 불가 |
| `innodb_doublewrite` | double write buffer 활성화 | Aurora에서는 불필요, 비활성 고정 |
| `innodb_log_file_size` | redo log 파일 크기 | Aurora 스토리지가 관리, 설정 불가 |
| `innodb_log_files_in_group` | redo log 파일 수 | 동일 |
| `innodb_checksum_algorithm` | page checksum 방식 | 스토리지 레이어에서 처리 |

### 여전히 유효한 파라미터

| 파라미터 | 설명 |
|---|---|
| `innodb_buffer_pool_size` | Aurora가 자동 설정하지만 조정 가능. 인스턴스 메모리의 75% 기본값 |
| `innodb_lock_wait_timeout` | Lock 대기 시간. 기존과 동일하게 동작 |
| `innodb_deadlock_detect` | Deadlock 감지. 기존과 동일 |
| `max_connections` | 최대 연결 수. 인스턴스 크기에 따라 기본값이 다름 |
| `long_query_time` | slow query log 기준 시간. 기존과 동일 |
| `innodb_print_all_deadlocks` | 모든 deadlock을 에러 로그에 기록. 운영 환경에서 활성화 권장 |

기존 MySQL에서 I/O 튜닝에 시간을 쏟았던 경험이 있다면, Aurora에서는 그 노력을 buffer pool 크기 최적화와 쿼리 최적화에 집중하는 것이 더 효과적이다.

## 정리

- 인덱스 설계, 쿼리 옵티마이저, 트랜잭션과 잠금은 기존 MySQL과 동일하게 동작한다.
- Aurora의 survivable buffer pool은 프로세스 재시작 시에도 캐시를 유지하여 cold start를 방지한다.
- Parallel query는 대용량 풀 스캔 쿼리에 적합하며, buffer pool에 캐싱된 hot data에는 오히려 비효율적이다.
- `innodb_io_capacity`, `innodb_flush_method` 등 로컬 디스크 관련 파라미터는 Aurora에서 효과가 없다.
- 쓰기 직후 읽기나 정합성이 중요한 읽기는 writer에서 실행하고, 분석/목록 조회는 reader로 분리한다.
