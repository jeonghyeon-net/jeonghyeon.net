# 복제와 읽기 분산

기존 MySQL 복제는 binlog를 네트워크로 전송하고, replica에서 relay log로 받아 재실행하는 구조다. Aurora는 이 방식을 사용하지 않는다. Writer와 reader가 동일한 스토리지를 공유하므로, 복제의 의미 자체가 달라진다.

## 기존 MySQL 복제의 구조

기존 MySQL의 복제 흐름을 먼저 정리한다.

```
[기존 MySQL 복제]

Source (Writer)                    Replica (Reader)
┌────────────┐                    ┌────────────┐
│            │  binlog 전송        │            │
│  binlog  ──┼───────────────────→│ relay log  │
│            │                    │     │      │
│  data      │                    │  SQL thread│
│  (InnoDB)  │                    │     │      │
│            │                    │  data      │
│            │                    │  (InnoDB)  │
└────────────┘                    └────────────┘
  자체 디스크                        자체 디스크
```

### 비동기 복제 (Asynchronous)

기본 모드다. Source가 binlog를 기록하고, replica의 I/O thread가 이를 가져와 relay log에 쓴다. SQL thread가 relay log를 읽어 재실행한다. Source는 replica의 처리 완료를 기다리지 않는다.

- 장점: source의 성능에 영향이 없다
- 단점: source 장애 시 아직 전송되지 않은 binlog의 데이터 유실 가능성이 있다

### 반동기 복제 (Semi-synchronous)

Source가 커밋할 때, 최소 1개의 replica가 binlog를 relay log에 기록했다는 ACK를 보낼 때까지 기다린다. Replica가 실제로 해당 트랜잭션을 적용(실행)했는지까지는 확인하지 않는다.

- 장점: source 장애 시에도 최소 1개의 replica에 데이터가 존재한다
- 단점: ACK 대기로 인해 커밋 지연 시간이 증가한다. 네트워크 지연이 직접 반영된다

### 복제 지연 (Replication lag)

두 방식 모두 replica가 source의 변경 사항을 "따라가는" 구조이므로 복제 지연이 발생한다. Source에서 대량의 쓰기가 발생하면, replica의 SQL thread가 처리 속도를 따라가지 못해 지연이 수 초에서 수 분까지 벌어질 수 있다.

복제 지연이 클 때 replica에서 읽기를 수행하면 stale data를 읽게 된다. 예를 들어 사용자가 주문을 넣고(writer에 INSERT) 바로 주문 내역을 조회하면(reader에서 SELECT), 복제가 아직 따라오지 못한 경우 주문이 보이지 않는 현상이 발생한다.

## Aurora 복제: 공유 스토리지 기반

Aurora의 복제는 근본적으로 다르다. Writer와 reader가 같은 물리적 스토리지를 바라본다.

```
[Aurora 복제]

Writer 인스턴스          Reader 인스턴스 #1     Reader 인스턴스 #2
┌──────────┐            ┌──────────┐           ┌──────────┐
│ buffer   │            │ buffer   │           │ buffer   │
│ pool     │            │ pool     │           │ pool     │
│          │            │          │           │          │
└────┬─────┘            └────┬─────┘           └────┬─────┘
     │                       │                      │
     │      redo log record  │                      │
     ├──────────────────────→│ (cache invalidation) │
     ├───────────────────────┼─────────────────────→│
     │                       │                      │
     ▼                       ▼                      ▼
┌────────────────────────────────────────────────────────┐
│              Aurora 공유 스토리지 레이어                  │
└────────────────────────────────────────────────────────┘
```

Reader는 data를 스토리지에서 직접 읽는다. Binlog를 받아서 재실행하는 과정이 없다. Writer가 스토리지에 쓴 redo log가 스토리지 레이어에 의해 data page에 반영되면, reader가 해당 page를 읽을 때 자동으로 최신 데이터를 얻는다.

그렇다면 reader의 buffer pool에 캐싱된 오래된 page는 어떻게 되는가?

## Writer에서 Reader로의 데이터 전파 경로

Writer가 data page를 변경하면, 해당 변경 사항이 reader에 반영되는 경로는 두 가지다:

### 1. Cache invalidation (redo log shipment)

Writer는 redo log record를 스토리지에 보내는 것과 동시에, 각 reader 인스턴스에도 전송한다. Reader는 이 log record를 받아서 자신의 buffer pool에서 해당 page를 무효화(invalidate)하거나, log를 적용하여 page를 갱신한다.

```
Writer: page X 변경 (redo log record 생성)
    │
    ├──→ 스토리지: redo log 영속화
    │
    └──→ Reader #1: "page X가 변경됨"
         │
         ├─ buffer pool에 page X가 있으면 → 무효화 또는 갱신
         └─ buffer pool에 page X가 없으면 → 아무것도 안 함
```

이 과정에서 reader는 redo log를 "적용"하지만, 기존 MySQL 복제처럼 SQL 문을 재실행하는 것이 아니라 buffer pool의 캐시를 갱신하는 것이다. 실제 데이터는 이미 공유 스토리지에 있다.

### 2. 스토리지에서 직접 읽기

Reader의 buffer pool에 해당 page가 없으면(cache miss), 스토리지에서 직접 page를 읽어온다. 이 page는 스토리지 레이어에 의해 최신 redo log가 적용된 상태이므로, 최신 데이터다.

## Replica lag 특성

Aurora의 replica lag는 기존 MySQL과 성격이 다르다.

기존 MySQL에서 replica lag는 "binlog를 얼마나 밀려서 적용하고 있는가"다. SQL thread가 relay log를 재실행하는 속도에 의존하므로, 대용량 DDL이나 heavy write 구간에서 수 분 이상 지연될 수 있다.

Aurora에서 replica lag는 "writer가 보낸 cache invalidation이 reader에 반영되기까지의 시간"이다. 데이터 자체는 공유 스토리지에 이미 있으므로, 지연의 의미가 다르다.

일반적인 수치:
- 평균 replica lag: 10~20ms
- 대부분의 경우 100ms 미만

하지만 이 수치는 보장이 아니다. 다음 상황에서 lag가 증가할 수 있다:

- **Reader의 buffer pool 압박**: Reader가 메모리 부족으로 page eviction이 빈번하면, cache invalidation 처리가 지연될 수 있다
- **대량의 쓰기 부하**: Writer가 초당 수만 건의 변경을 발생시키면, reader가 cache invalidation을 처리하는 속도가 따라가지 못할 수 있다
- **Reader의 CPU 부하**: Reader에서 무거운 쿼리가 실행 중이면 invalidation 처리 thread가 CPU를 할당받지 못할 수 있다

CloudWatch 메트릭 `AuroraReplicaLag`로 실시간 모니터링이 가능하다. 이 값이 지속적으로 수백 ms 이상이면 reader 인스턴스 사이징이나 쓰기 패턴을 점검해야 한다.

## Endpoint 구조

Aurora는 여러 인스턴스에 대한 접근을 endpoint로 추상화한다.

### Cluster endpoint (writer endpoint)

항상 현재 writer 인스턴스를 가리키는 DNS다. 쓰기 작업과 최신 데이터가 필요한 읽기에 사용한다. Failover가 발생하면 이 endpoint가 새로운 writer를 가리키도록 DNS가 업데이트된다.

```text
mydb-cluster.cluster-xxxx.us-east-1.rds.amazonaws.com
→ 현재 writer 인스턴스의 IP
```

### Reader endpoint

Reader 인스턴스들에 대한 로드 밸런서 역할을 하는 DNS다. 연결 시점에 가용한 reader 중 하나로 라우팅된다.

```text
mydb-cluster.cluster-ro-xxxx.us-east-1.rds.amazonaws.com
→ reader 인스턴스 중 하나의 IP (라운드 로빈)
```

주의: reader endpoint는 연결(connection) 수준에서 라우팅한다. 한 번 연결되면 해당 connection은 같은 reader에 고정된다. 쿼리 단위로 분산되는 것이 아니다. Connection pool을 사용하는 경우, pool의 connection 수가 충분해야 여러 reader에 고르게 분산된다.

### Custom endpoint

특정 reader 인스턴스들을 그룹으로 묶어 만드는 endpoint다. 용도별로 분리할 때 유용하다.

```text
예시:
- analytics-endpoint → r6g.4xlarge 인스턴스 2개 (분석 쿼리용)
- api-endpoint → r6g.xlarge 인스턴스 3개 (API 읽기용)
```

무거운 분석 쿼리가 API 응답용 reader의 성능에 영향을 주는 것을 방지할 수 있다.

### Instance endpoint

각 인스턴스의 고유 endpoint다. 특정 인스턴스에 직접 연결할 때 사용한다. 디버깅이나 모니터링 목적으로 사용하며, 애플리케이션 코드에서 직접 참조하는 것은 권장되지 않는다.

## Failover 동작

### 비계획 failover (장애 감지)

Writer 인스턴스에 장애가 발생하면 다음 과정이 진행된다:

```text
1. 장애 감지 (수 초)
   └─ Aurora 컨트롤 플레인이 writer의 health check 실패 감지

2. Writer 승격 (수 초)
   └─ 기존 reader 중 하나를 새 writer로 승격
   └─ 승격 우선순위: failover priority 설정에 따름 (tier-0 > tier-1 > ...)
   └─ 같은 tier 내에서는 reader 중 크기가 가장 큰 인스턴스 우선

3. DNS 업데이트 (수 초)
   └─ cluster endpoint가 새 writer의 IP를 가리키도록 변경
   └─ 기존 writer의 DNS 레코드 제거

4. 클라이언트 재연결 (애플리케이션 의존)
   └─ 기존 연결은 끊어짐
   └─ 새 연결은 cluster endpoint를 통해 새 writer에 연결
```

전체 과정은 보통 30초 이내에 완료된다. Reader 인스턴스가 없으면(writer만 있는 단일 인스턴스 구성) 새 인스턴스를 생성해야 하므로 수 분이 걸릴 수 있다.

Failover 시간을 최소화하려면:

- Reader 인스턴스를 최소 1개 유지한다 (같은 AZ 또는 다른 AZ)
- Failover priority를 설정하여 승격 대상을 명시한다
- Failover 대상 reader의 인스턴스 타입을 writer와 동일하게 유지한다. 작은 인스턴스가 writer로 승격되면 성능 저하가 발생한다

### 계획된 failover

`aws rds failover-db-cluster` 명령이나 콘솔에서 수동으로 failover를 트리거할 수 있다. 유지보수, 인스턴스 타입 변경 테스트, failover 절차 검증 등에 사용한다.

계획된 failover는 비계획 failover보다 빠르다. 기존 writer가 정상 상태이므로, 진행 중인 트랜잭션을 정리하고 깨끗하게 전환할 수 있다.

### 애플리케이션 레벨의 failover 대응

Failover가 발생하면 기존 connection은 끊어진다. 애플리케이션이 이를 처리하지 못하면 에러가 발생한다.

대응 방법:

**1. 재시도 로직**: Connection 에러 시 cluster endpoint로 재연결을 시도한다. DNS TTL이 만료된 후 새 writer의 IP를 얻을 수 있다.

**2. DNS 캐싱 주의**: JVM 등 일부 런타임은 DNS 결과를 오래 캐싱한다. Failover 후에도 기존 IP로 연결을 시도하면 실패한다. Java의 경우 `networkaddress.cache.ttl`을 짧게 설정해야 한다.

```java
// JVM DNS 캐시 TTL을 5초로 설정
java.security.Security.setProperty("networkaddress.cache.ttl", "5");
```

**3. Aurora 전용 드라이버**: AWS에서 제공하는 JDBC/Python 드라이버(aws-advanced-jdbc-wrapper, aws-advanced-python-wrapper 등)는 Aurora의 topology를 인식하고, failover를 자동으로 처리한다. Cluster endpoint의 DNS 변경을 기다리지 않고, 내부적으로 인스턴스 목록을 관리하여 즉시 새 writer에 연결한다. 일반 MySQL 드라이버보다 failover 시간이 단축된다.

## Multi-AZ 구성의 의미

기존 RDS MySQL의 multi-AZ는 standby replica를 다른 AZ에 두는 것이다. Standby는 읽기 트래픽을 받지 않으며, failover 전까지 유휴 상태다.

Aurora에서 multi-AZ는 reader 인스턴스를 다른 AZ에 배치하는 것이다. Aurora의 reader는 항상 읽기 트래픽을 처리할 수 있으므로, standby와 달리 자원이 낭비되지 않는다.

```
AZ-a                  AZ-b                  AZ-c
┌──────────┐          ┌──────────┐          ┌──────────┐
│  Writer  │          │ Reader#1 │          │ Reader#2 │
│          │          │(failover │          │          │
│          │          │ priority │          │          │
│          │          │  = 0)    │          │          │
└──────────┘          └──────────┘          └──────────┘
     │                     │                     │
     ▼                     ▼                     ▼
┌────────────────────────────────────────────────────┐
│              공유 스토리지 (3 AZ x 2 copy)           │
└────────────────────────────────────────────────────┘
```

Writer가 AZ-a에서 장애가 발생하면, AZ-b의 Reader#1이 writer로 승격된다. 스토리지는 이미 3개 AZ에 복제되어 있으므로 데이터 이동이 필요 없다.

한계도 있다. Aurora의 multi-AZ는 리전 내 가용성만 보장한다. 리전 전체 장애에는 대응할 수 없다. 리전 간 복제가 필요하면 Aurora Global Database를 사용해야 한다.

## Aurora Global Database

Aurora Global Database는 cross-region 복제를 제공한다. Primary 리전의 스토리지 레이어에서 secondary 리전의 스토리지 레이어로 redo log를 전송하는 방식이다.

```
Primary 리전 (us-east-1)           Secondary 리전 (ap-northeast-2)
┌─────────────────────┐            ┌─────────────────────┐
│ Writer + Readers    │            │ Readers             │
│                     │            │                     │
│ ┌─────────────────┐ │            │ ┌─────────────────┐ │
│ │  스토리지 레이어  │──┼── redo ──→│ │  스토리지 레이어  │ │
│ └─────────────────┘ │   log     │ └─────────────────┘ │
└─────────────────────┘            └─────────────────────┘
```

### 특징

- **복제 지연**: 보통 1초 이내 (cross-region 네트워크 지연에 의존)
- **스토리지 레벨 복제**: 컴퓨트 엔진을 거치지 않으므로 primary의 성능에 미치는 영향이 최소화된다
- **Secondary에서 읽기 가능**: Secondary 리전의 reader 인스턴스에서 읽기 쿼리를 처리할 수 있다
- **Cross-region failover**: Primary 리전 장애 시 secondary 리전을 primary로 승격할 수 있다. 수동 절차가 필요하며, 보통 1~2분이 소요된다

### 주의사항

- Secondary 리전에서는 쓰기가 불가능하다 (headless 모드 제외). 모든 쓰기는 primary 리전으로 보내야 한다
- Write forwarding 기능을 사용하면 secondary 리전에서 쓰기 쿼리를 실행할 수 있지만, 내부적으로 primary 리전으로 전달되므로 지연 시간이 추가된다
- Cross-region failover는 자동이 아니다. Planned failover(switchover)와 unplanned failover(detach and promote) 두 가지 방식이 있다

## 기존 MySQL 복제 vs Aurora 복제 비교

| 항목 | 기존 MySQL 복제 | Aurora 복제 |
|---|---|---|
| 복제 단위 | binlog event (SQL/Row) | redo log record (cache invalidation) |
| 데이터 전송 | 네트워크를 통한 전체 변경 사항 | 스토리지 공유, 메타데이터만 전송 |
| Replica의 저장소 | 독립적인 디스크 | 공유 스토리지 |
| 일반적인 lag | 수백 ms ~ 수 분 | 10~20ms |
| Replica 추가 시간 | 데이터 복사 필요 (수 시간) | 스토리지 공유로 수 분 |
| 최대 replica 수 | 제한 없음 (실질적으로 5~10개) | 15 |
| Failover 시 데이터 손실 | 가능 (비동기 복제 시) | 불가능 (공유 스토리지) |
| Cross-region | binlog 기반 | Global Database (스토리지 레벨) |

Aurora의 공유 스토리지 복제는 replica 추가가 빠르고, 복제 지연이 작고, failover 시 데이터 손실이 없다는 구조적 이점이 있다. 다만 최대 15개라는 reader 수 제한과, binlog 기반 외부 복제가 기본적으로 비활성화되어 있다는 점은 기존 MySQL 복제와 다른 운영 전략을 요구한다.

## 정리

- Aurora 복제는 공유 스토리지 기반이며, binlog 전송 없이 cache invalidation만으로 reader를 갱신한다.
- 복제 지연은 일반적으로 10~20ms 수준이며, reader의 buffer pool 압박이나 대량 쓰기 시 증가할 수 있다.
- Reader endpoint는 연결(connection) 수준에서 라우팅하므로, 쿼리 단위 분산이 아닌 커넥션 단위 분산이다.
- Failover는 보통 30초 이내에 완료되며, reader를 최소 1개 유지하고 failover priority를 설정해야 빠른 복구가 가능하다.
- Global Database는 cross-region 복제를 스토리지 레벨에서 제공하며, 보통 1초 이내의 복제 지연을 보인다.
