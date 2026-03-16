# Aurora 아키텍처

Amazon Aurora는 MySQL 호환 관계형 데이터베이스다. "MySQL 호환"이라는 수식어 때문에 단순히 AWS에서 관리해주는 MySQL로 오해하기 쉽지만, Aurora의 내부 구조는 기존 MySQL과 근본적으로 다르다. 이 차이를 정확히 이해해야 Aurora를 제대로 운영할 수 있다.

## 기존 MySQL의 I/O 경로

InnoDB에서 트랜잭션을 커밋할 때 어떤 일이 일어나는지 다시 살펴본다. 하나의 UPDATE 문이 커밋되면 다음 쓰기가 발생한다:

1. **redo log 쓰기** — WAL(Write-Ahead Logging) 원칙에 따라, 변경 사항을 먼저 redo log에 기록한다
2. **binlog 쓰기** — 복제를 위해 binary log에도 기록한다
3. **data page 쓰기** — 변경된 dirty page를 디스크의 데이터 파일에 flush한다
4. **double write buffer 쓰기** — partial write 방지를 위해 data page를 double write buffer에 먼저 쓴다
5. **메타데이터 쓰기** — 테이블스페이스 메타데이터 등을 갱신한다

복제 환경이라면 여기에 네트워크 전송이 추가된다. redo log와 binlog는 fsync로 디스크에 확정해야 하고, replica로 binlog를 전송한 뒤 ACK를 기다리는 과정(semi-synchronous replication)도 있다.

```
[기존 MySQL 쓰기 경로]

                    ┌─── redo log ──────→ 디스크 (fsync)
                    │
 InnoDB 엔진 ───────┼─── binlog ────────→ 디스크 (fsync)
                    │
                    ├─── double write ──→ 디스크
                    │
                    ├─── data page ─────→ 디스크
                    │
                    └─── binlog ────────→ replica (네트워크)
```

이 구조에서 하나의 트랜잭션 커밋은 최소 4번의 동기적 I/O를 수반한다. 디스크가 로컬에 있기 때문에 I/O 자체는 빠르지만, 복제 환경에서는 네트워크 왕복이 추가되고, 장애 시 데이터 정합성 보장이 복잡해진다.

## Amazon이 해결하려 한 문제

2017년 발표된 Aurora 논문("Amazon Aurora: Design Considerations for High Throughput Cloud-Native Relational Databases")은 기존 MySQL 아키텍처의 문제를 명확히 지적한다:

**네트워크가 병목이다.** 클라우드 환경에서 스토리지는 네트워크를 통해 연결된다. EBS(Elastic Block Store)를 사용하는 RDS MySQL은 모든 I/O가 네트워크를 거친다. 위에서 나열한 4~5종류의 쓰기가 전부 네트워크를 통해 발생한다. 네트워크 대역폭과 지연 시간이 데이터베이스 성능의 상한선이 된다.

**복제가 또 다른 네트워크 부하다.** 고가용성을 위해 multi-AZ로 구성하면, EBS 자체의 복제에 더해 MySQL 레벨의 binlog 복제까지 이중으로 네트워크 트래픽이 발생한다. 스토리지 복제와 데이터베이스 복제가 독립적으로 동작하면서 서로의 존재를 모르는 구조다.

**장애 복구가 느리다.** MySQL의 crash recovery는 마지막 checkpoint 이후의 redo log를 순차적으로 적용하는 과정이다. Checkpoint 간격이 길거나 redo log가 크면 복구 시간이 수십 분에 이를 수 있다.

Aurora의 설계는 이 세 가지 문제를 한 번에 해결하려는 시도다.

## The log is the database

Aurora의 핵심 아이디어는 한 문장으로 요약된다: **네트워크를 통해 redo log record만 전송한다.**

기존 MySQL이 data page, redo log, binlog, double write buffer 등 여러 종류의 데이터를 디스크에 쓰는 것과 달리, Aurora의 컴퓨트 노드는 redo log record만 스토리지 레이어로 전송한다. Data page 쓰기, double write, checkpoint — 이 모든 것이 사라진다.

```
[Aurora 쓰기 경로]

 Aurora 엔진 ───── redo log record ──→ Aurora 스토리지 (네트워크)

 끝.
```

스토리지 레이어가 redo log record를 받으면, 백그라운드에서 해당 log를 적용하여 data page를 생성(materialize)한다. 기존에 MySQL 엔진이 하던 작업 — dirty page flush, checkpoint — 을 스토리지가 대신 수행하는 것이다.

이 구조가 가져오는 변화는 극적이다:

| 항목 | 기존 MySQL | Aurora |
|---|---|---|
| 네트워크로 전송하는 데이터 | data page + redo log + binlog + double write | redo log record만 |
| 네트워크 트래픽 | 많음 | Aurora 논문 기준 약 7.7배 감소 |
| 동기적 I/O 종류 | 4~5가지 | 1가지 (redo log) |
| Double write buffer | 필요 | 불필요 |
| Checkpoint | 필요 | 불필요 |

네트워크 트래픽이 줄어든다는 것은 같은 네트워크 대역폭에서 더 많은 트랜잭션을 처리할 수 있다는 뜻이다. Aurora가 "MySQL 대비 5배 성능"을 주장하는 근거 중 하나가 이 I/O 경로 단순화다.

## 컴퓨트-스토리지 분리

Aurora는 컴퓨트(compute)와 스토리지(storage)를 물리적으로 분리한다. 이것은 EBS를 쓰는 RDS MySQL과 근본적으로 다른 아키텍처다.

```
[Aurora 아키텍처]

 ┌─────────────────────────────────────────────┐
 │              컴퓨트 레이어                     │
 │                                              │
 │  ┌─────────┐   ┌─────────┐   ┌─────────┐    │
 │  │ Writer  │   │ Reader  │   │ Reader  │    │
 │  │ (r6g.xl)│   │ (r6g.xl)│   │ (r6g.lg)│    │
 │  └────┬────┘   └────┬────┘   └────┬────┘    │
 └───────┼─────────────┼─────────────┼──────────┘
         │             │             │
    redo log record    │ (공유 스토리지)
         │             │             │
 ┌───────┼─────────────┼─────────────┼──────────┐
 │       ▼             ▼             ▼          │
 │          Aurora 분산 스토리지 레이어            │
 │                                              │
 │  AZ-a         AZ-b         AZ-c             │
 │  ┌────┐       ┌────┐       ┌────┐           │
 │  │copy│       │copy│       │copy│           │
 │  │ 1  │       │ 3  │       │ 5  │           │
 │  ├────┤       ├────┤       ├────┤           │
 │  │copy│       │copy│       │copy│           │
 │  │ 2  │       │ 4  │       │ 6  │           │
 │  └────┘       └────┘       └────┘           │
 │                                              │
 └──────────────────────────────────────────────┘
```

**컴퓨트 레이어**는 MySQL 호환 엔진(쿼리 파싱, 옵티마이저, 트랜잭션 관리, buffer pool 등)을 담당한다. Writer 인스턴스 1개와 Reader 인스턴스 최대 15개로 구성된다. 각 인스턴스는 독립적인 EC2 머신이다.

**스토리지 레이어**는 데이터의 영속성과 복제를 담당한다. 3개의 AZ(Availability Zone)에 걸쳐 데이터를 6개 복사본으로 유지한다. 스토리지는 클러스터 내 모든 컴퓨트 인스턴스가 공유한다.

이 분리가 가져오는 이점은 크다:

**독립적 확장.** 컴퓨트 성능이 부족하면 인스턴스 타입을 변경하거나 reader를 추가한다. 스토리지 용량은 데이터가 늘어나면 자동으로 확장된다. 컴퓨트 확장과 스토리지 확장이 서로 영향을 주지 않는다.

**공유 스토리지 기반 복제.** Reader 인스턴스는 writer와 같은 스토리지를 바라본다. 전통적인 MySQL 복제에서 필요한 binlog 전송, relay log 적용 과정이 없다. 복제 지연이 구조적으로 최소화된다.

**빠른 failover.** Writer에 장애가 발생하면 reader 중 하나가 writer로 승격된다. 스토리지를 복사할 필요가 없다. 이미 같은 스토리지를 공유하고 있기 때문이다.

## I/O 경로 비교: 상세

기존 MySQL에서 UPDATE 한 건의 커밋 과정을 네트워크 관점에서 더 자세히 비교한다.

### 기존 MySQL (EBS 기반 RDS)

```
1. redo log → EBS (네트워크 I/O + EBS 내부 복제)
2. binlog → EBS (네트워크 I/O + EBS 내부 복제)
3. data page → EBS (네트워크 I/O, 비동기이지만 발생함)
4. double write → EBS (네트워크 I/O)
5. binlog → replica (네트워크 I/O, semi-sync면 ACK 대기)
6. replica에서: relay log → EBS, data page → EBS ...
```

EBS 자체도 AZ 내에서 복제를 수행하므로, 눈에 보이지 않는 네트워크 I/O가 추가로 발생한다. Multi-AZ 구성이면 EBS 복제와 MySQL 복제가 이중으로 작동한다.

### Aurora

```
1. redo log record → 6개 스토리지 노드로 전송 (4/6 ACK를 받으면 커밋 완료)
```

Aurora는 redo log record를 6개의 스토리지 노드에 동시에 전송하고, 그 중 4개로부터 ACK를 받으면 커밋을 완료한다. Data page 쓰기, double write, binlog 전송 — 전부 없다.

왜 double write가 필요 없는가? Double write buffer는 16KB data page를 디스크에 쓰는 도중 crash가 발생하면 일부만 기록되는 partial write를 방지하기 위한 장치다. Aurora는 data page를 네트워크로 전송하지 않으므로 이 문제 자체가 존재하지 않는다. Redo log record는 data page보다 훨씬 작아서(보통 수십~수백 바이트) partial write 위험이 없다.

왜 checkpoint가 필요 없는가? 기존 MySQL에서 checkpoint는 "이 시점까지의 redo log는 data page에 반영되었으니 crash recovery 시 여기서부터 시작하면 된다"는 표시다. Aurora에서는 스토리지 레이어가 지속적으로 redo log를 적용하므로, 별도의 checkpoint 시점을 관리할 필요가 없다.

## 네트워크 기반 스토리지가 로컬 디스크보다 나은 조건

직관적으로 로컬 디스크가 네트워크 스토리지보다 빠를 것 같다. 실제로 단일 I/O의 지연 시간만 보면 로컬 NVMe SSD가 네트워크 스토리지보다 빠르다. 그런데 Aurora는 왜 네트워크 스토리지를 선택했는가?

**I/O 종류의 수가 줄면 총 네트워크 트래픽이 줄어든다.** 기존에 5종류의 데이터를 네트워크로 보내던 것을 1종류로 줄였다. 단일 I/O가 약간 느려지더라도 총량이 극적으로 감소하면 전체 throughput은 올라간다.

**복제와 내구성이 스토리지 레이어에 내장된다.** 로컬 디스크를 쓰면 복제를 데이터베이스 엔진이 직접 관리해야 한다. 네트워크 스토리지에 복제를 위임하면 엔진은 복제를 신경 쓸 필요가 없다.

**장애 도메인이 분리된다.** 컴퓨트 노드의 장애(EC2 인스턴스 문제)가 데이터 손실로 이어지지 않는다. 스토리지 노드의 장애도 quorum 구조 덕분에 데이터 가용성에 영향을 주지 않는다.

단, Aurora의 이 이점은 클라우드 환경에 특화된 것이다. 로컬 NVMe SSD에 직접 쓰는 온프레미스 MySQL이 단일 노드의 raw I/O 성능에서 Aurora를 능가할 수 있다. Aurora의 강점은 단일 I/O 속도가 아니라, 복제-장애복구-확장을 통합한 시스템 수준의 효율성에 있다.

## Aurora MySQL vs Aurora PostgreSQL

Aurora는 MySQL만을 위한 기술이 아니다. 동일한 스토리지 레이어 위에 MySQL 호환 엔진과 PostgreSQL 호환 엔진이 각각 올라간다.

```
┌──────────────┐  ┌──────────────┐
│ Aurora MySQL │  │Aurora Postgre│
│   엔진       │  │   SQL 엔진   │
│ (MySQL 호환) │  │(PostgreSQL   │
│              │  │  호환)       │
└──────┬───────┘  └──────┬───────┘
       │                 │
       ▼                 ▼
┌─────────────────────────────────┐
│     Aurora 분산 스토리지 레이어    │
│     (공통 인프라)                │
└─────────────────────────────────┘
```

스토리지 레이어는 동일하지만, 컴퓨트 엔진이 다르므로 동작 특성이 다르다:

| 항목 | Aurora MySQL | Aurora PostgreSQL |
|---|---|---|
| 호환 버전 | MySQL 5.7, 8.0 | PostgreSQL 13, 14, 15, 16 |
| 최대 reader 수 | 15 | 15 |
| Parallel query | 지원 | 미지원 |
| Backtrack | 지원 | 미지원 |
| MVCC 구현 | undo log 기반 | tuple versioning |

Aurora MySQL이 MySQL과 100% 호환되는 것은 아니다. MyISAM 등 InnoDB 이외의 storage engine은 사용할 수 없다. 이는 Aurora의 스토리지 레이어가 InnoDB의 redo log 형식을 전제로 설계되었기 때문이다. 시스템 테이블 등 일부에서 MyISAM이 사용되기도 하지만, 사용자 데이터는 반드시 InnoDB여야 한다.

## 기존 MySQL과 구조적으로 같은 부분

Aurora가 많은 것을 바꿨지만, 바꾸지 않은 것도 중요하다:

- **SQL 파서와 옵티마이저**: 기존 MySQL과 동일하다. 쿼리 실행 계획은 같은 방식으로 생성된다.
- **Buffer pool**: Aurora의 컴퓨트 노드에도 buffer pool이 있다. Data page를 메모리에 캐싱하는 구조는 동일하다.
- **InnoDB의 행 수준 잠금**: lock 메커니즘은 기존과 같다. Gap lock, next-key lock 등도 동일하게 동작한다.
- **트랜잭션과 MVCC**: undo log를 이용한 MVCC 구현은 기존 InnoDB와 동일하다.
- **인덱스 구조**: B+tree 인덱스, clustered index, secondary index 모두 동일하다.

쿼리 튜닝, 인덱스 설계, 트랜잭션 관리 등 지금까지 배운 InnoDB 지식은 Aurora에서도 그대로 적용된다. 달라지는 것은 I/O 경로, 복제, 장애 복구 등 인프라에 가까운 영역이다.

## 정리

- Aurora는 컴퓨트 노드에서 스토리지 레이어로 redo log record만 전송한다. data page 쓰기, double write, checkpoint가 사라지면서 네트워크 트래픽이 대폭 감소한다.
- 스토리지 레이어는 3개 AZ에 걸쳐 6개 복사본을 유지하며, 4/6 quorum으로 쓰기를 확정한다.
- 컴퓨트와 스토리지가 분리되어 독립적으로 확장할 수 있고, reader는 writer와 같은 스토리지를 공유하므로 binlog 기반 복제가 필요 없다.
- SQL 파서, 옵티마이저, buffer pool, InnoDB의 행 수준 잠금, MVCC, 인덱스 구조 등 쿼리 실행에 관한 부분은 기존 MySQL과 동일하다.
