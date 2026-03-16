# 스키마 변경과 마이그레이션

서비스가 운영되면 스키마는 반드시 변경된다. 컬럼 추가, 인덱스 생성, 데이터 타입 변경. 개발 환경에서는 `ALTER TABLE`을 실행하면 끝이지만, 운영 중인 대용량 테이블에서는 이 작업이 서비스 장애를 유발할 수 있다. 스키마 변경의 내부 동작을 이해하고, 무중단으로 수행하는 방법을 알아야 한다.

## ALTER TABLE의 내부 동작

MySQL에서 `ALTER TABLE`은 세 가지 알고리즘으로 실행될 수 있다.

### COPY 알고리즘

가장 오래된 방식이다. 내부적으로 다음 과정을 거친다:

1. 새로운 스키마의 빈 테이블을 생성한다.
2. 원본 테이블에 shared lock(읽기 허용, 쓰기 차단)을 건다.
3. 원본 데이터를 한 행씩 새 테이블에 복사한다.
4. 원본 테이블을 삭제하고 새 테이블의 이름을 변경한다.

```sql
ALTER TABLE users ADD COLUMN bio TEXT, ALGORITHM=COPY;
```

테이블 전체를 복사하므로 데이터가 클수록 시간이 오래 걸린다. 1억 행 테이블이면 수십 분에서 수 시간이 소요될 수 있다. 복사 중에 shared lock이 걸려 읽기는 가능하지만 DML(INSERT, UPDATE, DELETE)은 차단된다.

### INPLACE 알고리즘

MySQL 5.6에서 도입되었다. 테이블 전체를 복사하지 않고 InnoDB 내부에서 변경을 처리한다:

```sql
ALTER TABLE users ADD COLUMN bio TEXT, ALGORITHM=INPLACE;
```

INPLACE라고 해서 항상 데이터를 복사하지 않는 것은 아니다. 내부적으로 테이블 재구성(table rebuild)이 필요한 경우가 있다. 하지만 COPY와의 핵심 차이는 Online DDL과 결합하여 변경 중에도 DML이 가능하다는 점이다.

### INSTANT 알고리즘

MySQL 8.0.12에서 도입되었다. 메타데이터만 변경하고, 데이터를 전혀 건드리지 않는다:

```sql
ALTER TABLE users ADD COLUMN bio TEXT, ALGORITHM=INSTANT;
```

실행 시간이 테이블 크기와 무관하게 거의 즉시 완료된다. 1억 행 테이블이든 10억 행 테이블이든 밀리초 단위로 끝난다.

하지만 모든 변경에 INSTANT를 사용할 수 있는 것은 아니다:

| 작업 | INSTANT 가능 여부 |
|---|---|
| 컬럼 추가 (마지막 위치) | O (MySQL 8.0.12+) |
| 컬럼 추가 (임의 위치) | O (MySQL 8.0.29+) |
| 컬럼 기본값 변경 | O |
| ENUM/SET 값 추가 | O (목록 끝에 추가 시) |
| 컬럼 삭제 | O (MySQL 8.0.29+) |
| 인덱스 추가/삭제 | X (INPLACE 사용) |
| 컬럼 타입 변경 | X (보통 COPY 필요) |
| 컬럼 순서 변경 | X |

`ALGORITHM=INSTANT`를 명시했는데 해당 작업이 INSTANT를 지원하지 않으면 에러가 발생한다. 명시하지 않으면 MySQL이 INSTANT -> INPLACE -> COPY 순서로 가능한 알고리즘을 자동 선택한다.

## Online DDL

MySQL 5.6부터 도입된 Online DDL은 DDL 실행 중에도 DML을 허용하는 메커니즘이다. `LOCK` 옵션으로 제어한다:

```sql
ALTER TABLE users ADD INDEX idx_email (email), LOCK=NONE;
```

- `LOCK=NONE`: DDL 중에도 읽기와 쓰기 모두 허용된다.
- `LOCK=SHARED`: DDL 중 읽기는 허용, 쓰기는 차단된다.
- `LOCK=EXCLUSIVE`: DDL 중 읽기와 쓰기 모두 차단된다.
- `LOCK=DEFAULT`: MySQL이 가능한 가장 낮은 수준의 락을 자동 선택한다.

Online DDL이 가능한 주요 작업:

| 작업 | 알고리즘 | DML 허용 |
|---|---|---|
| 인덱스 추가 | INPLACE | O |
| 인덱스 삭제 | INPLACE | O |
| 컬럼 추가 (마지막) | INSTANT | O |
| 컬럼 기본값 변경 | INSTANT | O |
| VARCHAR 길이 증가 (256 미만 내) | INPLACE | O |
| 컬럼 타입 변경 | COPY | X |
| PRIMARY KEY 변경 | COPY | X |

Online DDL이라도 시작과 끝에 잠시 metadata lock이 필요하다. DDL이 시작되는 순간 테이블의 metadata lock을 획득해야 하고, 이때 해당 테이블에 대한 장시간 트랜잭션이 열려 있으면 metadata lock 대기가 발생한다. 이 대기는 이후의 모든 쿼리를 차단한다.

```sql
-- 이런 상황이 위험하다:
-- 세션 1: 장시간 트랜잭션이 열려 있음
START TRANSACTION;
SELECT * FROM users WHERE user_id = 1;
-- (커밋하지 않고 방치)

-- 세션 2: ALTER TABLE 실행
ALTER TABLE users ADD INDEX idx_name (name);
-- metadata lock 대기 상태

-- 세션 3: 일반 쿼리
SELECT * FROM users WHERE user_id = 2;
-- 세션 2가 대기 중이므로, 이 쿼리도 대기
```

Online DDL을 실행하기 전에 장시간 열려 있는 트랜잭션이 없는지 확인해야 한다:

```sql
SELECT * FROM information_schema.INNODB_TRX
WHERE TIME_TO_SEC(TIMEDIFF(NOW(), trx_started)) > 60;
```

## 대용량 테이블 ALTER의 위험 요소

### 락

위에서 설명한 metadata lock 문제다. Online DDL이 가능한 작업이라도, 시작 시점에 metadata lock을 획득하지 못하면 모든 후속 쿼리가 차단된다. 피크 시간대에 ALTER TABLE을 실행하면 서비스 장애로 이어질 수 있다.

### 디스크 공간

INPLACE 알고리즘으로 테이블을 재구성(컬럼 타입 변경 등)할 때는 원본 테이블과 동일한 크기의 임시 공간이 필요하다. 반면 인덱스 추가는 테이블 재구성이 아니므로 인덱스 크기에 비례하는 임시 공간만 필요하다. 어느 경우든 디스크 여유 공간이 부족하면 ALTER가 실패하고, 디스크가 꽉 차면 서비스 전체가 멈출 수 있다.

### 복제 지연

MySQL 복제 환경에서 ALTER TABLE은 source에서 실행된 후 replica에 전파된다. source에서 30분 걸린 ALTER는 replica에서도 30분이 걸린다. 이 동안 replica의 복제가 지연된다.

replica에서 읽기를 분산하는 구조라면, 복제 지연 중에 replica의 데이터가 source와 다른 상태가 된다. 읽기 쿼리가 오래된 데이터를 반환하거나, 새로운 컬럼을 참조하는 쿼리가 실패할 수 있다.

Statement-Based Replication(SBR)에서는 ALTER TABLE 문 자체가 replica에서 재실행된다. Row-Based Replication(RBR)이라도 DDL은 statement로 복제되므로, 대용량 테이블의 ALTER는 replica에서도 동일한 시간이 소요된다.

## pt-online-schema-change

Percona Toolkit의 `pt-online-schema-change`(pt-osc)는 대용량 테이블의 스키마를 무중단으로 변경하는 도구다.

### 동작 원리

1. 새로운 스키마의 빈 테이블(`_tablename_new`)을 생성한다.
2. 원본 테이블에 INSERT, UPDATE, DELETE 트리거를 생성한다. 원본에 가해지는 모든 DML이 새 테이블에도 반영된다.
3. 원본 데이터를 청크 단위로 새 테이블에 복사한다.
4. 복사가 완료되면 원본 테이블과 새 테이블의 이름을 교체한다(RENAME TABLE).
5. 트리거와 원본 테이블을 삭제한다.

```bash
pt-online-schema-change \
    --alter "ADD COLUMN bio TEXT" \
    --execute \
    D=mydb,t=users
```

### 장점과 한계

장점:

- 변경 중에도 DML이 가능하다.
- 청크 크기를 조절하여 서버 부하를 제어할 수 있다.
- `--max-load` 옵션으로 서버 부하가 높으면 자동으로 작업을 일시 중지한다.

한계:

- 트리거 기반이므로, 원본 테이블에 이미 트리거가 있으면 사용할 수 없다.
- DML마다 트리거가 실행되므로 쓰기 성능이 저하된다.
- FK가 있는 테이블에서 추가 고려사항이 있다.
- RENAME TABLE 시점에 짧은 순간 테이블이 잠긴다.

## gh-ost

GitHub에서 개발한 스키마 변경 도구다. pt-osc의 트리거 기반 접근의 한계를 해결하기 위해 만들어졌다.

### 동작 원리

1. 새로운 스키마의 빈 테이블(`_tablename_gho`)을 생성한다.
2. 원본 데이터를 청크 단위로 새 테이블에 복사한다.
3. MySQL binary log를 읽어서 복사 중에 발생한 DML 변경을 새 테이블에 적용한다.
4. 복사와 binlog 적용이 완료되면 테이블 이름을 교체한다.

pt-osc와의 핵심 차이: 트리거 대신 binlog를 사용한다.

```bash
gh-ost \
    --alter "ADD COLUMN bio TEXT" \
    --database mydb \
    --table users \
    --execute
```

### pt-osc와의 비교

| 특성 | pt-osc | gh-ost |
|---|---|---|
| 변경 감지 방식 | 트리거 | binlog |
| 기존 트리거 공존 | 불가 | 가능 |
| 쓰기 성능 영향 | 트리거로 인한 오버헤드 | 최소 (binlog는 어차피 기록됨) |
| 작업 제어 | 제한적 | 실행 중 일시 중지, 속도 조절 가능 |
| 복잡도 | 낮음 | 높음 (binlog 접근 필요) |
| FK 지원 | 제한적 | 미지원 |

gh-ost는 실행 중에도 동적으로 제어할 수 있다. 속도를 높이거나 낮추고, 일시 중지했다가 재개할 수 있다. 피크 시간에 일시 중지하고 트래픽이 줄면 재개하는 운영이 가능하다.

gh-ost를 사용하려면 binlog 형식이 ROW여야 한다. MIXED나 STATEMENT 형식에서는 동작하지 않는다.

## 무중단 마이그레이션 전략

스키마 변경이 단순한 DDL로 끝나지 않는 경우가 있다. 컬럼 이름 변경, 데이터 형식 변경, 테이블 분리 등 애플리케이션 코드 수정이 함께 필요한 변경은 여러 단계에 걸쳐 진행해야 한다.

### 컬럼 추가

가장 단순한 경우. 새 컬럼을 추가하고, 애플리케이션이 해당 컬럼을 사용하도록 배포한다:

1. `ALTER TABLE users ADD COLUMN bio TEXT` (INSTANT 가능)
2. 애플리케이션 코드에서 `bio` 컬럼을 사용하도록 변경 후 배포

INSTANT를 지원하는 컬럼 추가는 별도 도구 없이 `ALTER TABLE`만으로 충분하다.

### 컬럼 타입 변경

`VARCHAR(100)`을 `VARCHAR(500)`으로 변경하는 경우. 256바이트 경계를 넘지 않는 범위 내의 VARCHAR 확장은 INPLACE로 가능하다:

```sql
-- VARCHAR(100) -> VARCHAR(200): INPLACE, Online 가능
ALTER TABLE users MODIFY COLUMN name VARCHAR(200), ALGORITHM=INPLACE, LOCK=NONE;
```

256바이트 경계를 넘는 확장(예: VARCHAR(200) -> VARCHAR(500))은 길이 저장에 필요한 바이트가 1바이트에서 2바이트로 바뀌므로 COPY가 필요하다. 이 경우 pt-osc이나 gh-ost를 사용한다.

INT를 BIGINT로 변경하는 등 데이터 타입 자체를 바꾸는 경우도 COPY가 필요하다.

### 컬럼 이름 변경 (expand and contract)

운영 중인 컬럼의 이름을 바꾸려면, 한 번에 변경하면 이전 코드와 호환되지 않는다. expand and contract 패턴을 사용한다:

1. **Expand**: 새 컬럼을 추가한다. 기존 컬럼은 유지한다.
2. **Migrate**: 기존 데이터를 새 컬럼에 복사한다.
3. **Dual write**: 애플리케이션이 두 컬럼 모두에 쓰도록 변경한다.
4. **Switch read**: 읽기를 새 컬럼으로 전환한다.
5. **Contract**: 기존 컬럼에 대한 쓰기를 중단하고, 이후 기존 컬럼을 삭제한다.

```sql
-- 1. Expand: 새 컬럼 추가
ALTER TABLE users ADD COLUMN full_name VARCHAR(200);

-- 2. Migrate: 기존 데이터 복사
UPDATE users SET full_name = name WHERE full_name IS NULL LIMIT 10000;
-- (배치로 반복 실행)

-- 3~4. 애플리케이션 코드 변경 및 배포

-- 5. Contract: 기존 컬럼 삭제
ALTER TABLE users DROP COLUMN name;
```

이 과정은 여러 배포에 걸쳐 진행된다. 한 번의 배포로 끝내려 하면 다운타임이 발생한다.

## 컬럼 변경 시 주의사항

### 컬럼 추가

- MySQL 8.0.12+ 에서 마지막 위치 추가는 INSTANT. 가장 안전하다.
- `DEFAULT` 값을 지정하면 INSTANT에서도 기존 행이 새 기본값을 가진 것처럼 동작한다. 실제로 각 행을 수정하지 않는다.

### 컬럼 삭제

- MySQL 8.0.29+ 에서 INSTANT 가능.
- 삭제할 컬럼을 참조하는 인덱스, FK, generated column이 있으면 먼저 제거해야 한다.
- 삭제 전에 해당 컬럼을 사용하는 애플리케이션 코드가 모두 제거되었는지 확인한다. `SELECT *`를 사용하는 코드가 있다면, 컬럼 삭제 후 결과 매핑이 깨질 수 있다.

### 타입 변경

- 대부분 COPY 알고리즘이 필요하다. 대용량 테이블에서는 pt-osc이나 gh-ost를 사용한다.
- 데이터 손실 가능성을 확인한다. INT를 SMALLINT로 축소하면 범위를 초과하는 값이 잘린다.
- 변경 전에 현재 데이터가 새 타입에 적합한지 검증한다:

```sql
-- INT -> SMALLINT 변환 전 검증
SELECT COUNT(*) FROM users
WHERE age > 32767 OR age < -32768;
```

### 인덱스 추가/삭제

- INPLACE, Online으로 가능하다. DML이 차단되지 않는다.
- 인덱스 생성 중 임시 디스크 공간이 필요하다. `innodb_tmpdir` 설정으로 빠른 디스크를 지정할 수 있다.
- 인덱스 생성 시간은 테이블 크기에 비례한다. 수억 행이면 수십 분이 걸릴 수 있다.

## 마이그레이션 체크리스트

운영 환경에서 스키마 변경을 실행하기 전에 확인할 항목:

**사전 확인:**

- 변경 대상 테이블의 크기(행 수, 데이터 크기)를 확인한다.
- 사용 가능한 알고리즘(INSTANT, INPLACE, COPY)을 확인한다.
- 디스크 여유 공간이 테이블 크기 이상인지 확인한다.
- 장시간 열려 있는 트랜잭션이 없는지 확인한다.
- 복제 환경이면 현재 복제 지연이 없는지 확인한다.

**실행 계획:**

- INSTANT가 가능하면 직접 ALTER TABLE을 실행한다.
- INPLACE Online이 가능하면 ALTER TABLE에 `ALGORITHM=INPLACE, LOCK=NONE`을 명시한다.
- COPY가 필요한 대용량 테이블이면 pt-osc 또는 gh-ost를 사용한다.
- 피크 시간을 피해 실행한다.

**실행 중 모니터링:**

- 서버 부하(CPU, I/O, 스레드 수)를 관찰한다.
- 복제 지연을 모니터링한다.
- metadata lock 대기가 발생하지 않는지 확인한다:

```sql
SELECT * FROM performance_schema.metadata_locks
WHERE OBJECT_SCHEMA = 'mydb' AND OBJECT_NAME = 'users';
```

**롤백 계획:**

- INSTANT 변경은 반대 작업(예: 추가한 컬럼 삭제)으로 롤백할 수 있다.
- pt-osc이나 gh-ost는 원본 테이블이 보존되므로 RENAME으로 롤백할 수 있다.
- COPY 알고리즘의 직접 ALTER는 완료 후 롤백이 어렵다. 사전에 백업을 확보한다.

스키마 변경은 기능 개발만큼이나 중요한 운영 작업이다. 테이블이 작을 때는 아무 변경이나 즉시 실행해도 되지만, 운영 데이터가 쌓이면 DDL 하나에도 서비스 장애가 발생할 수 있다. 변경의 내부 동작을 이해하고, 안전한 실행 경로를 선택하는 것이 핵심이다.

## 정리

- ALTER TABLE은 INSTANT, INPLACE, COPY 세 가지 알고리즘이 있으며, INSTANT가 가장 빠르고 안전하다.
- Online DDL은 DDL 중에도 DML을 허용하지만, 시작과 끝에 metadata lock이 필요하므로 장시간 트랜잭션이 있으면 전체 쿼리가 차단될 수 있다.
- 대용량 테이블의 스키마 변경은 pt-online-schema-change나 gh-ost를 사용하여 무중단으로 수행한다.
- 컬럼 이름 변경 같은 호환성 문제가 있는 변경은 expand and contract 패턴으로 여러 배포에 걸쳐 진행한다.
- 운영 환경에서 DDL을 실행하기 전에 테이블 크기, 알고리즘, 디스크 여유 공간, 장시간 트랜잭션 유무를 확인해야 한다.
