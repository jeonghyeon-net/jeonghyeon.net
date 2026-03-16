# Aurora 실전 함정과 운영

Aurora를 운영하면서 마주치는 문제 중 상당수는 기존 MySQL과의 미묘한 차이에서 비롯된다. "MySQL 호환"이라는 말에 기대어 기존 MySQL과 똑같이 운영하면, 예상치 못한 성능 저하나 비용 증가를 겪게 된다. 이 글에서는 실전에서 자주 발생하는 함정과 대응 방법을 다룬다.

## 로컬 스토리지가 없다

Aurora 컴퓨트 인스턴스에는 영구 로컬 디스크가 없다. 데이터는 전부 네트워크 스토리지에 있다. 이 사실이 의외의 영향을 미치는 지점이 있다.

### 임시 테이블 (temporary table)

MySQL에서 복잡한 쿼리(GROUP BY, DISTINCT, 서브쿼리, UNION 등)를 실행하면 내부적으로 임시 테이블이 생성될 수 있다. 기존 MySQL에서는 임시 테이블이 먼저 메모리(TempTable 또는 MEMORY 엔진)에 생성되고, 크기가 `tmp_table_size`나 `max_heap_table_size`를 초과하면 디스크로 전환된다.

Aurora에서 임시 테이블이 디스크로 전환되면, 이 데이터는 로컬 임시 볼륨에 기록된다. Aurora MySQL 3.x(MySQL 8.0 호환)에서는 TempTable 스토리지 엔진이 기본이며, 메모리 한도를 초과하면 로컬 임시 스토리지를 사용한다. 이 임시 스토리지의 크기는 인스턴스 타입에 따라 제한된다.

```text
인스턴스 타입별 로컬 임시 스토리지 한도 (대략적):
- r6g.large:    ~16 GB
- r6g.xlarge:   ~32 GB
- r6g.2xlarge:  ~64 GB
```

이 한도를 초과하면 쿼리가 에러로 실패한다. 기존 MySQL에서는 로컬 디스크 용량이 충분하면 수백 GB의 임시 테이블도 생성할 수 있었지만, Aurora에서는 인스턴스 타입에 의해 상한이 정해진다.

대응 방법:
- 임시 테이블을 많이 사용하는 쿼리를 최적화한다 (인덱스 추가, 쿼리 재작성)
- `tmp_table_size`를 적절히 설정하여 메모리 내에서 처리되는 비율을 높인다
- 대용량 임시 테이블이 필요한 분석 쿼리는 큰 인스턴스 타입의 reader에서 실행한다

### Sort buffer

`ORDER BY`나 `GROUP BY`에서 filesort가 발생하면 `sort_buffer_size`만큼 메모리를 사용하고, 부족하면 디스크를 사용한다. 이 디스크 사용도 로컬 임시 스토리지에 해당한다. 위와 같은 한도가 적용된다.

## DDL 동작 차이

Aurora MySQL에서의 DDL(Data Definition Language) 동작은 기존 MySQL과 유사하지만, 몇 가지 차이가 있다.

### Instant DDL

MySQL 8.0에서 도입된 instant DDL은 테이블 메타데이터만 변경하여 DDL을 즉시 완료하는 기능이다. 테이블 크기에 관계없이 수 초 이내에 끝난다.

Aurora MySQL 3.x에서도 instant DDL을 지원한다. 지원 범위는 기존 MySQL 8.0과 대체로 동일하다:

```sql
-- instant DDL이 가능한 작업
ALTER TABLE orders ADD COLUMN memo VARCHAR(255), ALGORITHM=INSTANT;
ALTER TABLE orders DROP COLUMN memo, ALGORITHM=INSTANT;  -- MySQL 8.0.29+
ALTER TABLE orders RENAME COLUMN old_name TO new_name, ALGORITHM=INSTANT;
```

```sql
-- instant DDL이 불가능한 작업 (INPLACE 또는 COPY 필요)
ALTER TABLE orders ADD INDEX idx_date (order_date);      -- 인덱스 추가
ALTER TABLE orders MODIFY COLUMN name VARCHAR(100);      -- 타입 변경
ALTER TABLE orders ADD COLUMN id2 INT AFTER id;          -- 중간 위치 추가
```

### Fast DDL (Aurora 고유)

Aurora MySQL 2.x(MySQL 5.7 호환)에는 Aurora 고유의 fast DDL 기능이 있었다. 테이블 끝에 nullable 컬럼을 추가할 때 메타데이터만 변경하는 방식이다. MySQL 8.0의 instant DDL과 유사한 개념이지만, 지원 범위가 더 제한적이었다. Aurora MySQL 3.x에서는 MySQL 8.0의 instant DDL을 사용하므로 fast DDL은 더 이상 필요하지 않다.

### DDL과 replica lag

DDL 실행 중에는 reader에서 replica lag가 증가할 수 있다. 특히 인덱스 추가와 같은 긴 DDL 작업 중에, writer가 생성하는 redo log 양이 증가하고, reader의 cache invalidation 처리 부하도 올라간다.

## Binlog 관련 주의사항

Aurora는 기본적으로 binlog가 비활성화되어 있다. 24편에서 다룬 것처럼, Aurora의 복제는 공유 스토리지 기반이므로 binlog가 필요 없다.

### Binlog를 활성화해야 하는 경우

- Aurora에서 외부 MySQL replica로 복제해야 하는 경우
- DMS(Database Migration Service)로 다른 서비스에 변경 사항을 스트리밍하는 경우
- CDC(Change Data Capture) 파이프라인을 구축하는 경우

### 활성화 시 성능 영향

Binlog를 활성화하면 Aurora의 가장 큰 이점인 "redo log만 전송"의 원칙이 깨진다. Writer가 redo log에 더해 binlog도 생성해야 하므로 쓰기 성능이 저하된다.

```
[binlog 비활성화 (기본)]
Writer → redo log record → 스토리지

[binlog 활성화]
Writer → redo log record → 스토리지
       → binlog → 스토리지 (추가 I/O)
```

성능 저하 폭은 워크로드에 따라 다르지만, 쓰기 집약적인 워크로드에서 10~30%의 throughput 감소가 관찰되기도 한다.

binlog가 필요한 경우, `binlog_format`은 `ROW`로 설정하는 것이 권장된다. `STATEMENT` 형식은 non-deterministic 쿼리에서 replica와 데이터가 달라질 위험이 있고, `MIXED`는 일부 상황에서 `STATEMENT`로 전환되므로 같은 위험을 내포한다.

### Binlog 보존 기간

Aurora의 binlog 보존 기간은 `mysql.rds_set_configuration` 프로시저로 설정한다:

```sql
-- binlog 보존 기간을 24시간으로 설정
CALL mysql.rds_set_configuration('binlog retention hours', 24);

-- 현재 설정 확인
CALL mysql.rds_show_configuration;
```

기본값은 NULL(보존하지 않음)이다. CDC를 사용하는 경우, consumer의 처리 지연을 고려하여 충분한 보존 기간을 설정해야 한다. 보존 기간이 길수록 스토리지 사용량이 증가하고 비용이 올라간다.

## I/O 과금 이해

Aurora의 비용 구조에서 가장 예측하기 어려운 부분이 I/O 과금이다.

### 읽기 I/O와 쓰기 I/O

- **쓰기 I/O**: Writer가 스토리지에 redo log record를 쓸 때 발생한다. 4KB 단위로 과금된다.
- **읽기 I/O**: 컴퓨트 노드가 스토리지에서 data page를 읽을 때 발생한다. Buffer pool에 있는 데이터를 읽으면 I/O가 발생하지 않는다.

핵심: buffer pool hit ratio가 높을수록 읽기 I/O 비용이 줄어든다. 인스턴스를 한 단계 크게 가져가서 buffer pool을 늘리면, I/O 비용 절감이 인스턴스 비용 증가를 상쇄할 수 있다.

### I/O 비용 최적화 전략

**1. Buffer pool 크기 최적화**: Working set이 buffer pool에 들어가도록 인스턴스를 사이징한다. `BufferCacheHitRatio`가 99% 이상을 유지하는 것이 목표다.

**2. 불필요한 풀 스캔 제거**: 인덱스가 없는 쿼리, 비효율적인 JOIN, 큰 임시 테이블은 대량의 읽기 I/O를 발생시킨다.

**3. Reader 활용**: Reader에서 발생하는 읽기 I/O도 과금된다. 하지만 reader를 통해 writer의 buffer pool 오염(대량 읽기로 인한 hot page eviction)을 방지할 수 있다.

**4. Aurora I/O-Optimized**: I/O가 많은 워크로드에서는 Aurora I/O-Optimized 구성을 고려한다. 인스턴스 비용이 약 30% 높아지지만 I/O 과금이 없다. 전체 비용의 25% 이상이 I/O인 경우 유리할 수 있다.

### I/O 사용량 모니터링

```text
CloudWatch에서 확인 가능한 주요 I/O 메트릭:
- VolumeReadIOPs: 초당 읽기 I/O 횟수
- VolumeWriteIOPs: 초당 쓰기 I/O 횟수
```

Performance Insights에서 특정 쿼리가 발생시키는 I/O 양을 확인할 수 있다. Top SQL by I/O를 분석하면 비용을 가장 많이 발생시키는 쿼리를 식별할 수 있다.

## 모니터링 핵심 지표

### CloudWatch 메트릭

Aurora 운영에서 반드시 모니터링해야 할 메트릭:

| 메트릭 | 의미 | 주의 기준 |
|---|---|---|
| `CPUUtilization` | CPU 사용률 | 지속적으로 80% 이상 |
| `FreeableMemory` | 사용 가능한 메모리 | 인스턴스 메모리의 10% 미만 |
| `BufferCacheHitRatio` | Buffer pool hit 비율 | 95% 미만 |
| `AuroraReplicaLag` | Reader의 복제 지연 | 100ms 이상 지속 |
| `DatabaseConnections` | 현재 연결 수 | max_connections의 80% 이상 |
| `DMLLatency` | DML 평균 지연 시간 | 평소 대비 급격한 증가 |
| `VolumeReadIOPs` | 초당 읽기 I/O | 급격한 증가 (비용과 직결) |
| `VolumeWriteIOPs` | 초당 쓰기 I/O | 급격한 증가 |
| `VolumeBytesUsed` | 스토리지 사용량 | 예상치 못한 증가 |

### Performance Insights

CloudWatch가 인스턴스 수준의 메트릭을 제공한다면, Performance Insights는 쿼리 수준의 분석을 제공한다.

주요 활용 방법:
- **DB Load**: 활성 세션 수를 시간축으로 시각화한다. vCPU 수를 초과하는 load는 CPU 대기를 의미한다
- **Wait events**: 쿼리가 무엇을 기다리고 있는지 분석한다. `io/aurora_redo_log_flush`가 높으면 쓰기 병목, `io/table/sql/handler`가 높으면 읽기 병목이다
- **Top SQL**: 가장 많은 리소스를 소비하는 쿼리를 식별한다. DB Load 기여도 순으로 정렬하여 최적화 대상을 선정한다

### Enhanced Monitoring vs CloudWatch

CloudWatch 메트릭은 1분 간격(기본)으로 수집되며, 데이터베이스 엔진이 보고하는 값이다. Enhanced Monitoring은 인스턴스의 OS 수준 메트릭을 1초~60초 간격으로 수집한다.

Enhanced Monitoring에서 확인할 수 있는 것:
- 프로세스별 CPU, 메모리 사용량
- OS 수준의 파일 시스템, 네트워크 I/O
- 스왑 사용량

CloudWatch에서 CPU가 높아 보이지만 원인을 특정할 수 없을 때, Enhanced Monitoring에서 프로세스 목록을 확인하면 어떤 thread가 CPU를 소비하는지 식별할 수 있다.

## 장애 시나리오별 대응

### Writer 인스턴스 장애

가장 흔한 장애 시나리오다. Writer가 응답하지 않으면 Aurora 컨트롤 플레인이 자동으로 failover를 수행한다.

대응 체크리스트:
1. Reader가 최소 1개 있는지 확인 (없으면 새 인스턴스 생성이 필요하여 복구 시간이 길어짐)
2. Failover priority가 설정되어 있는지 확인
3. 애플리케이션의 재연결 로직이 동작하는지 확인
4. Failover 후 새 writer의 인스턴스 타입이 적절한지 확인

### 스토리지 노드 장애

스토리지 노드 하나의 장애는 사용자에게 투명하다. 6-copy quorum 구조에서 1~2개의 copy 손실은 자동으로 복구된다. 모니터링에서 일시적인 지연 증가가 관찰될 수 있지만, 서비스 중단은 발생하지 않는다.

### AZ 장애

하나의 AZ 전체가 장애를 일으키면:
- 해당 AZ에 있던 컴퓨트 인스턴스는 사용 불가
- 스토리지는 나머지 2개 AZ의 4개 copy로 정상 운영 (4/6 쓰기 quorum 충족)
- Writer가 장애 AZ에 있었다면 다른 AZ의 reader로 failover

AZ 장애에 대비하여:
- Writer와 failover 대상 reader를 서로 다른 AZ에 배치한다
- 모든 인스턴스를 같은 AZ에 두지 않는다

### 장시간 트랜잭션으로 인한 스토리지 증가

명시적인 장애는 아니지만, 운영에서 자주 마주치는 문제다. 장시간 실행 중인 트랜잭션이 있으면, 해당 트랜잭션 시작 이후의 undo log를 유지해야 한다. MVCC를 위해 해당 시점의 데이터를 보존해야 하기 때문이다.

```sql
-- 장시간 실행 중인 트랜잭션 확인
SELECT trx_id, trx_state, trx_started,
       TIMESTAMPDIFF(SECOND, trx_started, NOW()) AS duration_seconds,
       trx_rows_modified, trx_rows_locked
FROM information_schema.innodb_trx
ORDER BY trx_started ASC;
```

수 시간 이상 열려 있는 트랜잭션은 undo log를 수 GB까지 증가시킬 수 있다. Aurora에서 이것은 직접적으로 스토리지 비용으로 이어진다. `VolumeBytesUsed` 메트릭이 급증하면 장시간 트랜잭션을 의심해야 한다.

## 백업과 복원

### 연속 백업 (Continuous backup)

Aurora는 스토리지 레이어에서 자동으로 연속 백업을 수행한다. 별도의 백업 작업이 데이터베이스 성능에 영향을 주지 않는다. 이것은 기존 MySQL의 `mysqldump`나 `xtrabackup`처럼 백업 중에 I/O 부하가 증가하는 것과 대조적이다.

백업 보존 기간은 1~35일로 설정할 수 있다. 기본값은 API/CLI로 생성 시 1일, AWS 콘솔로 생성 시 7일이다.

### PITR (Point-in-Time Recovery)

보존 기간 내 임의의 시점으로 데이터베이스를 복원할 수 있다. 복원은 새로운 Aurora 클러스터를 생성하는 방식이다. 기존 클러스터를 덮어쓰지 않는다.

```
[PITR 복원]
원본 클러스터 (계속 운영 중)
    │
    └─→ 새 클러스터 생성 (지정 시점의 데이터로)
        └─ 데이터 확인 후 애플리케이션이 새 클러스터로 전환
```

복원 소요 시간은 데이터베이스 크기에 따라 다르지만, 대용량 데이터베이스에서도 수십 분 이내에 완료되는 경우가 많다. 기존 MySQL에서 수 TB의 `xtrabackup` 복원에 수 시간이 걸리는 것과 비교하면 크게 빠르다.

### 클러스터 스냅샷

수동으로 특정 시점의 스냅샷을 생성할 수 있다. 스냅샷은 보존 기간에 관계없이 명시적으로 삭제할 때까지 유지된다. 배포 전 백업, 마이그레이션 전 백업 등에 사용한다.

스냅샷에서 복원할 때도 새 클러스터가 생성된다. Cross-region으로 스냅샷을 복사하여 다른 리전에서 클러스터를 생성하는 것도 가능하다.

### Backtrack

Aurora MySQL 고유 기능으로, 클러스터를 과거 특정 시점으로 "되감기"할 수 있다. PITR과 달리 새 클러스터를 생성하지 않고, 기존 클러스터 자체를 되돌린다.

```bash
# backtrack 실행 (AWS CLI)
aws rds backtrack-db-cluster \
  --db-cluster-identifier my-cluster \
  --backtrack-to "2025-03-15T10:30:00Z"
```

주의사항:
- Backtrack window(최대 72시간)를 미리 설정해야 한다. 설정하지 않으면 사용할 수 없다
- Backtrack은 클러스터 전체를 되돌린다. 특정 테이블만 되돌릴 수 없다
- Backtrack 중에는 클러스터가 일시적으로 사용 불가 상태가 된다
- Backtrack window 유지에 추가 비용이 발생한다

## 흔한 실수와 회피 방법

### 1. Writer에서 분석 쿼리 실행

대용량 테이블의 풀 스캔 쿼리를 writer에서 실행하면, buffer pool의 hot page가 밀려나고(eviction), OLTP 쿼리의 cache hit ratio가 급락한다. 분석 쿼리는 반드시 reader에서, 가능하면 별도 custom endpoint의 dedicated reader에서 실행한다.

### 2. 모든 reader를 같은 인스턴스 타입으로 운영

API 읽기용 reader와 분석용 reader의 워크로드는 다르다. API용은 응답 시간이 중요하고, 분석용은 대용량 데이터 처리가 중요하다. Custom endpoint로 분리하고 각 용도에 맞는 인스턴스 타입을 사용한다.

### 3. Failover priority 미설정

기본적으로 모든 reader의 failover priority는 같다(tier-1). Writer 장애 시 어떤 reader가 승격될지 예측할 수 없다. 작은 인스턴스가 writer로 승격되면 성능 장애가 발생한다. Writer와 같은 스펙의 reader를 tier-0으로, 나머지를 tier-15로 설정하는 것이 일반적이다.

### 4. DNS 캐싱으로 인한 failover 지연

Failover 후 DNS가 업데이트되었는데, 애플리케이션이 기존 IP를 캐싱하고 있으면 새 writer에 연결하지 못한다. Java 애플리케이션에서 특히 흔하다. DNS TTL을 짧게 설정하거나, Aurora 전용 드라이버를 사용한다.

### 5. Aurora 스토리지 비용 무시

Aurora의 스토리지 비용은 RDS MySQL의 EBS 비용보다 GB 단가가 높다. 대용량 데이터를 장기 보관하면 비용이 빠르게 증가한다. 오래된 데이터는 S3로 아카이빙하거나, 파티셔닝과 DROP PARTITION으로 관리한다.

### 6. max_connections 과다 설정

인스턴스 메모리를 고려하지 않고 max_connections를 높게 설정하면, 동시 연결이 몰릴 때 메모리 부족으로 OOM(Out of Memory)이 발생할 수 있다. 각 connection은 `sort_buffer_size`, `join_buffer_size`, `read_buffer_size` 등의 세션 메모리를 할당받으므로, connection 수 x 세션 버퍼 합계가 가용 메모리를 초과하지 않아야 한다.

```sql
-- 현재 연결 수와 한도 확인
SHOW STATUS LIKE 'Threads_connected';
SHOW VARIABLES LIKE 'max_connections';
```

connection 수가 부족한 것이 아니라 connection 관리가 문제인 경우가 대부분이다. Connection pool 설정을 점검하고, 필요하면 RDS Proxy를 도입한다.

### 7. 대용량 트랜잭션의 redo log 폭발

수백만 건의 UPDATE나 DELETE를 단일 트랜잭션으로 실행하면, 대량의 redo log가 생성되어 스토리지 I/O가 급증한다. Aurora에서는 이것이 직접적으로 쓰기 I/O 비용으로 이어진다. 배치 작업은 적절한 크기(1,000~10,000건)로 분할하여 실행한다.

```sql
-- 나쁜 예: 100만 건 한 번에 삭제
DELETE FROM logs WHERE created_at < '2024-01-01';

-- 좋은 예: 분할 삭제
DELETE FROM logs WHERE created_at < '2024-01-01' LIMIT 5000;
-- 반복 실행, 각 실행 사이에 짧은 간격
```

Aurora의 아키텍처를 이해하면, 기존 MySQL 운영 경험을 그대로 가져가되 스토리지 I/O 비용, buffer pool 관리, failover 설계라는 세 가지 축에서 추가적인 주의를 기울여야 한다는 것을 알 수 있다.

## 정리

- Aurora 컴퓨트 인스턴스에는 로컬 영구 디스크가 없으므로, 임시 테이블과 filesort의 디스크 사용량이 인스턴스 타입에 의해 제한된다.
- Binlog 활성화는 Aurora의 "redo log만 전송" 원칙을 깨뜨려 쓰기 성능을 저하시킨다.
- I/O 과금은 buffer pool hit ratio와 직결되므로, 인스턴스를 한 단계 크게 가져가는 것이 총비용을 줄일 수 있다.
- Failover priority 미설정, DNS 캐싱, 장시간 트랜잭션은 운영에서 자주 발생하는 함정이다.
- 대량 DML은 배치로 분할하여 실행해야 redo log 폭발과 I/O 비용 급증을 방지한다.
