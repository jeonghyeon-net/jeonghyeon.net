# Aurora 스토리지 심화

Aurora의 스토리지 레이어는 단순한 디스크 교체가 아니다. 분산 시스템 이론에 기반한 독립적인 스토리지 서비스로, quorum 기반 복제, 자동 복구, 백그라운드 page materialization 등을 자체적으로 수행한다. 앞서 "redo log만 전송한다"고 요약한 구조의 내부를 들여다본다.

## 6-copy quorum 복제

Aurora는 데이터를 3개의 AZ(Availability Zone)에 걸쳐 6개의 복사본으로 유지한다. 각 AZ에 2개씩이다.

```
         AZ-a          AZ-b          AZ-c
       ┌──────┐      ┌──────┐      ┌──────┐
       │copy 1│      │copy 3│      │copy 5│
       │copy 2│      │copy 4│      │copy 6│
       └──────┘      └──────┘      └──────┘
```

쓰기와 읽기에 각각 다른 quorum을 적용한다:

- **쓰기 quorum**: 6개 중 4개 ACK → 커밋 완료 (4/6)
- **읽기 quorum**: 6개 중 3개 응답 → 최신 데이터 확인 (3/6)

이 숫자가 의미하는 바를 계산해본다. 쓰기 quorum과 읽기 quorum의 합이 복사본 수보다 커야 읽기에서 항상 최신 데이터를 포함하게 된다: 4 + 3 = 7 > 6. 따라서 읽기 quorum 3개 중 최소 1개는 반드시 최신 쓰기를 포함한다.

### 장애 내성

6-copy에 4/6 쓰기 quorum을 적용하면 다음 장애를 견딜 수 있다:

**AZ 하나 전체 장애**: 한 AZ가 사라지면 2개의 copy를 잃는다. 남은 4개로 쓰기 quorum(4/6)을 충족한다. 읽기도 3개면 충분하므로 문제없다. 서비스가 중단되지 않는다.

**AZ 장애 + 추가 노드 1개 장애**: 한 AZ 전체(2개)와 다른 AZ의 노드 1개가 동시에 장애를 일으키면, 남은 copy는 3개다. 쓰기 quorum(4개 필요)을 충족하지 못하므로 쓰기가 불가능하다. 하지만 읽기 quorum(3개)은 충족하므로 데이터 손실 없이 읽기는 가능하다.

이것이 6개 copy를 3개 AZ에 2개씩 배치하는 이유다. 4개나 5개가 아닌 6개를 유지하는 것은 AZ 장애와 개별 노드 장애가 동시에 발생하는 상황까지 고려한 설계다.

### 실제 쓰기 흐름

Writer 인스턴스가 redo log record를 쓰는 과정을 단계별로 보면:

```
Writer 인스턴스
    │
    ├──→ storage node 1 (AZ-a)  ✓ ACK
    ├──→ storage node 2 (AZ-a)  ✓ ACK
    ├──→ storage node 3 (AZ-b)  ✓ ACK
    ├──→ storage node 4 (AZ-b)  (느림, 아직 응답 없음)
    ├──→ storage node 5 (AZ-c)  ✓ ACK  ← 4개째 ACK → 커밋 완료!
    └──→ storage node 6 (AZ-c)  (아직 응답 없음)
```

6개에 동시에 전송하고 4개 ACK를 받으면 즉시 커밋한다. 가장 느린 2개의 응답을 기다리지 않는다. 이 구조가 tail latency를 줄이는 데 기여한다. 네트워크 지연이 불균일한 클라우드 환경에서, 가장 느린 노드에 의해 전체 지연이 결정되는 것을 방지한다.

## Protection group (segment)

Aurora 스토리지는 10GB 단위의 segment로 분할되며, 이를 protection group이라고 부른다. 각 protection group은 독립적으로 6개의 copy를 유지한다.

```
데이터베이스 전체 볼륨

┌──────────┬──────────┬──────────┬──────────┐
│  PG #1   │  PG #2   │  PG #3   │  PG #4   │ ...
│  10GB    │  10GB    │  10GB    │  10GB    │
│ 6 copies │ 6 copies │ 6 copies │ 6 copies │
└──────────┴──────────┴──────────┴──────────┘
```

### 왜 10GB 단위인가

10GB는 장애 복구 시간과 스토리지 효율 사이의 균형점이다.

스토리지 노드 하나가 장애를 일으키면 해당 노드가 가진 segment의 데이터를 다른 노드에 복제하여 6-copy를 복원해야 한다. 이 복구 과정에서 데이터 전체를 옮기는 것이 아니라, 장애가 발생한 segment만 복구하면 된다.

10GB segment를 10Gbps 네트워크로 복구하면 약 10초가 걸린다. 이 복구 시간 동안 같은 protection group에서 추가 장애가 발생할 확률은 매우 낮다. 반면 segment가 100GB라면 복구에 100초가 걸리고, 그 사이 추가 장애가 발생할 확률이 높아진다.

segment가 너무 작으면(예: 1GB) 관리해야 할 protection group의 수가 지나치게 많아져 메타데이터 오버헤드가 커진다. 10GB는 이 트레이드오프의 적정 지점이다.

### Protection group과 스토리지 노드의 관계

하나의 protection group은 6개의 서로 다른 스토리지 노드에 분산된다. 하나의 스토리지 노드는 여러 protection group의 데이터를 담당한다.

```
스토리지 노드 A (AZ-a): PG#1의 copy1, PG#5의 copy1, PG#12의 copy2 ...
스토리지 노드 B (AZ-a): PG#1의 copy2, PG#7의 copy1, PG#15의 copy1 ...
스토리지 노드 C (AZ-b): PG#1의 copy3, PG#5의 copy3, PG#9의 copy4 ...
...
```

특정 스토리지 노드가 장애를 일으키면, 그 노드가 담당하던 모든 protection group에서 해당 copy를 다른 노드에 복제한다. 이 복제는 protection group 단위로 병렬 수행되므로, 대용량 데이터베이스에서도 복구 시간이 데이터 크기에 비례하여 늘어나지 않는다.

## 쓰기 증폭(write amplification) 감소

기존 MySQL에서 하나의 행을 변경하면 어떤 쓰기가 발생하는지 다시 보자:

```
[기존 MySQL: 행 1건 UPDATE]

1. redo log record          ~수백 바이트
2. binlog event             ~수백 바이트
3. double write buffer      16KB (page 전체)
4. data page                16KB (page 전체)
───────────────────────────────
합계: ~32KB + α
```

행 하나를 200바이트 변경했는데, 실제로 디스크에 쓰는 양은 32KB 이상이다. 이것이 쓰기 증폭이다. 16KB page 안에서 200바이트만 바뀌었는데 page 전체를 써야 하기 때문이다.

```
[Aurora: 행 1건 UPDATE]

1. redo log record          ~수백 바이트
───────────────────────────────
합계: ~수백 바이트
```

Aurora는 redo log record만 전송한다. 변경된 내용을 서술하는 log record는 보통 수십~수백 바이트에 불과하다. 6개 copy에 전송하므로 총 네트워크 전송량은 수백 바이트 x 6 ≈ 수 KB 수준이다. 기존 MySQL의 32KB와 비교하면 극적으로 감소한다.

이 차이는 쓰기가 집중되는 워크로드에서 특히 두드러진다. OLTP 시스템에서 초당 수천 건의 UPDATE가 발생하면, 기존 MySQL은 수백 MB/s의 쓰기 대역폭이 필요하지만 Aurora는 그 일부만으로 동일한 처리량을 달성할 수 있다.

## 스토리지 노드에서의 redo log 적용

스토리지 노드가 redo log record를 받은 후 어떤 일이 일어나는지 살펴본다.

### 수신 즉시: log record 영속화

스토리지 노드는 redo log record를 받으면 즉시 자신의 로컬 디스크에 기록하고 ACK를 반환한다. 이 시점에서 log record는 영속화되었지만, 아직 data page에 반영된 것은 아니다.

### 백그라운드: page materialization

스토리지 노드는 백그라운드에서 축적된 redo log record를 data page에 적용한다. 이 과정을 page materialization 또는 coalescing이라고 한다.

```
스토리지 노드의 동작:

[수신]                    [백그라운드 적용]
log record #100 → 저장    ─→ page X에 적용
log record #101 → 저장    ─→ page X에 적용
log record #102 → 저장    ─→ page Y에 적용
log record #103 → 저장    ─→ page X에 적용
                              │
                              ▼
                          page X: 3개의 log를
                          한 번에 적용하여
                          최신 page 생성
```

하나의 page에 대한 여러 log record를 모아서 한 번에 적용한다. 기존 MySQL에서는 dirty page를 flush할 때마다 16KB를 통째로 쓰지만, Aurora 스토리지에서는 log를 배치로 적용하므로 쓰기 효율이 높다.

### 읽기 요청 시: on-demand materialization

컴퓨트 노드가 특정 data page를 요청했는데, 해당 page가 아직 최신 log를 반영하지 않은 상태라면 어떻게 되는가?

스토리지 노드는 기존 page에 아직 적용하지 않은 log record를 즉석에서 적용하여 최신 page를 생성한 뒤 반환한다. 이것을 on-demand materialization이라고 한다.

```
컴퓨트 노드: "page X 줘"
    │
    ▼
스토리지 노드:
    page X (version 100) + log #101, #103 적용
    → page X (version 103) 생성
    → 컴퓨트 노드에 반환
```

이 설계 덕분에 백그라운드 materialization이 지연되더라도 데이터 정합성에 문제가 없다. 읽기 시점에 항상 최신 상태의 page를 반환할 수 있기 때문이다.

## Crash recovery가 빠른 이유

기존 MySQL의 crash recovery 과정을 보자:

```
[기존 MySQL crash recovery]

1. 마지막 checkpoint 위치 확인
2. 해당 위치부터 redo log를 순차 적용 (redo phase)
3. 불완전한 트랜잭션 롤백 (undo phase)
4. 완료될 때까지 서비스 불가
```

Checkpoint 이후 축적된 redo log가 많을수록 recovery 시간이 길어진다. 쓰기가 많은 시스템에서 갑작스러운 crash가 발생하면, recovery에 수 분에서 수십 분이 걸릴 수 있다.

Aurora의 crash recovery는 근본적으로 다르다:

```
[Aurora crash recovery]

1. 스토리지 레이어에 이미 모든 redo log가 영속화되어 있음
2. 스토리지 레이어가 지속적으로 redo를 적용하고 있으므로 별도의 redo phase 불필요
3. 컴퓨트 노드 재시작 시 필요한 것:
   - 스토리지에 "가장 최근 커밋된 트랜잭션이 무엇인가?" 확인
   - 불완전한 트랜잭션에 대한 undo 처리
4. undo 처리도 서비스를 시작한 후 백그라운드에서 진행 가능
```

핵심 차이는 **redo phase가 사라진다**는 것이다. 기존 MySQL에서 crash recovery 시간의 대부분을 차지하는 것이 redo phase다. Aurora에서는 스토리지 레이어가 이미 redo를 적용하고 있으므로, 컴퓨트 노드가 재시작될 때 이 과정을 거칠 필요가 없다.

결과적으로 Aurora의 crash recovery 시간은 데이터베이스 크기나 쓰기 양에 거의 영향을 받지 않는다. 컴퓨트 노드 재시작과 undo 처리(백그라운드) 시간만 소요되며, 보통 수 초에서 수십 초 이내에 서비스가 재개된다.

## Log Sequence Number (LSN) 관리

Aurora는 분산 환경에서 데이터 일관성을 유지하기 위해 LSN(Log Sequence Number)을 정밀하게 추적한다.

**VCL (Volume Complete LSN)**: 스토리지 볼륨 전체에서 빈 구멍 없이 연속적으로 적용된 가장 높은 LSN이다. 이 값 이하의 모든 log record가 quorum을 달성했다는 보장이다.

**VDL (Volume Durable LSN)**: VCL 이하인 CPL(Consistency Point LSN) 중 가장 높은 값이다. CPL은 mini-transaction의 완료 경계를 표시하는 LSN으로, VDL은 스토리지에 완전히 영속화된 가장 높은 일관성 지점을 나타낸다.

crash recovery 시, Aurora는 VDL보다 높은 LSN의 log record를 잘라낸다(truncate). 이 record들은 커밋되지 않은 트랜잭션에 속하므로 버려도 된다. 이 truncation이 곧 recovery의 핵심이며, 전체 redo log를 재적용하는 것보다 훨씬 빠르다.

## 스토리지 자동 확장과 축소

Aurora 스토리지는 데이터가 늘어나면 자동으로 확장된다. 최소 10GB에서 시작하여 최대 128TiB까지 증가할 수 있다. Protection group 단위(10GB)로 새로운 segment가 할당되며, 사용자가 수동으로 용량을 지정할 필요가 없다.

축소는 약간 다르다. Aurora MySQL 버전에 따라 동작이 다른데:

- **Aurora MySQL 2.x (MySQL 5.7 호환)**: 2.09.0(2020년 9월) 이후 dynamic resizing을 지원한다. 데이터 삭제 시 스토리지가 자동으로 줄어든다. 그 이전 버전에서는 `OPTIMIZE TABLE`이나 logical dump/restore로 공간을 회수해야 한다.
- **Aurora MySQL 3.x (MySQL 8.0 호환)**: 동일하게 dynamic resizing을 지원한다. 데이터 삭제 시 스토리지가 자동으로 줄어든다. 단, 즉각적이지는 않고 백그라운드에서 점진적으로 수행된다.

## 스토리지 비용 구조

Aurora의 스토리지 비용은 두 가지로 구성된다:

**1. 스토리지 용량 과금**: 실제 사용한 스토리지 양에 대해 GB-month 단위로 과금된다. 6-copy의 비용이 포함된 가격이므로, 사용자가 복제 비용을 별도로 지불하지 않는다.

**2. I/O 과금**: 읽기 I/O와 쓰기 I/O에 대해 요청 수 기준으로 과금된다. 이 부분은 28편에서 자세히 다룬다.

스토리지 용량 과금에서 주의할 점이 있다. Aurora 스토리지의 사용량은 data page, redo log, 임시 데이터 등을 모두 포함한다. 대용량 트랜잭션이 장시간 실행되면 undo log가 쌓여 스토리지 사용량이 일시적으로 급증할 수 있다.

```sql
-- 현재 Aurora 클러스터의 테이블별 스토리지 사용량 확인
SELECT table_schema,
       table_name,
       ROUND((data_length + index_length) / 1024 / 1024, 2) AS size_mb
FROM information_schema.tables
WHERE table_schema NOT IN ('mysql', 'information_schema', 'performance_schema', 'sys')
ORDER BY data_length + index_length DESC;
```

CloudWatch의 `VolumeBytesUsed` 메트릭이 실제 과금 기준에 가장 가까운 값을 제공한다. `information_schema`에서 조회한 값과 다를 수 있는데, redo log와 undo log가 차지하는 공간은 information_schema에 반영되지 않기 때문이다.

## 기존 MySQL 스토리지와의 비교 정리

| 항목 | 기존 MySQL (InnoDB) | Aurora 스토리지 |
|---|---|---|
| 데이터 복제 | MySQL 레벨 (binlog) + 디스크 레벨 (EBS) | 스토리지 레벨 (quorum) |
| 쓰기 단위 | 16KB page + redo log + binlog | redo log record (수백 바이트) |
| Crash recovery | redo log 순차 적용 | redo phase 불필요, undo만 수행 |
| Double write buffer | 필요 | 불필요 |
| Checkpoint | 주기적으로 수행 | 불필요 |
| 용량 관리 | 수동 (디스크 사이즈 지정) | 자동 확장 |
| 최대 용량 | 디스크 크기에 의존 | 128TiB |
| 장애 내성 | 디스크 장애 = 데이터 손실 위험 | AZ 1개 + 노드 1개 동시 장애까지 견딤 |

Aurora의 스토리지 레이어는 "데이터베이스의 하위 절반"을 분리하여 독립적인 분산 시스템으로 만든 것이다. 이 분리 덕분에 컴퓨트 노드는 쿼리 처리에만 집중할 수 있고, 스토리지는 내구성과 가용성에 집중할 수 있다.

## 정리

- Aurora 스토리지는 3개 AZ에 6개 복사본을 유지하며, 4/6 쓰기 quorum으로 AZ 하나 전체 장애를 견딘다.
- 10GB protection group 단위로 데이터를 관리하여, 장애 복구 시 해당 segment만 빠르게 복원한다.
- redo log record만 전송하므로 기존 MySQL 대비 쓰기 증폭이 극적으로 감소한다.
- 스토리지 노드가 백그라운드로 redo log를 적용하여 data page를 생성하므로, crash recovery에서 redo phase가 불필요하다.
- 스토리지는 데이터 증가에 따라 자동 확장되며, 최대 128TiB까지 지원한다.
