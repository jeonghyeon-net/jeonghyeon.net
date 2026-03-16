# 행과 페이지

InnoDB는 데이터를 16KB 페이지 단위로 관리한다. 모든 디스크 I/O는 페이지 단위로 이루어진다. 쿼리 성능을 이해하려면 행(row)이 페이지 안에 어떻게 배치되고, 페이지가 어떻게 구성되는지 알아야 한다.

## 페이지의 구조

InnoDB의 **페이지**(page)는 16KB(16,384 bytes) 크기의 고정 블록이다. 디스크에서 데이터를 읽을 때 최소 단위가 이 페이지다. 행 하나만 필요해도 16KB 전체를 읽는다.

하나의 페이지는 다음과 같은 영역으로 구성된다:

```
┌──────────────────────────────────────┐
│ File Header (38 bytes)               │  ← 페이지 번호, 타입, 체크섬
├──────────────────────────────────────┤
│ Page Header (56 bytes)               │  ← 행 수, 여유 공간 포인터
├──────────────────────────────────────┤
│ Infimum / Supremum Records           │  ← 가상의 최소/최대 레코드
├──────────────────────────────────────┤
│                                      │
│ User Records (행 데이터)              │  ← 실제 데이터가 저장되는 영역
│                                      │
│         ↓ (아래로 채워짐)              │
│                                      │
│         ↑ (위로 채워짐)               │
│                                      │
│ Free Space                           │  ← 아직 사용되지 않은 공간
│                                      │
├──────────────────────────────────────┤
│ Page Directory                       │  ← 행 검색을 위한 슬롯 배열
├──────────────────────────────────────┤
│ File Trailer (8 bytes)               │  ← 무결성 검증용 체크섬
└──────────────────────────────────────┘
```

**File Header**는 페이지의 메타데이터를 담는다. 페이지 번호, 이 페이지가 어떤 타입인지(데이터 페이지, 인덱스 페이지, undo 페이지 등), 이전/다음 페이지의 포인터가 포함된다. 데이터 페이지들은 이 포인터를 통해 **이중 연결 리스트**로 연결된다.

**Page Header**는 이 페이지에 저장된 행의 수, 여유 공간의 시작 위치, 삭제된 행의 목록 등을 관리한다.

**Infimum과 Supremum**은 실제 데이터가 아닌 가상의 레코드다. Infimum은 이 페이지에서 가장 작은 키보다 작은 값을, Supremum은 가장 큰 키보다 큰 값을 나타낸다. 페이지 내에서 행을 탐색할 때 경계 역할을 한다.

**User Records** 영역에 실제 행 데이터가 저장된다. 새 행이 삽입되면 이 영역이 아래로 확장되고, **Page Directory**는 위로 확장된다. 둘 사이의 **Free Space**가 0에 가까워지면 페이지가 가득 찬 것이다.

**Page Directory**는 페이지 내에서 특정 행을 빠르게 찾기 위한 슬롯 배열이다. 행들은 primary key 순서로 단방향 연결 리스트를 구성하는데, 처음부터 순차 탐색하면 느리다. Page Directory의 각 슬롯이 4~8개의 행 그룹을 가리키고 있어서, 이진 탐색으로 원하는 그룹을 찾은 뒤 그 안에서 순차 탐색하는 방식으로 검색 속도를 높인다.

16KB에서 헤더, 트레일러, Infimum/Supremum, Page Directory 등을 제외하면 실제 행 데이터에 사용할 수 있는 공간은 약 15KB 내외다.

## 행이 페이지 안에 저장되는 방식

페이지 안에서 행들은 primary key 순서로 정렬된 단방향 연결 리스트를 형성한다. 각 행은 다음 행을 가리키는 포인터(next record offset)를 갖는다.

```sql
CREATE TABLE users (
    id INT PRIMARY KEY,
    name VARCHAR(50),
    email VARCHAR(100)
);

INSERT INTO users VALUES (1, 'Alice', 'alice@example.com');
INSERT INTO users VALUES (5, 'Eve', 'eve@example.com');
INSERT INTO users VALUES (3, 'Charlie', 'charlie@example.com');
```

`id`가 primary key이므로, 삽입 순서와 무관하게 페이지 내의 논리적 순서는 다음과 같다:

```
Infimum → [id=1, Alice] → [id=3, Charlie] → [id=5, Eve] → Supremum
```

물리적으로는 삽입된 순서대로 Free Space 영역에 배치되지만, next record offset이 primary key 순서를 유지하므로 논리적으로는 항상 정렬된 상태다.

InnoDB에서 primary key는 단순한 유니크 제약이 아니다. 데이터의 물리적 저장 순서를 결정하는 **clustered index**의 키다. 모든 행 데이터가 primary key의 B-tree 리프 노드에 저장된다. 이 구조를 이해하면 primary key 설계가 성능에 얼마나 큰 영향을 미치는지 알 수 있다.

## Row Format

InnoDB는 행을 디스크에 저장하는 형식을 여러 가지 제공한다. 이것을 **row format**이라 한다.

```sql
-- 테이블의 row format 확인
SHOW TABLE STATUS LIKE 'users'\G
```

```text
Row_format: Dynamic
```

현재 InnoDB의 기본 row format은 **DYNAMIC**이다. 사용 가능한 format은 네 가지다:

| Row Format | 기본 여부 | 특징 |
|------------|----------|------|
| REDUNDANT | MySQL 5.0 이전 기본 | 호환성용, 공간 비효율적 |
| COMPACT | MySQL 5.0~5.6 기본 | REDUNDANT보다 약 20% 공간 절약 |
| DYNAMIC | MySQL 5.7+ 기본 | COMPACT의 개선, 긴 컬럼 처리 최적화 |
| COMPRESSED | 선택적 | 페이지를 압축하여 저장, CPU 오버헤드 존재 |

실무에서는 DYNAMIC을 쓰면 된다. 이미 기본값이므로 별도 설정이 필요 없다. COMPRESSED는 디스크 공간이 중요한 경우 사용하지만, 압축/해제에 CPU를 사용하고 buffer pool 효율이 떨어질 수 있어 신중하게 선택해야 한다.

COMPACT과 DYNAMIC의 행 저장 구조를 들여다본다:

```
┌─────────────────────────────────────────────────────────────┐
│                        행 하나의 구조                         │
├──────────────┬────────────┬──────────┬──────────────────────┤
│ Variable-    │ NULL       │ Record   │ Column Data          │
│ Length List  │ Bitmap     │ Header   │                      │
│ (가변 길이    │ (NULL 비트맵)│ (5 bytes)│ (실제 컬럼 값들)       │
│  컬럼 길이)   │            │          │                      │
└──────────────┴────────────┴──────────┴──────────────────────┘
```

- **Variable-Length List**: `VARCHAR`, `VARBINARY` 등 가변 길이 컬럼의 실제 데이터 길이를 역순으로 저장한다.
- **NULL Bitmap**: 어떤 컬럼이 NULL인지 비트 단위로 기록한다.
- **Record Header**: 5 bytes. 다음 레코드로의 포인터, 삭제 플래그, 레코드 타입 등을 포함한다.
- **Column Data**: 실제 컬럼 값이 저장되는 영역이다.

## NULL 처리: NULL Bitmap

**NULL bitmap**은 행에서 NULL 가능한 컬럼의 NULL 여부를 비트 단위로 기록한다. NOT NULL로 정의된 컬럼은 bitmap에 포함되지 않는다.

```sql
CREATE TABLE example (
    id INT NOT NULL,           -- NOT NULL: bitmap에 포함 안 됨
    name VARCHAR(50),          -- NULL 가능: bitmap 1번째 비트
    email VARCHAR(100),        -- NULL 가능: bitmap 2번째 비트
    phone VARCHAR(20)          -- NULL 가능: bitmap 3번째 비트
);
```

NULL 가능한 컬럼이 3개이므로 NULL bitmap은 1 byte(8비트 중 3비트 사용)다. 만약 NULL 가능한 컬럼이 9개면 2 bytes가 필요하다.

```sql
INSERT INTO example VALUES (1, 'Alice', NULL, NULL);
```

이 행의 NULL bitmap은 `00000110`(이진수)이 된다. email과 phone이 NULL이므로 해당 비트가 1이다.

NULL의 저장 비용에 관한 오해가 있다. NULL인 컬럼은 실제 데이터를 저장하지 않는다. bitmap의 비트 하나만 차지할 뿐이다. `VARCHAR(255)` 컬럼이 NULL이면 0 bytes를 사용한다. 같은 컬럼에 빈 문자열 `''`을 넣으면 가변 길이 리스트에 1 byte(길이 0을 나타내는)가 필요하다. 순수 저장 공간 관점에서 NULL이 빈 문자열보다 오히려 적은 공간을 차지한다.

다만 NULL은 인덱스 동작에 영향을 준다. 이 부분은 05편에서 자세히 설명한다.

## VARCHAR의 실제 저장 방식

**VARCHAR**는 선언된 최대 길이가 아니라 실제 저장된 데이터의 길이만큼만 공간을 사용한다.

```sql
CREATE TABLE messages (
    id INT PRIMARY KEY,
    content VARCHAR(1000)
);

INSERT INTO messages VALUES (1, 'Hello');       -- 5 bytes + 길이 정보
INSERT INTO messages VALUES (2, 'World!!!!');   -- 9 bytes + 길이 정보
```

`VARCHAR(1000)`이라고 선언했지만, `'Hello'`는 5 bytes만 사용한다. 1,000 bytes를 예약하지 않는다. 이것이 `CHAR`와의 핵심 차이다.

실제 저장되는 바이트 수:

- **데이터 길이 정보**: 최대 길이가 255 bytes 이하면 1 byte, 초과하면 2 bytes. `VARCHAR(1000)`은 최대 길이가 255를 넘으므로 2 bytes.
- **실제 데이터**: UTF-8(utf8mb4) 인코딩에서 영문은 글자당 1 byte, 한글은 글자당 3 bytes.

```sql
INSERT INTO messages VALUES (3, '안녕하세요');
-- '안녕하세요' = 5글자 × 3 bytes = 15 bytes + 2 bytes(길이) = 17 bytes
```

`CHAR(10)`은 고정 길이로 선언되지만, InnoDB의 COMPACT/DYNAMIC row format에서는 utf8mb4 같은 멀티바이트 character set을 사용할 경우 내부적으로 가변 길이로 저장한다. 실제 데이터에 따라 10~40 bytes 사이의 공간을 차지한다. 단, 최소 10 bytes(문자당 1 byte × 10)는 항상 확보된다. 값의 길이가 일정한 컬럼(국가 코드, 통화 코드 등)에는 CHAR가 적합하고, 길이가 가변적인 컬럼에는 VARCHAR가 적합하다.

`VARCHAR(50)`과 `VARCHAR(255)`는 실제 데이터가 같다면 디스크 저장 공간에 차이가 없다. 둘 다 실제 데이터 길이만큼만 저장한다. 그러나 차이가 발생하는 지점이 있다. MySQL이 임시 테이블을 생성할 때(정렬, 그룹화 등) 최대 길이를 기준으로 메모리를 할당하는 경우가 있다. `VARCHAR(10000)`으로 선언하면 실제 데이터가 10 bytes여도 임시 테이블에서 10,000 bytes를 할당할 수 있다. 불필요하게 큰 VARCHAR 선언은 피해야 한다.

## 페이지 분할 (Page Split)

페이지가 가득 찬 상태에서 새 행을 삽입해야 할 때, InnoDB는 **페이지 분할**(page split)을 수행한다.

순차 증가하는 primary key(`AUTO_INCREMENT`)를 사용하는 경우를 먼저 본다:

```sql
CREATE TABLE orders (
    id BIGINT AUTO_INCREMENT PRIMARY KEY,
    product_name VARCHAR(100),
    amount DECIMAL(10, 2)
);
```

`AUTO_INCREMENT`에서는 새 행이 항상 마지막 페이지에 추가된다. 마지막 페이지가 가득 차면 새 페이지를 할당하고 거기에 쓴다. 기존 페이지의 데이터를 이동시킬 필요가 없다. 효율적이다.

문제는 비순차적인 primary key를 사용할 때 발생한다:

```sql
CREATE TABLE sessions (
    id CHAR(36) PRIMARY KEY,  -- UUID
    user_id BIGINT,
    data TEXT
);

-- UUID는 랜덤 값이므로 삽입 위치가 매번 다르다
INSERT INTO sessions VALUES ('a1b2c3d4-...', 1, '...');
INSERT INTO sessions VALUES ('f8e7d6c5-...', 2, '...');
INSERT INTO sessions VALUES ('3c4d5e6f-...', 3, '...');
```

UUID는 정렬 순서가 랜덤이다. 새 행이 이미 가득 찬 페이지의 중간에 들어가야 할 수 있다. 이때 페이지 분할이 발생한다:

```
분할 전:
┌─────────────────────┐
│ Page A (가득 참)      │
│ [1] [3] [5] [7] [9] │
└─────────────────────┘
   ↑ 여기에 [4]를 넣어야 함

분할 후:
┌─────────────────────┐    ┌─────────────────────┐
│ Page A               │    │ Page B (새로 할당)    │
│ [1] [3] [4]          │    │ [5] [7] [9]          │
└─────────────────────┘    └─────────────────────┘
```

페이지 분할의 비용:

- 새 페이지를 할당해야 한다.
- 기존 페이지의 행 절반을 새 페이지로 복사해야 한다.
- 연결 리스트의 포인터를 갱신해야 한다.
- 상위 B-tree 노드도 갱신해야 한다.
- 분할 후 두 페이지 모두 절반만 차 있어 공간 효율이 떨어진다.

랜덤 UUID를 primary key로 사용하면 삽입 성능이 순차 키 대비 크게 저하되는 이유가 여기에 있다. 페이지 분할이 빈번하게 발생하고, 페이지 점유율(fill factor)도 낮아진다. 순차 UUID(UUID v7 등)나 `AUTO_INCREMENT`를 사용하면 이 문제를 회피할 수 있다.

## Overflow Page

행 하나가 페이지 하나에 담기지 않을 때는 어떻게 되는가?

InnoDB 페이지는 16KB다. 하나의 페이지에 최소 2개의 행이 들어갈 수 있어야 한다는 규칙이 있다. 이를 뒤집으면, 행 하나의 크기가 약 8KB를 넘으면 한 페이지에 담을 수 없다.

```sql
CREATE TABLE articles (
    id INT PRIMARY KEY,
    title VARCHAR(200),
    body TEXT                -- 수만 글자가 들어갈 수 있음
);
```

`TEXT` 타입의 `body` 컬럼에 10KB의 글이 들어가면 행 전체가 페이지 하나에 담기지 않는다. 이때 InnoDB의 처리 방식은 row format에 따라 다르다.

### COMPACT Row Format

COMPACT format에서는 긴 컬럼의 데이터 중 처음 768 bytes를 행 내부(데이터 페이지)에 저장하고, 나머지를 별도의 **overflow page**에 저장한다. 행에는 overflow page를 가리키는 20 bytes 포인터가 포함된다.

```
데이터 페이지:
┌─────────────────────────────────────────────┐
│ [id=1] [title=...] [body 첫 768 bytes + ptr]│
└───────────────────────────────────┬─────────┘
                                    │ 포인터
                                    ▼
                          ┌──────────────────┐
                          │ Overflow Page     │
                          │ (나머지 body 데이터)│
                          └──────────────────┘
```

### DYNAMIC Row Format (기본값)

DYNAMIC format에서는 긴 컬럼의 데이터를 행 내부에 전혀 저장하지 않는다. 20 bytes 포인터만 남기고 전체 데이터를 overflow page에 저장한다.

```
데이터 페이지:
┌───────────────────────────────────────┐
│ [id=1] [title=...] [body → 20B ptr]  │
└─────────────────────────────┬─────────┘
                              │ 포인터
                              ▼
                    ┌──────────────────┐
                    │ Overflow Page     │
                    │ (전체 body 데이터)  │
                    └──────────────────┘
```

DYNAMIC이 더 효율적인 이유: 데이터 페이지에 768 bytes의 prefix를 저장하지 않으므로, 한 페이지에 더 많은 행을 담을 수 있다. `body` 컬럼을 읽지 않는 쿼리(`SELECT id, title FROM articles`)에서는 overflow page에 접근할 필요가 없어 I/O가 줄어든다.

overflow가 발생하는 기준은 행의 전체 크기에 의존한다:

```sql
-- 한 행의 최대 크기 확인 (InnoDB)
-- 대략 페이지 크기의 절반 ≒ 약 8,126 bytes

CREATE TABLE wide_table (
    c1 VARCHAR(4000),
    c2 VARCHAR(4000),
    c3 VARCHAR(4000)
);
-- ERROR 1118 (42000): Row size too large.
-- 세 컬럼의 최대 합산이 12,000 bytes를 넘을 수 있어 에러 발생 가능
```

`VARCHAR`나 `TEXT` 컬럼이 많은 테이블에서 이 제한에 걸리는 경우가 있다. 이때의 대응:

- row format을 DYNAMIC으로 사용하면 긴 컬럼이 overflow page로 빠져 제한을 우회할 수 있다.
- `TEXT`, `BLOB`은 대부분 overflow page에 저장되므로 행 크기 계산에서 20 bytes(포인터)만 차지한다.
- 설계 단계에서 한 행에 과도한 가변 길이 컬럼을 넣지 않는 것이 근본적인 해결이다.

## 실무에서의 의미

페이지와 행의 물리적 구조가 실무에 미치는 영향을 요약한다.

**Primary key는 순차적으로**: `AUTO_INCREMENT`나 순차 UUID를 사용하면 페이지 분할을 최소화할 수 있다. 랜덤 UUID는 삽입 성능을 저하시킨다.

**행 크기를 의식한다**: 한 페이지에 행이 많이 들어갈수록 범위 스캔이 효율적이다. 불필요하게 큰 컬럼, 사용하지 않는 컬럼이 행 크기를 키우면 같은 양의 데이터를 읽는 데 더 많은 페이지를 읽어야 한다.

**SELECT *를 피해야 하는 이유**: `TEXT`나 `BLOB` 컬럼이 있는 테이블에서 `SELECT *`를 실행하면 overflow page까지 읽어야 한다. 필요한 컬럼만 명시하면 overflow page 접근을 피할 수 있다.

**VARCHAR 선언 크기는 적절하게**: 실제 데이터 길이에 맞는 선언이 임시 테이블 메모리 사용량을 줄인다.

**NULL은 비용이 거의 없다**: NULL bitmap에 비트 하나만 차지한다. "NULL 대신 빈 문자열이나 0을 넣는 것이 성능에 좋다"는 흔한 오해다. 저장 공간 면에서는 오히려 NULL이 효율적이다.

## 정리

- InnoDB는 16KB 페이지 단위로 데이터를 읽고 쓴다. 행 하나만 필요해도 페이지 전체를 읽는다.
- 행은 페이지 안에서 primary key 순서로 연결 리스트를 형성하며, DYNAMIC row format이 기본이다.
- VARCHAR는 실제 데이터 길이만큼만 저장하지만, 선언 크기가 임시 테이블 메모리 할당에 영향을 줄 수 있다.
- 비순차 PK(UUID 등)는 페이지 분할을 유발하여 삽입 성능과 공간 효율을 저하시킨다.
- 행 크기가 페이지의 절반을 초과하면 overflow page에 데이터가 분리 저장된다.
